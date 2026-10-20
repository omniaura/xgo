package git

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/xhd2015/xgo/cmd/xgo/internal/vendir/github.com/xhd2015/go-coverage/sh"
)

// a special commit that referes to current working directory(a pseduo commit)
const COMMIT_WORKING = "WORKING"

// find rename
// git diff --find-renames --diff-filter=R   HEAD~10 HEAD|grep -A 3 '^diff --git a/'|grep rename
// FindRenames returns a mapping from new name to old name
func FindRenames(dir string, oldCommit string, newCommit string) (map[string]string, error) {
	repo := &GitRepo{Dir: dir}
	return repo.FindRenames(oldCommit, newCommit)
}
func FindRenamesV2(dir string, oldCommit string, newCommit string, fn func(newFile string, oldFile string, percent string)) error {
	repo := &GitRepo{Dir: dir}
	return repo.FindRenamesV2(oldCommit, newCommit, fn)
}

func FindUpdate(dir string, oldCommit string, newCommit string) ([]string, error) {
	repo := &GitRepo{Dir: dir}
	return repo.FindUpdate(oldCommit, newCommit)
}

func FindUpdateAndRenames(dir string, oldCommit string, newCommit string) (newToOld map[string]string, err error) {
	repo := &GitRepo{Dir: dir}
	updates, err := repo.FindUpdate(oldCommit, newCommit)
	if err != nil {
		return nil, err
	}
	m, err := repo.FindRenames(oldCommit, newCommit)
	if err != nil {
		return nil, err
	}
	for _, u := range updates {
		if _, ok := m[u]; ok {
			return nil, fmt.Errorf("invalid file: %s found both renamed and updated", u)
		}
		m[u] = u
	}
	return m, nil
}
func FindAll(dir string, oldCommit string, newCommit string) (allFiles []string, newToOld map[string]string, err error) {
	newToOld, err = FindUpdateAndRenames(dir, oldCommit, newCommit)
	if err != nil {
		return
	}
	allFiles, err = NewSnapshot(dir, newCommit).ListFiles()
	return
}

type GitRepo struct {
	Dir string
	// PathPatterns are optional git pathspecs appended after `--` (e.g. "*.go", ":(exclude)vendor").
	PathPatterns []string
}

func (c *GitRepo) pathSpecArg() string {
	if c == nil || len(c.PathPatterns) == 0 {
		return ""
	}
	return " -- " + sh.Quotes(c.PathPatterns...)
}

func NewGitRepo(dir string) *GitRepo {
	return &GitRepo{
		Dir: dir,
	}
}
func NewSnapshot(dir string, commit string) *GitSnapshot {
	return &GitSnapshot{
		Dir:    dir,
		Commit: commit,
	}
}

func QuoteCommit(commit string) string {
	if commit == COMMIT_WORKING {
		return ""
	}
	return sh.Quote(getRef(commit))
}

func (c *GitRepo) FindUpdate(oldCommit string, newCommit string) ([]string, error) {
	cmd := fmt.Sprintf(`git -C %s diff --diff-filter=M --name-only --ignore-submodules %s %s%s`, sh.Quote(c.Dir), QuoteCommit(oldCommit), QuoteCommit(newCommit), c.pathSpecArg())
	stdout, _, err := sh.RunBashCmdOpts(cmd, sh.RunBashOptions{
		NeedStdOut: true,
	})
	if err != nil {
		return nil, err
	}
	return splitLinesFilterEmpty(stdout), nil
}
func (c *GitRepo) FindAdded(oldCommit string, newCommit string) ([]string, error) {
	cmd := fmt.Sprintf(`git -C %s diff --diff-filter=A --name-only --ignore-submodules %s %s%s`, sh.Quote(c.Dir), QuoteCommit(oldCommit), QuoteCommit(newCommit), c.pathSpecArg())
	stdout, _, err := sh.RunBashCmdOpts(cmd, sh.RunBashOptions{
		NeedStdOut: true,
	})
	if err != nil {
		return nil, err
	}
	return splitLinesFilterEmpty(stdout), nil
}
func (c *GitRepo) FindRenames(oldCommit string, newCommit string) (map[string]string, error) {
	// example:
	// 	$ git diff --find-renames --diff-filter=R   HEAD~10 HEAD
	// diff --git a/test/stubv2/boot/boot.go b/test/stub/boot/boot.go
	// similarity index 94%
	// rename from test/stubv2/boot/boot.go
	// rename to test/stub/boot/boot.go
	// index e0e86051..56c49801 100644
	// --- a/test/stubv2/boot/boot.go
	// +++ b/test/stub/boot/boot.go
	// @@ -4,8 +4,10 @@ import (
	cmd := fmt.Sprintf(`git -C %s diff --find-renames --diff-filter=R --ignore-submodules %s %s%s|grep -A 3 '^diff --git a/'|grep -E '^rename' || true`, sh.Quote(c.Dir), QuoteCommit(oldCommit), QuoteCommit(newCommit), c.pathSpecArg())
	stdout, stderr, err := sh.RunBashCmdOpts(cmd, sh.RunBashOptions{
		// Verbose:    true,
		NeedStdOut: true,
		// NeedStdErr: true,
	})
	// fmt.Printf("stderr:%v", stderr)
	_ = stderr
	if err != nil {
		return nil, err
	}
	lines := splitLinesFilterEmpty(stdout)
	if len(lines)%2 != 0 {
		return nil, fmt.Errorf("internal error, expect git return rename pairs, found:%d", len(lines))
	}

	m := make(map[string]string, len(lines)/2)
	for i := 0; i < len(lines); i += 2 {
		from := strings.TrimPrefix(lines[i], "rename from ")
		to := strings.TrimPrefix(lines[i+1], "rename to ")

		m[to] = from
	}
	return m, nil
}

