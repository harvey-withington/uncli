package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// suggestingRepo makes a repository whose .uncli suggests containers
// (file's name and text), and returns its root.
func suggestingRepo(t *testing.T, name, text string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".uncli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src", "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSuggestions(t, root, name, text)
	return root
}

func writeSuggestions(t *testing.T, root, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".uncli", name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

const teamDev = `version: 1
containers:
  - id: team-dev
    label: Team dev
    base: alpine-3.24
    packages: [nodejs, npm]
    brain: shared
    secrets: shared
    setup:
      - npm i -g pnpm
      - curl -fsSL https://example.com/install.sh | sh
  - id: odd
    label: Odd
    base: debian-13
`

func suggested(t *testing.T, svc *Service, folder string) Suggestions {
	t.Helper()
	s, err := svc.SuggestedContainers(context.Background(), folder)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSuggestedContainers(t *testing.T) {
	svc := editService(t)
	ctx := context.Background()
	root := suggestingRepo(t, "containers.yml", teamDev)

	// Found from a folder inside the repository, with what each would reach.
	s := suggested(t, svc, filepath.Join(root, "src", "web"))
	if !sameFolder(s.Repo, root) || s.Name != filepath.Base(root) || s.Problem != "" || len(s.Containers) != 2 {
		t.Fatalf("suggestions = %+v", s)
	}
	dev, odd := s.Containers[0], s.Containers[1]
	if dev.Status != "new" || !slices.Equal(dev.Reaches, []string{"brain", "secrets"}) {
		t.Errorf("team-dev = %+v", dev)
	}
	if len(dev.Steps) != 1 || dev.Steps[0].Step != "curl -fsSL https://example.com/install.sh | sh" || !slices.Contains(dev.Steps[0].Why, "runs-code") {
		t.Errorf("flagged steps = %+v", dev.Steps)
	}
	if odd.Status != "unusable" || odd.Problem != "base" {
		t.Errorf("odd = %+v", odd)
	}

	// Imported as the user left it in the editor (secrets off, renamed).
	mine := dev.Profile
	mine.ID, mine.Label, mine.Secrets = "dev", "Our dev", ""
	if err := svc.ImportContainer(ctx, mine, root, "team-dev"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ImportContainer(ctx, mine, root, "odd"); err == nil {
		t.Error("imported a container on a base UNCLI doesn't have")
	}
	got, ok := find(svc.Containers(ctx), "dev")
	if !ok || got.From == nil || got.From.ID != "team-dev" || !sameFolder(got.From.Repo, root) || got.From.Imported.IsZero() {
		t.Fatalf("dev = %+v", got)
	}
	// Turning secrets off makes it differ, by the user's hand.
	if c := suggested(t, svc, root).Containers[0]; c.Status != "differs" || c.By != "you" || c.Yours != "dev" {
		t.Errorf("after an import with secrets off: %+v", c)
	}

	// Synced as suggested: the same, whatever its name.
	mine.Secrets = SecretsShared
	if err := svc.ImportContainer(ctx, mine, root, "team-dev"); err != nil {
		t.Fatal(err)
	}
	if c := suggested(t, svc, root).Containers[0]; c.Status != "imported" {
		t.Errorf("after syncing: %+v", c)
	}

	// Saving from Settings keeps where it came from; a change there is the user's.
	mine.Packages = append(mine.Packages, "jq")
	mine.From = nil
	if err := svc.SaveContainer(mine); err != nil {
		t.Fatal(err)
	}
	if got, _ := find(svc.Containers(ctx), "dev"); got.From == nil {
		t.Error("saving from Settings forgot where it came from")
	}
	if c := suggested(t, svc, root).Containers[0]; c.Status != "differs" || c.By != "you" {
		t.Errorf("after the user's change: %+v", c)
	}

	// The project's change is the project's, with when it was made.
	writeSuggestions(t, root, "containers.yml", teamDev+"\n  - id: more\n    label: More\n    base: alpine-3.24\n")
	if c := suggested(t, svc, root).Containers[0]; c.By != "you" {
		t.Errorf("another container added changed team-dev: %+v", c)
	}
	writeSuggestions(t, root, "containers.yml", `version: 1
containers:
  - id: team-dev
    label: Team dev
    base: alpine-3.24
    packages: [nodejs, npm, postgresql-client]
`)
	if c := suggested(t, svc, root).Containers[0]; c.Status != "differs" || c.By != "project" || c.Changed.IsZero() {
		t.Errorf("after the project's change: %+v", c)
	}
}

func TestSuggestedContainersProblems(t *testing.T) {
	svc := editService(t)

	// No repository, or one without .uncli: nothing.
	if s := suggested(t, svc, t.TempDir()); s.Repo != "" || len(s.Containers) != 0 {
		t.Errorf("outside a repository: %+v", s)
	}

	// Both names: neither is used.
	root := suggestingRepo(t, "containers.yml", teamDev)
	writeSuggestions(t, root, "containers.yaml", teamDev)
	if s := suggested(t, svc, root); s.Problem != "both" || len(s.Containers) != 0 {
		t.Errorf("both files: %+v", s)
	}

	// A version UNCLI doesn't know, and a file that doesn't parse.
	root = suggestingRepo(t, "containers.yaml", "version: 2\ncontainers: []\n")
	if s := suggested(t, svc, root); s.Problem != "version" {
		t.Errorf("version 2: %+v", s)
	}
	writeSuggestions(t, root, "containers.yaml", "version: [1\n")
	if s := suggested(t, svc, root); s.Problem != "unreadable" || s.Detail == "" {
		t.Errorf("broken file: %+v", s)
	}

	// A suggestion that isn't valid says why, and where it came from isn't read.
	writeSuggestions(t, root, "containers.yaml", "version: 1\ncontainers:\n  - id: Bad\n    label: Bad\n    base: alpine-3.24\n    from: {repo: elsewhere, id: x}\n")
	s := suggested(t, svc, root)
	if len(s.Containers) != 1 || s.Containers[0].Problem != "invalid" || s.Containers[0].Profile.From != nil {
		t.Errorf("invalid: %+v", s)
	}
}
