package uncli

import "golang.org/x/sys/windows"

// serviceAccounts are Windows' own accounts that services run as: none is
// a person's, and LocalSystem can do anything on the machine.
var serviceAccounts = map[string]string{
	"S-1-5-18": "LocalSystem",
	"S-1-5-19": "LocalService",
	"S-1-5-20": "NetworkService",
}

// serviceAccount names the Windows service account this process runs as,
// or "" for a person's account.
func serviceAccount() string {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return ""
	}
	return serviceAccounts[u.User.Sid.String()]
}
