package store

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSafeList(t *testing.T) {
	s := searchStore(t)
	proj := filepath.Join(t.TempDir(), "Repo")
	other := t.TempDir()
	s.SetSafeEntry(SafeEntry{Kind: KindCommand, Words: "npm  run test", Verdict: Safe, Folder: proj})
	s.SetSafeEntry(SafeEntry{Kind: KindCommand, Words: "npm run test", Verdict: Unsafe, Folder: proj}) // same class: verdict changes
	s.SetSafeEntry(SafeEntry{Kind: KindCommand, Words: "git push", Flags: "--force", Verdict: Blocked})
	s.SetSafeEntry(SafeEntry{Kind: KindTool, Words: "mcp__notes__touch_note", Verdict: Safe, Folder: other})
	got, _ := s.SafeList(proj + string(filepath.Separator))
	if len(got) != 2 || got[0] != (SafeEntry{Kind: KindCommand, Words: "npm run test", Verdict: Unsafe, Folder: proj}) || got[1].Folder != "" {
		t.Fatalf("list = %+v", got)
	}
	if runtime.GOOS == "windows" {
		if l, _ := s.SafeList(strings.ToUpper(proj)); len(l) != 2 {
			t.Error("Windows paths match whatever their case")
		}
	}
	if err := s.SetSafeEntry(SafeEntry{Kind: KindCommand, Words: "x", Verdict: "sometimes"}); err == nil {
		t.Error("unknown verdicts are refused")
	}
	if err := s.SetSafeEntry(SafeEntry{Kind: KindCommand, Words: " ", Verdict: Safe}); err == nil {
		t.Error("an entry needs a class")
	}

	// Moving to all projects: it applies elsewhere too, and replaces an
	// entry already there for the same class.
	s.SetSafeEntry(SafeEntry{Kind: KindCommand, Words: "npm run test", Verdict: Safe})
	if err := s.MoveSafeEntry(SafeEntry{Kind: KindCommand, Words: "npm run test", Verdict: Unsafe, Folder: proj}, ""); err != nil {
		t.Fatal(err)
	}
	l, _ := s.SafeList(other)
	found := false
	for _, e := range l {
		if e.Words == "npm run test" {
			found = e.Folder == "" && e.Verdict == Unsafe
		}
	}
	if !found || len(l) != 3 {
		t.Errorf("after moving = %+v", l)
	}
	s.DeleteSafeEntry(SafeEntry{Kind: KindTool, Words: "mcp__notes__touch_note", Folder: other})
	if l, _ := s.SafeList(other); len(l) != 2 {
		t.Errorf("after delete = %+v", l)
	}
	if NormFlags("-r  -f -r") != "-f -r" {
		t.Error("flags are sorted and unique")
	}
}

// A database from before the safe list: its project rules, tool classes
// and session modes move over.
func TestMigrationToSafeList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, _ := sql.Open("sqlite", "file:"+path)
	db.Exec(`CREATE TABLE tool_rules (workdir TEXT NOT NULL, tool TEXT NOT NULL, prefix TEXT NOT NULL DEFAULT '',
		action TEXT NOT NULL, created_at INTEGER, PRIMARY KEY (workdir, tool, prefix))`)
	db.Exec(`INSERT INTO tool_rules VALUES ('c:\repo','Bash','git:read','allow',1),('c:\repo','PowerShell','npm publish','deny',1),
		('c:\repo','Bash','','allow',1),('c:\repo','Write','','ask',1)`)
	db.Exec(`CREATE TABLE tool_classes (tool TEXT PRIMARY KEY, write INTEGER NOT NULL, open_world INTEGER NOT NULL, updated_at INTEGER)`)
	db.Exec(`INSERT INTO tool_classes VALUES ('mcp__notes__read_note',0,0,1),('mcp__notes__touch_note',1,0,1)`)
	db.Exec(`CREATE TABLE sessions (id TEXT PRIMARY KEY, title TEXT, adapter TEXT NOT NULL, runtime TEXT NOT NULL,
		runtime_ref TEXT, profile_id TEXT NOT NULL, workdir TEXT NOT NULL, provider_sid TEXT, cli_version TEXT,
		model TEXT NOT NULL, modifiers TEXT NOT NULL DEFAULT '[]', sort_order REAL, archived INTEGER DEFAULT 0,
		created_at INTEGER, updated_at INTEGER)`)
	db.Exec(`INSERT INTO sessions (id, adapter, runtime, profile_id, workdir, model) VALUES ('a','claude','local','code','/','m'),('b','claude','local','code','/','m')`)
	db.Exec(`ALTER TABLE sessions ADD COLUMN mode TEXT`)
	db.Exec(`UPDATE sessions SET mode='readonly' WHERE id='a'`)
	db.Exec(`UPDATE sessions SET mode='full' WHERE id='b'`)
	db.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	l, _ := s.SafeList(`c:\repo`)
	want := map[string]string{"command git:read": Safe, "command npm publish": Blocked, "tool Write": Unsafe,
		"tool mcp__notes__read_note": Safe, "tool mcp__notes__touch_note": Unsafe}
	if len(l) != len(want) {
		t.Fatalf("migrated = %+v", l)
	}
	for _, e := range l {
		if want[e.Kind+" "+e.Words] != e.Verdict {
			t.Errorf("%+v", e)
		}
	}
	if a, _ := s.GetSession("a"); a.Mode != "always" {
		t.Errorf("readonly became %q", a.Mode)
	}
	if b, _ := s.GetSession("b"); b.Mode != "never" {
		t.Errorf("full became %q", b.Mode)
	}
}

func TestJudgements(t *testing.T) {
	s := searchStore(t)
	if _, ok := s.Judgement("cmd:frob"); ok {
		t.Fatal("nothing judged yet")
	}
	s.SetJudgement("cmd:frob", Judgement{Level: "risky", Risk: "deletes", Note: "Deletes the cache.", Model: "haiku"})
	s.SetJudgement("cmd:frob", Judgement{Level: "routine", Note: "Builds the docs.", Model: "haiku"})
	if j, ok := s.Judgement("cmd:frob"); !ok || j != (Judgement{Level: "routine", Note: "Builds the docs.", Model: "haiku"}) {
		t.Errorf("judgement = %+v, %v", j, ok)
	}
}
