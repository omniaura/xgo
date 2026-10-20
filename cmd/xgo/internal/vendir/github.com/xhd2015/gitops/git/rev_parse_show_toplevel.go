package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func ShowToplevel(dir string) (string, error) {
	localVars, err := gitLocalEnvVars()
	if err != nil {
		return "", err
	}

	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	cmd.Env = withoutEnvVars(os.Environ(), localVars)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func gitLocalEnvVars() (map[string]struct{}, error) {
	out, err := exec.Command("git", "rev-parse", "--local-env-vars").Output()
	if err != nil {
		return nil, fmt.Errorf("git rev-parse --local-env-vars: %w", err)
	}
	vars := make(map[string]struct{})
	for _, line := range strings.Split(string(out), "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			vars[name] = struct{}{}
		}
	}
	return vars, nil
}

func withoutEnvVars(env []string, names map[string]struct{}) []string {
	filtered := env[:0]
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		if _, drop := names[name]; drop {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
