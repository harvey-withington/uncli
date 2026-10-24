package app

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// Folder listing: a remote client picks a session's folder on the server by
// browsing it, since the OS folder dialog only opens on the server's own
// screen (decision 0021). It lists folders only, never files.

// FolderListing is one folder's subfolders. Path "" is the top: the home
// folder and, on Windows, each drive.
type FolderListing struct {
	Path    string        `json:"path"`
	Parent  string        `json:"parent"` // "" at a drive's root (back to the top)
	Folders []FolderEntry `json:"folders"`
}

type FolderEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ListFolder lists a folder's subfolders, hidden ones left out, sorted by
// name.
func (a *API) ListFolder(path string) (FolderListing, error) { return ListFolder(path) }

func ListFolder(path string) (FolderListing, error) {
	if path == "" {
		return topFolders(), nil
	}
	if !filepath.IsAbs(path) {
		return FolderListing{}, errors.New("give a full path")
	}
	path = filepath.Clean(path)
	entries, err := os.ReadDir(path)
	if err != nil {
		return FolderListing{}, err
	}
	out := FolderListing{Path: path, Folders: []FolderEntry{}}
	if parent := filepath.Dir(path); parent != path {
		out.Parent = parent
	}
	for _, e := range entries {
		if !isFolder(path, e) || hiddenFolder(path, e) {
			continue
		}
		out.Folders = append(out.Folders, FolderEntry{Name: e.Name(), Path: filepath.Join(path, e.Name())})
	}
	slices.SortFunc(out.Folders, func(x, y FolderEntry) int {
		return strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
	})
	return out, nil
}

// isFolder follows links (a junction to a folder counts).
func isFolder(dir string, e os.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&os.ModeSymlink == 0 {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, e.Name()))
	return err == nil && st.IsDir()
}

func topFolders() FolderListing {
	out := FolderListing{Folders: []FolderEntry{}}
	if home, err := os.UserHomeDir(); err == nil {
		out.Folders = append(out.Folders, FolderEntry{Name: home, Path: home})
	}
	if runtime.GOOS == "windows" {
		for d := 'A'; d <= 'Z'; d++ {
			root := string(d) + `:\`
			if _, err := os.Stat(root); err == nil {
				out.Folders = append(out.Folders, FolderEntry{Name: root, Path: root})
			}
		}
	} else {
		out.Folders = append(out.Folders, FolderEntry{Name: "/", Path: "/"})
	}
	return out
}
