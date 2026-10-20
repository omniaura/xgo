package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RunGitOptions configures optional env and timeout for argv git runs.
type RunGitOptions struct {
	Env     map[string]string
	Timeout time.Duration
	// OkExits lists process exit codes treated as success (stdout still returned).
	OkExits []int
}

// RunGit runs `git <args...>` with cmd.Dir=dir and returns trimmed stdout.
// Arguments are passed as argv — never through a shell — so values like
// "HEAD|id" cannot trigger shell metacharacter evaluation.
func RunGit(dir string, args ...string) (string, error) {
	return RunGitOpts(dir, nil, args...)
}

// RunGitAllowExit is like RunGit but treats the listed process exit codes as
// success (stdout still returned). Useful for `git grep`, which exits 1 when
// there are no matches.
func RunGitAllowExit(dir string, okExits []int, args ...string) (string, error) {
	return RunGitOpts(dir, &RunGitOptions{OkExits: okExits}, args...)
}

// RunGitOpts runs git with optional env and timeout. Args are never passed
// through a shell.
func RunGitOpts(dir string, opts *RunGitOptions, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("requires git args")
	}
	var env map[string]string
	var timeout time.Duration
	var okExits []int
	if opts != nil {
		env = opts.Env
		timeout = opts.Timeout
		okExits = opts.OkExits
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = mergeProcessEnv(env)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSuffix(stdout.String(), "\n")
	if err == nil {
		return out, nil
	}
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("git %v: timeout after %s", args, timeout)
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		code := exitErr.ExitCode()
		for _, c := range okExits {
			if code == c {
				return out, nil
			}
		}
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return out, fmt.Errorf("git %v: exit %d: %s", args, code, msg)
		}
		return out, fmt.Errorf("git %v: exit %d", args, code)
	}
	return out, fmt.Errorf("git %v: %w", args, err)
}

// RunGitErr is like RunGitOpts but discards stdout (convenient for fetch/clone).
func RunGitErr(dir string, opts *RunGitOptions, args ...string) error {
	_, err := RunGitOpts(dir, opts, args...)
	return err
}

func mergeProcessEnv(extra map[string]string) []string {
	merged := append([]string(nil), os.Environ()...)
	merged = append(merged, "GIT_TERMINAL_PROMPT=0")
	for k, v := range extra {
		merged = append(merged, k+"="+v)
	}
	return merged
}
