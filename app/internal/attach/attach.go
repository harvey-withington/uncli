// Package attach turns files the user drops or pastes into turn
// attachments: images, PDFs and text files, read from disk and checked
// against size limits. Anything else (folders, other binaries, files too
// big) is reported with a reason, so the UI can fall back to the path.
package attach

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"uncli/internal/core"
)

// Kinds of attachment, and what can't be attached.
const (
	Image       = "image"
	PDF         = "pdf"
	Text        = "text"
	Folder      = "folder"
	Unsupported = "unsupported"
)

// Limits per file: the API's image limit, its PDF limit, and enough text
// for a long document without filling the context.
const (
	MaxImage = 5 << 20
	MaxPDF   = 32 << 20
	MaxText  = 512 << 10
)

var imageTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp",
}

// Info describes a dropped or pasted path: what it would attach as, or why
// it can't be.
type Info struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Kind      string `json:"kind"` // image | pdf | text | folder | unsupported
	MediaType string `json:"mediaType,omitempty"`
	Reason    string `json:"reason,omitempty"` // why it can't be attached
}

// Attachable reports whether the path can be sent as an attachment.
func (i Info) Attachable() bool { return i.Kind == Image || i.Kind == PDF || i.Kind == Text }

// Describe looks at a path without reading more than its first bytes.
func Describe(path string) Info {
	info := Info{Path: path, Name: filepath.Base(path)}
	st, err := os.Stat(path)
	if err != nil {
		info.Kind, info.Reason = Unsupported, "it can't be read"
		return info
	}
	if st.IsDir() {
		info.Kind, info.Reason = Folder, "folders can't be attached"
		return info
	}
	info.Size = st.Size()
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case imageTypes[ext] != "":
		info.Kind, info.MediaType = Image, imageTypes[ext]
	case ext == ".pdf":
		info.Kind, info.MediaType = PDF, "application/pdf"
	default:
		head, err := readHead(path, 8<<10)
		if err != nil {
			info.Kind, info.Reason = Unsupported, "it can't be read"
			return info
		}
		if !isText(head) {
			info.Kind, info.Reason = Unsupported, "only images, PDFs and text files can be attached"
			return info
		}
		info.Kind, info.MediaType = Text, "text/plain"
	}
	if limit := maxFor(info.Kind); info.Size > limit {
		info.Reason = fmt.Sprintf("it's larger than the %s limit for %s", size(limit), kindLabel(info.Kind))
		info.Kind = Unsupported
	}
	return info
}

// Load reads an attachable file.
func Load(path string) (core.Attachment, error) {
	info := Describe(path)
	if !info.Attachable() {
		return core.Attachment{}, fmt.Errorf("can't attach %s: %s", info.Name, info.Reason)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return core.Attachment{}, fmt.Errorf("can't attach %s: %w", info.Name, err)
	}
	if info.Kind == Text && !utf8.Valid(data) {
		return core.Attachment{}, fmt.Errorf("can't attach %s: it isn't UTF-8 text", info.Name)
	}
	return core.Attachment{Name: info.Name, Path: path, MediaType: info.MediaType, Data: data}, nil
}

// FromData makes an attachment from bytes with no file behind them (a
// pasted screenshot). Only images.
func FromData(name, mediaType string, data []byte) (core.Attachment, error) {
	if !strings.HasPrefix(mediaType, "image/") {
		mediaType = http.DetectContentType(data)
	}
	switch mediaType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return core.Attachment{}, errors.New("only images can be pasted as attachments")
	}
	if len(data) > MaxImage {
		return core.Attachment{}, fmt.Errorf("the pasted image is larger than the %s limit", size(MaxImage))
	}
	if name == "" {
		name = "Pasted image"
	}
	return core.Attachment{Name: name, MediaType: mediaType, Data: data}, nil
}

func readHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	k, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return buf[:k], nil
}

// isText: no NUL bytes, and valid UTF-8 once a rune cut off at the end of
// the sample is dropped.
func isText(b []byte) bool {
	if bytes.IndexByte(b, 0) >= 0 {
		return false
	}
	for i := 0; i < 3 && len(b) > 0 && !utf8.Valid(b); i++ {
		b = b[:len(b)-1]
	}
	return utf8.Valid(b)
}

func maxFor(kind string) int64 {
	switch kind {
	case Image:
		return MaxImage
	case PDF:
		return MaxPDF
	default:
		return MaxText
	}
}

func kindLabel(kind string) string {
	switch kind {
	case Image:
		return "images"
	case PDF:
		return "PDFs"
	default:
		return "text files"
	}
}

func size(n int64) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%d MB", n>>20)
	}
	return fmt.Sprintf("%d KB", n>>10)
}
