package wsl

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// localCalls are the local script's arguments: project, root, paths.
func localCalls(f *fake) [][]string {
	var out [][]string
	for _, c := range f.calls {
		if i := slices.Index(c, localScript); i >= 0 {
			out = append(out, c[i+2:])
		}
	}
	return out
}

// A project's local paths are bound once, after the project, from a
// folder of the project's own on the distro's disk; they're listed in the
// manifest as local, and one the profile drops is taken out.
func TestLocalPathsMountAfterTheProject(t *testing.T) {
	f := &fake{}
	r := heldRuntime(f)
	local := []string{"node_modules", "apps/web/node_modules"}
	r.Local = func() []string { return local }
	ctx := context.Background()
	dir := t.TempDir()
	point, err := r.Mount(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if i, j := slices.IndexFunc(f.calls, func(c []string) bool { return slices.Contains(c, mountScript) }),
		slices.IndexFunc(f.calls, func(c []string) bool { return slices.Contains(c, localScript) }); i < 0 || j < i {
		t.Fatalf("the project wasn't mounted before its local paths: %q", f.calls)
	}
	root := LocalRoot + "/" + strings.TrimPrefix(point, WorkspaceRoot+"/")
	if got := localCalls(f); len(got) != 1 || !slices.Equal(got[0], []string{point, root, "apps/web/node_modules", "node_modules"}) {
		t.Fatalf("local calls = %q", got)
	}
	var recs []Mount
	for _, m := range r.Mounts() {
		if m.Kind == KindLocal {
			recs = append(recs, m)
		}
	}
	if len(recs) != 2 || recs[1].Target != point+"/node_modules" || recs[1].Source != root+"/node_modules" || recs[1].Name != "node_modules" || recs[1].Access != ReadWrite {
		t.Errorf("records = %+v", recs)
	}

	// Already bound: the next start leaves them be.
	f.calls = nil
	if _, err := r.Mount(ctx, dir); err != nil {
		t.Fatal(err)
	}
	if len(localCalls(f)) != 0 || len(unmounts(f)) != 0 {
		t.Errorf("mounted again: %q", f.calls)
	}

	// Dropped from the profile: unmounted, the other kept.
	local = []string{"node_modules"}
	f.calls = nil
	if _, err := r.Mount(ctx, dir); err != nil {
		t.Fatal(err)
	}
	if u := unmounts(f); !slices.Equal(u, []string{point + "/apps/web/node_modules"}) || len(localCalls(f)) != 0 {
		t.Errorf("unmounts = %q, calls %q", u, f.calls)
	}
}

// Taking a project out (another folder there, or the distro restarted)
// unmounts what's under it first, and forgets its local paths, so they're
// bound again over the new mount.
func TestLocalPathsFollowTheProject(t *testing.T) {
	f := &fake{}
	r := heldRuntime(f)
	r.Local = func() []string { return []string{"node_modules"} }
	ctx := context.Background()
	dir := t.TempDir()
	point, _ := r.Mount(ctx, dir)
	if !strings.Contains(unmountScript, "/proc/self/mountinfo") || !strings.Contains(unmountScript, "sort -rn") {
		t.Error("unmounting must take what's under a folder first, deepest first")
	}
	if err := r.Unmount(ctx, point); err != nil {
		t.Fatal(err)
	}
	if len(r.Mounts()) != 0 {
		t.Errorf("records = %+v", r.Mounts())
	}
	f.calls = nil
	_, _ = r.Mount(ctx, dir)
	if len(localCalls(f)) != 1 {
		t.Errorf("not bound again: %q", f.calls)
	}
}

// Closing the runtime takes the local paths out before letting the distro
// go; with none, it costs no call.
func TestCloseUnmountsLocalPaths(t *testing.T) {
	f := &fake{}
	r := heldRuntime(f)
	ctx := context.Background()
	_, _ = r.Mount(ctx, t.TempDir())
	f.calls = nil
	r.Close()
	if len(f.calls) != 0 {
		t.Errorf("calls without local paths: %q", f.calls)
	}

	f = &fake{}
	r = heldRuntime(f)
	r.Local = func() []string { return []string{"node_modules", "web/node_modules"} }
	point, _ := r.Mount(ctx, t.TempDir())
	f.calls = nil
	r.Close()
	if len(f.calls) != 1 || !slices.Contains(f.calls[0], unmountScript) {
		t.Fatalf("calls = %q", f.calls)
	}
	args := f.calls[0][slices.Index(f.calls[0], unmountScript)+2:]
	if !slices.Equal(args, []string{point + "/web/node_modules", point + "/node_modules"}) {
		t.Errorf("unmounted %q", args)
	}
}

// A path that's a file or leads through a link isn't followed out of the
// project; the script checks before it creates or binds anything.
func TestLocalScriptStaysInTheProject(t *testing.T) {
	for _, want := range []string{`realpath "$e"`, `realpath "$t"`, `[ -d "$t" ]`, "mount --bind", "chown 1000:1000"} {
		if !strings.Contains(localScript, want) {
			t.Errorf("the script lacks %s", want)
		}
	}
	if strings.Index(localScript, `realpath "$t"`) > strings.Index(localScript, "mount --bind") {
		t.Error("the path must be checked before it's bound")
	}
}
