package worktree

import (
	"os/exec"
	"strings"

	"github.com/xhd2015/xgo/support/cmd"
)

// IsClean reports whether the worktree has no tracked differences vs HEAD
// (historically diff-based; untracked files do not make it dirty).
func IsClean(dir string) (ok bool, err error) {
	return workTreeClean(dir, false)
}

// IndexClean reports whether the index has no staged differences vs HEAD.
func IndexClean(dir string) (ok bool, err error) {
	return workTreeClean(dir, true)
}

// IsDiffClean reports whether `git diff --quiet HEAD` would succeed
// (tracked-tree diff vs HEAD is empty; untracked may still be clean).
func IsDiffClean(dir string) (bool, error) {
	return workTreeClean(dir, false)
}

// IsPorcelainClean reports whether `git status --porcelain` output is empty
// (untracked files make the worktree dirty).
func IsPorcelainClean(dir string) (bool, error) {
	out, err := cmd.Dir(dir).Output("git", "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "", nil
}

func workTreeClean(dir string, index bool) (bool, error) {
	// 0: clean, 1: not clean
	args := []string{"diff", "--exit-code", "--quiet"}
	if index {
		args = append(args, "--cached")
	} else {
		// compare HEAD with working tree
		args = append(args, "HEAD")
	}

	// exit code: 0=clean, 1=not clean
	err := cmd.Dir(dir).Run("git", args...)
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
