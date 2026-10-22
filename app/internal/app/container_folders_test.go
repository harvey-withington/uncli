package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"uncli/config"
	"uncli/internal/runtime/wsl"
)

// home sets up a user profile in a temp folder, as Windows lays it out.
func home(t *testing.T) string {
	t.Helper()
	h := filepath.Join(t.TempDir(), "Users", "you")
	for _, d := range []string{`AppData\Roaming`, `AppData\Local`, `.claude\skills`, "notes", ".secrets"} {
		if err := os.MkdirAll(filepath.Join(h, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("USERPROFILE", h)
	t.Setenv("HOME", h)
	t.Setenv("APPDATA", filepath.Join(h, `AppData\Roaming`))
	t.Setenv("LOCALAPPDATA", filepath.Join(h, `AppData\Local`))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	return h
}

// A profile's folders name no user: only these three variables expand, and
// what they give must be a full path on a drive.
func TestExpandSource(t *testing.T) {
	h := home(t)
	if got, err := expandSource(`%USERPROFILE%\notes`); err != nil || got != filepath.Join(h, "notes") {
		t.Errorf("got %q, %v", got, err)
	}
	if got, err := expandSource(`%localappdata%\Tools`); err != nil || got != filepath.Join(h, `AppData\Local\Tools`) {
		t.Errorf("lower case: %q, %v", got, err)
	}
	for _, bad := range []string{`%TEMP%\x`, `notes`, `\\server\share`, `%USERNAME%`} {
		if _, err := expandSource(bad); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
}

// Folders that would undo the isolation, or hand over sign-ins, are
// refused; ordinary folders, inside the profile too, are fine.
func TestFolderProblem(t *testing.T) {
	h := home(t)
	for _, bad := range []string{`C:\`, h, filepath.Dir(h), filepath.Join(h, "AppData"), filepath.Join(h, `AppData\Roaming`),
		filepath.Join(h, ".claude"), filepath.Join(h, `.claude\skills`), os.Getenv("SystemRoot")} {
		if bad == "" {
			continue
		}
		if folderProblem(bad) == "" {
			t.Errorf("%s should be refused", bad)
		}
	}
	for _, ok := range []string{filepath.Join(h, "notes"), filepath.Join(h, `AppData\Roaming\Code`), `S:\Source\arc`} {
		if why := folderProblem(ok); why != "" {
			t.Errorf("%s: %s", ok, why)
		}
	}
}

// The profile's folders and secrets are checked with the rest of it.
func TestFoldersInAProfile(t *testing.T) {
	home(t)
	defaults, _ := fs.Sub(config.Defaults, "defaults")
	dir := t.TempDir()
	write := func(s string) error {
		_ = os.WriteFile(filepath.Join(dir, "containers.yaml"), []byte("containers:\n  - id: dev\n    label: Dev\n    base: alpine-3.24\n"+s), 0o644)
		_, err := loadContainers(defaults, dir)
		return err
	}
	if err := write("    secrets: shared\n    folders:\n      - {source: '%USERPROFILE%\\notes', name: notes}\n      - {source: 'S:\\Source', name: src, access: rw}\n"); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]string{
		"a drive":         "    folders:\n      - {source: 'C:\\', name: c}\n",
		"the profile":     "    folders:\n      - {source: '%USERPROFILE%', name: me}\n",
		"a dotted name":   "    folders:\n      - {source: 'S:\\Source', name: .ssh}\n",
		"a name twice":    "    folders:\n      - {source: 'S:\\A', name: a}\n      - {source: 'S:\\B', name: a}\n",
		"other access":    "    folders:\n      - {source: 'S:\\A', name: a, access: write}\n",
		"secrets: always": "    secrets: always\n",
	} {
		if err := write(bad); err == nil {
			t.Errorf("%s should be refused", name)
		}
	}
}

// What a session's container mounts: the secrets read-only with their
// variable, the profile's folders read-only unless they say rw, and
// nothing that's missing (the settings list those).
func TestFoldersFor(t *testing.T) {
	h := home(t)
	p := ContainerProfile{ID: "dev", Secrets: SecretsShared, Folders: []ContainerFolder{
		{Source: `%USERPROFILE%\notes`, Name: "notes"},
		{Source: `S:\no-such-folder-here`, Name: "gone", Access: "rw"},
	}}
	want, env := foldersFor(p)
	s, n := want["/home/uncli/.secrets"], want["/home/uncli/notes"]
	if len(want) != 2 || s.Access != wsl.ReadOnly || s.Source != filepath.Join(h, ".secrets") || n.Access != wsl.ReadOnly || n.Kind != wsl.KindCustom {
		t.Errorf("mounts = %+v", want)
	}
	if env[SecretsEnv] != "/home/uncli/.secrets" {
		t.Errorf("env = %v", env)
	}
	if m := missingFolders(p); len(m) != 1 || !strings.Contains(m[0], "no-such-folder-here") {
		t.Errorf("missing = %v", m)
	}

	_ = os.Remove(filepath.Join(h, ".secrets"))
	want, env = foldersFor(p)
	if _, ok := want["/home/uncli/.secrets"]; ok || env[SecretsEnv] != "" {
		t.Errorf("no secrets folder: mounts %v env %v", want, env)
	}
	if m := missingFolders(p); len(m) != 2 {
		t.Errorf("missing = %v", m)
	}
}

// The model is told where to look, and about the secrets only when
// they're mounted.
func TestContainerBrief(t *testing.T) {
	p := ContainerProfile{ID: "sandbox", Label: "Sandbox"}
	b := containerBrief(p, nil)
	if !strings.Contains(b, `"Sandbox"`) || !strings.Contains(b, wsl.ManifestPath) || strings.Contains(b, "secrets") {
		t.Errorf("brief = %s", b)
	}
	b = containerBrief(p, []wsl.Mount{{Name: "secrets", Target: "/home/uncli/.secrets", Access: wsl.ReadOnly}})
	if !strings.Contains(b, "/home/uncli/.secrets") || !strings.Contains(b, "$"+SecretsEnv) || !strings.Contains(b, "never print it") {
		t.Errorf("brief = %s", b)
	}
}
