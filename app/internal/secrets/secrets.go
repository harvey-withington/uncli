// Package secrets keeps UNCLI's secrets (the decision model's API key, the
// CLI's OAuth token for containers) in the operating system's credential
// store, never in UNCLI's database or files. On Windows that is the
// Credential Manager, as generic credentials named "uncli:<name>".
package secrets

import "errors"

// ErrUnsupported means this platform has no credential store UNCLI uses yet.
var ErrUnsupported = errors.New("no credential store on this platform")

// prefix keeps UNCLI's entries apart from everything else in the store.
const prefix = "uncli:"

// maxSecret is the biggest secret kept (the Windows limit is 2560 bytes).
const maxSecret = 2048

// Get returns the secret, or "" when there is none.
func Get(name string) (string, error) { return get(prefix + name) }

// Set saves the secret; an empty value deletes it.
func Set(name, value string) error {
	if value == "" {
		return Delete(name)
	}
	if len(value) > maxSecret {
		return errors.New("that secret is too long to keep")
	}
	return set(prefix+name, value)
}

// Delete removes the secret; a missing one is not an error.
func Delete(name string) error { return del(prefix + name) }
