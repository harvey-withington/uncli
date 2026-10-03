package session

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"uncli/internal/core"
	"uncli/internal/store"
)

type Session struct {
	m *Manager

	mu    sync.Mutex
	rec   store.Session
	state State
	err   string

	// The running process. gen increases on every spawn and deliberate
	// kill, so output from an old process is ignored.
	gen        int
	proc       core.Proc
	cancel     context.CancelFunc
	procEffort string
	lastCost   float64 // the CLI's running cost total on this process

	page         *store.Page // the open page, nil when idle
	evN          int         // events logged on the open page
	runningTools int
	notice       string // last notice on the open page, used if the answer is empty
	pendingModel string // set_model waiting for the turn to end
	interrupting *time.Timer
	lastMods     []string // modifiers sent with the previous turn
	lastSeq      int
	pending      []Approval       // tool uses waiting for the user's answer
	sessionRules []store.ToolRule // "for this session" rules, in memory only
	// Unattended: anything that would wait for the user is declined
	// (approvals.go). In memory only, so it's off after a restart.
	unattended     bool
	toldUnattended bool // what the model was last told about it
}

func newSession(m *Manager, rec store.Session) *Session {
	s := &Session{m: m, rec: rec, state: Idle}
	if pages, err := m.d.Store.ListPages(rec.ID); err == nil && len(pages) > 0 {
		last := pages[len(pages)-1]
		s.lastMods, s.lastSeq = last.Modifiers, last.Seq
	}
	return s
}

func (s *Session) View() View {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.viewLocked()
}

func (s *Session) viewLocked() View {
	return View{Session: s.rec, State: s.state, Running: s.proc != nil, Busy: s.page != nil, Error: s.err,
		Approvals: append([]Approval{}, s.pending...), Unattended: s.unattended}
}

func (s *Session) changed() { s.m.d.Sink.SessionChanged(s.viewLocked()) }

func (s *Session) setState(st State) {
	if s.state != st {
		s.state = st
		s.changed()
	}
}

func (s *Session) savePage() {
	if s.page == nil {
		return
	}
	// The UI gets lists, never null.
	if s.page.Trace == nil {
		s.page.Trace = []store.TraceItem{}
	}
	if s.page.TouchedFiles == nil {
		s.page.TouchedFiles = []store.TouchedFile{}
	}
	if s.page.Modifiers == nil {
		s.page.Modifiers = []string{}
	}
	if s.page.Attachments == nil {
		s.page.Attachments = []store.PageAttachment{}
	}
	if err := s.m.d.Store.SavePage(s.page); err != nil {
		s.err = "Could not save the page: " + err.Error()
	}
	s.m.d.Sink.PageChanged(*s.page)
}

// spawn starts the CLI for this session, resuming its conversation when
// there is one. Caller holds s.mu.
func (s *Session) spawn(ctx context.Context, spec core.LaunchSpec) error {
	bin, version, err := s.m.d.Binary(ctx)
	if err != nil {
		return err
	}
	if s.rec.ProviderSID != "" {
		spec.ResumeID = s.rec.ProviderSID
	} else {
		spec.SessionID = s.rec.ID
	}
	cmd, err := s.m.d.Adapter.BuildCommand(bin, spec)
	if err != nil {
		return err
	}
	pctx, cancel := context.WithCancel(context.Background())
	proc, err := s.m.d.Runtime.Start(pctx, cmd, s.rec.Workdir)
	if err != nil {
		cancel()
		return fmt.Errorf("could not start the CLI: %w", err)
	}
	s.gen++
	s.proc, s.cancel, s.procEffort, s.lastCost = proc, cancel, spec.Effort, 0
	s.rec.CLIVersion = version
	s.state = Starting
	gen := s.gen
	parser := s.m.d.Adapter.NewParser()
	stderr := &tail{max: 4096}
	go func() { _, _ = io.Copy(stderr, proc.Stderr()) }()
	go s.read(gen, proc, parser, stderr)
	if b, ok := s.m.d.Adapter.EncodeControl(core.Control{Kind: core.CtlInitialize}); ok {
		_, _ = proc.Stdin().Write(b)
	}
	s.changed()
	return nil
}

// stopLocked kills the process without reporting its exit. Caller holds s.mu.
func (s *Session) stopLocked() {
	if s.proc == nil {
		return
	}
	s.gen++
	_ = s.proc.Kill()
	s.cancel()
	s.proc, s.cancel = nil, nil
	s.pending = nil
}

func (s *Session) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
	if s.page != nil {
		s.closePage("interrupted", "")
	}
}

