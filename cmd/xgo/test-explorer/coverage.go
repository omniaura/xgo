package test_explorer

import (
	"net/http"

	"github.com/omniaura/xgo/cmd/xgo/coverage/serve"
	"github.com/omniaura/xgo/cmd/xgo/internal/vendir/github.com/xhd2015/lines-annotation/load/loadcov"
	"github.com/omniaura/xgo/cmd/xgo/test-explorer/icov"
)

// see https://github.com/xhd2015/xgo/issues/215
func setupCoverageHandler(server *http.ServeMux, covController icov.Controller, opts loadcov.LoadAllOptions, getPort func() int) {
	// install /coverage, /coverage/fileAnnotations, /coverage/fileDetail, /coverage/diffFileDetail
	serve.RouteServer(server, "/coverage", getPort, opts)
}
