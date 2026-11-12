package wsl

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
)

// Local paths (decision 0032): parts of the project, such as node_modules,
// that the container keeps on its own disk, bound over the project mount at
// that path. Windows keeps its own files at the same path and never sees
// the container's, so neither side's install breaks the other's.

// localScript binds each local path ($3…, relative to the project mount
// $1) to its folder under $2, as root. The folder on the distro's disk is
// the CLI's user's; a path missing in the project is created there (an
// empty folder on Windows), as the user's too. A path that is a file, or
// leads through a link, is refused rather than followed out of the
// project, and a bind that lands anywhere but the path (a link swapped in
// meanwhile) is taken out again.
const localScript = `p=$1; root=$2; shift 2
fail() { echo "$rel: $1" >&2; exit 1; }
for rel; do
  t="$p/$rel"; s="$root/$rel"
  if [ ! -d "$s" ]; then mkdir -p "$s" && chown 1000:1000 "$s" || fail "could not make the container's folder"; fi
  e=$t; while [ ! -e "$e" ] && [ ! -L "$e" ]; do e=$(dirname "$e"); done
  [ "$(realpath "$e")" = "$e" ] || fail "leads through a link, so it isn't in the project"
  if [ "$e" != "$t" ]; then
    mkdir -p "$t" || fail "could not make it in the project"
    d=$t; while [ "$d" != "$e" ]; do chown 1000:1000 "$d"; d=$(dirname "$d"); done
  fi
  [ -d "$t" ] || fail "is a file in the project, not a folder"
  [ "$(realpath "$t")" = "$t" ] || fail "leads through a link, so it isn't in the project"
  mountpoint -q "$t" && continue
  mount --bind "$s" "$t" || fail "could not mount the container's folder there"
  at=$(printf '%b' "$(tail -n 1 /proc/self/mountinfo | cut -d' ' -f5)")
  if [ "$at" != "$t" ]; then umount -l "$at"; fail "changed while it was mounted"; fi
done`

// unmountScript unmounts each of its arguments that is mounted, with what
// is mounted under it, deepest first (a project's local paths before the
// project), lazily when something still uses it.
const unmountScript = `for t; do
mountpoint -q "$t" || continue
P="$t/" awk 'index($5, ENVIRON["P"]) == 1 { print length($5) " " $5 }' /proc/self/mountinfo | sort -rn | cut -d' ' -f2- |
while IFS= read -r m; do m=$(printf '%b' "$m"); umount "$m" 2>/dev/null || umount -l "$m"; done
umount "$t" 2>/dev/null || umount -l "$t"
done`

// LocalSource is where the distro keeps a project's copy of a local path.
func LocalSource(point, rel string) string {
	return path.Join(LocalRoot, path.Base(point), rel)
}

// mountLocal binds the profile's local paths (Local) over the project
// mounted at point, and takes out those it no longer lists. Paths already
// bound are left alone, so a session start costs nothing more once they
// are.
func (r *Runtime) mountLocal(ctx context.Context, point string) error {
	var want []string
	if r.Local != nil {
		want = slices.Clone(r.Local())
	}
	slices.Sort(want) // a folder before what's in it
	wanted := map[string]bool{}
	for _, rel := range want {
		wanted[path.Join(point, rel)] = true
	}
	var gone []string
	have := map[string]bool{}
	for _, m := range r.Mounts() {
		if m.Kind != KindLocal || !strings.HasPrefix(m.Target, point+"/") {
			continue
		}
		if wanted[m.Target] {
			have[m.Target] = true
		} else {
			gone = append(gone, m.Target)
		}
	}
	for _, at := range gone {
		if err := r.Unmount(ctx, at); err != nil {
			return fmt.Errorf("could not take %s out of the project: %w", at, err)
		}
	}
	if len(have) == len(want) {
		return nil
	}
	root := path.Join(LocalRoot, path.Base(point))
	args := append([]string{"--distribution", r.Distro, "--user", "root", "--exec", "sh", "-c", localScript, "sh", point, root}, want...)
	if _, err := r.run(ctx, nil, args...); err != nil {
		return fmt.Errorf("could not keep the container's own copies in the project: %w", err)
	}
	r.mu.Lock()
	if r.mounts == nil {
		r.mounts = map[string]Mount{}
	}
	for _, rel := range want {
		at := path.Join(point, rel)
		r.mounts[at] = Mount{Name: rel, Source: LocalSource(point, rel), Target: at, Access: ReadWrite, Kind: KindLocal}
	}
	r.mu.Unlock()
	return nil
}

// unmountLocal takes out every local path, before the distro is let go.
func (r *Runtime) unmountLocal(ctx context.Context) {
	var at []string
	for _, m := range r.Mounts() {
		if m.Kind == KindLocal {
			at = append(at, m.Target)
		}
	}
	if len(at) == 0 {
		return
	}
	slices.Reverse(at) // what's in a folder before it
	args := append([]string{"--distribution", r.Distro, "--user", "root", "--exec", "sh", "-c", unmountScript, "sh"}, at...)
	if _, err := r.run(ctx, nil, args...); err == nil {
		r.mu.Lock()
		for _, t := range at {
			r.forget(t)
		}
		r.mu.Unlock()
	}
}
