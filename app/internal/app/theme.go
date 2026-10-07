package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ThemeFile is the user's theme.yaml in the config folder: values for the
// public --uncli-* tokens, by scheme, without the prefix (decision 0010).
// The frontend checks the names and values; here it is only read.
type ThemeFile struct {
	Path  string            `json:"path"`
	Found bool              `json:"found"`
	Light map[string]string `json:"light" yaml:"light"`
	Dark  map[string]string `json:"dark" yaml:"dark"`
}

// maxTheme is the biggest theme.yaml read.
const maxTheme = 64 << 10

// Theme reads theme.yaml; a missing file is not an error.
func (s *Service) Theme() (ThemeFile, error) {
	t := ThemeFile{Path: filepath.Join(s.Paths.Config, "theme.yaml")}
	st, err := os.Stat(t.Path)
	if errors.Is(err, os.ErrNotExist) {
		return t, nil
	}
	if err != nil {
		return t, err
	}
	if st.Size() > maxTheme {
		return t, fmt.Errorf("%s is bigger than %d KB", t.Path, maxTheme>>10)
	}
	b, err := os.ReadFile(t.Path)
	if err != nil {
		return t, err
	}
	if err := yaml.Unmarshal(b, &t); err != nil {
		return t, fmt.Errorf("%s: %w", t.Path, err)
	}
	t.Found = true
	return t, nil
}
