package ide

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestArgs(t *testing.T) {
	got, err := Args("code {folder} -g {file}:{line}", `C:\w\a b.go`, 12, `C:\w`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"code", `C:\w`, "-g", `C:\w\a b.go:12`}; !reflect.DeepEqual(got, want) {
		t.Errorf("args = %q, want %q", got, want)
	}
	// No line: line 1. No folder: the word is left out.
	got, _ = Args(`"C:\Program Files\Ed\ed.exe" {folder} --line {line} {file}`, "/x.txt", 0, "")
	if want := []string{`C:\Program Files\Ed\ed.exe`, "--line", "1", "/x.txt"}; !reflect.DeepEqual(got, want) {
		t.Errorf("args = %q, want %q", got, want)
	}
	if _, err := Args(`"unclosed {file}`, "f", 1, ""); err == nil {
		t.Error("an unclosed quote was accepted")
	}
	if _, err := Args("   ", "f", 1, ""); err == nil {
		t.Error("an empty command was accepted")
	}
}

func TestResolve(t *testing.T) {
	eds := []Editor{{Preset: Presets[0]}, {Preset: Presets[1], Found: true}, {Preset: Presets[2]}}
	if label, _, ok := Resolve(Auto, "", eds); !ok || label != "Cursor" {
		t.Errorf("auto = %q %v", label, ok)
	}
	if label, tpl, ok := Resolve("vscode", "", eds); !ok || label != "VS Code" || !strings.HasPrefix(tpl, "code ") {
		t.Errorf("vscode = %q %q %v", label, tpl, ok)
	}
	if label, tpl, ok := Resolve(Custom, " subl {file}:{line} ", eds); !ok || label != "subl" || tpl != "subl {file}:{line}" {
		t.Errorf("custom = %q %q %v", label, tpl, ok)
	}
	if _, _, ok := Resolve(Auto, "", []Editor{{Preset: Presets[0]}}); ok {
		t.Error("auto found an editor that isn't installed")
	}
	if _, _, ok := Resolve(Custom, "", eds); ok {
		t.Error("an empty custom command resolved")
	}
}

func TestOpenDefaultOnlyOpensDocuments(t *testing.T) {
	for _, p := range []string{"report.html", "notes.MD", "chart.svg", "data.csv", "deck.pptx"} {
		if !CanOpenDefault(p) {
			t.Errorf("%s should open", p)
		}
	}
	for _, p := range []string{"setup.exe", "run.bat", "x.cmd", "a.ps1", "s.js", "s.vbs", "link.lnk", "page.hta", "x.msi", "Makefile"} {
		if CanOpenDefault(p) {
			t.Errorf("%s must never be opened", p)
		}
		if err := OpenDefault(p); err == nil {
			t.Errorf("OpenDefault(%s) didn't refuse", p)
		}
	}
}

// A batch-file launcher (code.cmd) passes its arguments through cmd.exe,
// so a file name with cmd syntax in it is refused, and a plain one gets
// through.
func TestBatchLauncherRefusesCmdSyntax(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("batch files are a Windows thing")
	}
	bin := t.TempDir()
	out := filepath.Join(bin, "got.txt")
	if err := os.WriteFile(filepath.Join(bin, "fakeed.cmd"), []byte("@echo %~1> \""+out+"\"\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	work := t.TempDir()
	bad := filepath.Join(work, "a&calc.txt")
	good := filepath.Join(work, "plain.txt")
	for _, p := range []string{bad, good} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := OpenInEditor("fakeed {file}", bad, 1, ""); err == nil || !strings.Contains(err.Error(), "safely") {
		t.Errorf("unsafe name: err = %v", err)
	}
	if err := OpenInEditor("fakeed {file}", good, 1, ""); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(out); err == nil && strings.TrimSpace(string(b)) != "" {
			if got := strings.TrimSpace(string(b)); got != good {
				t.Errorf("editor got %q, want %q", got, good)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("the editor never ran")
}

func TestOpenInEditorNeedsAFile(t *testing.T) {
	if err := OpenInEditor("code {file}", filepath.Join(t.TempDir(), "missing.txt"), 1, ""); err == nil {
		t.Error("a missing file was opened")
	}
	if err := OpenInEditor("code {file}", t.TempDir(), 1, ""); err == nil {
		t.Error("a folder was opened as a file")
	}
}
