package app

import (
	"context"
	"os"
	"os/exec"
	"sort"
	"time"

	"uncli/internal/ide"
)

// Some CLIs sign in only in their own terminal interface (Provider.SignIn
// "elsewhere"): UNCLI opens the CLI in a terminal window, set up as it runs
// it for sessions (its own state folder, no self-update), and watches for
// the sign-in to land. The Antigravity CLI keeps the sign-in in the user's
// credential store whatever its state folder, so the sign-in made there is
// the one its sessions use.

const signInWatch = 15 * time.Minute

// SignInTerminal opens a provider's CLI in a terminal window to sign in.
func (s *Service) SignInTerminal(ctx context.Context, id string) error {
	c, err := s.cli(id)
	if err != nil {
		return err
	}
	bin, _, err := s.binaryOf(ctx, c.adapter.ID())
	if err != nil {
		return err
	}
	lc := c.adapter.LoginCommand(bin)
	cmd := exec.Command(lc.Path, lc.Args...)
	env := os.Environ()
	keys := make([]string, 0, len(lc.Env))
	for k := range lc.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env = append(env, k+"="+lc.Env[k])
	}
	cmd.Env = env
	if err := newConsole(cmd); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	go s.watchProviderSignIn(c.adapter.ID())
	return nil
}

// watchProviderSignIn checks a provider's sign-in now and then until it's signed in
// or a while has passed, and tells the UI when it is.
func (s *Service) watchProviderSignIn(id string) {
	s.signInMu.Lock()
	if s.watching == nil {
		s.watching = map[string]bool{}
	}
	if s.watching[id] {
		s.signInMu.Unlock()
		return
	}
	s.watching[id] = true
	s.signInMu.Unlock()
	defer func() {
		s.signInMu.Lock()
		delete(s.watching, id)
		s.signInMu.Unlock()
	}()
	deadline := time.Now().Add(signInWatch)
	for time.Now().Before(deadline) {
		time.Sleep(4 * time.Second)
		st := s.ProviderStatus(context.Background(), id, true)
		if st.LoggedIn {
			s.emitStatus(st)
			return
		}
	}
}

// RevealCLI shows a provider's CLI binary in its folder.
func (s *Service) RevealCLI(ctx context.Context, id string) error {
	bin, _, err := s.binaryOf(ctx, id)
	if err != nil {
		return err
	}
	return ide.Reveal(bin)
}
