// Package ide opens files where the user works on them: in their editor at
// a line (VS Code, Cursor, Antigravity, or a command of their own), in the
// system's default app for documents, or shown in their folder. Paths come
// from what a model wrote, so nothing here runs a file: an editor gets it
// as an argument, the default app only gets document and media types, and
// a batch-file launcher never gets an argument cmd.exe would read as syntax.
package ide

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Preset is a known editor: its command template uses {file}, {line} and
// {folder} (the session folder, so the editor reuses that window).
type Preset struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Command string `json:"command"`
}

// Presets are the editors UNCLI knows, in the order "auto" tries them.
var Presets = []Preset{
	{ID: "vscode", Label: "VS Code", Command: "code {folder} -g {file}:{line}"},
	{ID: "cursor", Label: "Cursor", Command: "cursor {folder} -g {file}:{line}"},
	{ID: "antigravity", Label: "Antigravity", Command: "antigravity {folder} -g {file}:{line}"},
}

// Editor choices besides a preset's id.
const (
	Auto   = "auto"   // the first preset installed
	Custom = "custom" // the user's own command template
)

// Editor is a preset as found on this computer.
type Editor struct {
	Preset
	Found bool `json:"found"` // its command is on PATH
}

// Detect says which presets are installed.
func Detect() []Editor {
	out := make([]Editor, 0, len(Presets))
	for _, p := range Presets {
		_, err := exec.LookPath(program(p.Command))
		out = append(out, Editor{Preset: p, Found: err == nil})
	}
	return out
}

// Resolve picks the command template for a choice: a preset id, Auto (the
// first one installed) or Custom (command). ok is false when there is none.
func Resolve(choice, command string, editors []Editor) (label, template string, ok bool) {
	switch choice {
	case Custom:
		command = strings.TrimSpace(command)
		return program(command), command, command != ""
	case "", Auto:
		for _, e := range editors {
			if e.Found {
				return e.Label, e.Command, true
			}
		}
		return "", "", false
	}
	for _, e := range editors {
		if e.ID == choice {
			return e.Label, e.Command, true
		}
	}
	return "", "", false
}

// Args fills a command template: words split at spaces (double quotes keep
// a word together), placeholders replaced. A line below 1 is 1.
func Args(template, file string, line int, folder string) ([]string, error) {
	words, err := split(template)
	if err != nil {
		return nil, err
	}
	if len(words) == 0 {
		return nil, errors.New("the editor command is empty")
	}
	if line < 1 {
		line = 1
	}
	r := strings.NewReplacer("{file}", file, "{line}", strconv.Itoa(line), "{folder}", folder)
	out := make([]string, 0, len(words))
	for _, w := range words {
		if w == "{folder}" && folder == "" {
			continue
		}
		out = append(out, r.Replace(w))
	}
	return out, nil
}

func split(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	quoted, inWord := false, false
	for _, r := range s {
		switch {
		case r == '"':
			quoted, inWord = !quoted, true
		case (r == ' ' || r == '\t') && !quoted:
			if inWord {
				out = append(out, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quoted {
		return nil, errors.New("the editor command has an unclosed quote")
	}
	if inWord {
		out = append(out, cur.String())
	}
	return out, nil
}

func program(template string) string {
	if w, err := split(template); err == nil && len(w) > 0 {
		return w[0]
	}
	return ""
}

// batchUnsafe are characters cmd.exe treats as syntax even in an argument,
// which a batch-file launcher (code.cmd) would pass through to it.
const batchUnsafe = "&|<>^%!\"\r\n"

// OpenInEditor opens file at line with a command template.
func OpenInEditor(template, file string, line int, folder string) error {
	if err := existingFile(file); err != nil {
		return err
	}
	args, err := Args(template, file, line, folder)
	if err != nil {
		return err
	}
	bin, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("couldn't find %s: is the editor installed and on PATH?", args[0])
	}
	if ext := strings.ToLower(filepath.Ext(bin)); ext == ".cmd" || ext == ".bat" {
		for _, a := range args[1:] {
			if strings.ContainsAny(a, batchUnsafe) {
				return fmt.Errorf("%s can't be passed to %s safely; use Show in folder", filepath.Base(file), filepath.Base(bin))
			}
		}
	}
	cmd := exec.Command(bin, args[1:]...)
	hide(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("couldn't start %s: %w", filepath.Base(bin), err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// documentTypes are the files OpenDefault hands to the system's default
// app: documents, media and plain data. Anything that could run (programs,
// scripts, shortcuts, installers) is only ever shown in its folder.
var documentTypes = map[string]bool{
	".html": true, ".htm": true, ".svg": true, ".md": true, ".markdown": true, ".txt": true, ".pdf": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true, ".ico": true,
	".csv": true, ".tsv": true, ".json": true, ".xml": true, ".yaml": true, ".yml": true, ".log": true,
	".docx": true, ".xlsx": true, ".pptx": true, ".odt": true, ".ods": true, ".odp": true, ".rtf": true,
	".mp3": true, ".wav": true, ".mp4": true, ".webm": true, ".mov": true,
}

// CanOpenDefault says whether OpenDefault would open a file.
func CanOpenDefault(path string) bool { return documentTypes[strings.ToLower(filepath.Ext(path))] }

// OpenDefault opens a document with the system's default app.
func OpenDefault(path string) error {
	if !CanOpenDefault(path) {
		return fmt.Errorf("UNCLI doesn't open %s files itself; use Show in folder", strings.ToLower(filepath.Ext(path)))
	}
	if err := existingFile(path); err != nil {
		return err
	}
	return openDefault(path)
}

// Reveal shows a file in its folder (selected, where the system can), or
// a folder itself.
func Reveal(path string) error {
	if _, err := os.Stat(path); err != nil {
		if dir := filepath.Dir(path); dir != path {
			if st, derr := os.Stat(dir); derr == nil && st.IsDir() {
				return reveal(dir, true) // the file is gone: its folder
			}
		}
		return errors.New("that file no longer exists")
	}
	return reveal(path, false)
}

func existingFile(p string) error {
	st, err := os.Stat(p)
	if err != nil {
		return errors.New("that file no longer exists")
	}
	if st.IsDir() {
		return errors.New("that's a folder, not a file")
	}
	return nil
}
