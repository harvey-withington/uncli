package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"uncli/internal/core"
)

// Slash commands: the CLI reports the ones it takes (core.EvCommands), and
// the UI offers them as the user types "/". Each session keeps its own
// list; the manager keeps the last list each CLI reported, so a session
// that hasn't started its CLI yet still has one to offer.
//
// UNCLI gates every tool call, so a command that would switch that off is
// never offered or sent: some CLIs have one (Grok's /always-approve skips
// every permission prompt). Which commands do that isn't reported, so it's
// judged by name, and errs towards refusing.

var bypassRE = regexp.MustCompile(`(?i)approve|bypass|yolo|skip.?perm|dangerous|auto.?mode|trust`)

// blockedCommand reports whether a command could stop the CLI asking UNCLI
// before it uses tools.
func blockedCommand(name string) bool { return bypassRE.MatchString(name) }

var slashRE = regexp.MustCompile(`^/([A-Za-z0-9][\w:.-]*)(\s|$)`)

// slashName is the command a message starts with ("/compact now" is
// compact); empty when it doesn't start with one (a path such as /usr/bin
// doesn't).
func slashName(text string) string {
	m := slashRE.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return m[1]
}

// checkSlash refuses a message that starts with a command the session's CLI
// can't take, or one UNCLI never sends.
func checkSlash(text, cli string, caps core.Capabilities) error {
	name := slashName(text)
	switch {
	case name == "":
		return nil
	case !caps.SlashPassthrough:
		return fmt.Errorf("%s doesn't take slash commands", cli)
	case blockedCommand(name):
		return fmt.Errorf("UNCLI doesn't send /%s: it could stop the CLI asking before it uses tools, and UNCLI checks every tool use", name)
	}
	return nil
}

// usable is a CLI's commands less the ones UNCLI never sends.
func usable(list []core.CommandInfo) []core.CommandInfo {
	out := []core.CommandInfo{}
	for _, c := range list {
		if c.Name != "" && !blockedCommand(c.Name) {
			out = append(out, c)
		}
	}
	return out
}

func commandsKey(adapter string) string { return "commands:" + adapter }

// setCommandsLocked keeps the commands the CLI reported, for this session
// and as its CLI's latest. Caller holds s.mu.
func (s *Session) setCommandsLocked(c core.Commands) {
	s.commands = usable(c.List)
	if b, err := json.Marshal(s.commands); err == nil {
		_ = s.m.d.Store.SetSetting(commandsKey(s.ad().ID()), string(b))
	}
}

// Commands is the slash commands a session can send: what its CLI reported,
// else what that CLI reported last in any session; none for a CLI that
// takes none.
func (m *Manager) Commands(id string) ([]core.CommandInfo, error) {
	s, err := m.get(id)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	have, ad := slices.Clone(s.commands), s.ad()
	s.mu.Unlock()
	if !ad.Capabilities().SlashPassthrough {
		return []core.CommandInfo{}, nil
	}
	if have != nil {
		return have, nil
	}
	// Its CLI reports its commands as it starts, so start it now (nothing
	// goes to the model): they arrive as an EvCommands the UI reloads on.
	// Meanwhile, what that CLI reported last, if anything.
	go func() { _ = s.warm(context.Background()) }()
	out := []core.CommandInfo{}
	if raw, err := m.d.Store.Setting(commandsKey(ad.ID())); err == nil && raw != "" {
		if json.Unmarshal([]byte(raw), &out) != nil {
			return nil, errors.New("couldn't read the last commands")
		}
	}
	return usable(out), nil
}
