package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/harvey-withington/uncli/app/internal/core"
	"github.com/harvey-withington/uncli/app/internal/notify"
	"github.com/harvey-withington/uncli/app/internal/runtime/local"
	"github.com/harvey-withington/uncli/app/internal/session"
)

// A repository suggests containers in .uncli/containers.yaml (or .yml),
// and the user imports them into their own (decision 0028). Nothing in the
// file applies by itself: importing opens the container editor on it, and
// what the user saves there is an ordinary container that remembers where
// it came from.

// ContainerSource is the project container a container was imported from.
type ContainerSource struct {
	Repo     string    `yaml:"repo" json:"repo"`         // the repository's root folder
	ID       string    `yaml:"id" json:"id"`             // the container's id there
	Imported time.Time `yaml:"imported" json:"imported"` // when it was last imported or synced
	// Sum fingerprints the suggestion as it was then, to tell the
	// project's changes from the user's (a commit's date can be older than
	// the import it's pulled after).
	Sum string `yaml:"sum" json:"sum"`
	// Device is the paired device it was imported (or last synced) from;
	// empty at this PC (decision 0033).
	Device string `yaml:"device,omitempty" json:"device,omitempty"`
}

// Suggestions are the containers a session's folder's repository suggests.
type Suggestions struct {
	Repo       string               `json:"repo,omitempty"`    // the repository's root; empty when the folder has no .uncli
	Name       string               `json:"name,omitempty"`    // its folder's name, to show
	Problem    string               `json:"problem,omitempty"` // the file as a whole: both, unreadable, version
	Detail     string               `json:"detail,omitempty"`  // unreadable: the parser's words
	Containers []SuggestedContainer `json:"containers"`
}

// SuggestedContainer is one container the repository suggests, against
// the user's own.
type SuggestedContainer struct {
	Profile ContainerProfile `json:"profile"`
	// Status: new (none of the user's came from it), imported (the user's
	// is the same), differs, or unusable (Problem says why).
	Status  string    `json:"status"`
	Yours   string    `json:"yours,omitempty"`   // the user's container that came from it
	By      string    `json:"by,omitempty"`      // differs: project (it changed after the import) or you
	Changed time.Time `json:"changed,omitzero"`  // differs: when the project's file last changed
	Problem string    `json:"problem,omitempty"` // unusable: base (one UNCLI doesn't have) or invalid
	Detail  string    `json:"detail,omitempty"`  // invalid: what's wrong
	// Reaches: what of the user's it would share (brain, mcp, connectors,
	// secrets, folders), and Steps: its setup steps UNCLI's risk rules flag.
	Reaches []string    `json:"reaches"`
	Steps   []StepRisks `json:"steps"`
}

// StepRisks is a setup step UNCLI's risk rules flag, and why (session
// SetupStepRisks's words, or unreadable).
type StepRisks struct {
	Step string   `json:"step"`
	Why  []string `json:"why"`
}

const maxSuggestionsFile = 256 << 10

// repoRoot is the repository a folder is in: the nearest folder holding
// .git, or "" when there's none.
func repoRoot(folder string) string {
	if folder == "" {
		return ""
	}
	dir := filepath.Clean(folder)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		up := filepath.Dir(dir)
		if up == dir {
			return ""
		}
		dir = up
	}
}

func sameFolder(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// suggestionsFile is the repository's file of suggestions: its path, or
// a problem (both names there).
func suggestionsFile(root string) (path, problem string) {
	var found []string
	for _, n := range []string{"containers.yaml", "containers.yml"} {
		p := filepath.Join(root, ".uncli", n)
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			found = append(found, p)
		}
	}
	switch len(found) {
	case 0:
		return "", ""
	case 1:
		return found[0], ""
	}
	return "", "both"
}

