package git

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// BranchRelation describes the relationship between two refs.
type BranchRelation int

const (
	BranchRelationSame           BranchRelation = iota // same commit
	BranchRelationAIsAncestorOfB                       // A is an ancestor of B → B is ahead
	BranchRelationBIsAncestorOfA                       // B is an ancestor of A → A is ahead
	BranchRelationDiverged                             // neither is ancestor of the other
)

// CompareBranchesResult holds the full comparison between two refs.
type CompareBranchesResult struct {
	Relation      BranchRelation
	CommitsAheadA int    // commits in refA not in refB (rev-list --count refB..refA)
	CommitsAheadB int    // commits in refB not in refA (rev-list --count refA..refB)
	MergeBase     string // populated only when Diverged
	DiffFileCount int    // populated only when Diverged
}

// CompareBranches determines the relationship between refA and refB.
func CompareBranches(dir, refA, refB string) (*CompareBranchesResult, error) {
	revA, err := revParseForCompare(dir, refA)
	if err != nil {
		return nil, err
	}
	revB, err := revParseForCompare(dir, refB)
	if err != nil {
		return nil, err
	}

	if revA == revB {
		return &CompareBranchesResult{
			Relation: BranchRelationSame,
		}, nil
	}

	aIsAncestorOfB, err := IsAncesorOf(dir, revA, revB)
	if err != nil {
		return nil, err
	}
	bIsAncestorOfA, err := IsAncesorOf(dir, revB, revA)
	if err != nil {
		return nil, err
	}

	result := &CompareBranchesResult{}

	if aIsAncestorOfB {
		result.Relation = BranchRelationAIsAncestorOfB
		count, err := revListCountForCompare(dir, revA, revB)
		if err != nil {
			return nil, err
		}
		result.CommitsAheadB = count
		return result, nil
	}

	if bIsAncestorOfA {
		result.Relation = BranchRelationBIsAncestorOfA
		count, err := revListCountForCompare(dir, revB, revA)
		if err != nil {
			return nil, err
		}
		result.CommitsAheadA = count
		return result, nil
	}

	// Diverged
	result.Relation = BranchRelationDiverged

	base, err := mergeBaseForCompare(dir, revA, revB)
	if err != nil {
		return nil, err
	}
	result.MergeBase = base

	fileCount, err := diffFileCountForCompare(dir, revA, revB)
	if err != nil {
		return nil, err
	}
	result.DiffFileCount = fileCount

	countA, err := revListCountForCompare(dir, revB, revA)
	if err != nil {
		return nil, err
	}
	result.CommitsAheadA = countA

	countB, err := revListCountForCompare(dir, revA, revB)
	if err != nil {
		return nil, err
	}
	result.CommitsAheadB = countB

	return result, nil
}

func revParseForCompare(dir, ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			gitErr := strings.TrimSpace(string(exitErr.Stderr))
			if gitErr != "" {
				return "", fmt.Errorf("failed to resolve '%s': %s", ref, gitErr)
			}
		}
		return "", fmt.Errorf("failed to resolve '%s': %v", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func revListCountForCompare(dir, from, to string) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", from+".."+to)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to count commits: %w", err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}
	return count, nil
}

func mergeBaseForCompare(dir, refA, refB string) (string, error) {
	cmd := exec.Command("git", "merge-base", refA, refB)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			gitErr := strings.TrimSpace(string(exitErr.Stderr))
			if gitErr != "" {
				return "", fmt.Errorf("%s", gitErr)
			}
		}
		return "", fmt.Errorf("failed to find merge base: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func diffFileCountForCompare(dir, refA, refB string) (int, error) {
	cmd := exec.Command("git", "diff", "--name-only", refA, refB)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to diff: %w", err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return 0, nil
	}
	return len(strings.Split(trimmed, "\n")), nil
}
