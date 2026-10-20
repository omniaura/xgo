package git

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/xhd2015/xgo/support/cmd"
)

// IsSubmodule reports whether path (a path relative to dir, or absolute) is
// tracked by the dir repository as a submodule gitlink — i.e. its index entry
// has mode 160000. This distinguishes a real submodule of dir from a nested
// separate git repo that merely carries its own .git.
//
// It runs:
//
//	git -C <dir> ls-files --stage -z -- <path>
//
// and inspects the leading mode field of each returned entry. If dir is not a
// git repo, IsSubmodule returns false and a nil error. If path is not tracked
// at all, it returns false and a nil error. If git is not installed on PATH,
// an error is returned.
func IsSubmodule(dir, path string) (bool, error) {
	if err := checkGitInstalled(); err != nil {
		return false, err
	}
	inside, err := IsInsideGit(dir)
	if err != nil {
		return false, err
	}
	if !inside {
		return false, nil
	}

	output, err := cmd.Dir(dir).Output("git", "ls-files", "--stage", "-z", "--", path)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	if output == "" {
		return false, nil
	}
	// Each entry is: "<mode> <hash> <stage>\t<path>".
	for _, entry := range strings.Split(output, "\x00") {
		if entry == "" {
			continue
		}
		// mode is the first whitespace-delimited token.
		fields := strings.Fields(entry)
		if len(fields) < 1 {
			continue
		}
		if fields[0] == "160000" {
			return true, nil
		}
	}
	return false, nil
}