// Send starts a turn: the text and any attached files.
func (s *Session) Send(ctx context.Context, text string, files []core.Attachment) error {
	text = strings.TrimSpace(text)
	if text == "" && len(files) == 0 {
		return errors.New("nothing to send")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.page != nil {
		return errors.New("this session is still answering; wait or stop it first")
	}
	set := s.m.d.Profiles
	prof, ok := set.Profile(s.rec.ProfileID)
	if !ok {
		return fmt.Errorf("profile %q no longer exists", s.rec.ProfileID)
	}
	active := append([]string{}, s.rec.Modifiers...)
	spec := set.Resolve(prof, s.rec.Model, active, s.rec.Workdir)
	// Tool uses outside the allowlist come to the user as approval cards
	// when the CLI can route them; otherwise the CLI denies them.
	spec.Approvals = s.m.d.Adapter.Capabilities().Approvals

	// Effort is a launch flag, so a change respawns with resume.
	if s.proc != nil && spec.Effort != s.procEffort {
		s.stopLocked()
	}
	if s.proc == nil {
		if err := s.spawn(ctx, spec); err != nil {
			s.err = err.Error()
			s.state = Errored
			s.changed()
			return err
		}
	}

	directives := set.RenderDirectives(s.rec.Model, active, s.lastMods, s.unattendedDirective()...)
	caps := s.m.d.Adapter.Capabilities()
	for _, f := range files {
		if strings.HasPrefix(f.MediaType, "image/") && !caps.Images || !strings.HasPrefix(f.MediaType, "image/") && !caps.Documents {
			return fmt.Errorf("this CLI can't take %s as an attachment", f.Name)
		}
	}
	line, err := s.m.d.Adapter.EncodeTurn(core.UserTurn{Text: text, Directives: directives, Attachments: files})
	if err != nil {
		return err
	}
	seq, err := s.m.d.Store.NextSeq(s.rec.ID)
	if err != nil {
		return err
	}
	attached := make([]store.PageAttachment, 0, len(files))
	for _, f := range files {
		attached = append(attached, store.PageAttachment{Name: f.Name, Path: f.Path, MediaType: f.MediaType, Size: int64(len(f.Data))})
	}
	s.page = &store.Page{ID: NewID(), SessionID: s.rec.ID, Seq: seq, Question: text, Directives: directives,
		Model: s.rec.Model, Modifiers: active, Status: "open", StartedAt: time.Now().UnixMilli(), Attachments: attached}
	s.evN, s.runningTools, s.notice, s.err = 0, 0, "", ""
	s.lastMods, s.lastSeq = active, seq
	if s.rec.Title == "" {
		if text != "" {
			s.rec.Title = titleFrom(text)
		} else {
			s.rec.Title = titleFrom(files[0].Name)
		}
	}
	_ = s.m.d.Store.UpdateSession(&s.rec)
	s.savePage()
	if _, err := s.proc.Stdin().Write(line); err != nil {
		s.closePage("error", "Could not send to the CLI: "+err.Error())
		s.stopLocked()
		return err
	}
	s.toldUnattended = s.unattended
	if s.state != Starting {
		s.state = Thinking
	}
	s.changed()
	return nil
}

func titleFrom(text string) string {
	t := strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(t) > 60 {
		r := []rune(t)
		t = strings.TrimSpace(string(r[:59])) + "…"
	}
	return t
}

// Interrupt stops the current turn through the control protocol, killing
// the process if the CLI doesn't finish the turn within the grace period.
func (s *Session) Interrupt() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.page == nil || s.proc == nil {
		return nil
	}
	// A tool use waiting for approval holds the turn: answer it first.
	s.denyPendingLocked("The user stopped this turn.")
	b, ok := s.m.d.Adapter.EncodeControl(core.Control{Kind: core.CtlInterrupt})
	if ok {
		if _, err := s.proc.Stdin().Write(b); err == nil {
			page := s.page
			s.interrupting = time.AfterFunc(s.m.InterruptGrace, func() {
				s.mu.Lock()
				defer s.mu.Unlock()
				if s.page == page {
					s.stopLocked()
					s.closePage("interrupted", "")
				}
			})
			return nil
		}
	}
	s.stopLocked()
	s.closePage("interrupted", "")
	return nil
}

// SetModel switches model live when the adapter can, otherwise on the
// next spawn. Mid-turn switches wait for the turn to end.
func (s *Session) SetModel(model string) error {
	if model == "" {
		return errors.New("no model")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rec.Model = model
	if err := s.m.d.Store.UpdateSession(&s.rec); err != nil {
		return err
	}
	if s.proc != nil {
		if s.page != nil {
			s.pendingModel = model
		} else {
			s.applyModelLocked(model)
		}
	}
	s.changed()
	return nil
}

func (s *Session) applyModelLocked(model string) {
	if s.m.d.Adapter.Capabilities().LiveModelSwitch {
		if b, ok := s.m.d.Adapter.EncodeControl(core.Control{Kind: core.CtlSetModel, Model: model}); ok {
			if _, err := s.proc.Stdin().Write(b); err == nil {
				return
			}
		}
	}
	s.stopLocked() // respawn with resume on the next turn
}

func (s *Session) ToggleModifier(id string, on bool) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m.d.Profiles.Modifier(id); !ok {
		return View{}, fmt.Errorf("unknown modifier %q", id)
	}
	s.rec.Modifiers = s.m.d.Profiles.Toggle(s.rec.Modifiers, id, on)
	if err := s.m.d.Store.UpdateSession(&s.rec); err != nil {
		return View{}, err
	}
	s.changed()
	return s.viewLocked(), nil
}

