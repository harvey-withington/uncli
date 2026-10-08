package app

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"uncli/internal/core"
	"uncli/internal/runtime/local"
	"uncli/internal/runtime/wsl"
	"uncli/internal/session"
)

// Containers that share the user's claude.ai connectors (Jira, Office…) sign
// the CLI in to the user's whole account instead of using the models-only
// token: `claude auth login` run in a container, with the same link and
// code. The credentials it keeps go in a folder UNCLI owns on Windows, one
// per provider, mounted as the container's ~/.claude, so every container
// that shares connectors uses one sign-in and it survives rebuilds. Other
// containers never see it.

// accountDir is where the provider's full sign-in for containers lives.
func (s *Service) accountDir() string {
	return filepath.Join(s.Paths.Config, "wsl-data", "accounts", s.Adapter.ID())
}

// containerAccount is the folder holding the full sign-in, "" when there is
// none.
func (s *Service) containerAccount() string {
	dir := s.accountDir()
	if st, err := os.Stat(filepath.Join(dir, ".credentials.json")); err == nil && st.Size() > 0 {
		return dir
	}
	return ""
}

// accountSignIn is `claude auth login` running in a container, waiting for
// its code.
type accountSignIn struct {
	proc    *local.Interactive
	runtime *wsl.Runtime
	keep    bool // the container shares connectors: leave the folder mounted
}

// StartContainerAccountSignIn starts the full account sign-in in a built
// container and returns the link to approve.
func (s *Service) StartContainerAccountSignIn(ctx context.Context) (string, error) {
	s.CancelContainerAccountSignIn()
	st, err := checkWSL(ctx)
	if err != nil {
		return "", err
	}
	// A container that shares connectors, else any built one.
	id, keep := "", false
	s.reloadContainers()
	s.containers.mu.Lock()
	for _, p := range s.containers.cfg.Containers {
		if slices.Contains(st.Distros, wsl.DistroName(p.ID)) && (id == "" || p.Connectors == ConnectorsShared && !keep) {
			id, keep = p.ID, p.Connectors == ConnectorsShared
		}
	}
	s.containers.mu.Unlock()
	if id == "" {
		return "", errors.New("build a container first: sign-in runs inside one")
	}
	rt, err := s.runtimeFor(session.RuntimeWSL, id)
	if err != nil {
		return "", err
	}
	r := rt.(*wsl.Runtime)
	dir := s.accountDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := r.MountAt(ctx, dir, path.Join("/home", wsl.User, ".claude")); err != nil {
		return "", err
	}
	login := s.Adapter.LoginCommand("")
	p, err := local.StartInteractive(core.Command{Path: wsl.Exe,
		Args: append([]string{"--distribution", r.Distro, "--user", wsl.User, "--cd", "/home/" + wsl.User, "--exec", wsl.CLIPath}, login.Args...),
		Env:  map[string]string{"WSL_UTF8": "1"}})
	if err != nil {
		return "", err
	}
	s.signInMu.Lock()
	s.accountLogin = &accountSignIn{proc: p, runtime: r, keep: keep}
	s.signInMu.Unlock()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if u, ok := s.Adapter.LoginURL(p.Output()); ok {
			return u, nil
		}
		select {
		case <-p.Done():
			s.CancelContainerAccountSignIn()
			return "", errors.New("the CLI stopped without offering a sign-in link")
		case <-time.After(200 * time.Millisecond):
		}
	}
	s.CancelContainerAccountSignIn()
	return "", errors.New("the CLI didn't offer a sign-in link")
}

// FinishContainerAccountSignIn sends the code from the sign-in page and
// waits for the CLI to keep the sign-in.
func (s *Service) FinishContainerAccountSignIn(code string) error {
	code = strings.TrimSpace(code)
	if code == "" || strings.ContainsAny(code, "\r\n") {
		return errors.New("paste the code the sign-in page shows")
	}
	s.signInMu.Lock()
	a := s.accountLogin
	s.signInMu.Unlock()
	if a == nil {
		return errors.New("start the sign-in again")
	}
	defer s.CancelContainerAccountSignIn()
	if err := a.proc.Send(code); err != nil {
		return err
	}
	select {
	case <-a.proc.Done():
	case <-time.After(90 * time.Second):
		return errors.New("sign-in timed out")
	}
	if s.containerAccount() == "" {
		out := strings.TrimSpace(string(a.proc.Output()))
		if i := strings.LastIndex(out, "\n"); i >= 0 {
			out = out[i+1:]
		}
		return errors.New("sign-in didn't finish: " + out)
	}
	s.emitContainers()
	return nil
}

// CancelContainerAccountSignIn stops a sign-in waiting for its code and, in
// a container that doesn't share connectors, takes the folder out again.
func (s *Service) CancelContainerAccountSignIn() {
	s.signInMu.Lock()
	a := s.accountLogin
	s.accountLogin = nil
	s.signInMu.Unlock()
	if a == nil {
		return
	}
	a.proc.Kill()
	if !a.keep {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = a.runtime.Unmount(ctx, path.Join("/home", wsl.User, ".claude"))
	}
}

// SignOutContainerAccount forgets the full account sign-in; containers that
// share connectors fall back to the models-only token from their next start.
func (s *Service) SignOutContainerAccount() error {
	err := os.Remove(filepath.Join(s.accountDir(), ".credentials.json"))
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	s.emitContainers()
	return err
}
