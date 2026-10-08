package app

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"uncli/internal/core"
	"uncli/internal/runtime/local"
	"uncli/internal/runtime/wsl"
)

// Containers sign in with a long-lived token from `claude setup-token`, run
// in a built distro (decision 0011). There it can't open a browser, so it
// prints a link; the user approves it on any device and pastes back the
// code the page shows. The token goes straight into the credential store
// and reaches the CLI through WSLENV; it is never shown or written to a
// file.
//
// setup-token draws a full-screen interface, so it runs in a
// pseudo-terminal (script), 1000 columns wide so nothing wraps.

const setupTokenCmd = "stty cols 1000 rows 50; claude setup-token"

var (
	signInURLRE = regexp.MustCompile(`https://claude\.com/cai/oauth/authorize\?[^\x07\x1b\s]+`)
	oscRE       = regexp.MustCompile("\x1b\\][^\x07\x1b]*(\x07|\x1b\\\\)")
	csiRE       = regexp.MustCompile("\x1b\\[[0-9;?<>=]*[A-Za-z]")
	charsetRE   = regexp.MustCompile("\x1b[()][A-Za-z0-9]")
	// A token is whole once something that can't be part of it follows,
	// or the CLI has exited.
	tokenRE    = regexp.MustCompile(`(sk-ant-oat01-[A-Za-z0-9_-]{20,})[^A-Za-z0-9_-]`)
	tokenEndRE = regexp.MustCompile(`(sk-ant-oat01-[A-Za-z0-9_-]{20,})$`)
)

// signInURL finds the sign-in link in setup-token's raw output; the OSC 8
// hyperlink carries it whole even where the text wraps.
func signInURL(raw string) string { return signInURLRE.FindString(raw) }

// screenText is the output as text. The CLI moves the cursor instead of
// printing spaces, so escape sequences become spaces: removing them would
// glue the next words onto the token.
func screenText(raw string) string {
	s := oscRE.ReplaceAllString(raw, " ")
	s = csiRE.ReplaceAllString(s, " ")
	return charsetRE.ReplaceAllString(s, " ")
}

// tokenIn finds a whole token in the output.
func tokenIn(raw string, exited bool) (string, bool) {
	s := screenText(raw)
	if m := tokenRE.FindStringSubmatch(s); m != nil {
		return m[1], true
	}
	if exited {
		if m := tokenEndRE.FindStringSubmatch(strings.TrimRight(s, " \r\n")); m != nil {
			return m[1], true
		}
	}
	return "", false
}

// containerSignIn is a setup-token run waiting for its code.
type containerSignIn struct {
	proc core.Proc
	mu   sync.Mutex
	out  strings.Builder
	done chan struct{}
}

func (c *containerSignIn) text() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.done:
		return c.out.String(), true
	default:
		return c.out.String(), false
	}
}

// StartContainerSignIn runs setup-token in a built container and returns
// the link to approve.
func (s *Service) StartContainerSignIn(ctx context.Context) (string, error) {
	s.CancelContainerSignIn()
	st, err := wsl.CheckStatus(ctx)
	if err != nil {
		return "", err
	}
	if len(st.Distros) == 0 {
		return "", errors.New("build a container first: sign-in runs inside one")
	}
	p, err := local.New().Start(context.Background(), core.Command{Path: wsl.Exe,
		Args: []string{"--distribution", st.Distros[0], "--user", wsl.User, "--cd", "/home/" + wsl.User,
			"--exec", "script", "-qfc", setupTokenCmd, "/dev/null"},
		Env: map[string]string{"WSL_UTF8": "1"}}, "")
	if err != nil {
		return "", err
	}
	c := &containerSignIn{proc: p, done: make(chan struct{})}
	go func() { _, _ = io.Copy(io.Discard, p.Stderr()) }()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := p.Stdout().Read(buf)
			c.mu.Lock()
			c.out.Write(buf[:n])
			c.mu.Unlock()
			if err != nil {
				break
			}
		}
		_ = p.Wait()
		close(c.done)
	}()
	s.signInMu.Lock()
	s.containerSignIn = c
	s.signInMu.Unlock()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		raw, exited := c.text()
		if u := signInURL(raw); u != "" {
			return u, nil
		}
		if exited {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	s.CancelContainerSignIn()
	return "", errors.New("the CLI didn't offer a sign-in link")
}

// FinishContainerSignIn sends the code from the sign-in page and saves the
// token the CLI then prints.
func (s *Service) FinishContainerSignIn(code string) error {
	code = strings.TrimSpace(code)
	if code == "" || strings.ContainsAny(code, "\r\n") {
		return errors.New("paste the code the sign-in page shows")
	}
	s.signInMu.Lock()
	c := s.containerSignIn
	s.signInMu.Unlock()
	if c == nil {
		return errors.New("start the sign-in again")
	}
	defer s.CancelContainerSignIn()
	// Typed and submitted apart: sent together, the paste never submits.
	if _, err := io.WriteString(c.proc.Stdin(), code); err != nil {
		return err
	}
	time.Sleep(800 * time.Millisecond)
	if _, err := io.WriteString(c.proc.Stdin(), "\r"); err != nil {
		return err
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		raw, exited := c.text()
		if t, ok := tokenIn(raw, exited); ok {
			if err := s.setSecret(secretContainerToken, "", t); err != nil {
				return err
			}
			s.emitContainers()
			return nil
		}
		if exited {
			return errors.New("sign-in didn't finish: check the code and try again")
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("sign-in timed out")
}

// CancelContainerSignIn stops a sign-in waiting for its code.
func (s *Service) CancelContainerSignIn() {
	s.signInMu.Lock()
	c := s.containerSignIn
	s.containerSignIn = nil
	s.signInMu.Unlock()
	if c != nil {
		_ = c.proc.Kill()
	}
}

// SignOutContainers forgets the containers' token.
func (s *Service) SignOutContainers() error {
	err := s.setSecret(secretContainerToken, "", "")
	s.emitContainers()
	return err
}
