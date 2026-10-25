package remote

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"uncli/internal/app"
)

// Signed links let a device's browser open an artifact or an upload
// directly (a new tab, a download), where it can't send the Authorization
// header. Each link names one file, expires after linkLife, and its
// signature covers its route, so it opens that file and nothing else
// (decision 0022). A link names the device it was made for, and stops
// working when that device is revoked. The key is made when UNCLI starts;
// a restart ends every link.

const linkLife = 15 * time.Minute

type links struct {
	key []byte
	now func() time.Time

	mu      sync.Mutex
	revoked map[string]bool // devices revoked since UNCLI started: their links stop
}

func newLinks() *links {
	k := make([]byte, 32)
	_, _ = rand.Read(k)
	return &links{key: k, now: time.Now, revoked: map[string]bool{}}
}

// sign signs the parts, each prefixed with its length, so no two lists of
// parts sign alike.
func (l *links) sign(parts ...string) string {
	m := hmac.New(sha256.New, l.key)
	for _, p := range parts {
		m.Write([]byte(strconv.Itoa(len(p)) + ":" + p))
	}
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// revoke ends the links made for a device.
func (l *links) revoke(device string) {
	l.mu.Lock()
	l.revoked[device] = true
	l.mu.Unlock()
}

func (l *links) isRevoked(device string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.revoked[device]
}

func (l *links) expiry() string { return strconv.FormatInt(l.now().Add(linkLife).Unix(), 10) }

// artifact is the link to an artifact: a version by hash or a file in the
// session's artifacts folder by path.
func (l *links) artifact(device, sessionID string, ref app.ArtifactRef) string {
	e := l.expiry()
	q := url.Values{"d": {device}, "s": {sessionID}, "h": {ref.Hash}, "p": {ref.Path}, "e": {e}}
	q.Set("sig", l.sign("artifact", device, sessionID, ref.Hash, ref.Path, e))
	return "/l/artifact?" + q.Encode()
}

func (l *links) upload(device, id string) string {
	e := l.expiry()
	q := url.Values{"d": {device}, "id": {id}, "e": {e}}
	q.Set("sig", l.sign("upload", device, id, e))
	return "/l/upload?" + q.Encode()
}

// valid checks a link's expiry, its device (parts[1]) and its signature
// over parts (which end with the expiry).
func (l *links) valid(sig string, parts ...string) bool {
	e, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil || l.now().Unix() > e {
		return false
	}
	return !l.isRevoked(parts[1]) && hmac.Equal([]byte(sig), []byte(l.sign(parts...)))
}

// serveArtifact serves GET /l/artifact.
func (s *Server) serveArtifact(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sess, hash, rel, e := q.Get("s"), q.Get("h"), q.Get("p"), q.Get("e")
	if !s.links.valid(q.Get("sig"), "artifact", q.Get("d"), sess, hash, rel, e) {
		http.Error(w, "This link has expired. Open it again from UNCLI.", http.StatusForbidden)
		return
	}
	content, err := s.svc.ReadArtifact(sess, app.ArtifactRef{Hash: hash, Path: rel})
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	data, err := base64.StdEncoding.DecodeString(content.Data)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	name := path.Base(strings.ReplaceAll(rel, `\`, "/"))
	if rel == "" {
		name = "artifact"
	}
	serveFile(w, name, data)
}

// serveUpload serves GET /l/upload.
func (s *Server) serveUpload(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id, e := q.Get("id"), q.Get("e")
	if !s.links.valid(q.Get("sig"), "upload", q.Get("d"), id, e) {
		http.Error(w, "This link has expired. Open it again from UNCLI.", http.StatusForbidden)
		return
	}
	p, err := s.svc.UploadPath(id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": filepath.Base(p)}))
	http.ServeFile(w, r, p)
}

// serveFile sends a file's bytes for the browser to show. It runs
// sandboxed, in an origin of its own, so an HTML artifact's scripts can't
// reach UNCLI's page or the device's token.
func serveFile(w http.ResponseWriter, name string, data []byte) {
	ct := mime.TypeByExtension(filepath.Ext(name))
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": name}))
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(data)
}
