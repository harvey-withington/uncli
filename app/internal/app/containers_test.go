package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"uncli/config"
)

func TestContainerProfiles(t *testing.T) {
	defaults, _ := fs.Sub(config.Defaults, "defaults")
	c, err := loadContainers(defaults, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Containers) != 2 || c.Containers[0].ID != "sandbox" || c.Containers[0].Brain != BrainShared || c.Containers[0].MCP != MCPShared || c.Containers[0].Connectors != ConnectorsShared || c.Containers[1].Connectors != ConnectorsNone ||
		c.Containers[1].Brain != BrainSandboxed || len(c.Bases) == 0 || len(c.Bases[0].SHA256) != 64 {
		t.Fatalf("built-in = %+v", c)
	}

	dir := t.TempDir()
	user := "containers:\n  - id: sandbox\n    label: My sandbox\n    base: alpine-3.24\n    packages: [go]\n  - id: node-dev\n    label: Node\n    base: alpine-3.24\n    packages: [nodejs, npm]\n"
	if err := os.WriteFile(filepath.Join(dir, "containers.yaml"), []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err = loadContainers(defaults, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Containers) != 3 || c.Containers[0].Label != "My sandbox" || c.Containers[1].ID != "isolated" || c.Containers[2].ID != "node-dev" {
		t.Errorf("merged = %+v", c.Containers)
	}

	for _, bad := range []string{
		"containers:\n  - id: Bad Name\n    base: alpine-3.24\n",
		"containers:\n  - id: nobase\n    base: debian-99\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, "containers.yaml"), []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadContainers(defaults, dir); err == nil || !strings.Contains(err.Error(), "container") {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}
