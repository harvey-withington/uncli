package secrets

import (
	"fmt"
	"testing"
	"time"
)

// Round trip through the real Credential Manager, under a name no one else
// uses, removed afterwards.
func TestCredentialManager(t *testing.T) {
	name := fmt.Sprintf("test-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = Delete(name) })
	if v, err := Get(name); err != nil || v != "" {
		t.Fatalf("missing: %q, %v", v, err)
	}
	if err := Set(name, "sk-ünïcode-123"); err != nil {
		t.Fatal(err)
	}
	if v, err := Get(name); err != nil || v != "sk-ünïcode-123" {
		t.Fatalf("after set: %q, %v", v, err)
	}
	if err := Set(name, ""); err != nil {
		t.Fatal(err)
	}
	if v, _ := Get(name); v != "" {
		t.Fatalf("after clearing: %q", v)
	}
	if err := Delete(name); err != nil {
		t.Errorf("deleting a missing secret: %v", err)
	}
}
