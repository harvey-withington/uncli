package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"uncli/internal/runtime/wsl"
)

func editService(t *testing.T) *Service {
	t.Helper()
	checkWSL = func(context.Context) (wsl.Status, error) {
		return wsl.Status{Installed: true, Distros: []string{}}, nil
	}
	t.Cleanup(func() { checkWSL = wsl.CheckStatus })
	dir := t.TempDir()
	svc, err := New(Paths{Config: filepath.Join(dir, "c"), Cache: filepath.Join(dir, "k"), Scratch: filepath.Join(dir, "s")}, &recEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	return svc
}

func find(info ContainersInfo, id string) (ContainerInfo, bool) {
	for _, c := range info.Containers {
		if c.ID == id {
			return c, true
		}
	}
	return ContainerInfo{}, false
}

func TestEditContainers(t *testing.T) {
	svc := editService(t)
	ctx := context.Background()

	// A new container of the user's own, tidied as it's saved.
	err := svc.SaveContainer(ContainerProfile{ID: "web", Label: " Web ", Base: "alpine-3.24", Packages: []string{"nodejs", " npm", "nodejs", ""},
		Setup: []string{"npm i -g pnpm", "  "}, Brain: BrainShared, MCP: MCPNone, Connectors: ConnectorsNone})
	if err != nil {
		t.Fatal(err)
	}
	web, ok := find(svc.Containers(ctx), "web")
	if !ok || web.Label != "Web" || !slices.Equal(web.Packages, []string{"nodejs", "npm"}) || !slices.Equal(web.Setup, []string{"npm i -g pnpm"}) || web.Builtin || !web.Edited {
		t.Fatalf("web = %+v", web)
	}
	b, _ := os.ReadFile(filepath.Join(svc.Paths.Config, "containers.yaml"))
	if !strings.HasPrefix(string(b), "# Your containers for UNCLI") || !strings.Contains(string(b), "npm i -g pnpm") {
		t.Errorf("file = %s", b)
	}

	// A built-in one, edited: the user's version, until reset.
	sandbox, _ := find(svc.Containers(ctx), "sandbox")
	if !sandbox.Builtin || sandbox.Edited {
		t.Fatalf("sandbox = %+v", sandbox)
	}
	sandbox.Packages = append(sandbox.Packages, "go")
	if err := svc.SaveContainer(sandbox.ContainerProfile); err != nil {
		t.Fatal(err)
	}
	if got, _ := find(svc.Containers(ctx), "sandbox"); !got.Edited || !slices.Contains(got.Packages, "go") {
		t.Errorf("edited sandbox = %+v", got)
	}
	if err := svc.RemoveContainerConfig(ctx, "sandbox"); err != nil {
		t.Fatal(err)
	}
	if got, ok := find(svc.Containers(ctx), "sandbox"); !ok || got.Edited || slices.Contains(got.Packages, "go") {
		t.Errorf("reset sandbox = %+v", got)
	}

	// The user's own, deleted.
	if err := svc.RemoveContainerConfig(ctx, "web"); err != nil {
		t.Fatal(err)
	}
	if _, ok := find(svc.Containers(ctx), "web"); ok {
		t.Error("web is still there")
	}

	// Refused, with the file untouched.
	for _, bad := range []ContainerProfile{
		{ID: "Bad Name", Label: "x", Base: "alpine-3.24"},
		{ID: "x", Label: "", Base: "alpine-3.24"},
		{ID: "x", Label: "x", Base: "debian-99"},
		{ID: "x", Label: "x", Base: "alpine-3.24", Packages: []string{"go; rm -rf /"}},
		{ID: "x", Label: "x", Base: "alpine-3.24", Connectors: "everything"},
	} {
		if err := svc.SaveContainer(bad); err == nil {
			t.Errorf("%+v was saved", bad)
		}
	}
	if _, ok := find(svc.Containers(ctx), "x"); ok {
		t.Error("a refused container was saved")
	}
}

// Setup steps are part of how a container is built.
func TestSetupStepsCountAsChanges(t *testing.T) {
	svc := editService(t)
	p, b, _ := svc.profileAndBase("isolated")
	was := recordFor(p, b, "2.1.285")
	p.Setup = []string{"pip install httpie"}
	if err := svc.saveBuildRecord("isolated", was); err != nil {
		t.Fatal(err)
	}
	if got := svc.changesSince("isolated", recordFor(p, b, "2.1.285")); !slices.Equal(got, []string{ChangedSteps}) {
		t.Errorf("changes = %v", got)
	}
}
