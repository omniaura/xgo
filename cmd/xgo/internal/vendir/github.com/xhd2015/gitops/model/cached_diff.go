package model

// CachedDiff is an in-memory model of a parsed staged patch:
// the list of file patches plus optional raw text.
type CachedDiff struct {
	Files []FilePatch
	Raw   string
}

// FilePatch is one file's change in a cached diff.
type FilePatch struct {
	OldPath string
	NewPath string
	Kind    string
	Binary  bool
	Hunks   []Hunk
}

// Hunk is one hunk header plus body lines.
type Hunk struct {
	Header string
	Lines  []string
}
