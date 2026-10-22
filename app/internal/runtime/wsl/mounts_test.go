package wsl

import (
	"context"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"testing"
)

// held is a keeper that is already running, so no wsl.exe starts.
type held struct{}

func (held) Stdin() io.Writer  { return io.Discard }
func (held) Stdout() io.Reader { return strings.NewReader("") }
func (held) Stderr() io.Reader { return strings.NewReader("") }
func (held) Wait() error       { return nil }
func (held) Kill() error       { return nil }

func heldRuntime(f *fake) *Runtime {
	return &Runtime{Distro: "uncli-dev", run: f.run, keeper: held{}, mounts: map[string]Mount{}}
}

// The mount script's arguments: target, source, access.
func mountCalls(f *fake) [][]string {
	var out [][]string
	for _, c := range f.calls {
		if i := slices.Index(c, mountScript); i >= 0 {
			out = append(out, c[i+2:])
		}
	}
	return out
}

func unmounts(f *fake) []string {
	var out []string
	for _, c := range f.calls {
		if len(c) > 0 && strings.Contains(strings.Join(c, " "), "umount") {
			out = append(out, c[len(c)-1])
		}
	}
	return out
}

// Read-only comes from a remount (drvfs ignores -o ro), each mount is
// recorded, and a target already showing the same folder isn't unmounted
// to mount it again.
func TestMountFolderAccessAndRecords(t *testing.T) {
	f := &fake{}
	r := heldRuntime(f)
	ctx := context.Background()
	if err := r.MountFolder(ctx, Mount{Name: "secrets", Source: `C:\Users\you\.secrets`, Target: "/home/uncli/.secrets", Access: ReadOnly, Kind: KindShare}); err != nil {
		t.Fatal(err)
	}
	if err := r.MountAt(ctx, `C:\data\projects`, "/home/uncli/.claude/projects"); err != nil {
		t.Fatal(err)
	}
	got := mountCalls(f)
	if len(got) != 2 || !slices.Equal(got[0], []string{"/home/uncli/.secrets", `C:\Users\you\.secrets`, "ro"}) || got[1][2] != "rw" {
		t.Fatalf("mounts = %q", got)
	}
	if !strings.Contains(mountScript, `mount -o remount,"$3" "$1"`) {
		t.Error("access must be set by a remount")
	}
	ms := r.Mounts()
	if len(ms) != 2 || ms[0].Target != "/home/uncli/.claude/projects" || ms[0].Kind != KindBuiltin || ms[1].Access != ReadOnly {
		t.Errorf("records = %+v", ms)
	}

	f.calls = nil
	_ = r.MountFolder(ctx, Mount{Name: "secrets", Source: `C:\Users\you\.secrets`, Target: "/home/uncli/.secrets", Access: ReadOnly, Kind: KindShare})
	if len(unmounts(f)) != 0 {
		t.Errorf("the same folder again was unmounted: %q", f.calls)
	}
	// Another folder at the same place replaces it.
	_ = r.MountFolder(ctx, Mount{Name: "notes", Source: `C:\Users\you\other`, Target: "/home/uncli/.secrets", Kind: KindCustom})
	if u := unmounts(f); !slices.Equal(u, []string{"/home/uncli/.secrets"}) {
		t.Errorf("unmounts = %q", u)
	}
}

// Unmounting a folder forgets what was mounted under it too.
func TestUnmountForgetsWhatWasUnder(t *testing.T) {
	f := &fake{}
	r := heldRuntime(f)
	ctx := context.Background()
	_ = r.MountAt(ctx, `C:\acct`, "/home/uncli/.claude")
	_ = r.MountAt(ctx, `C:\data\projects`, "/home/uncli/.claude/projects")
	_ = r.MountAt(ctx, `C:\work`, "/workspace/work-1234")
	if err := r.Unmount(ctx, "/home/uncli/.claude"); err != nil {
		t.Fatal(err)
	}
	if ms := r.Mounts(); len(ms) != 1 || ms[0].Target != "/workspace/work-1234" {
		t.Errorf("records = %+v", ms)
	}
}

// The manifest lists every mount, written by root where sessions can read
// it but not change it.
func TestWriteManifest(t *testing.T) {
	f := &fake{}
	r := heldRuntime(f)
	ctx := context.Background()
	_, _ = r.Mount(ctx, t.TempDir())
	_ = r.MountFolder(ctx, Mount{Name: "notes", Source: `C:\Users\you\notes`, Target: "/home/uncli/notes", Access: ReadOnly, Kind: KindCustom})
	if err := r.WriteManifest(ctx); err != nil {
		t.Fatal(err)
	}
	last := f.calls[len(f.calls)-1]
	if !slices.Contains(last, "root") || last[len(last)-1] != ManifestPath || !strings.Contains(strings.Join(last, " "), "chmod 644") {
		t.Errorf("manifest call = %q", last)
	}
	var got []Mount
	if err := json.Unmarshal([]byte(f.stdins[len(f.stdins)-1]), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "notes" || got[1].Name != "project" || got[1].Access != ReadWrite || !strings.HasPrefix(got[1].Target, WorkspaceRoot+"/") {
		t.Errorf("manifest = %+v", got)
	}
}
