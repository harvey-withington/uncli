package app

import (
	"path/filepath"
	"testing"
)

// A key saved in the database by an earlier version moves into the
// credential store the first time it's read, and leaves the database.
func TestDecisionKeyMovesToTheCredentialStore(t *testing.T) {
	keyring = &memSecrets{}
	dir := t.TempDir()
	svc, err := New(Paths{Config: filepath.Join(dir, "c"), Cache: filepath.Join(dir, "k"), Scratch: filepath.Join(dir, "s")}, nopEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if err := svc.Store.SetSetting(settingDecisionKey, "sk-old"); err != nil {
		t.Fatal(err)
	}
	if k := svc.decisionKey(); k != "sk-old" {
		t.Fatalf("key = %q", k)
	}
	if v, _ := svc.Store.Setting(settingDecisionKey); v != "" {
		t.Errorf("the database still holds the key: %q", v)
	}
	if v, _ := keyring.Get(secretDecisionKey); v != "sk-old" {
		t.Errorf("credential store = %q", v)
	}
	if err := svc.SetDecisionKey(""); err != nil || svc.HasDecisionKey() {
		t.Errorf("removing the key: %v", err)
	}
}
