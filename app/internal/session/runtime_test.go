package session

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"uncli/internal/core"
)

// otherRuntime is the fake runtime under another id (a container).
type otherRuntime struct{ *fakeRuntime }

func (otherRuntime) ID() string { return "wsl" }

// placeIn moves a new session to another runtime, as creating it in a
// container profile will.
func placeIn(t *testing.T, h *harness, id, runtime, ref string) {
	t.Helper()
	s, err := h.m.get(id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.rec.Runtime, s.rec.RuntimeRef = runtime, ref
	s.mu.Unlock()
}

// A session starts in its own runtime, found by its id and ref.
func TestSessionStartsInItsRuntime(t *testing.T) {
	h := newHarness(t, "single-turn")
	var asked []string
	h.m.d.Runtimes = func(id, ref string) (core.Runtime, error) {
		asked = append(asked, id+"/"+ref)
		return otherRuntime{h.rt}, nil
	}
	v, _ := h.m.Create("chat", "", "haiku")
	placeIn(t, h, v.ID, "wsl", "go-dev")
	pages := h.send(v.ID, "hi")
	if len(asked) == 0 || slices.ContainsFunc(asked, func(a string) bool { return a != "wsl/go-dev" }) || len(pages) != 1 {
		t.Errorf("asked %v, pages %d", asked, len(pages))
	}
}

// A session in a runtime this UNCLI doesn't have says so instead of
// starting locally.
func TestUnknownRuntimeIsRefused(t *testing.T) {
	h := newHarness(t, "single-turn")
	v, _ := h.m.Create("chat", "", "haiku")
	placeIn(t, h, v.ID, "wsl", "go-dev")
	err := h.m.Send(context.Background(), v.ID, "hi")
	if err == nil || !strings.Contains(err.Error(), "wsl") {
		t.Errorf("err = %v", err)
	}
	h.rt.mu.Lock()
	defer h.rt.mu.Unlock()
	if len(h.rt.starts) != 0 {
		t.Error("it started locally")
	}
}

// placedRuntime sees the session folder at /workspace/p, as a WSL distro
// mounts it.
type placedRuntime struct{ otherRuntime }

func (placedRuntime) HostPath(host, p string) (string, bool) {
	if rest, ok := strings.CutPrefix(p, "/workspace/p/"); ok {
		return filepath.Join(host, filepath.FromSlash(rest)), true
	}
	return "", false
}

// A file the CLI wrote under its own path for the session folder is
// recorded at its Windows path; one only the CLI's system has is left out.
func TestChangedFilesMapBackToTheSessionFolder(t *testing.T) {
	h := newHarness(t, "perm-stdio-allow")
	h.rt.onControl = true
	h.m.d.Runtimes = func(string, string) (core.Runtime, error) { return placedRuntime{otherRuntime{h.rt}}, nil }
	inFolder := map[string]string{"file_path": "/workspace/p/notes/hello.txt", "content": "hi"}
	elsewhere := map[string]string{"file_path": "/home/uncli/scratch.txt", "content": "x"}
	h.rt.turns = [][]byte{
		seg(toolUseLine("toolu_1", "Write", inFolder), askLine("req_1", "toolu_1", "Write", inFolder)),
		seg(toolResultLine("toolu_1", "ok", false), toolUseLine("toolu_2", "Write", elsewhere), askLine("req_2", "toolu_2", "Write", elsewhere)),
		seg(toolResultLine("toolu_2", "ok", false), scriptedResult),
	}
	dir := t.TempDir()
	v, _ := h.m.Create("code", dir, "")
	placeIn(t, h, v.ID, "wsl", "dev")
	s, _ := h.m.get(v.ID)
	h.m.Send(context.Background(), v.ID, "write")
	// Writes may be allowed by the profile or asked about; allow any card.
	waitUntil(t, "the turn", func() bool {
		for _, a := range s.View().Approvals {
			_ = h.m.Answer(v.ID, a.RequestID, Allow, "")
		}
		return !s.View().Busy
	})
	pages := h.waitIdle(v.ID)
	files := pages[len(pages)-1].TouchedFiles
	if len(files) != 1 || files[0].Path != filepath.Join(dir, "notes", "hello.txt") {
		t.Errorf("touched = %+v", files)
	}
}

// Stopping a container's sessions stops only those, and they start again
// with their next message.
func TestStopIn(t *testing.T) {
	h := newHarness(t, "single-turn")
	h.m.d.Runtimes = func(string, string) (core.Runtime, error) { return otherRuntime{h.rt}, nil }
	in, _ := h.m.Create("chat", "", "haiku")
	placeIn(t, h, in.ID, "wsl", "sandbox")
	h.send(in.ID, "hi")
	out, _ := h.m.Create("chat", "", "haiku")
	h.rt.next = 0
	h.send(out.ID, "hi")

	h.m.StopIn("wsl", "sandbox")
	si, _ := h.m.get(in.ID)
	so, _ := h.m.get(out.ID)
	if v := si.View(); v.Running || v.State != Exited {
		t.Errorf("in the container: running %v, state %s", v.Running, v.State)
	}
	if !so.View().Running {
		t.Error("a session elsewhere was stopped")
	}
}

// briefRuntime is a container that says where the session folder is and
// has something to tell the model.
type briefRuntime struct{ otherRuntime }

func (briefRuntime) GuestPath(host string) string { return "/workspace/chat-1234" }
func (briefRuntime) Brief() string                { return "This session runs in a Linux container." }

// A session type with artifacts is told where they go, at the path its CLI
// sees; in a container, the model is told about it once.
func TestArtifactsEnvAndContainerBrief(t *testing.T) {
	h := newHarness(t, "multi-turn-partial")
	v, _ := h.m.Create("chat", "", "haiku")
	h.send(v.ID, "one")
	if got := h.rt.starts[0].Env[ArtifactsEnv]; got != filepath.Join(v.Workdir, "artifacts") {
		t.Errorf("local: %s = %q", ArtifactsEnv, got)
	}

	h2 := newHarness(t, "multi-turn-partial")
	h2.m.d.Runtimes = func(id, ref string) (core.Runtime, error) { return briefRuntime{otherRuntime{h2.rt}}, nil }
	v2, _ := h2.m.Create("chat", "", "haiku")
	placeIn(t, h2, v2.ID, "wsl", "sandbox")
	pages := h2.send(v2.ID, "one")
	pages = append(pages[:0], h2.send(v2.ID, "two")...)
	if got := h2.rt.starts[0].Env[ArtifactsEnv]; got != "/workspace/chat-1234/artifacts" {
		t.Errorf("container: %s = %q", ArtifactsEnv, got)
	}
	if !strings.Contains(pages[0].Directives, "Linux container") || strings.Contains(pages[1].Directives, "Linux container") {
		t.Errorf("told %q then %q", pages[0].Directives, pages[1].Directives)
	}
}
