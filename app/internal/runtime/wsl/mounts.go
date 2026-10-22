package wsl

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Mount is a Windows folder UNCLI mounted in the distro. Each one is
// recorded, so the manifest (ManifestPath) lists everything a session can
// see and where it comes from, and a share that's turned off can be taken
// out again.
type Mount struct {
	Name   string `json:"name"`   // what it is: project, skills/<name>, secrets, a custom folder's name
	Source string `json:"source"` // the Windows folder
	Target string `json:"target"` // where it is in the distro
	Access string `json:"access"` // rw or ro
	Kind   string `json:"kind"`   // builtin, share or custom
}

// Mount kinds.
const (
	KindBuiltin = "builtin" // UNCLI's own: the session folder, conversations, the account
	KindShare   = "share"   // one of the user's, given by a share (memories, skills, secrets)
	KindCustom  = "custom"  // a folder the container profile lists
)

// Access modes.
const (
	ReadWrite = "rw"
	ReadOnly  = "ro"
)

// ManifestPath is where the distro lists its mounts, written by root and
// readable by the CLI's user (MountsEnv points at it).
const ManifestPath = "/etc/uncli/mounts.json"

// MountsEnv names the manifest for the CLI and its tools.
const MountsEnv = "UNCLI_MOUNTS"

// mountScript mounts $2 (a Windows folder) at $1 unless something already
// is, then sets it to $3 (rw or ro). drvfs with metadata keeps Linux
// permissions, owned by the CLI's user; it ignores -o ro, so read-only
// comes from a remount, which only root can undo. Folders it creates in
// the user's home are the user's.
const mountScript = `if ! mountpoint -q "$1"; then
mkdir -p "$1"
d="$1"
while d=$(dirname "$d"); [ "$d" != / ] && [ "$d" != /home ]; do
  case "$d" in /home/uncli|/home/uncli/*) chown 1000:1000 "$d" ;; esac
done
mount -t drvfs "$2" "$1" -o metadata,uid=1000,gid=1000 || exit 1
fi
mount -o remount,"$3" "$1"`

// MountFolder mounts m.Source at m.Target with its access (read-write
// unless it says ro), replacing whatever else was there.
func (r *Runtime) MountFolder(ctx context.Context, m Mount) error {
	if m.Access != ReadOnly {
		m.Access = ReadWrite
	}
	if m.Kind == "" {
		m.Kind = KindBuiltin
	}
	m.Source = filepath.Clean(m.Source)
	if err := r.keepRunning(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	had, known := r.mounts[m.Target]
	r.mu.Unlock()
	// A folder mounted from elsewhere (or before UNCLI knew of it) goes
	// first, so the target shows what it says it does.
	if !known || had.Source != m.Source {
		if err := r.Unmount(ctx, m.Target); err != nil {
			return err
		}
	}
	if _, err := r.run(ctx, nil, "--distribution", r.Distro, "--user", "root", "--exec", "sh", "-c", mountScript, "sh", m.Target, m.Source, m.Access); err != nil {
		return fmt.Errorf("could not mount %s in %s: %w", m.Source, r.Distro, err)
	}
	r.mu.Lock()
	if r.mounts == nil {
		r.mounts = map[string]Mount{}
	}
	r.mounts[m.Target] = m
	r.mu.Unlock()
	return nil
}

// Mounts is what UNCLI has mounted in the distro, by target.
func (r *Runtime) Mounts() []Mount {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Mount, 0, len(r.mounts))
	for _, m := range r.mounts {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out
}

// forget drops the records of a target and everything mounted under it
// (unmounting a folder takes those with it). Caller holds r.mu.
func (r *Runtime) forget(at string) {
	for t := range r.mounts {
		if t == at || strings.HasPrefix(t, strings.TrimSuffix(at, "/")+"/") {
			delete(r.mounts, t)
		}
	}
}

// WriteManifest writes the list of mounts to ManifestPath, as root, so
// the CLI's user can read it but not change it.
func (r *Runtime) WriteManifest(ctx context.Context) error {
	b, err := json.MarshalIndent(r.Mounts(), "", "  ")
	if err != nil {
		return err
	}
	_, err = r.run(ctx, strings.NewReader(string(b)+"\n"), "--distribution", r.Distro, "--user", "root", "--exec", "sh", "-c",
		`mkdir -p "$(dirname "$1")" && cat > "$1.tmp" && chmod 644 "$1.tmp" && mv "$1.tmp" "$1"`, "sh", ManifestPath)
	if err != nil {
		return fmt.Errorf("could not list the mounts in %s: %w", r.Distro, err)
	}
	return nil
}

// Brief is what a session is told about the container (Briefing).
func (r *Runtime) Brief() string {
	if r.Briefing == nil {
		return ""
	}
	return r.Briefing()
}

// GuestPath is where a session folder is seen in the distro.
func (r *Runtime) GuestPath(host string) string { return MountPoint(host) }
