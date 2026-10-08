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
func prepareContainer(profile func() ContainerProfile, data string, account func() string) func(ctx context.Context, r *wsl.Runtime, host, cli string) error {
	shellChecked := false
	return func(ctx context.Context, r *wsl.Runtime, host, cli string) error {
		p := profile() // as configured now: a change to what it shares needs no rebuild
		// Containers built before UNCLI installed bash can't run a single
		// command (the CLI's shell tool needs it): say so instead.
		if !shellChecked {
			if !r.Has(ctx, "bash") {
				return ErrContainerOutdated
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
				return err
			}
		} else if r.Mounted(ctx, dotClaude) {
			if err := r.Unmount(ctx, dotClaude); err != nil {
				return fmt.Errorf("could not take the account sign-in out of the container: %w", err)
			}
		}
		projects := filepath.Join(data, "projects")
		if err := os.MkdirAll(projects, 0o755); err != nil {
			return err
		}
		if err := r.MountAt(ctx, projects, path.Join("/home", wsl.User, ".claude", "projects")); err != nil {
			return err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return err
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
				return err
			}
		}
		if p.MCP == MCPShared {
			b, _ := os.ReadFile(claudeJSON)
			servers, _ := claude.ContainerMCP(b, host)
			existing, err := r.ReadFile(ctx, path.Join(in, ".claude.json"))
			if err != nil {
				return fmt.Errorf("reading the container's settings: %w", err)
			}
			out, err := claude.WithMCP(existing, servers)
			if err != nil {
				return err
			}
			if err := r.WriteFile(ctx, path.Join(in, ".claude.json"), out); err != nil {
				return fmt.Errorf("sharing MCP servers: %w", err)
			}
		}
		return nil
	}
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
		if err := r.MountAt(ctx, sh.Host, sh.Path); err != nil {
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
