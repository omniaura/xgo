package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/omniaura/xgo/cmd/xgo/internal/vendir/github.com/xhd2015/gitops/model"
)

// DiffCached runs `git -C dir diff --cached` and parses the staged unified
// diff into *model.CachedDiff. Empty/whitespace stdout yields (nil, nil).
// Fatal git failures return an ordinary error. Non-empty unparseable stdout
// returns *DiffCachedParseError with Dir and Raw set.
func DiffCached(dir string) (*model.CachedDiff, error) {
	cmd := exec.Command("git", "-C", dir, "diff", "--cached")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	raw := string(out)
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	diff, err := ParseCachedDiff(raw)
	if err != nil {
		if perr, ok := err.(*DiffCachedParseError); ok {
			return nil, &DiffCachedParseError{
				Dir: dir,
				Raw: perr.Raw,
				Err: perr.Err,
			}
		}
		return nil, &DiffCachedParseError{
			Dir: dir,
			Raw: raw,
			Err: err,
		}
	}
	return diff, nil
}

// ParseCachedDiff parses a raw unified-diff string into *model.CachedDiff.
// Empty or whitespace-only input yields (nil, nil). Non-empty input that
// cannot be parsed as a unified diff yields *DiffCachedParseError with Raw set.
func ParseCachedDiff(raw string) (*model.CachedDiff, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	files, err := parseUnifiedDiffFiles(raw)
	if err != nil {
		return nil, &DiffCachedParseError{
			Raw: raw,
			Err: err,
		}
	}
	if len(files) == 0 {
		return nil, &DiffCachedParseError{
			Raw: raw,
			Err: fmt.Errorf("no file patches found in unified diff"),
		}
	}
	return &model.CachedDiff{
		Files: files,
		Raw:   raw,
	}, nil
}

// parseUnifiedDiffFiles walks a git-style unified diff and returns FilePatch
// entries. Returns an error when the text is non-empty but not a valid
// unified-diff stream (no diff --git headers, etc.).
func parseUnifiedDiffFiles(raw string) ([]model.FilePatch, error) {
	// Normalize line endings; keep trailing empty lines out of the way.
	text := strings.ReplaceAll(raw, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")

	// A real git unified diff is expected to contain at least one
	// "diff --git" file header. Pure garbage without headers is malformed.
	hasGitHeader := false
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			hasGitHeader = true
			break
		}
	}
	if !hasGitHeader {
		return nil, fmt.Errorf("not a unified diff: missing 'diff --git' header")
	}

	var files []model.FilePatch
	var cur *model.FilePatch
	var curHunk *model.Hunk

	flushHunk := func() {
		if cur != nil && curHunk != nil {
			cur.Hunks = append(cur.Hunks, *curHunk)
			curHunk = nil
		}
	}
	flushFile := func() {
		flushHunk()
		if cur != nil {
			// Kind is required for consumers; default to modify if still empty.
			if cur.Kind == "" {
				cur.Kind = "modify"
			}
			files = append(files, *cur)
			cur = nil
		}
	}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flushFile()
			oldPath, newPath := parseDiffGitPaths(line)
			cur = &model.FilePatch{
				OldPath: oldPath,
				NewPath: newPath,
				Kind:    "modify",
			}

		case cur == nil:
			// Skip preamble / noise before the first file header.
			continue

		case strings.HasPrefix(line, "new file mode"):
			cur.Kind = "add"
			// New files often use /dev/null as OldPath once --- is seen;
			// clear a/ prefix path remains fine.

		case strings.HasPrefix(line, "deleted file mode"):
			cur.Kind = "delete"

		case strings.HasPrefix(line, "rename from "):
			cur.Kind = "rename"
			cur.OldPath = strings.TrimPrefix(line, "rename from ")

		case strings.HasPrefix(line, "rename to "):
			cur.Kind = "rename"
			cur.NewPath = strings.TrimPrefix(line, "rename to ")

		case strings.HasPrefix(line, "similarity index "),
			strings.HasPrefix(line, "dissimilarity index "),
			strings.HasPrefix(line, "index "),
			strings.HasPrefix(line, "old mode "),
			strings.HasPrefix(line, "new mode "):
			// metadata — ignore

		case strings.HasPrefix(line, "Binary files ") || strings.HasPrefix(line, "GIT binary patch"):
			cur.Binary = true

		case strings.HasPrefix(line, "--- "):
			p := parseDiffPath(strings.TrimPrefix(line, "--- "))
			if p == "/dev/null" {
				cur.OldPath = ""
				if cur.Kind == "modify" {
					cur.Kind = "add"
				}
			} else if p != "" {
				cur.OldPath = p
			}

		case strings.HasPrefix(line, "+++ "):
			p := parseDiffPath(strings.TrimPrefix(line, "+++ "))
			if p == "/dev/null" {
				cur.NewPath = ""
				if cur.Kind == "modify" {
					cur.Kind = "delete"
				}
			} else if p != "" {
				cur.NewPath = p
			}

		case strings.HasPrefix(line, "@@"):
			flushHunk()
			curHunk = &model.Hunk{Header: line}

		default:
			if curHunk != nil {
				// Hunk body lines: context/add/del, or "\ No newline at end of file".
				curHunk.Lines = append(curHunk.Lines, line)
			}
		}
	}
	flushFile()

	if len(files) == 0 {
		return nil, fmt.Errorf("no file patches found in unified diff")
	}
	return files, nil
}

