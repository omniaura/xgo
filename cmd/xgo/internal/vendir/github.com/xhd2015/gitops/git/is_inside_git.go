package git

import (
	"errors"
	"io"
	"os/exec"
	"strings"
)

// IsInsideGit returns true if dir is inside a git work tree.
func IsInsideGit(dir string) (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir

	// necessary because stderr would show:
	//    fatal: not a git repository (or any of the parent directories): .git
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}