func (s *Session) SetSortOrder(order float64) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rec.SortOrder = order
	if err := s.m.d.Store.UpdateSession(&s.rec); err != nil {
		return View{}, err
	}
	s.changed()
	return s.viewLocked(), nil
}

func (s *Session) Rename(title string) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rec.Title = strings.TrimSpace(title)
	if err := s.m.d.Store.UpdateSession(&s.rec); err != nil {
		return View{}, err
	}
	s.changed()
	return s.viewLocked(), nil
}

func (s *Session) markRead() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == Unread {
		s.setState(Idle)
	}
}

// closePage finishes the open page. Caller holds s.mu.
func (s *Session) closePage(status, errText string) {
	if s.interrupting != nil {
		s.interrupting.Stop()
		s.interrupting = nil
	}
	s.pending = nil // the CLI no longer waits on them
	if s.page == nil {
		return
	}
	p := s.page
	p.Status, p.FinishedAt = status, time.Now().UnixMilli()
	if errText != "" {
		p.Error = errText
	}
	for i := range p.Trace {
		if !p.Trace[i].Done {
			p.Trace[i].Done = true
		}
	}
	s.savePage()
	s.page, s.runningTools = nil, 0
	switch {
	case status == "error":
		s.err = errText
		s.state = Errored
	case s.m.isFocused(s.rec.ID):
		s.state = Idle
	default:
		s.state = Unread
	}
	if s.pendingModel != "" && s.proc != nil {
		s.applyModelLocked(s.pendingModel)
	}
	s.pendingModel = ""
	_ = s.m.d.Store.PruneEvents(s.rec.ID)
	s.changed()
}

func (s *Session) read(gen int, proc core.Proc, parser core.Parser, stderr *tail) {
	sc := bufio.NewScanner(proc.Stdout())
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		evs, err := parser.Feed(sc.Bytes())
		if err != nil {
			evs = []core.Event{core.NewEvent(core.EvError, core.ErrorInfo{Message: err.Error()}, sc.Bytes())}
		}
		for _, ev := range evs {
			s.handle(gen, ev)
		}
	}
	if err := sc.Err(); err != nil {
		// Can't keep reading (a line over 64 MB, say): report it and stop
		// the process so the turn closes instead of hanging.
		s.handle(gen, core.NewEvent(core.EvError, core.ErrorInfo{Message: "Could not read the CLI's output: " + err.Error()}, nil))
		_ = proc.Kill()
	}
	err := proc.Wait()
	code := 0
	var ec interface{ ExitCode() int }
	if errors.As(err, &ec) {
		code = ec.ExitCode()
	}
	s.exited(gen, code, stderr.String())
}

