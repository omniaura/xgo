package loadcov

import (
	"github.com/omniaura/xgo/cmd/xgo/internal/vendir/github.com/xhd2015/lines-annotation/compute/coverage"
	"github.com/omniaura/xgo/cmd/xgo/internal/vendir/github.com/xhd2015/lines-annotation/model"
	cov_model "github.com/omniaura/xgo/cmd/xgo/internal/vendir/github.com/xhd2015/lines-annotation/model/coverage"
)

func ComputeCoverageSummary(project *model.ProjectAnnotation) map[string]*cov_model.Summary {
	return coverage.ComputeCoverageSummary(project, &coverage.ComputeOptions{
		DisableFunc: true,
	})
}
