package session

import (
	"path"
	"path/filepath"

	"uncli/internal/artifacts"
	"uncli/internal/core"
)

// placed is a runtime whose CLI sees the session folder at another path: a
// WSL distro mounts it under /workspace (decision 0011). Paths the CLI
// reports are mapped back to the session folder, so changed files and IDE
// links point at the real files.
//
// Approvals still judge against the session folder's Windows path, so a
// command naming the CLI's own paths looks like it reaches outside the
// folder and is asked about: more prompts, never fewer.
type placed interface {
	HostPath(host, p string) (string, bool)
}

// guested is a runtime that can say where a session folder is seen.
type guested interface {
	GuestPath(host string) string
}

// briefed is a runtime with something to tell the model about where it
// runs (a container: what's mounted, where the secrets are).
type briefed interface {
	Brief() string
}

// ArtifactsEnv names the session's artifacts folder for the CLI and its
// tools, for session types that keep artifacts.
const ArtifactsEnv = "UNCLI_ARTIFACTS_DIR"

// artifactsEnvLocked sets ArtifactsEnv for a session type with artifacts,
// at the path the CLI sees. Caller holds s.mu.
func (s *Session) artifactsEnvLocked(spec *core.LaunchSpec, rt core.Runtime) {
	p, ok := s.m.d.Profiles.Profile(s.rec.ProfileID)
	if !ok || !p.Artifacts {
		return
	}
	dir := filepath.Join(s.rec.Workdir, artifacts.Folder)
	if g, ok := rt.(guested); ok {
		dir = path.Join(g.GuestPath(s.rec.Workdir), artifacts.Folder)
	}
	env := map[string]string{ArtifactsEnv: dir}
	for k, v := range spec.Env {
		env[k] = v
	}
	spec.Env = env
}

// placeDirectives tells the model about where it runs, when that's news:
// the first turn, or after it changed (a share turned on). Caller holds
// s.mu, after the CLI has started.
func (s *Session) placeDirectives() []string {
	s.placeBrief = ""
	rt, err := s.m.runtime(s.rec)
	if err != nil {
		return nil
	}
	if b, ok := rt.(briefed); ok {
		s.placeBrief = b.Brief()
	}
	if s.placeBrief == "" || s.placeBrief == s.toldPlace {
		return nil
	}
	return []string{s.placeBrief}
}

// hostPath maps a path the CLI reported to the session folder; ok is false
// for a path that only exists where the CLI runs. Caller holds s.mu.
func (s *Session) hostPath(p string) (string, bool) {
	rt, err := s.m.runtime(s.rec)
	if err != nil {
		return p, true
	}
	pl, ok := rt.(placed)
	if !ok {
		return p, true
	}
	return pl.HostPath(s.rec.Workdir, p)
}

// StopIn stops the CLI of every session running in one place of a runtime
// (a container), so the container can be stopped or rebuilt without its
// work being killed from outside: each CLI is asked to end (its input
// closes) and a turn in progress ends as interrupted. Their next message
// starts them again.
func (m *Manager) StopIn(runtime, ref string) {
	m.mu.Lock()
	all := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		all = append(all, s)
	}
	m.mu.Unlock()
	// One lock at a time: a session holding its own asks the manager.
	for _, s := range all {
		s.mu.Lock()
		if s.rec.Runtime == runtime && s.rec.RuntimeRef == ref && s.proc != nil {
			if s.page != nil {
				s.closePage("interrupted", "")
			}
			s.stopLocked()
			s.state = Exited
			s.changed()
		}
		s.mu.Unlock()
	}
}
