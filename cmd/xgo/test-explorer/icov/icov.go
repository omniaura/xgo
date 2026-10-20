package icov

import (
	"github.com/omniaura/xgo/cmd/xgo/internal/vendir/github.com/xhd2015/lines-annotation/model/coverage"
)

type Controller interface {
	GetCoverage() (*coverage.Detail, error)
	GetCoverageProfilePath() (string, error)
	Clear() error
	Refresh() error
}
