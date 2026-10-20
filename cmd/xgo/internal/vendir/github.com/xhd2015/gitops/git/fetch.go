package git

import (
	"fmt"
	"time"

	"github.com/xhd2015/xgo/cmd/xgo/internal/vendir/github.com/xhd2015/gitops/git/fetch"
)

type FetchOptions struct {
	Timeout time.Duration
	Depth   int
	Env     map[string]string
}

func FetchAll(dir string, opts *FetchOptions) error {
	var timeout time.Duration
	var depth int
	var env map[string]string
	if opts != nil {
		timeout = opts.Timeout
		depth = opts.Depth
		env = opts.Env
	}
	args := fetch.FormatFetch("", &fetch.Options{
		AllTags: true,
		Depth:   depth,
	})
	return RunGitErr(dir, &RunGitOptions{Timeout: timeout, Env: env}, args...)
}

func FetchSingle(dir string, origin string, ref string, opts *FetchOptions) error {
	if origin == "" {
		return fmt.Errorf("requires origin")
	}
	if ref == "" {
		return fmt.Errorf("requires ref")
	}
	if ref == COMMIT_WORKING {
		return nil
	}
	var timeout time.Duration
	var depth int
	var env map[string]string
	if opts != nil {
		timeout = opts.Timeout
		depth = opts.Depth
		env = opts.Env
	}

	args := fetch.FormatFetch(origin, &fetch.Options{
		Branch:  ref,
		AllTags: true,
		Depth:   depth,
	})
	return RunGitErr(dir, &RunGitOptions{Timeout: timeout, Env: env}, args...)
}