// without --ignore-submodules, git diff may include diff of dirs
func (c *GitRepo) FindRenamesV2(oldCommit string, newCommit string, fn func(newFile string, oldFile string, percent string)) error {
	cmd := fmt.Sprintf(`git -C %s diff --diff-filter=R --summary --ignore-submodules %s %s%s || true`, sh.Quote(c.Dir), QuoteCommit(oldCommit), QuoteCommit(newCommit), c.pathSpecArg())
	stdout, stderr, err := sh.RunBashCmdOpts(cmd, sh.RunBashOptions{
		// Verbose:    true,
		NeedStdOut: true,
		// NeedStdErr: true,
	})
	// fmt.Printf("stderr:%v", stderr)
	_ = stderr
	if err != nil {
		return err
	}
	parseRenames(stdout, fn)
	return nil
}

func parseRenames(renamedFilesWithSummary string, fn func(newFile string, oldFile string, percent string)) {
	renames := splitLinesFilterEmpty(renamedFilesWithSummary)
	for _, line := range renames {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "rename ") {
			continue
		}
		s := strings.TrimSpace(line[len("rename "):])

		// parse percent
		//  rename src/{module_funder/id/submodule_funder_seabank/cl => module_funder_bke/id}/bcl/task/repair_clawback_task.go (64%)
		var percent string
		pEidx := strings.LastIndex(s, ")")
		if pEidx >= 0 {
			s = strings.TrimSpace(s[:pEidx])
			pIdx := strings.LastIndex(s, "(")
			if pIdx >= 0 {
				percent = strings.TrimSpace(s[pIdx+1:])
				s = strings.TrimSpace(s[:pIdx])
			}
		}

		bIdx := strings.Index(s, "{")
		if bIdx < 0 {
			continue
		}
		bEIdx := strings.LastIndex(s, "}")
		if bEIdx < 0 {
			continue
		}
		prefix := s[:bIdx]
		var suffix string
		if bEIdx+1 < len(s) {
			suffix = s[bEIdx+1:]
		}
		s = s[bIdx+1 : bEIdx]
		sep := " => "
		toIdx := strings.Index(s, sep)
		if toIdx < 0 {
			continue
		}
		oldPath := s[:toIdx]
		var newPath string
		if toIdx+len(sep) < len(s) {
			newPath = s[toIdx+len(sep):]
		}

		file := joinPath(prefix, newPath, suffix)
		oldFile := joinPath(prefix, oldPath, suffix)

		fn(file, oldFile, percent)
	}
}

func joinPath(p ...string) string {
	j := 0
	for i := 0; i < len(p); i++ {
		e := strings.TrimPrefix(p[i], "/")
		e = strings.TrimSuffix(e, "/")
		if e != "" {
			p[j] = e
			j++
		}
	}
	return strings.Join(p[:j], "/")
}