// readSuggestions reads a repository's suggestions; a broken file comes
// back as a problem to show, not an error.
func readSuggestions(path string) (out []ContainerProfile, problem, detail string) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, "unreadable", err.Error()
	}
	if fi.Size() > maxSuggestionsFile {
		return nil, "unreadable", "the file is too big"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "unreadable", err.Error()
	}
	var f struct {
		Version    int                `yaml:"version"`
		Containers []ContainerProfile `yaml:"containers"`
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, "unreadable", err.Error()
	}
	if f.Version != 1 {
		return nil, "version", fmt.Sprint(f.Version)
	}
	for _, p := range f.Containers {
		p.From = nil // only importing sets it
		out = append(out, tidyProfile(p))
	}
	return out, "", ""
}

// SuggestedContainers lists the containers a folder's repository suggests,
// each against the user's own. A folder outside a repository, or without
// .uncli, has none.
func (s *Service) SuggestedContainers(ctx context.Context, folder string) (Suggestions, error) {
	none := Suggestions{Containers: []SuggestedContainer{}}
	if s.embedded {
		return none, nil
	}
	root := repoRoot(folder)
	if root == "" {
		return none, nil
	}
	path, problem := suggestionsFile(root)
	if path == "" && problem == "" {
		return none, nil
	}
	out := Suggestions{Repo: root, Name: filepath.Base(root), Problem: problem, Containers: []SuggestedContainer{}}
	if problem != "" {
		return out, nil
	}
	profiles, problem, detail := readSuggestions(path)
	if problem != "" {
		out.Problem, out.Detail = problem, detail
		return out, nil
	}
	u, err := s.readUserContainers()
	if err != nil {
		return out, err
	}
	s.reloadContainers()
	s.containers.mu.Lock()
	bases := append([]Base{}, s.containers.cfg.Bases...)
	s.containers.mu.Unlock()
	var changed time.Time // read once, and only when needed
	for _, p := range profiles {
		sc := SuggestedContainer{Profile: p, Status: "new", Reaches: reaches(p), Steps: flaggedSteps(p.Setup)}
		switch err := validProfile(p, bases); {
		case err != nil && !slices.ContainsFunc(bases, func(b Base) bool { return b.ID == p.Base }):
			sc.Status, sc.Problem = "unusable", "base"
		case err != nil:
			sc.Status, sc.Problem, sc.Detail = "unusable", "invalid", err.Error()
		}
		i := slices.IndexFunc(u.Containers, func(c ContainerProfile) bool {
			return c.From != nil && c.From.ID == p.ID && sameFolder(c.From.Repo, root)
		})
		if i >= 0 && sc.Status != "unusable" {
			yours := u.Containers[i]
			sc.Yours, sc.Status = yours.ID, "imported"
			if !sameContainer(yours, p) {
				if changed.IsZero() {
					changed = lastChanged(ctx, root, path)
				}
				sc.Status, sc.Changed, sc.By = "differs", changed, "you"
				if yours.From.Sum != suggestionSum(p) {
					sc.By = "project"
				}
			}
		}
		out.Containers = append(out.Containers, sc)
	}
	return out, nil
}

// canonProfile is what a profile builds and shares: names, descriptions
// and where it came from don't count, and defaults are spelled out.
func canonProfile(p ContainerProfile) ContainerProfile {
	p = tidyProfile(p)
	p.ID, p.Label, p.Description, p.From = "", "", "", nil
	if p.Brain == "" {
		p.Brain = BrainSandboxed
	}
	if p.MCP == "" {
		p.MCP = MCPNone
	}
	if p.Connectors == "" {
		p.Connectors = ConnectorsNone
	}
	return p
}

func sameContainer(a, b ContainerProfile) bool {
	return reflect.DeepEqual(canonProfile(a), canonProfile(b))
}

