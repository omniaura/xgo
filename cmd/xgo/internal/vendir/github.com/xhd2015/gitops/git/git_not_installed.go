package git

import (
	"errors"
	"fmt"
	"os/exec"
)

// ErrGitNotInstalled is returned when the git executable cannot be found on PATH.
var ErrGitNotInstalled = errors.New("git is not installed or not on PATH")

// checkGitInstalled returns ErrGitNotInstalled if the git executable is not
// resolvable on PATH. Functions in this package that shell out to git should
// call this first (or rely on IsInsideGit) so a missing git is reported as an
// explicit error rather than silently treated as "not a repo".
func checkGitInstalled() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("%w: %v", ErrGitNotInstalled, err)
	}
	return nil
}