type GitSnapshot struct {
	Dir    string
	Commit string

	filesInit sync.Once
	files     []string
	filesErr  error
	fileMap   map[string]bool

	// gitCommonDir is the bare / common git directory resolved from Dir.
	// Resolved once via disk parse (no git -C checkout), so GetContent/ListFiles
	// still work after a linked worktree admin dir under bare/worktrees is deleted.
	gitDirInit   sync.Once
	gitCommonDir string
	gitDirErr    error
}

func (c *GitSnapshot) GetContent(file string) (string, error) {
	normFile := strings.TrimPrefix(file, "./")
	if normFile == "" {
		return "", fmt.Errorf("invalid file:%v", file)
	}

	c.ensureInitList()
	if c.filesErr != nil {
		return "", c.filesErr
	}
	if !c.fileMap[normFile] {
		return "", fmt.Errorf("not a file, maybe a dir:%v", file)
	}
	if c.Commit == COMMIT_WORKING {
		content, err := ioutil.ReadFile(path.Join(c.Dir, normFile))
		if err != nil {
			return "", err
		}
		return string(content), nil
	}

	// Direct argv (no bash): git --git-dir=<common> cat-file -p ref:file.
	content, err := c.runGitCatFile(normFile)
	if err == nil {
		return content, nil
	}
	// ECHILD harden: Wait can fail with "no child processes" after the child
	// already wrote stdout. Prefer content when we have it; otherwise retry once.
	if isNoChildProcessErr(err) {
		if content != "" {
			return content, nil
		}
		content, err = c.runGitCatFile(normFile)
		if err == nil {
			return content, nil
		}
		if isNoChildProcessErr(err) && content != "" {
			return content, nil
		}
	}
	return content, err
}

// ensureGitCommonDir returns the absolute git common (bare) directory for c.Dir.
// Resolution is disk-only (parse .git / commondir / worktrees layout) so it does
// not require a live linked admin gitdir under bare/worktrees at call time.
func (c *GitSnapshot) ensureGitCommonDir() (string, error) {
	c.gitDirInit.Do(func() {
		c.gitCommonDir, c.gitDirErr = resolveGitCommonDir(c.Dir)
	})
	return c.gitCommonDir, c.gitDirErr
}

// resolveGitCommonDir derives the git common dir from a checkout path without
// invoking `git -C <checkout>`, which fails when the linked admin is deleted.
func resolveGitCommonDir(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("resolve git common dir: empty dir")
	}
	gitPath := filepath.Join(dir, ".git")
	st, err := os.Stat(gitPath)
	if err != nil {
		// Dir itself may already be a bare/common git directory.
		if isGitDir(dir) {
			return filepath.Clean(dir), nil
		}
		return "", fmt.Errorf("resolve git common dir: %w", err)
	}
	if st.IsDir() {
		// Normal repo: common dir is .git unless .git/commondir says otherwise.
		if common, ok := readCommonDir(gitPath); ok {
			return common, nil
		}
		return filepath.Clean(gitPath), nil
	}
	// Linked worktree: .git is a "gitdir: <admin>" pointer file.
	admin, err := parseGitDirPointer(gitPath, dir)
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(admin); err == nil && st.IsDir() {
		if common, ok := readCommonDir(admin); ok {
			return common, nil
		}
		return admin, nil
	}
	// Admin already deleted: standard layout bare/worktrees/<id> → bare.
	if filepath.Base(filepath.Dir(admin)) == "worktrees" {
		return filepath.Clean(filepath.Dir(filepath.Dir(admin))), nil
	}
	return "", fmt.Errorf("resolve git common dir: linked admin missing: %s", admin)
}

// parseGitDirPointer reads a .git file of the form "gitdir: <path>".
func parseGitDirPointer(gitPath, checkout string) (string, error) {
	data, err := ioutil.ReadFile(gitPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", gitPath, err)
	}
	line := strings.TrimSpace(string(data))
	// Git may write multi-line; first line is the pointer.
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	const prefix = "gitdir:"
	lower := strings.ToLower(line)
	if !strings.HasPrefix(lower, prefix) {
		return "", fmt.Errorf("not a gitdir pointer: %s", gitPath)
	}
	rest := strings.TrimSpace(line[len(prefix):])
	if rest == "" {
		return "", fmt.Errorf("empty gitdir pointer: %s", gitPath)
	}
	if !filepath.IsAbs(rest) {
		rest = filepath.Join(checkout, rest)
	}
	return filepath.Clean(rest), nil
}

