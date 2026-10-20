package model

import (
	"fmt"
	"strings"
)

// FileCount returns the number of file patches in the cached diff.
// A nil receiver yields 0.
func (d *CachedDiff) FileCount() int {
	if d == nil {
		return 0
	}
	return len(d.Files)
}

// Unified re-renders a reasonable unified-diff text from Files/Hunks.
// A nil receiver yields "".
func (d *CachedDiff) Unified() string {
	if d == nil {
		return ""
	}
	sections := d.fileSections()
	if len(sections) == 0 {
		// Fall back to raw when structured files are empty but raw is set.
		return d.Raw
	}
	return strings.Join(sections, "\n")
}

// UnifiedTruncated re-renders the unified diff, keeping at most
// maxLinesPerFile lines per file section. When a section is longer,
// appends "\n...(N more lines omitted)" with the remaining line count.
// A nil receiver yields "".
func (d *CachedDiff) UnifiedTruncated(maxLinesPerFile int) string {
	if d == nil {
		return ""
	}
	sections := d.fileSections()
	if len(sections) == 0 && strings.TrimSpace(d.Raw) != "" {
		// Truncate raw by splitting on diff --git sections (commit_msg style).
		return truncateUnifiedDiff(d.Raw, maxLinesPerFile)
	}
	var b strings.Builder
	for i, section := range sections {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(truncateSection(section, maxLinesPerFile))
	}
	return b.String()
}

// fileSections returns one unified-diff section per FilePatch.
func (d *CachedDiff) fileSections() []string {
	if d == nil || len(d.Files) == 0 {
		return nil
	}
	out := make([]string, 0, len(d.Files))
	for _, fp := range d.Files {
		out = append(out, renderFilePatch(fp))
	}
	return out
}

// renderFilePatch produces a git-style unified-diff section for one file.
func renderFilePatch(fp FilePatch) string {
	var b strings.Builder

	aPath, bPath := displayPaths(fp)
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", aPath, bPath)

	switch fp.Kind {
	case "add":
		b.WriteString("new file mode 100644\n")
	case "delete":
		b.WriteString("deleted file mode 100644\n")
	case "rename":
		if fp.OldPath != "" {
			fmt.Fprintf(&b, "rename from %s\n", fp.OldPath)
		}
		if fp.NewPath != "" {
			fmt.Fprintf(&b, "rename to %s\n", fp.NewPath)
		}
	}

	if fp.Binary {
		fmt.Fprintf(&b, "Binary files a/%s and b/%s differ\n", aPath, bPath)
		return strings.TrimRight(b.String(), "\n")
	}

	// Pure rename (no content change): meta only — no ---/+++ or hunks.
	if fp.Kind == "rename" && len(fp.Hunks) == 0 {
		return strings.TrimRight(b.String(), "\n")
	}

	// --- / +++ headers
	if fp.Kind == "add" || fp.OldPath == "" {
		b.WriteString("--- /dev/null\n")
	} else {
		fmt.Fprintf(&b, "--- a/%s\n", fp.OldPath)
	}
	if fp.Kind == "delete" || fp.NewPath == "" {
		b.WriteString("+++ /dev/null\n")
	} else {
		fmt.Fprintf(&b, "+++ b/%s\n", fp.NewPath)
	}

	for _, h := range fp.Hunks {
		header := h.Header
		if header != "" {
			b.WriteString(header)
			if !strings.HasSuffix(header, "\n") {
				b.WriteByte('\n')
			}
		}
		for _, line := range h.Lines {
			// Skip a trailing empty line artifact from Split on a final \n.
			// Keep all other lines (including blank context) as stored.
			b.WriteString(line)
			if !strings.HasSuffix(line, "\n") {
				b.WriteByte('\n')
			}
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// displayPaths picks a/ and b/ paths for the diff --git header.
func displayPaths(fp FilePatch) (aPath, bPath string) {
	aPath = fp.OldPath
	bPath = fp.NewPath
	if aPath == "" {
		aPath = bPath
	}
	if bPath == "" {
		bPath = aPath
	}
	if aPath == "" {
		aPath = "unknown"
		bPath = "unknown"
	}
	return aPath, bPath
}

// truncateSection keeps the first maxLines lines of a single file section.
func truncateSection(section string, maxLines int) string {
	lines := strings.Split(strings.TrimRight(section, "\n"), "\n")
	// Empty section edge: Split("") yields [""] — treat as one empty line.
	if maxLines < 0 || len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	kept := strings.Join(lines[:maxLines], "\n")
	return kept + fmt.Sprintf("\n...(%d more lines omitted)", len(lines)-maxLines)
}

// truncateUnifiedDiff splits a full unified diff on file headers and truncates
// each section, matching the historical commit_msg truncateDiffPerFile policy.
func truncateUnifiedDiff(diff string, maxLines int) string {
	parts := strings.Split("\n"+diff, "\ndiff --git ")
	var b strings.Builder
	first := true
	for _, part := range parts {
		if part == "" {
			continue
		}
		section := "diff --git " + part
		if !first {
			b.WriteByte('\n')
		}
		first = false
		b.WriteString(truncateSection(section, maxLines))
	}
	return b.String()
}
