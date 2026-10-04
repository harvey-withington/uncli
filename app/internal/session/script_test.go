package session

import (
	"os"
	"strings"
	"testing"
)

// How the reader splits real command lines (shapes from Harvey's sessions)
// into the commands they run: "prog args" per part, ">" for files written,
// "|" for a part reading a pipe, "·" between parts. Structure (keywords,
// assignments, bare strings) is left out.
func TestReadScript(t *testing.T) {
	show := func(cmd, dialect string) (string, bool) {
		parts, _, ok := readScript(cmd, dialect, "/repo")
		var out []string
		for _, p := range parts {
			if p.kind != "" {
				continue
			}
			s := strings.ReplaceAll(strings.Join(p.words, " "), os.TempDir(), "$env:TEMP")
			if p.piped {
				s = "|" + s
			}
			for _, w := range p.writes {
				s += " >" + w
			}
			if p.inline {
				s += " (inline)"
			}
			out = append(out, s)
		}
		return strings.Join(out, " · "), ok
	}
	ps := map[string]string{
		`npm test 2>&1 | Select-Object -Last 25; "=== LINT ==="; npm run lint 2>&1 | Select-Object -Last 20`:                                                    "npm test · |select-object -Last 25 · npm run lint · |select-object -Last 20",
		`npm test 2>&1 | Select-String -Pattern "FAIL|✓ src" | ForEach-Object { $_.Line } | Select-Object -First 50`:                                            "npm test · |select-string -Pattern FAIL|✓ src · |foreach-object · |select-object -First 50",
		`$d = Join-Path $env:TEMP 'ugrid'; if (Test-Path $d) { "exists"; Get-ChildItem $d } else { New-Item -ItemType Directory $d | Out-Null; "created" }; $d`: "join-path $env:TEMP ugrid · test-path $d · get-childitem $d · new-item -ItemType Directory $d · |out-null",
		`& node_modules/.bin/vitest.cmd run --root $d 2>&1 | Out-String -Width 220`:                                                                             "vitest run --root $d · |out-string -Width 220",
		`git log --oneline -5 > log.txt`:          "git log --oneline -5 >log.txt",
		`npm --prefix "S:\My Projects\core" test`: "npm test",
	}
	for cmd, want := range ps {
		if got, ok := show(cmd, DialectPowerShell); !ok || got != want {
			t.Errorf("PowerShell %s\n got  %s (%v)\n want %s", cmd, got, ok, want)
		}
	}
	bash := map[string]string{
		`cd "S:/repo/packages/core/src" && grep -n "_toCell\|_colModel" sel/Sel.ts | head -60 && echo ---- && sed -n 30,140p grid/Core.ts`: `cd S:/repo/packages/core/src · grep -n _toCell\|_colModel sel/Sel.ts · |head -60 · echo ---- · sed -n 30,140p grid/Core.ts`,
		`B=https://x.example; echo latest=$(curl -s $B/latest)`:                                                                            "echo latest= · curl -s https://x.example/latest",
		`for f in a b c; do echo "$f"; done`:                                                                                               `echo $f`,
		"cat > notes.md <<'EOF'\n# Title\nrm -rf / ; not a command\nEOF\ngit add notes.md":                                                 "cat >notes.md · git add notes.md",
		"git commit -q -m \"$(cat <<'EOF'\nfeat: x; y\nEOF\n)\"":                                                                           "git commit -q -m $(cat <<'EOF'\nfeat: x; y\nEOF\n) · cat",
		"python - <<'EOF'\nimport shutil\nEOF":                                                                                             "python - (inline)",
		`timeout 100 npm test`:                                                                                                             "npm test",
		`find . -name "*.tmp" -exec rm {} \;`:                                                                                              `find . -name *.tmp -exec rm {} \;`,
		`ls # just looking`:                                                                                                                "ls",
	}
	for cmd, want := range bash {
		if got, ok := show(cmd, DialectBash); !ok || got != want {
			t.Errorf("bash %q\n got  %q (%v)\n want %q", cmd, got, ok, want)
		}
	}
	for _, bad := range []string{`echo "unclosed`, `echo $(missing`, `{ ls`} {
		if _, _, ok := readScript(bad, DialectBash, ""); ok {
			t.Errorf("%q can't be read", bad)
		}
	}
	// cd moves the shell for the parts after it, and the folder it ends in
	// is reported.
	parts, cwd, _ := readScript(`cd app/frontend && sed -n 1,50p ../scripts/e2e.mjs`, DialectBash, "/repo")
	if len(parts) != 2 || strings.ReplaceAll(parts[1].cwd, `\`, "/") != "/repo/app/frontend" || strings.ReplaceAll(cwd, `\`, "/") != "/repo/app/frontend" {
		t.Errorf("cwd = %q, parts = %+v", cwd, parts)
	}
}
