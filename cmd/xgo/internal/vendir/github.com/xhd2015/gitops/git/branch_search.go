package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/xhd2015/xgo/support/cmd"
)

func SearchBranchesContainingRef(dir string, ref string) ([]string, error) {
	refs, err := SearchRefsContainingRef(dir, ref)
	if err != nil {
		return nil, err
	}
	return TrimRefsAsBranches(refs), nil
}

// git branch -l --all --sort=-committerdate --format='%(refname)' --contains xxx
// refs/heads/dev-master
// refs/remotes/origin/dev-master
// refs/remotes/origin/fix-my-tun
//
// Prefer passing a resolved commit id when the local refs/heads/<branch> may be missing
// (caller / server responsibility). On failure with exit status 129, attach git stderr
// (e.g. "error: malformed object name …") so APIs do not only show the bare exit code.
func SearchRefsContainingRef(dir string, ref string) ([]string, error) {
	if ref == "" {
		return nil, fmt.Errorf("requires ref")
	}
	var stderr bytes.Buffer
	// --exact-match
	output, err := cmd.Dir(dir).Stderr(&stderr).Output("git", "branch", "-l", "--all", "--sort=-committerdate", "--format=%(refname)", "--contains", ref)
	if err != nil {
		return nil, wrapGitBranchContainsErr(ref, err, stderr.String())
	}
	return splitLinesFilterEmpty(output), nil
}

func wrapGitBranchContainsErr(ref string, err error, stderr string) error {
	msg := strings.TrimSpace(stderr)
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 129 && msg != "" {
		return fmt.Errorf("%w: %s", err, msg)
	}
	if msg != "" {
		// still attach stderr for other non-zero exits when present
		return fmt.Errorf("%w: %s", err, msg)
	}
	return err
}

func GetBranchesHoldingRef(dir string, ref string) ([]string, error) {
	if ref == "" {
		return nil, fmt.Errorf("requires ref")
	}

	// resolve possible refs
	refs, err := SearchRefsContainingRef(dir, ref)
	if err != nil {
		return nil, err
	}
	var possibleBranches []string
	for _, branch := range refs {
		ok, err := IsFirstParentReachable(dir, ref, branch)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		possibleBranches = append(possibleBranches, branch)
	}
	return TrimRefsAsBranches(possibleBranches), nil
}
