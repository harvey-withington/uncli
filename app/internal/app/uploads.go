package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"uncli/internal/attach"
)

// Uploads are files a remote client sends to attach to a turn (a photo
// from the phone): they're kept in the data folder, each in a folder named
// by a random id, and a turn refers to one by that id, never by a path the
// client chose (decision 0021). They're cleared a week after upload.

// MaxUpload is the most an upload may be: the largest attachment (a PDF).
const MaxUpload = attach.MaxPDF

// MaxUploads is the most all uploads together may take, so a device can't
// fill the disk before the week is up.
const MaxUploads = 1 << 30

const uploadLife = 7 * 24 * time.Hour

// Upload is a stored upload: its id and what it would attach as.
type Upload struct {
	ID string `json:"id"`
	attach.Info
}

func (s *Service) uploadsDir() string { return filepath.Join(s.Paths.Config, "uploads") }

// SaveUpload stores a file sent by a remote client. A file larger than
// MaxUpload is refused and nothing is kept.
func (s *Service) SaveUpload(name string, body io.Reader) (Upload, error) {
	name = cleanUploadName(name)
	if s.uploadsSize() >= MaxUploads {
		return Upload{}, fmt.Errorf("uploads on this PC have reached their %d GB limit; they're cleared a week after they're sent", MaxUploads>>30)
	}
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return Upload{}, err
	}
	id := hex.EncodeToString(b[:])
	dir := filepath.Join(s.uploadsDir(), id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Upload{}, err
	}
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Upload{}, err
	}
	n, err := io.Copy(f, io.LimitReader(body, MaxUpload+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > MaxUpload {
		err = fmt.Errorf("%s is larger than the %d MB limit", name, MaxUpload>>20)
	}
	if err != nil {
		_ = os.RemoveAll(dir)
		return Upload{}, err
	}
	return Upload{ID: id, Info: attach.Describe(path)}, nil
}

// UploadPath is where an upload is on disk, or an error when there's no
// such upload.
func (s *Service) UploadPath(id string) (string, error) {
	if len(id) != 24 || strings.Trim(id, "0123456789abcdef") != "" {
		return "", errors.New("no such upload")
	}
	dir := filepath.Join(s.uploadsDir(), id)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].IsDir() {
		return "", errors.New("no such upload")
	}
	return filepath.Join(dir, entries[0].Name()), nil
}

// uploadsSize is the bytes all uploads take.
func (s *Service) uploadsSize() int64 {
	var total int64
	_ = filepath.WalkDir(s.uploadsDir(), func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// clearOldUploads deletes uploads older than uploadLife.
func (s *Service) clearOldUploads() {
	entries, err := os.ReadDir(s.uploadsDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > uploadLife {
			_ = os.RemoveAll(filepath.Join(s.uploadsDir(), e.Name()))
		}
	}
}

// cleanUploadName keeps a client's file name only as a plain name: no
// folders, nothing Windows can't store.
func cleanUploadName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "upload"
	}
	if r := []rune(name); len(r) > 120 {
		name = string(r[len(r)-120:])
	}
	return name
}
