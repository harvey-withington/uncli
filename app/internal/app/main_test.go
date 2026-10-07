package app

import (
	"os"
	"testing"
)

// Tests never touch the user's credential store.
func TestMain(m *testing.M) {
	keyring = &memSecrets{}
	os.Exit(m.Run())
}