func (s *Session) exited(gen, code int, stderr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if gen != s.gen { // killed on purpose
		return
	}
	s.proc = nil
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	ev := core.NewEvent(core.EvExited, core.Exited{Code: code, Stderr: stderr}, nil)
	ev.TurnSeq = s.lastSeq
	s.m.d.Sink.SessionEvent(s.rec.ID, ev)
	if s.page != nil {
		msg := "The CLI stopped unexpectedly"
		if t := strings.TrimSpace(stderr); t != "" {
			msg += ": " + lastLine(t)
		}
		s.closePage("error", msg)
		return
	}
	if code != 0 {
		s.err = fmt.Sprintf("The CLI exited with code %d", code)
		s.state = Exited
	}
	s.changed()
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// handle applies one parsed event. Called from the reader goroutine.
func (s *Session) handle(gen int, ev core.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if gen != s.gen {
		return
	}
	turnOpen := s.page != nil
	ev.TurnSeq = s.lastSeq
	if turnOpen {
		ev.TurnSeq = s.page.Seq
	}
	s.evN++
	_ = s.m.d.Store.AppendEvent(s.rec.ID, ev.TurnSeq, s.evN, string(ev.Kind), ev.At, ev.Data, ev.Raw)

	prev := s.state
	switch ev.Kind {
	case core.EvSessionReady:
		r, _ := core.Decode[core.SessionReady](ev)
		if r.ProviderSID != "" && r.ProviderSID != s.rec.ProviderSID {
			s.rec.ProviderSID = r.ProviderSID
			_ = s.m.d.Store.UpdateSession(&s.rec)
		}
		if s.state == Starting && !turnOpen {
			s.state = Idle
		}
	case core.EvAccount:
		a, _ := core.Decode[core.Account](ev)
		if len(a.Models) > 0 {
			go s.m.setModels(a.Models)
		}
	case core.EvUsageLimit:
		u, _ := core.Decode[core.UsageLimit](ev)
		go s.m.setUsage(u)
	case core.EvNotice:
		n, _ := core.Decode[core.Notice](ev)
		switch n.Kind {
		case core.NoticeConversationReset:
			s.rec.ProviderSID = "" // the next init carries the new id
			s.notice = "Conversation cleared."
		case core.NoticeCompacted:
			s.notice = "Conversation compacted."
		case core.NoticeCommandOutput:
			if n.Text != "" {
				s.notice = n.Text
			}
		}
	case core.EvTextBlock:
		if turnOpen {
			b, _ := core.Decode[core.TextBlock](ev)
			if s.page.AnswerMD != "" {
				s.page.AnswerMD += "\n\n"
			}
			s.page.AnswerMD += b.Text
			s.savePage()
		}
	case core.EvToolStarted:
		if turnOpen {
			t, _ := core.Decode[core.ToolStarted](ev)
			s.page.Trace = append(s.page.Trace, store.TraceItem{ID: t.ID, Name: t.Name, Summary: t.Summary})
			s.runningTools++
			s.savePage()
		}
	case core.EvToolFinished:
		if turnOpen {
			t, _ := core.Decode[core.ToolFinished](ev)
			found := false
			for i := range s.page.Trace {
				if it := &s.page.Trace[i]; it.ID == t.ID && !it.Done {
					it.Done, it.OK, it.Output = true, t.OK, t.Output
					it.Denied = it.Denied || t.Denied // a denial on its card already marked it
					found = true
					s.runningTools--
				}
			}
			if !found {
				s.page.Trace = append(s.page.Trace, store.TraceItem{ID: t.ID, Name: "tool", Done: true, OK: t.OK, Denied: t.Denied, Output: t.Output})
			}
			s.savePage()
		}
	case core.EvFileTouched:
		if turnOpen {
			f, _ := core.Decode[core.FileTouched](ev)
			s.page.TouchedFiles = append(s.page.TouchedFiles, store.TouchedFile{Path: f.Path, Line: f.Line, How: f.How})
		}
	case core.EvApprovalAsked:
		// The user answers it on an approval card, unless a session rule
		// already does (approvals.go).
		a, _ := core.Decode[core.ApprovalAsked](ev)
		if !s.askLocked(a) {
			s.m.d.Sink.SessionEvent(s.rec.ID, ev)
			return
		}
		s.changed()
	case core.EvTurnResult:
		r, _ := core.Decode[core.TurnResult](ev)
		s.finishTurn(r)
		s.m.d.Sink.SessionEvent(s.rec.ID, ev)
		return
	case core.EvError:
		e, _ := core.Decode[core.ErrorInfo](ev)
		s.err = e.Message
	}
	s.state = next(s.state, ev.Kind, s.runningTools, turnOpen)
	if len(s.pending) > 0 && s.page != nil {
		s.state = NeedsApproval // still waiting on the user, whatever else arrives
	}
	if s.state != prev {
		s.changed()
	}
	s.m.d.Sink.SessionEvent(s.rec.ID, ev)
}

func (s *Session) finishTurn(r core.TurnResult) {
	if s.page == nil {
		return
	}
	p := s.page
	cost := r.TotalCost
	if r.CostIsTotal {
		cost = r.TotalCost - s.lastCost
		s.lastCost = r.TotalCost
	}
	if cost < 0 {
		cost = 0
	}
	p.InputTokens, p.OutputTokens = r.Usage.InputTokens, r.Usage.OutputTokens
	p.CacheRead, p.CacheWrite = r.Usage.CacheRead, r.Usage.CacheWrite
	p.CostUSD, p.DurationMS = cost, r.DurationMS
	if p.AnswerMD == "" {
		switch {
		case r.Text != "" && !r.IsError:
			p.AnswerMD = r.Text
		case s.notice != "":
			p.AnswerMD = s.notice
		}
	}
	switch {
	case r.Interrupted:
		s.closePage("interrupted", "")
	case r.IsError:
		msg := r.Text
		if msg == "" {
			msg = "The turn failed"
		}
		if r.ErrorCode != "" {
			msg = r.ErrorCode + ": " + msg
		}
		s.closePage("error", msg)
	default:
		s.closePage("done", "")
	}
}

// tail keeps the last max bytes written to it.
type tail struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}