// suggestionSum fingerprints what a suggestion builds and shares.
func suggestionSum(p ContainerProfile) string {
	b, _ := json.Marshal(canonProfile(p))
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// reaches is what of the user's a container shares beyond itself. Local
// paths don't count: they keep files in the container, not out of it.
func reaches(p ContainerProfile) []string {
	out := []string{}
	if p.Brain == BrainShared {
		out = append(out, "brain")
	}
	if p.MCP == MCPShared {
		out = append(out, "mcp")
	}
	if p.Connectors == ConnectorsShared {
		out = append(out, "connectors")
	}
	if p.Secrets == SecretsShared {
		out = append(out, "secrets")
	}
	if len(p.Folders) > 0 {
		out = append(out, "folders")
	}
	return out
}

// flaggedSteps are the setup steps UNCLI's risk rules flag.
func flaggedSteps(steps []string) []StepRisks {
	out := []StepRisks{}
	for _, st := range steps {
		whys, ok := session.SetupStepRisks(st)
		if !ok {
			whys = []string{"unreadable"}
		}
		if len(whys) > 0 {
			out = append(out, StepRisks{Step: st, Why: whys})
		}
	}
	return out
}

// lastChanged is when a repository's file last changed: its last commit,
// when git has it as committed, else when the file was last written.
func lastChanged(ctx context.Context, root, path string) time.Time {
	var mod time.Time
	if fi, err := os.Stat(path); err == nil {
		mod = fi.ModTime()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return mod
	}
	rel = filepath.ToSlash(rel)
	git := func(args ...string) (string, error) {
		out, err := local.Run(ctx, core.Command{Path: "git", Args: append([]string{"-C", root}, args...)})
		return strings.TrimSpace(string(out)), err
	}
	if dirty, err := git("status", "--porcelain", "--", rel); err != nil || dirty != "" {
		return mod // not in git, or changed since its last commit
	}
	at, err := git("log", "-1", "--format=%cI", "--", rel)
	if err != nil || at == "" {
		return mod
	}
	t, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return mod
	}
	return t
}

// ImportContainer saves a container the user imported (or synced) from a
// repository's suggestion: p as they left it in the editor, which
// remembers the repository and the suggestion's id, imported now.
func (s *Service) ImportContainer(ctx context.Context, p ContainerProfile, repo, suggestion string) error {
	return s.ImportContainerFrom(ctx, p, repo, suggestion, "")
}

// ImportContainerFrom is ImportContainer from a paired device (decision
// 0033): the container remembers the device, and this PC shows a
// notification, so a container can't change here unseen.
func (s *Service) ImportContainerFrom(ctx context.Context, p ContainerProfile, repo, suggestion, device string) error {
	if s.embedded {
		return errEmbedded
	}
	sug, err := s.SuggestedContainers(ctx, repo)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(sug.Containers, func(c SuggestedContainer) bool { return c.Profile.ID == suggestion })
	if sug.Repo == "" || !sameFolder(sug.Repo, repo) || i < 0 {
		return errors.New("that project doesn't suggest that container any more")
	}
	if sug.Containers[i].Status == "unusable" {
		return errors.New("UNCLI can't build that container")
	}
	syncing := sug.Containers[i].Yours != ""
	p.From = &ContainerSource{Repo: sug.Repo, ID: suggestion, Imported: time.Now(), Sum: suggestionSum(sug.Containers[i].Profile), Device: device}
	if err := s.saveContainer(p); err != nil {
		return err
	}
	if device != "" {
		what := "imported"
		if syncing {
			what = "synced"
		}
		s.showNote(notify.Note{Key: "container-" + p.ID, Title: "Container " + what,
			Body: fmt.Sprintf("%s %s the container %q from %s. See Settings → Containers.", device, what, p.Label, sug.Name)})
	}
	return nil
}

// BuildContainerFrom is BuildContainer from a paired device (decision
// 0033), with a notification here, as for an import.
func (s *Service) BuildContainerFrom(id, device string) error {
	if err := s.BuildContainer(id); err != nil {
		return err
	}
	label := id
	if p, ok := s.profile(id); ok && p.Label != "" {
		label = p.Label
	}
	s.showNote(notify.Note{Key: "container-" + id, Title: "Building a container",
		Body: fmt.Sprintf("%s started building the container %q here.", device, label)})
	return nil
}
