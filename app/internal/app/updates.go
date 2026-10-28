package app

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"uncli/internal/buildinfo"
	"uncli/internal/update"
)

// Updates (decision 0025): a release build checks GitHub for a newer
// release at start and every few hours, unless the user turned it off.
// Update now downloads the installer, checks its SHA-256 against the
// release's SHA256SUMS.txt, and hands it to the install script, which quits
// UNCLI, installs and starts it again. A paired phone sees the same status
// and can start it too. Development builds never check: they're updated
// with npm run install-build.

// EvtUpdate carries the update status whenever it changes.
const EvtUpdate = "update:status" // UpdateStatus

// Update checks: automatic (at start and every few hours) or off.
const (
	UpdateAuto = "auto"
	UpdateOff  = "off"
)

const (
	updateFirstCheck = 30 * time.Second // after start, once UNCLI has settled
	updateEvery      = 6 * time.Hour
)

// Update states.
const (
	UpdateIdle        = "idle"
	UpdateChecking    = "checking"
	UpdateDownloading = "downloading"
	UpdateInstalling  = "installing" // handed to the installer: UNCLI quits shortly
	UpdateFailed      = "error"
)

// UpdateStatus is what the UI shows about updates.
type UpdateStatus struct {
	Current   string          `json:"current"` // this build's version, or "dev"
	Dev       bool            `json:"dev"`     // a development build: no checks
	State     string          `json:"state"`
	Latest    *update.Release `json:"latest,omitempty"` // the newest release found
	Available bool            `json:"available"`        // Latest is newer than this build
	Percent   int             `json:"percent,omitempty"`
	Error     string          `json:"error,omitempty"`
	CheckedAt int64           `json:"checkedAt,omitempty"` // ms
}

type updates struct {
	mu      sync.Mutex
	status  UpdateStatus
	release update.Release
	// What tests replace: where releases are listed, the HTTP client, and
	// the step that hands the installer over.
	url     string
	client  *http.Client
	install func(installer string) error
}

func (s *Service) initUpdates() {
	s.updates.url = update.ReleasesURL
	s.updates.client = &http.Client{Timeout: 5 * time.Minute}
	s.updates.install = update.Install
	s.updates.status = UpdateStatus{Current: buildinfo.Version, Dev: buildinfo.Dev(), State: UpdateIdle}
}

// UpdateStatus is the update status now.
func (s *Service) UpdateStatus() UpdateStatus {
	s.updates.mu.Lock()
	defer s.updates.mu.Unlock()
	return s.updates.status
}

// setUpdate changes the status and tells the UI.
func (s *Service) setUpdate(f func(*UpdateStatus)) UpdateStatus {
	s.updates.mu.Lock()
	f(&s.updates.status)
	st := s.updates.status
	s.updates.mu.Unlock()
	s.emit.Emit(EvtUpdate, st)
	return st
}

var errDevBuild = errors.New("this is a development build: it's updated with npm run install-build, not from releases")

// CheckForUpdates looks for a newer release now.
func (s *Service) CheckForUpdates(ctx context.Context) (UpdateStatus, error) {
	cur := s.UpdateStatus()
	if cur.Dev {
		return cur, errDevBuild
	}
	if cur.State == UpdateDownloading || cur.State == UpdateInstalling || cur.State == UpdateChecking {
		return cur, nil
	}
	s.setUpdate(func(u *UpdateStatus) { u.State, u.Error = UpdateChecking, "" })
	rel, err := update.Latest(ctx, s.updates.client, s.updates.url)
	if err != nil {
		st := s.setUpdate(func(u *UpdateStatus) {
			u.State, u.Error, u.CheckedAt = UpdateFailed, err.Error(), time.Now().UnixMilli()
		})
		return st, err
	}
	s.updates.mu.Lock()
	s.updates.release = rel
	s.updates.mu.Unlock()
	st := s.setUpdate(func(u *UpdateStatus) {
		u.State, u.CheckedAt = UpdateIdle, time.Now().UnixMilli()
		u.Latest = &rel
		u.Available = update.Compare(rel.Version, u.Current) > 0
	})
	return st, nil
}

// InstallUpdate downloads the newer release, checks it, and hands it to the
// installer, which quits UNCLI, installs and starts it again. It returns
// once that has started; UNCLI quits a few seconds later.
func (s *Service) InstallUpdate(ctx context.Context) (UpdateStatus, error) {
	cur := s.UpdateStatus()
	switch {
	case cur.Dev:
		return cur, errDevBuild
	case cur.State == UpdateDownloading || cur.State == UpdateInstalling:
		return cur, nil
	case !cur.Available:
		return cur, errors.New("there's no newer release to install")
	}
	s.updates.mu.Lock()
	rel := s.updates.release
	s.updates.mu.Unlock()
	dir, err := update.Dir()
	if err != nil {
		return s.failUpdate(err)
	}
	s.setUpdate(func(u *UpdateStatus) { u.State, u.Percent, u.Error = UpdateDownloading, 0, "" })
	path, err := update.Download(ctx, s.updates.client, rel, filepath.Join(dir, "download"), func(p int) {
		s.setUpdate(func(u *UpdateStatus) { u.Percent = p })
	})
	if err != nil {
		return s.failUpdate(err)
	}
	s.setUpdate(func(u *UpdateStatus) { u.State = UpdateInstalling })
	if err := s.updates.install(path); err != nil {
		return s.failUpdate(err)
	}
	return s.UpdateStatus(), nil
}

func (s *Service) failUpdate(err error) (UpdateStatus, error) {
	st := s.setUpdate(func(u *UpdateStatus) { u.State, u.Error, u.Percent = UpdateFailed, err.Error(), 0 })
	return st, err
}

// watchUpdates checks shortly after start and then every few hours, while
// the preference says to. A failed check waits for the next one.
func (s *Service) watchUpdates() {
	if s.UpdateStatus().Dev {
		return
	}
	wait := updateFirstCheck
	for {
		select {
		case <-s.life.Done():
			return
		case <-time.After(wait):
		}
		wait = updateEvery
		if s.Preferences().UpdateCheck == UpdateOff {
			continue
		}
		ctx, cancel := context.WithTimeout(s.life, time.Minute)
		_, _ = s.CheckForUpdates(ctx)
		cancel()
	}
}
