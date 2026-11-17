package update

import "strings"

// What Update now needs on this PC that can be checked without side effects
// (README, "Updates"; decision 0025): the install script must run, so
// Windows PowerShell must be in full language mode and let a local script
// run despite any Group Policy execution policy, which -ExecutionPolicy
// Bypass can't override. Whether a scheduled task or WMI may start it is
// only known by trying; the script falls back from one to the other.

// powershellProbe is what Blocked asks PowerShell: its language mode, then
// the execution policy in effect with the flags the install script runs with.
const powershellProbe = "$ExecutionContext.SessionState.LanguageMode; Get-ExecutionPolicy"

// probeProblem reads the probe's output: why the install script couldn't
// run, or "" when it can. A local script UNCLI wrote has no download mark,
// so RemoteSigned lets it run too.
func probeProblem(out string) string {
	var lines []string
	for _, l := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) < 2 {
		return "Windows PowerShell didn't answer as expected (" + strings.Join(lines, " ") + ")"
	}
	if mode := lines[0]; mode != "FullLanguage" {
		return "Windows PowerShell runs in " + mode + " mode on this PC (set by a policy such as AppLocker or WDAC), where UNCLI's install script can't run"
	}
	switch policy := lines[1]; policy {
	case "Bypass", "Unrestricted", "RemoteSigned":
		return ""
	default:
		return "Group Policy sets Windows PowerShell's execution policy to " + policy + " on this PC, so UNCLI's install script can't run"
	}
}
