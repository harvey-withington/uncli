package update

import (
	"strings"
	"testing"
)

// The probe's answers: what lets the install script run, and why not.
func TestProbeProblem(t *testing.T) {
	for out, want := range map[string]string{
		"FullLanguage\r\nBypass\r\n":    "",
		"FullLanguage\nRemoteSigned\n":  "",
		"FullLanguage\nUnrestricted\n":  "",
		"FullLanguage\nAllSigned\n":     "execution policy to AllSigned",
		"FullLanguage\nRestricted\n":    "execution policy to Restricted",
		"ConstrainedLanguage\nBypass\n": "runs in ConstrainedLanguage mode",
		"":                              "didn't answer as expected",
	} {
		got := probeProblem(out)
		if (want == "") != (got == "") || !strings.Contains(got, want) {
			t.Errorf("%q: got %q, want %q", out, got, want)
		}
	}
}

// This PC's PowerShell answers the probe (it's what Blocked runs).
func TestProbeRunsHere(t *testing.T) {
	if testing.Short() {
		t.Skip("runs PowerShell")
	}
	t.Logf("Blocked() = %q", Blocked(t.Context()))
}
