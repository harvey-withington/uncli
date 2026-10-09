package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"uncli/internal/runtime/local"
)

// Device sign-in (decision 0014): a CLI that prints a link and a code,
// approved in a browser on any device. UNCLI runs it hidden, with the same
// environment as the CLI's sessions (its own state folder), shows the link
// and code, and tells the UI when it has finished.

// deviceSigner is an adapter whose CLI signs in with a device code.
type deviceSigner interface {
	DeviceSignIn(out []byte) (url, code string, ok bool)
}

// DeviceSignIn is what the user approves.
type DeviceSignIn struct {
	URL  string `json:"url"`
	Code string `json:"code"`
}

// StartDeviceSignIn starts a provider's device sign-in and returns its link
// and code; the provider's status follows when it finishes.
func (s *Service) StartDeviceSignIn(ctx context.Context, id string) (DeviceSignIn, error) {
	c, err := s.cli(id)
	if err != nil {
		return DeviceSignIn{}, err
	}
	ds, ok := c.adapter.(deviceSigner)
	if !ok {
		return DeviceSignIn{}, fmt.Errorf("%s doesn't sign in with a code", id)
	}
	bin, _, err := s.binaryOf(ctx, c.adapter.ID())
	if err != nil {
		return DeviceSignIn{}, err
	}
	s.CancelDeviceSignIn(id)
	p, err := local.StartInteractive(c.adapter.LoginCommand(bin))
	if err != nil {
		return DeviceSignIn{}, err
	}
	s.signInMu.Lock()
	if s.devices == nil {
		s.devices = map[string]*local.Interactive{}
	}
	s.devices[id] = p
	s.signInMu.Unlock()

	deadline := time.After(60 * time.Second)
	for {
		if u, code, ok := ds.DeviceSignIn(p.Output()); ok {
			go s.waitDeviceSignIn(id, p)
			return DeviceSignIn{URL: u, Code: code}, nil
		}
		select {
		case <-p.Done():
			st := s.ProviderStatus(context.Background(), id, true)
			s.emitStatus(st)
			if st.LoggedIn {
				return DeviceSignIn{}, nil // it was signed in already
			}
			return DeviceSignIn{}, fmt.Errorf("the sign-in stopped without showing a code: %s", lastLine(string(p.Output())))
		case <-deadline:
			s.CancelDeviceSignIn(id)
			return DeviceSignIn{}, errors.New("the sign-in didn't show a code within a minute")
		case <-ctx.Done():
			s.CancelDeviceSignIn(id)
			return DeviceSignIn{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// waitDeviceSignIn tells the UI how the sign-in ended.
func (s *Service) waitDeviceSignIn(id string, p *local.Interactive) {
	select {
	case <-p.Done():
	case <-time.After(signInWatch):
		p.Kill()
	}
	s.signInMu.Lock()
	if s.devices[id] == p {
		delete(s.devices, id)
	}
	s.signInMu.Unlock()
	s.emitStatus(s.ProviderStatus(context.Background(), id, true))
}

// CancelDeviceSignIn stops a device sign-in in progress, if any.
func (s *Service) CancelDeviceSignIn(id string) {
	s.signInMu.Lock()
	p := s.devices[id]
	delete(s.devices, id)
	s.signInMu.Unlock()
	if p != nil {
		p.Kill()
	}
}
