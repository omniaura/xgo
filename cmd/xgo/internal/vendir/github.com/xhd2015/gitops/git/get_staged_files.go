package git

import (
	"os/exec"
	"strings"
)

func GetStagedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "-C", dir, "diff", "--cached", "--name-only", "--diff-filter=ACMRT", "--")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
