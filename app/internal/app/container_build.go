package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"uncli/internal/runtime/wsl"
)

// A container is built from its profile's packages and base, a CLI
// version and UNCLI's own way of setting distros up. A record of those is
// kept when it's built, so Settings can say when the container no longer
// matches and a rebuild would update it. What a profile shares (brain,
// mcp) applies at each start and needs no rebuild.

// buildRecord is what a container was built from.
type buildRecord struct {
	Setup      string   `json:"setup"` // wsl.SetupVersion()
	Base       string   `json:"base"`  // id and checksum of the root filesystem
	CLIVersion string   `json:"cliVersion"`
	Packages   []string `json:"packages"` // sorted
}

// What changed since a container was built (ContainerInfo.Changes).
const (
	ChangedPackages = "packages"
	ChangedBase     = "base"
	ChangedCLI      = "cli"
	ChangedSetup    = "setup"
	ChangedUnknown  = "unknown" // built before UNCLI kept a record
)

func recordFor(p ContainerProfile, b Base, cliVersion string) buildRecord {
	pkgs := append([]string{}, p.Packages...)
	slices.Sort(pkgs)
	return buildRecord{Setup: wsl.SetupVersion(), Base: b.ID + "@" + strings.ToLower(b.SHA256), CLIVersion: cliVersion, Packages: pkgs}
}

func (s *Service) buildRecordPath(id string) string {
	return filepath.Join(s.Paths.Config, "wsl-data", id, "build.json")
}

func (s *Service) saveBuildRecord(id string, r buildRecord) error {
	path := s.buildRecordPath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// changesSince lists what differs between how a built container was built
// and how it would be built now.
func (s *Service) changesSince(id string, now buildRecord) []string {
	b, err := os.ReadFile(s.buildRecordPath(id))
	if err != nil {
		return []string{ChangedUnknown}
	}
	var was buildRecord
	if json.Unmarshal(b, &was) != nil {
		return []string{ChangedUnknown}
	}
	var out []string
	if !slices.Equal(was.Packages, now.Packages) {
		out = append(out, ChangedPackages)
	}
	if was.Base != now.Base {
		out = append(out, ChangedBase)
	}
	if was.CLIVersion != now.CLIVersion {
		out = append(out, ChangedCLI)
	}
	if was.Setup != now.Setup {
		out = append(out, ChangedSetup)
	}
	return out
}
