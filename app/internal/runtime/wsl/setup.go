package wsl

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

// User is the unprivileged account the CLI runs as in every distro; it has
// no sudo and su isn't setuid, so it can't become root.
const User = "uncli"

// CLIPath is where the CLI's Linux build lives in a distro.
const CLIPath = "/usr/local/bin/claude"

// WorkspaceRoot is where Windows folders are mounted.
const WorkspaceRoot = "/workspace"

// wslConf locks a distro down (decision 0011): no Windows drives, no
// Windows programs, the CLI's user by default. interop=false alone leaves
// the WSLInterop handler registered, so a Windows program copied into the
// distro still tries to start; the boot command unregisters it.
const wslConf = `[automount]
enabled = false
mountFsTab = false

[interop]
enabled = false
appendWindowsPath = false

[user]
default = uncli

[boot]
command = "echo 0 > /proc/sys/fs/binfmt_misc/WSLInterop"
`

// basePackages are what every distro needs: the CLI's runtime libraries
// (the musl build links libstdc++), bash (the CLI's shell tool refuses to
// run without it: "No suitable shell found", so every command fails), git,
// and script for the sign-in's pseudo-terminal.
var basePackages = []string{"libgcc", "libstdc++", "bash", "git", "util-linux-misc", "ca-certificates"}

var packageRE = regexp.MustCompile(`^[a-z0-9][a-z0-9.+_-]{0,63}$`)

// setupScript is run once by root in a new distro.
func setupScript(packages []string) (string, error) {
	all := append([]string(nil), basePackages...)
	for _, p := range packages {
		if !packageRE.MatchString(p) {
			return "", fmt.Errorf("%q isn't a package name", p)
		}
		all = append(all, p)
	}
	var b strings.Builder
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "apk add --no-cache %s >/dev/null\n", strings.Join(all, " "))
	fmt.Fprintf(&b, "id -u %[1]s >/dev/null 2>&1 || adduser -D -h /home/%[1]s -s /bin/bash %[1]s\n", User)
	fmt.Fprintf(&b, "install -d -o root -g root -m 0755 %s\n", WorkspaceRoot)
	fmt.Fprintf(&b, "cat > /etc/wsl.conf <<'UNCLI_EOF'\n%sUNCLI_EOF\n", wslConf)
	return b.String(), nil
}

// relock runs after a container's own setup steps, which run as root and
// could change the lockdown: wsl.conf is written again (WSL applies it at
// the next start, so the running distro's check wouldn't notice) and its
// checksum printed for the build to compare.
var relock = `set -e
rm -f /etc/wsl.conf
cat > /etc/wsl.conf <<'UNCLI_EOF'
` + wslConf + `UNCLI_EOF
chown root:root /etc/wsl.conf
chmod 0644 /etc/wsl.conf
sha256sum /etc/wsl.conf
`

// becomesRoot prints "root" when the CLI's user can make itself root
// without a password, which would let a session undo the lockdown.
const becomesRoot = `if sudo -n true >/dev/null 2>&1 || doas -n true >/dev/null 2>&1; then echo root; else echo user; fi
`

func wslConfSum() string {
	sum := sha256.Sum256([]byte(wslConf))
	return hex.EncodeToString(sum[:])
}

// installCLI streams the binary from stdin into place.
const installCLI = `set -e
cat > ` + CLIPath + `.new
chmod 0755 ` + CLIPath + `.new
mv ` + CLIPath + `.new ` + CLIPath

// checkLockdown prints what a locked-down distro must show, one fact a
// line: the user, the interop handler, and what /mnt holds.
const checkLockdown = `id -un
head -1 /proc/sys/fs/binfmt_misc/WSLInterop 2>/dev/null || echo absent
grep -cE ' (9p|drvfs|virtiofs) ' /proc/mounts | head -1
`

// lockedDown checks checkLockdown's output: the CLI's user, interop off,
// and nothing from Windows mounted but the read-only GPU drivers.
func lockedDown(out string, driverMounts int) error {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 3 {
		return fmt.Errorf("unexpected lockdown check output %q", out)
	}
	if strings.TrimSpace(lines[0]) != User {
		return fmt.Errorf("runs as %q, not %s", strings.TrimSpace(lines[0]), User)
	}
	if s := strings.TrimSpace(lines[1]); s != "disabled" && s != "absent" {
		return fmt.Errorf("Windows interop is %s", s)
	}
	if n := strings.TrimSpace(lines[2]); n != fmt.Sprint(driverMounts) {
		return fmt.Errorf("%s Windows mounts, expected %d", n, driverMounts)
	}
	return nil
}

// SetupVersion fingerprints how UNCLI sets a distro up (its base packages,
// the lockdown, how the CLI goes in), so a container built by an UNCLI
// that set things up differently can be told to rebuild.
func SetupVersion() string {
	sum := sha256.Sum256([]byte(strings.Join(basePackages, " ") + "\x00" + wslConf + "\x00" + installCLI + "\x00" + checkLockdown + "\x00" + User))
	return hex.EncodeToString(sum[:8])
}

// withWSLConf copies a root filesystem tarball (gzip) to dst with UNCLI's
// /etc/wsl.conf added, so the distro starts locked down the first time.
func withWSLConf(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	zr, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(out)
	tw := tar.NewWriter(zw)
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			out.Close()
			return err
		}
		if path.Clean(strings.TrimPrefix(h.Name, "./")) == "etc/wsl.conf" {
			continue // UNCLI's replaces it
		}
		if err := tw.WriteHeader(h); err != nil {
			out.Close()
			return err
		}
		if _, err := io.Copy(tw, tr); err != nil {
			out.Close()
			return err
		}
	}
	err = tw.WriteHeader(&tar.Header{Name: "./etc/wsl.conf", Mode: 0o644, Size: int64(len(wslConf)), Typeflag: tar.TypeReg, ModTime: time.Now()})
	if err == nil {
		_, err = io.WriteString(tw, wslConf)
	}
	for _, c := range []io.Closer{tw, zw, out} {
		if cerr := c.Close(); err == nil {
			err = cerr
		}
	}
	return err
}

// ValidPackage reports whether a name can be an Alpine package.
func ValidPackage(name string) bool { return packageRE.MatchString(name) }