// parseDiffGitPaths extracts a/ and b/ paths from a
// "diff --git a/path b/path" line. Handles simple unquoted paths.
func parseDiffGitPaths(line string) (oldPath, newPath string) {
	// Format: diff --git a/<old> b/<new>
	// Paths may contain spaces only when quoted; we handle the common case.
	rest := strings.TrimPrefix(line, "diff --git ")
	// Prefer splitting on " b/" when both sides are unquoted a/... b/...
	const bSep = " b/"
	idx := strings.Index(rest, bSep)
	if idx >= 0 && strings.HasPrefix(rest, "a/") {
		oldPath = rest[len("a/"):idx]
		newPath = rest[idx+len(bSep):]
		return oldPath, newPath
	}
	// Quoted form: diff --git "a/foo bar" "b/foo bar"
	fields := splitDiffGitFields(rest)
	if len(fields) >= 2 {
		oldPath = stripABPrefix(fields[0])
		newPath = stripABPrefix(fields[1])
		return oldPath, newPath
	}
	if len(fields) == 1 {
		p := stripABPrefix(fields[0])
		return p, p
	}
	return "", ""
}

func splitDiffGitFields(rest string) []string {
	var fields []string
	s := strings.TrimSpace(rest)
	for s != "" {
		if s[0] == '"' {
			// quoted path
			end := 1
			for end < len(s) {
				if s[end] == '\\' && end+1 < len(s) {
					end += 2
					continue
				}
				if s[end] == '"' {
					break
				}
				end++
			}
			if end < len(s) && s[end] == '"' {
				fields = append(fields, s[1:end])
				s = strings.TrimSpace(s[end+1:])
				continue
			}
			// malformed quote — take rest
			fields = append(fields, s)
			break
		}
		// unquoted: next space-separated token
		sp := strings.IndexByte(s, ' ')
		if sp < 0 {
			fields = append(fields, s)
			break
		}
		fields = append(fields, s[:sp])
		s = strings.TrimSpace(s[sp+1:])
	}
	return fields
}

func stripABPrefix(p string) string {
	if strings.HasPrefix(p, "a/") || strings.HasPrefix(p, "b/") {
		return p[2:]
	}
	return p
}

// parseDiffPath strips the a/ or b/ prefix from --- / +++ path fields.
// Input is the remainder after "--- " or "+++ " (may include tab+timestamp).
func parseDiffPath(field string) string {
	// Strip optional timestamp after tab.
	if i := strings.IndexByte(field, '\t'); i >= 0 {
		field = field[:i]
	}
	field = strings.TrimSpace(field)
	// Quoted paths: "b/foo bar"
	if len(field) >= 2 && field[0] == '"' {
		if end := strings.LastIndex(field, `"`); end > 0 {
			field = field[1:end]
		}
	}
	if field == "/dev/null" {
		return "/dev/null"
	}
	return stripABPrefix(field)
}
