package snapshot

import (
	"os/exec"
	"sync"
)

var (
	gitOnce sync.Once
	gitPath string
	gitErr  error
)

// lookGit finds git on PATH once.
func lookGit() (string, error) {
	gitOnce.Do(func() { gitPath, gitErr = exec.LookPath("git") })
	return gitPath, gitErr
}
