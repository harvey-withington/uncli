package app

import (
	"errors"
	"sync"

	"uncli/internal/secrets"
)

// keyring keeps the app's secrets: the OS credential store where there is
// one (package secrets), otherwise UNCLI's database as before. Tests swap
// it for a map so they never touch the user's credential store.
var keyring secretStore = osSecrets{}

type secretStore interface {
	Get(name string) (string, error)
	Set(name, value string) error
}

type osSecrets struct{}

func (osSecrets) Get(name string) (string, error) { return secrets.Get(name) }
func (osSecrets) Set(name, value string) error    { return secrets.Set(name, value) }

// memSecrets is a credential store in memory (tests).
type memSecrets struct {
	mu sync.Mutex
	m  map[string]string
}

func (s *memSecrets) Get(name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[name], nil
}

func (s *memSecrets) Set(name, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]string{}
	}
	if value == "" {
		delete(s.m, name)
	} else {
		s.m[name] = value
	}
	return nil
}

// secret reads a secret. One saved in the database by an earlier version
// (setting is its key there) moves into the credential store on first read.
func (s *Service) secret(name, setting string) string {
	v, err := keyring.Get(name)
	if errors.Is(err, secrets.ErrUnsupported) {
		v, _ = s.Store.Setting(setting)
		return v
	}
	if v != "" || setting == "" {
		return v
	}
	old, _ := s.Store.Setting(setting)
	if old != "" && keyring.Set(name, old) == nil {
		_ = s.Store.SetSetting(setting, "")
	}
	return old
}

// setSecret saves a secret (empty removes it), in the database only where
// there is no credential store.
func (s *Service) setSecret(name, setting, value string) error {
	err := keyring.Set(name, value)
	if errors.Is(err, secrets.ErrUnsupported) && setting != "" {
		return s.Store.SetSetting(setting, value)
	}
	if err != nil {
		return err
	}
	if setting != "" {
		_ = s.Store.SetSetting(setting, "")
	}
	return nil
}
