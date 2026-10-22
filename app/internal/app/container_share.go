package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"uncli/internal/adapter/claude"
	"uncli/internal/core"
	"uncli/internal/runtime/local"
	"uncli/internal/runtime/wsl"
)

// prepareContainer is a container's Prepare (decision 0011), run before
// each CLI start with the session folder (host) mounted at cli. The CLI's
// conversations live on Windows, in data (UNCLI's own folder for this
// container), so a rebuild keeps them and sessions can resume. Profiles
// that share the user's setup get it on top.
//
// A container sharing the user's connectors gets the provider's full
// account sign-in: account is the folder that holds it (a whole ~/.claude:
// the CLI saves its credentials by writing a new file and renaming it, so
// only a mounted folder keeps them), "" when there's none. Any other
// container must not keep it, so it's unmounted there.
//
// It returns the environment for the CLI it prepares: where the secrets
// are, when they're shared. Last, it writes the list of mounts
// (wsl.ManifestPath) for the session to read.
func prepareContainer(profile func() ContainerProfile, data string, account func() string) func(ctx context.Context, r *wsl.Runtime, host, cli string) (map[string]string, error) {
	shellChecked := false
	return func(ctx context.Context, r *wsl.Runtime, host, cli string) (map[string]string, error) {
		p := profile() // as configured now: a change to what it shares needs no rebuild
		// Containers built before UNCLI installed bash can't run a single
		// command (the CLI's shell tool needs it): say so instead.
		if !shellChecked {
			if !r.Has(ctx, "bash") {
				return nil, ErrContainerOutdated
			}
			shellChecked = true
		}
		dotClaude := path.Join("/home", wsl.User, ".claude")
		acct := ""
		if p.Connectors == ConnectorsShared {
			acct = account()
		}
		if acct != "" {
			if err := r.MountAt(ctx, acct, dotClaude); err != nil {
				return nil, err
			}
		} else if r.Mounted(ctx, dotClaude) {
			if err := r.Unmount(ctx, dotClaude); err != nil {
				return nil, fmt.Errorf("could not take the account sign-in out of the container: %w", err)
			}
		}
		projects := filepath.Join(data, "projects")
		if err := os.MkdirAll(projects, 0o755); err != nil {
			return nil, err
		}
		if err := r.MountAt(ctx, projects, path.Join("/home", wsl.User, ".claude", "projects")); err != nil {
			return nil, err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		configDir := os.Getenv("CLAUDE_CONFIG_DIR")
		claudeJSON := filepath.Join(home, ".claude.json")
		if configDir == "" {
			configDir = filepath.Join(home, ".claude")
		} else {
			claudeJSON = filepath.Join(configDir, ".claude.json")
		}
		in := "/home/" + wsl.User
		if p.Brain == BrainShared {
			if err := shareBrain(ctx, r, configDir, host, cli, in, acct == ""); err != nil {
				return nil, err
			}
		}
		if p.MCP == MCPShared {
			b, _ := os.ReadFile(claudeJSON)
			servers, _ := claude.ContainerMCP(b, host)
			existing, err := r.ReadFile(ctx, path.Join(in, ".claude.json"))
			if err != nil {
				return nil, fmt.Errorf("reading the container's settings: %w", err)
			}
			out, err := claude.WithMCP(existing, servers)
			if err != nil {
				return nil, err
			}
			if err := r.WriteFile(ctx, path.Join(in, ".claude.json"), out); err != nil {
				return nil, fmt.Errorf("sharing MCP servers: %w", err)
			}
		}
		env, err := shareFolders(ctx, r, p)
		if err != nil {
			return nil, err
		}
		if err := r.WriteManifest(ctx); err != nil {
			return nil, err
		}
		return env, nil
	}
}

// shareFolders mounts the user's secrets folder (read-only) when the
// profile shares it, and the profile's own folders; a missing folder is
// left out (the settings say so). Those it mounted before that the profile
// no longer lists are taken out. It returns where the secrets are, for
// the CLI's environment.
func shareFolders(ctx context.Context, r *wsl.Runtime, p ContainerProfile) (map[string]string, error) {
	want, env := foldersFor(p)
	for _, m := range r.Mounts() {
		_, keep := want[m.Target]
		if !keep && (m.Kind == wsl.KindCustom || m.Name == "secrets") {
			if err := r.Unmount(ctx, m.Target); err != nil {
				return nil, fmt.Errorf("could not take %s out of the container: %w", m.Source, err)
			}
		}
	}
	for _, m := range want {
		if err := r.MountFolder(ctx, m); err != nil {
			return nil, err
		}
	}
	return env, nil
}

// foldersFor is what shareFolders mounts for a profile, by target, and the
// environment that says where the secrets are.
func foldersFor(p ContainerProfile) (map[string]wsl.Mount, map[string]string) {
	env := map[string]string{}
	exists := func(p string) bool { st, err := os.Stat(p); return err == nil && st.IsDir() }
	want := map[string]wsl.Mount{}
	if p.Secrets == SecretsShared {
		if src, err := expandSource(secretsSource); err == nil && exists(src) {
			at := path.Join("/home", wsl.User, ".secrets")
			want[at] = wsl.Mount{Name: "secrets", Source: src, Target: at, Access: wsl.ReadOnly, Kind: wsl.KindShare}
			env[SecretsEnv] = at
		}
	}
	for _, f := range p.Folders {
		src, err := expandSource(f.Source)
		if err != nil || folderProblem(src) != "" || !exists(src) {
			continue
		}
		access := wsl.ReadOnly
		if f.Access == wsl.ReadWrite {
			access = wsl.ReadWrite
		}
		at := path.Join("/home", wsl.User, f.Name)
		want[at] = wsl.Mount{Name: f.Name, Source: src, Target: at, Access: access, Kind: wsl.KindCustom}
	}
	return want, env
}

// containerBrief is what a session in a container is told about it, so the
// model knows where to look rather than guessing at paths: what's mounted
// and from where, and where the secrets are when they're shared.
func containerBrief(p ContainerProfile, mounts []wsl.Mount) string {
	label := p.Label
	if label == "" {
		label = p.ID
	}
	b := fmt.Sprintf("This session runs in a Linux container (UNCLI's %q) that sees only the folders mounted in it. "+
		"Each one, the Windows folder it comes from and whether it's read-only is listed in %s ($%s).", label, wsl.ManifestPath, wsl.MountsEnv)
	for _, m := range mounts {
		if m.Name == "secrets" {
			b += fmt.Sprintf(" The user's secrets are in %s ($%s, read-only), one file per service named for it (such as bitbucket-token). "+
				"Read a secret from there when a command needs one; never print it, or copy it anywhere else.", m.Target, SecretsEnv)
		}
	}
	return b
}

// shareBrain mounts the user's memories, skills, agents and commands, and
// copies in their CLAUDE.md and git name and email. The project's memory
// folder is created on Windows if it doesn't exist yet, so what Claude
// remembers in the container is kept there.
func shareBrain(ctx context.Context, r *wsl.Runtime, configDir, host, cli, home string, synced bool) error {
	for i, sh := range claude.SharedBrain(configDir, host, cli, home, synced) {
		if sh.File {
			if b, err := os.ReadFile(sh.Host); err == nil {
				if err := r.WriteFile(ctx, sh.Path, b); err != nil {
					return err
				}
			}
			continue
		}
		if i == 0 {
			_ = os.MkdirAll(sh.Host, 0o755) // the project's memory
		}
		if st, err := os.Stat(sh.Host); err != nil || !st.IsDir() {
			continue
		}
		name := strings.TrimPrefix(sh.Path, path.Join(home, ".claude")+"/")
		if err := r.MountFolder(ctx, wsl.Mount{Name: name, Source: sh.Host, Target: sh.Path, Access: wsl.ReadWrite, Kind: wsl.KindShare}); err != nil {
			return err
		}
	}
	if cfg := gitIdentity(ctx); cfg != "" {
		return r.WriteFile(ctx, path.Join(home, ".gitconfig"), []byte(cfg))
	}
	return nil
}

// gitIdentity is the user's git name and email as a .gitconfig, so commits
// made in a container are theirs; nothing else of their git setup is
// shared (no credentials, no hooks).
func gitIdentity(ctx context.Context) string {
	get := func(key string) string {
		out, err := local.Run(ctx, core.Command{Path: "git", Args: []string{"config", "--global", "--get", key}})
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	name, email := get("user.name"), get("user.email")
	if name == "" && email == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("[user]\n")
	if name != "" {
		fmt.Fprintf(&b, "\tname = %s\n", quoteGit(name))
	}
	if email != "" {
		fmt.Fprintf(&b, "\temail = %s\n", quoteGit(email))
	}
	return b.String()
}

func quoteGit(s string) string {
	s = strings.NewReplacer(`\`, `\`, `"`, `\"`, "\n", " ").Replace(s)
	return `"` + s + `"`
}

// ErrContainerOutdated: the container lacks what the CLI needs, because an
// earlier UNCLI built it.
var ErrContainerOutdated = errors.New("this container was built by an earlier UNCLI and is missing bash, which the CLI needs to run commands. Rebuild it in Settings → Containers (conversations are kept)")
