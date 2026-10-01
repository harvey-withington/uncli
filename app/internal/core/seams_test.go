package core

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The seams that keep later phases additive: core imports nothing from
// UNCLI, and only the bridge imports Wails.
func TestSeams(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", "frontend", "testdata", "build":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if strings.HasPrefix(rel, "internal/core/") && strings.HasPrefix(p, "uncli/") {
				t.Errorf("%s: core must not import %s", rel, p)
			}
			if strings.HasPrefix(p, "github.com/wailsapp/") && !strings.HasPrefix(rel, "internal/bridge/") {
				t.Errorf("%s: only internal/bridge may import Wails (%s)", rel, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
