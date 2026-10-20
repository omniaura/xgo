package git

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/xhd2015/xgo/support/cmd"
)

// ListIgnoredDirs returns the slash-relative paths of directories under dir that
// git reports as ignored (via .gitignore / exclude rules), as given by:
//
//	git -C <dir> ls-files --others --ignored --exclude-standard --directory -z -- .
//
// Each returned entry is a directory path relative to dir, slash-joined, with no
// trailing slash and no "./" prefix (e.g. "build", "node_modules"). If dir is not
// inside a git work tree, an empty slice and nil error are returned (git-based
// skips are simply inapplicable). If git is not installed on PATH, an error is
// returned.
func ListIgnoredDirs(dir string) ([]string, error) {
	if err := checkGitInstalled(); err != nil {
		return nil, err
	}
	inside, err := IsInsideGit(dir)
	if err != nil {
		return nil, err
	}
	if !inside {
		return nil, nil
	}

	output, err := cmd.Dir(dir).Output("git", "ls-files", "--others", "--ignored", "--exclude-standard", "--directory", "-z", "--", ".")
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// git failed (e.g. not a work tree after all); treat as no ignored dirs.
			return nil, nil
		}
		return nil, err
	}

	var dirs []string
	for _, entry := range strings.Split(output, "\x00") {
		if entry == "" {
			continue
		}
		entry = filepath.ToSlash(entry)
		if !strings.HasSuffix(entry, "/") {
			// --directory lists ignored directories with a trailing slash; ignore
			// anything that does not look like a directory entry.
			continue
		}
		entry = strings.TrimSuffix(entry, "/")
		entry = strings.TrimPrefix(entry, "./")
		entry = filepath.ToSlash(filepath.Clean(entry))
		if entry == "" || entry == "." {
			continue
		}
		dirs = append(dirs, entry)
	}
	return dirs, nil
}
