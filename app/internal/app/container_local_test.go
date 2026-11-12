package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/harvey-withington/uncli/app/internal/runtime/wsl"
)

// A local path is a folder in the project, cleaned to how it's mounted.
func TestLocalPathCleans(t *testing.T) {
	for in, want := range map[string]string{
		"node_modules":              "node_modules",
		" apps/web/node_modules/ ":  "apps/web/node_modules",
		`apps\web\node_modules`:     "apps/web/node_modules",
		"./node_modules":            "node_modules",
		"apps//web/./.venv":         "apps/web/.venv",
		".gitignored-cache":         ".gitignored-cache",
		"packages/my lib/node_mods": "packages/my lib/node_mods",
	} {
		if got, err := localPath(in); err != nil || got != want {
			t.Errorf("localPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

// What would leave the project, isn't a Windows folder, or is git's own is
// refused, and the error names the entry and why.
func TestValidLocalRefuses(t *testing.T) {
	for entry, why := range map[string]string{
		"":                        "is empty",
		"   ":                     "is empty",
		"/node_modules":           "not a full path",
		`\node_modules`:           "not a full path",
		`C:\work\node_modules`:    "not a full path",
		"c:node_modules":          "not a full path",
		"../node_modules":         "can't go up",
		"apps/../../x":            "can't go up",
		".git":                    ".git",
		".GIT/hooks":              ".git",
		"sub/.git":                ".git",
		".":                       "the project itself",
		"./":                      "the project itself",
		"a|b":                     "character",
		"a\nb":                    "character",
		"cache.":                  "Windows can't keep",
		strings.Repeat("a", 201): "too long",
	} {
		err := validLocal("dev", []string{"node_modules", entry})
		if err == nil || !strings.Contains(err.Error(), why) || !strings.Contains(err.Error(), fmt.Sprintf("%q", entry)) {
			t.Errorf("%q: err = %v, want one naming it and %q", entry, err, why)
		}
	}
}

// Each path once, and none inside another, whatever its case or slashes.
func TestValidLocalDuplicatesAndNesting(t *testing.T) {
	for _, c := range []struct {
		paths []string
		why   string
	}{
		{[]string{"node_modules", "node_modules"}, `"node_modules" is listed twice`},
		{[]string{"node_modules", "./Node_Modules/"}, `"./Node_Modules/" is listed twice`},
		{[]string{"apps/web/node_modules", `apps\web\node_modules`}, "listed twice"},
		{[]string{"node_modules", "node_modules/.cache"}, `"node_modules/.cache" is inside "node_modules"`},
		{[]string{"apps/web/node_modules", "apps"}, `"apps" holds "apps/web/node_modules"`},
	} {
		if err := validLocal("dev", c.paths); err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%q: err = %v, want %q", c.paths, err, c.why)
		}
	}
	if err := validLocal("dev", []string{"node_modules", "apps/web/node_modules", "apps/web/node_modules_old", ".venv"}); err != nil {
		t.Errorf("valid paths refused: %v", err)
	}
}

// A profile with local paths loads and saves them cleaned; one with none
// fingerprints as it did before they existed, so no import shows as
// changed; and they don't reach outside the container.
func TestProfileLocalPaths(t *testing.T) {
	p := ContainerProfile{ID: "dev", Base: "alpine-3.24", Local: []string{` apps\web\node_modules `, "node_modules/"}}
	bases := []Base{{ID: "alpine-3.24"}}
	if err := validProfile(p, bases); err != nil {
		t.Fatal(err)
	}
	if got := tidyProfile(p).Local; !slices.Equal(got, []string{"apps/web/node_modules", "node_modules"}) {
		t.Errorf("tidied = %q", got)
	}
	if err := validProfile(ContainerProfile{ID: "dev", Base: "alpine-3.24", Local: []string{"../x"}}, bases); err == nil || !strings.Contains(err.Error(), `local path "../x"`) {
		t.Errorf("err = %v", err)
	}
	if tidyProfile(ContainerProfile{Local: []string{}}).Local != nil {
		t.Error("no local paths should save as none")
	}
	plain := ContainerProfile{ID: "dev", Base: "alpine-3.24", Packages: []string{"nodejs"}}
	if sum := suggestionSum(plain); sum != suggestionSum(ContainerProfile{ID: "dev", Base: "alpine-3.24", Packages: []string{"nodejs"}, Local: []string{}}) {
		t.Error("an empty list changed the fingerprint")
	}
	if suggestionSum(plain) == suggestionSum(ContainerProfile{ID: "dev", Base: "alpine-3.24", Packages: []string{"nodejs"}, Local: []string{"node_modules"}}) {
		t.Error("local paths should count as a change")
	}
	if r := reaches(p); len(r) != 0 {
		t.Errorf("reaches = %q", r)
	}
	if got := localPaths([]string{"node_modules", "NODE_MODULES", "../x", "web/node_modules"}); !slices.Equal(got, []string{"node_modules", "web/node_modules"}) {
		t.Errorf("localPaths = %q", got)
	}
}

// The session is told which paths are the container's own.
func TestBriefNamesLocalPaths(t *testing.T) {
	p := ContainerProfile{ID: "dev", Label: "Dev"}
	if b := containerBrief(p, nil); strings.Contains(b, "container's own") {
		t.Errorf("brief without local paths: %q", b)
	}
	b := containerBrief(p, []wsl.Mount{
		{Name: "node_modules", Target: "/workspace/a-1/node_modules", Kind: wsl.KindLocal},
		{Name: "node_modules", Target: "/workspace/b-2/node_modules", Kind: wsl.KindLocal},
		{Name: "apps/web/node_modules", Target: "/workspace/a-1/apps/web/node_modules", Kind: wsl.KindLocal},
	})
	if !strings.Contains(b, "container's own") || !strings.Contains(b, `"local"`) || !strings.Contains(b, "node_modules, apps/web/node_modules.") || !strings.Contains(b, "isn't on Windows") {
		t.Errorf("brief = %q", b)
	}
}