// readCommonDir reads gitDir/commondir and returns the absolute common path.
func readCommonDir(gitDir string) (string, bool) {
	data, err := ioutil.ReadFile(filepath.Join(gitDir, "commondir"))
	if err != nil {
		return "", false
	}
	rel := strings.TrimSpace(string(data))
	if rel == "" {
		return "", false
	}
	if filepath.IsAbs(rel) {
		return filepath.Clean(rel), true
	}
	return filepath.Clean(filepath.Join(gitDir, rel)), true
}

// isGitDir reports whether path looks like a git directory (has HEAD).
func isGitDir(path string) bool {
	st, err := os.Stat(filepath.Join(path, "HEAD"))
	return err == nil && !st.IsDir()
}

// runGitCatFile runs `git --git-dir=<common> cat-file -p ref:file` without a shell.
// On error it still returns any stdout collected (needed for ECHILD harden).
func (c *GitSnapshot) runGitCatFile(normFile string) (string, error) {
	gitDir, err := c.ensureGitCommonDir()
	if err != nil {
		return "", err
	}
	cmd := exec.Command("git", "--git-dir="+gitDir, "cat-file", "-p", c.ref()+":"+normFile)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err = cmd.Run()
	content := stdoutBuf.String()
	if err != nil {
		if isNoChildProcessErr(err) {
			// Surface raw wait error so caller can harden; keep collected stdout.
			return content, err
		}
		stderr := strings.TrimSpace(stderrBuf.String())
		if stderr != "" {
			return content, fmt.Errorf("git cat-file -p %s:%s: %v stderr:%s", c.ref(), normFile, err, stderr)
		}
		return content, fmt.Errorf("git cat-file -p %s:%s: %v", c.ref(), normFile, err)
	}
	return content, nil
}

// isNoChildProcessErr reports wait/ECHILD failures seen under concurrent exec.
func isNoChildProcessErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ECHILD) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == syscall.ECHILD {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "no child processes") ||
		strings.Contains(msg, "wait: no child processes") ||
		strings.Contains(msg, "waitid: no child processes")
}
func (c *GitSnapshot) ListFiles() ([]string, error) {
	c.ensureInitList()
	return c.files, c.filesErr
}
func (c *GitSnapshot) ensureInitList() {
	c.filesInit.Do(func() {
		// COMMIT_WORKING needs the work tree for ls-files; use -C Dir.
		// Committed snapshots use --git-dir=<common> so list still works when
		// the linked admin under bare/worktrees is gone.
		var stdout string
		var err error
		if c.Commit == COMMIT_WORKING {
			stdout, _, err = sh.RunBashWithOpts([]string{
				fmt.Sprintf("git -C %s ls-files", sh.Quote(c.Dir)),
			}, sh.RunBashOptions{
				Verbose:    true,
				NeedStdOut: true,
			})
		} else {
			gitDir, gerr := c.ensureGitCommonDir()
			if gerr != nil {
				c.filesErr = gerr
				return
			}
			// Direct argv: no shell, paths with spaces safe, no dependency on admin.
			cmd := exec.Command("git", "--git-dir="+gitDir, "ls-files", "--with-tree", c.ref())
			var stdoutBuf, stderrBuf bytes.Buffer
			cmd.Stdout = &stdoutBuf
			cmd.Stderr = &stderrBuf
			err = cmd.Run()
			if err != nil {
				stderr := strings.TrimSpace(stderrBuf.String())
				if stderr != "" {
					err = fmt.Errorf("git ls-files --with-tree %s: %v stderr:%s", c.ref(), err, stderr)
				} else {
					err = fmt.Errorf("git ls-files --with-tree %s: %v", c.ref(), err)
				}
			} else {
				stdout = stdoutBuf.String()
			}
		}
		if err != nil {
			c.filesErr = err
			return
		}
		c.files = splitLinesFilterEmpty(stdout)
		c.fileMap = make(map[string]bool)
		for _, e := range c.files {
			c.fileMap[e] = true
		}
	})
}

func (c *GitSnapshot) ref() string {
	return getRef(c.Commit)
}

func getRef(commit string) string {
	if commit == "" {
		return "HEAD"
	}
	return commit
}

func splitLinesFilterEmpty(s string) []string {
	list := strings.Split(s, "\n")
	idx := 0
	for _, e := range list {
		if e != "" {
			list[idx] = e
			idx++
		}
	}
	return list[:idx]
}
