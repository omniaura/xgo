package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

// CheckIgnore returns true if the given path is gitignored in dir.
// dir must be inside a git repository.
// Non-existent paths are never considered gitignored.
func CheckIgnore(dir, path string) (bool, error) {
	cmd := exec.Command("git", "check-ignore", "-q", path)
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		if _, statErr := os.Stat(filepath.Join(dir, path)); os.IsNotExist(statErr) {
			return false, nil
		}
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}
