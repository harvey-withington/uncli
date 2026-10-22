package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A container sees only the folders UNCLI mounts in it (decision 0011).
// Beyond the session folder and what its shares give, a container profile
// can list folders of its own (Folders), and share the user's secrets
// folder (Secrets). Both apply from a session's next start, like the other
// shares; decision 0017.

// ContainerFolder is a Windows folder a container profile mounts at
// /home/uncli/<name>.
type ContainerFolder struct {
	// Source may start with %USERPROFILE%, %APPDATA% or %LOCALAPPDATA%, so
	// a profile names no user and can be shared.
	Source string `yaml:"source" json:"source"`
	Name   string `yaml:"name" json:"name"`
	Access string `yaml:"access,omitempty" json:"access"` // ro (the default) or rw
}

const (
	SecretsShared = "shared"
	SecretsNone   = "none"
	// secretsSource is the user's secrets folder, one file per service.
	secretsSource = `%USERPROFILE%\.secrets`
	// SecretsEnv names where the secrets are in the container.
	SecretsEnv = "UNCLI_SECRETS_DIR"
)

var folderNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,40}$`)

// folderVars are the variables a folder's source may use.
var folderVars = []string{"USERPROFILE", "APPDATA", "LOCALAPPDATA"}

var varRE = regexp.MustCompile(`%([^%]*)%`)

// expandSource is a folder's source as a Windows path. Only folderVars are
// expanded, and the result must be a full path on a drive.
func expandSource(src string) (string, error) {
	src = strings.TrimSpace(src)
	var bad error
	out := varRE.ReplaceAllStringFunc(src, func(m string) string {
		name := strings.ToUpper(strings.Trim(m, "%"))
		for _, v := range folderVars {
			if name == v {
				if val := os.Getenv(v); val != "" {
					return val
				}
				bad = fmt.Errorf("%%%s%% isn't set", v)
				return m
			}
		}
		bad = fmt.Errorf("%s: only %%USERPROFILE%%, %%APPDATA%% and %%LOCALAPPDATA%% can be used", m)
		return m
	})
	if bad != nil {
		return "", bad
	}
	if !regexp.MustCompile(`^[A-Za-z]:[\\/]`).MatchString(out) {
		return "", fmt.Errorf("%q isn't a full path on a drive", src)
	}
	return filepath.Clean(out), nil
}

// guards are the folders a container's folder may not be or contain:
// mounting one would undo the container's isolation (the user profile, the
// system) or hand over sign-ins. Sealed ones may not even be entered: the
// CLI's config folder and UNCLI's own data hold credentials (what's safe
// in there, such as skills, comes with the shares).
func guards() (closed, sealed []string) {
	for _, v := range []string{"USERPROFILE", "APPDATA", "LOCALAPPDATA", "SystemRoot", "ProgramFiles", "ProgramFiles(x86)", "ProgramData"} {
		if p := os.Getenv(v); p != "" {
			closed = append(closed, p)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		closed = append(closed, filepath.Join(home, "AppData"))
		sealed = append(sealed, filepath.Join(home, ".claude"))
	}
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		sealed = append(sealed, d)
	}
	if paths, err := DefaultPaths(); err == nil {
		sealed = append(sealed, paths.Config, paths.Cache)
	}
	return append(closed, sealed...), sealed
}

// folderProblem says why a folder can't be mounted, or "" if it can.
func folderProblem(src string) string {
	p := strings.ToLower(filepath.Clean(src))
	if len(p) <= 3 { // C:		return "a whole drive can't be mounted"
	}
	under := func(child, parent string) bool { return strings.HasPrefix(child, strings.TrimSuffix(parent, `\`)+`\`) }
	closed, sealed := guards()
	for _, g := range closed {
		g = strings.ToLower(filepath.Clean(g))
		if p == g || under(g, p) {
			return fmt.Sprintf("%s is or holds your user profile, the system or your sign-ins, so it can't be mounted", src)
		}
	}
	for _, g := range sealed {
		if under(p, strings.ToLower(filepath.Clean(g))) {
			return fmt.Sprintf("%s is inside a folder that holds your sign-ins; use the container's shares for what's in there", src)
		}
	}
	return ""
}

// validFolders checks a profile's folders (validProfile).
func validFolders(id string, folders []ContainerFolder) error {
	seen := map[string]bool{}
	for i, f := range folders {
		if !folderNameRE.MatchString(f.Name) {
			return fmt.Errorf("container %q: folder %d's name %q: use lower-case letters, digits, dots, dashes and underscores", id, i+1, f.Name)
		}
		if seen[f.Name] {
			return fmt.Errorf("container %q: two folders are named %q", id, f.Name)
		}
		seen[f.Name] = true
		if f.Access != "" && f.Access != "ro" && f.Access != "rw" {
			return fmt.Errorf("container %q: folder %q's access is ro or rw, not %q", id, f.Name, f.Access)
		}
		src, err := expandSource(f.Source)
		if err != nil {
			return fmt.Errorf("container %q: folder %q: %w", id, f.Name, err)
		}
		if why := folderProblem(src); why != "" {
			return fmt.Errorf("container %q: folder %q: %s", id, f.Name, why)
		}
	}
	return nil
}

// missingFolders lists what a profile would mount but can't find, for the
// settings to say so; a session starts without them.
func missingFolders(p ContainerProfile) []string {
	out := []string{}
	exists := func(p string) bool { st, err := os.Stat(p); return err == nil && st.IsDir() }
	if p.Secrets == SecretsShared {
		if src, err := expandSource(secretsSource); err == nil && !exists(src) {
			out = append(out, src)
		}
	}
	for _, f := range p.Folders {
		if src, err := expandSource(f.Source); err == nil && !exists(src) {
			out = append(out, src)
		}
	}
	return out
}
