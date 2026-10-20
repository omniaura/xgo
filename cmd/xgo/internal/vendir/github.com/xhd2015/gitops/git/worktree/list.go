package worktree

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xhd2015/xgo/support/cmd"
)

// Entry represents one row from `git worktree list --porcelain`.
type Entry struct {
	Path   string
	Branch string // empty when detached
	HEAD   string
	IsMain bool   // true when .git is a directory (main checkout)
}

// IsDead reports whether the worktree directory no longer exists on disk.
func IsDead(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// IsMainRepo reports whether path is a main git checkout (.git is a directory).
func IsMainRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

// IsLinked reports whether path is a linked worktree (.git is a file, not a directory).
func IsLinked(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.Mode().IsRegular()
}

// ReadMainRepo resolves the main repository path from a linked worktree's .git file.
func ReadMainRepo(linkedPath string) (string, error) {
	gitFile := filepath.Join(linkedPath, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		return "", fmt.Errorf("read .git file: %w", err)
	}
	s := strings.TrimSpace(string(content))
	const prefix = "gitdir: "
	if !strings.HasPrefix(s, prefix) {
		return "", fmt.Errorf("unexpected .git file format in worktree %s", linkedPath)
	}
	gitdir := strings.TrimSpace(s[len(prefix):])
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(linkedPath, gitdir)
	}
	// gitdir is <mainRepo>/.git/worktrees/<name>
	mainRepo := filepath.Dir(filepath.Dir(filepath.Dir(gitdir)))
	return normalizePath(mainRepo), nil
}

// ResolveMainRepo returns the main repository for path, whether path is a main
// checkout or a linked worktree.
func ResolveMainRepo(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	abs = normalizePath(abs)
	if IsLinked(abs) {
		return ReadMainRepo(abs)
	}
	if IsMainRepo(abs) {
		return abs, nil
	}
	return "", fmt.Errorf("%s is not a git repository", abs)
}

// List returns all worktrees including the main checkout (including dead paths
// until prune).
func List(repoPath string) ([]Entry, error) {
	out, err := cmd.Dir(repoPath).Output("git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	entries := ParseListPorcelain(out)
	for i := range entries {
		entries[i].IsMain = IsMainRepo(entries[i].Path)
	}
	return entries, nil
}

// ListLinked returns linked worktrees only (excludes the main checkout).
func ListLinked(repoPath string) ([]Entry, error) {
	all, err := List(repoPath)
	if err != nil {
		return nil, err
	}
	var linked []Entry
	for _, e := range all {
		if !e.IsMain {
			linked = append(linked, e)
		}
	}
	return linked, nil
}

// WorktreesOnBranch returns registered worktrees whose Branch equals branch.
// Detached entries (Branch empty) never match. Multiple matches are returned
// as data only — no policy error.
func WorktreesOnBranch(repoPath, branch string) ([]Entry, error) {
	all, err := List(repoPath)
	if err != nil {
		return nil, err
	}
	var matched []Entry
	for _, e := range all {
		if e.Branch == branch {
			matched = append(matched, e)
		}
	}
	return matched, nil
}

// ParseListPorcelain parses `git worktree list --porcelain` output into entries.
// Branch is the short name with refs/heads/ stripped; detached leaves Branch empty.
func ParseListPorcelain(output string) []Entry {
	var entries []Entry
	scanner := bufio.NewScanner(strings.NewReader(output))
	var current Entry
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				entries = append(entries, current)
			}
			current = Entry{Path: line[len("worktree "):]}
		case strings.HasPrefix(line, "HEAD "):
			current.HEAD = line[len("HEAD "):]
		case strings.HasPrefix(line, "branch "):
			ref := line[len("branch "):]
			const prefix = "refs/heads/"
			if strings.HasPrefix(ref, prefix) {
				current.Branch = ref[len(prefix):]
			} else {
				current.Branch = ref
			}
		}
	}
	if current.Path != "" {
		entries = append(entries, current)
	}
	return entries
}

// normalizePath cleans path and resolves symlinks (or parent + base when missing).
func normalizePath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	if resolvedDir, err := filepath.EvalSymlinks(dir); err == nil {
		return filepath.Join(resolvedDir, base)
	}
	return path
}
