package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"uncli/config"
	"uncli/internal/runtime/wsl"
	"uncli/internal/session"
)

// Settings → Containers edits the user's containers.yaml: a container saved
// there adds one or, under a built-in's id, replaces it. The file stays the
// one place containers are defined, and can still be edited by hand (the
// editor's saves don't keep its comments).

const userContainersHeader = `# Your containers for UNCLI (docs/decisions/0011). Settings → Containers
# writes this file; you can edit it by hand too, but saving from Settings
# rewrites it without comments. A container here with the id of a built-in
# one (sandbox, isolated) replaces it.
`

func (s *Service) userContainersPath() string {
	return filepath.Join(s.Paths.Config, "containers.yaml")
}

func (s *Service) readUserContainers() (containersFile, error) {
	var u containersFile
	b, err := os.ReadFile(s.userContainersPath())
	if errors.Is(err, os.ErrNotExist) {
		return u, nil
	}
	if err != nil {
		return u, err
	}
	if err := yaml.Unmarshal(b, &u); err != nil {
		return u, fmt.Errorf("%s: %w", s.userContainersPath(), err)
	}
	return u, nil
}

func (s *Service) writeUserContainers(u containersFile) error {
	b, err := yaml.Marshal(u)
	if err != nil {
		return err
	}
	tmp := s.userContainersPath() + ".tmp"
	if err := os.WriteFile(tmp, append([]byte(userContainersHeader), b...), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.userContainersPath())
}

func builtinContainers() (containersFile, error) {
	defaults, _ := fs.Sub(config.Defaults, "defaults")
	return loadContainers(defaults, "")
}

// SaveContainer adds a container, or replaces one by its id, in the user's
// containers.yaml.
func (s *Service) SaveContainer(p ContainerProfile) error {
	p.Label, p.Description = strings.TrimSpace(p.Label), strings.TrimSpace(p.Description)
	if p.Label == "" {
		return errors.New("give the container a name")
	}
	pkgs := []string{}
	for _, x := range p.Packages {
		if x = strings.TrimSpace(x); x != "" && !slices.Contains(pkgs, x) {
			pkgs = append(pkgs, x)
		}
	}
	p.Packages = pkgs
	steps := []string{}
	for _, x := range p.Setup {
		if x = strings.TrimSpace(x); x != "" {
			steps = append(steps, x)
		}
	}
	p.Setup = steps
	s.reloadContainers()
	s.containers.mu.Lock()
	bases := append([]Base{}, s.containers.cfg.Bases...)
	s.containers.mu.Unlock()
	if err := validProfile(p, bases); err != nil {
		return err
	}
	u, err := s.readUserContainers()
	if err != nil {
		return err
	}
	u.Containers = mergeID(u.Containers, []ContainerProfile{p}, func(c ContainerProfile) string { return c.ID })
	if err := s.writeUserContainers(u); err != nil {
		return err
	}
	s.reloadContainers()
	s.emitContainers()
	return nil
}

// RemoveContainerConfig takes the user's version of a container out of
// containers.yaml: a built-in one goes back to how it ships (its built
// container stays), one of their own is deleted with its built container.
func (s *Service) RemoveContainerConfig(ctx context.Context, id string) error {
	builtin, err := builtinContainers()
	if err != nil {
		return err
	}
	isBuiltin := slices.ContainsFunc(builtin.Containers, func(c ContainerProfile) bool { return c.ID == id })
	u, err := s.readUserContainers()
	if err != nil {
		return err
	}
	if !isBuiltin {
		st, _ := checkWSL(ctx)
		if slices.Contains(st.Distros, wsl.DistroName(id)) {
			if s.Sessions != nil {
				s.Sessions.StopIn(session.RuntimeWSL, id)
			}
			s.closeRuntime(id)
			if err := wsl.Remove(ctx, wsl.DistroName(id)); err != nil {
				return err
			}
		}
		_ = os.RemoveAll(filepath.Join(s.Paths.Config, "wsl", id))
		_ = os.Remove(s.buildRecordPath(id)) // its conversations stay, in wsl-data
	}
	u.Containers = slices.DeleteFunc(u.Containers, func(c ContainerProfile) bool { return c.ID == id })
	if err := s.writeUserContainers(u); err != nil {
		return err
	}
	s.refreshContainers()
	return nil
}

// editState is whether a container ships with UNCLI and whether the user's
// containers.yaml has their own version of it.
func (s *Service) editState() (builtin, edited map[string]bool) {
	builtin, edited = map[string]bool{}, map[string]bool{}
	if b, err := builtinContainers(); err == nil {
		for _, c := range b.Containers {
			builtin[c.ID] = true
		}
	}
	if u, err := s.readUserContainers(); err == nil {
		for _, c := range u.Containers {
			edited[c.ID] = true
		}
	}
	return builtin, edited
}
