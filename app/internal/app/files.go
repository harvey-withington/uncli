package app

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"uncli/internal/ide"
)

// Opening the files a page lists: sessions whose type links to an IDE
// (Code) open them in the user's editor at the line; the others open
// documents with the default app. Any of them can be shown in its folder.

func validEditor(choice, command string) error {
	switch choice {
	case ide.Auto:
		return nil
	case ide.Custom:
		if strings.TrimSpace(command) == "" {
			return errors.New("enter the command that opens your editor, such as subl {file}:{line}")
		}
		_, err := ide.Args(command, "f", 1, "")
		return err
	}
	for _, p := range ide.Presets {
		if p.ID == choice {
			return nil
		}
	}
	return fmt.Errorf("unknown editor %q", choice)
}

// Editors lists the editors UNCLI knows and which are installed.
func (s *Service) Editors() []ide.Editor { return ide.Detect() }

// OpenFile opens a file a session's page lists: in the editor at line for
// a session that links to an IDE, else with the default app (documents only).
func (s *Service) OpenFile(sessionID, path string, line int) error {
	v, err := s.Sessions.View(sessionID)
	if err != nil {
		return err
	}
	if p, ok := s.Profiles.Profile(v.ProfileID); ok && p.IDELinks {
		prefs := s.Preferences()
		if _, template, ok := ide.Resolve(prefs.Editor, prefs.EditorCommand, ide.Detect()); ok {
			return ide.OpenInEditor(template, path, line, v.Workdir)
		}
		if !ide.CanOpenDefault(path) {
			return errors.New("no editor found: install VS Code, Cursor or Antigravity, or set your own in Settings → Editor")
		}
	}
	return ide.OpenDefault(path)
}

// OpenPath follows a link in an answer to a file or folder: a file opens as
// OpenFile does (never run: programs aren't opened, only shown), a folder
// is shown.
func (s *Service) OpenPath(sessionID, path string, line int) error {
	st, err := os.Stat(path)
	if err != nil {
		return errors.New("that file doesn't exist")
	}
	if st.IsDir() {
		return ide.Reveal(path)
	}
	return s.OpenFile(sessionID, path, line)
}

// RevealFile shows a file in its folder.
func (s *Service) RevealFile(path string) error { return ide.Reveal(path) }
