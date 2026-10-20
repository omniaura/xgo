package filter

import "strings"

type Options struct {
	Suffix        []string // include .go
	ExcludeSuffix []string // exclude _test.go

	Include []string
	Exclude []string

	// ExcludePathSegments drops files whose path has any exact path-component
	// matching a listed name (e.g. "vendor", "node_modules"). Callers supply
	// the list; the library does not hardcode defaults.
	ExcludePathSegments []string
}

// HasExcludedPathSegment reports whether file has any path component that
// exactly equals one of the given segment names. Backslashes are normalized
// to slashes; empty components and empty segment entries are ignored.
func HasExcludedPathSegment(file string, segments []string) bool {
	if len(segments) == 0 {
		return false
	}
	var names []string
	for _, s := range segments {
		if s != "" {
			names = append(names, s)
		}
	}
	if len(names) == 0 {
		return false
	}

	normalized := strings.ReplaceAll(file, "\\", "/")
	for _, part := range strings.Split(normalized, "/") {
		if part == "" {
			continue
		}
		for _, name := range names {
			if part == name {
				return true
			}
		}
	}
	return false
}

// Deprecated: use Options instead
type LegacyOptions struct {
	Suffix        []string // include .go
	ExcludeSuffix []string // exclude _test.go

	ExcludePath []string // exclude vendor/
}

func (c *LegacyOptions) Match(file string) bool {
	return checkFileOk(file, c)
}

func (c *Options) MatchSuffix(file string) bool {
	if c == nil {
		return true
	}
	return checkSuffixMatch(file, c.Suffix, c.ExcludeSuffix)
}

func checkSuffixMatch(file string, suffix []string, excludeSuffix []string) bool {
	if !matchAnySuffix(file, suffix, true) {
		return false
	}
	if matchAnySuffix(file, excludeSuffix, false) {
		return false
	}
	return true
}

func checkFileOk(file string, opts *LegacyOptions) bool {
	if opts == nil {
		return true
	}
	if !checkSuffixMatch(file, opts.Suffix, opts.ExcludeSuffix) {
		return false
	}
	if matchAnyPath(file, opts.ExcludePath, false) {
		return false
	}
	return true
}

func matchAnySuffix(file string, suffix []string, defaultVal bool) bool {
	if len(suffix) == 0 {
		return defaultVal
	}
	for _, s := range suffix {
		if strings.HasSuffix(file, s) {
			return true
		}
	}
	return false
}

func matchAnyPath(file string, paths []string, defaultVal bool) bool {
	if len(paths) == 0 {
		return defaultVal
	}
	for _, path := range paths {
		if matchPath(file, path) {
			return true
		}
	}
	return false
}

// matchPath("a/b/c", "a/b") == true
// matchPath("a/b", "a/b") == true
// matchPath("a/bc", "a/b") == false
func matchPath(file string, path string) bool {
	if !strings.HasPrefix(file, path) {
		return false
	}
	// len(file) >= len(path)
	if len(file) == len(path) {
		return true
	}
	if file[len(path)] == '/' {
		return true
	}
	return false
}
