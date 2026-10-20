package filter

import (
	"github.com/omniaura/xgo/cmd/xgo/internal/vendir/github.com/xhd2015/lines-annotation/model/filter"
	path_filter "github.com/omniaura/xgo/cmd/xgo/internal/vendir/github.com/xhd2015/lines-annotation/path/filter"
)

type checker struct {
	opts       *filter.Options
	pathFilter *path_filter.FileFilter
}

func NewChecker(opts *filter.Options) *checker {
	if opts == nil {
		return nil
	}
	noFileFilter := len(opts.Include) == 0 && len(opts.Exclude) == 0
	noSegments := len(opts.ExcludePathSegments) == 0
	if len(opts.Suffix) == 0 && len(opts.ExcludeSuffix) == 0 && noFileFilter && noSegments {
		return nil
	}
	var pathFilter *path_filter.FileFilter
	if !noFileFilter {
		pathFilter = path_filter.NewFileFilter(opts.Include, opts.Exclude)
	}

	return &checker{
		opts:       opts,
		pathFilter: pathFilter,
	}
}

func (c *checker) MatchFile(file string) bool {
	if c == nil {
		return true
	}
	if !c.opts.MatchSuffix(file) {
		return false
	}
	if c.pathFilter != nil && !c.pathFilter.MatchFile(file) {
		return false
	}
	if filter.HasExcludedPathSegment(file, c.opts.ExcludePathSegments) {
		return false
	}
	return true
}
