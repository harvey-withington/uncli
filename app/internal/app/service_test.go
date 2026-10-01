package app

import "testing"

func TestLoginProblem(t *testing.T) {
	// 2.1.285 prints "Login successful." and exits 0 even for a bad code.
	out := "Paste code here if prompted > Login successful.\nInvalid code. Please make sure the full code was copied.\n"
	if got := loginProblem(out); got != "Invalid code. Please make sure the full code was copied." {
		t.Errorf("got %q", got)
	}
	if got := loginProblem("Paste code here if prompted > Login successful.\n"); got != "" {
		t.Errorf("got %q", got)
	}
}
