package git

import "fmt"

// DiffCachedParseError is returned when parsing git diff --cached output fails.
// It carries Dir, the raw patch text, and the underlying cause.
type DiffCachedParseError struct {
	Dir string
	Raw string
	Err error
}

func (e *DiffCachedParseError) Error() string {
	if e == nil {
		return "diff cached parse error"
	}
	if e.Err != nil {
		return fmt.Sprintf("parse diff --cached in %s: %v", e.Dir, e.Err)
	}
	return fmt.Sprintf("parse diff --cached in %s", e.Dir)
}

func (e *DiffCachedParseError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
