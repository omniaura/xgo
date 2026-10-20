package constants

import "path/filepath"

const RUNTIME_MODULE = "github.com/omniaura/xgo/runtime"

const (
	RUNTIME_INTERNAL_RUNTIME_PKG = "github.com/omniaura/xgo/runtime/internal/runtime"
	RUNTIME_INTERNAL_TRAP_PKG    = "github.com/omniaura/xgo/runtime/internal/trap"
	RUNTIME_TRAP_FLAGS_PKG       = "github.com/omniaura/xgo/runtime/internal/flags"
	RUNTIME_FUNCTAB_PKG          = "github.com/omniaura/xgo/runtime/functab"
	RUNTIME_CORE_PKG             = "github.com/omniaura/xgo/runtime/core"
	RUNTIME_MOCK_PKG             = "github.com/omniaura/xgo/runtime/mock"
	RUNTIME_TRACE_PKG            = "github.com/omniaura/xgo/runtime/trace"
	RUNTIME_TRAP_PKG             = "github.com/omniaura/xgo/runtime/trap"
)

const (
	RUNTIME_LINK_TEMPLATE_FILE = "runtime_link_template.go"
	RUNTIME_LINK_FILE          = "runtime_link.go" // xgo/runtime/internal/runtime/runtime_link.go

	XGO_TRAP_TEMPLATE_FILE = "xgo_trap_template.go"
	XGO_TRAP_FILE          = "xgo_trap.go" // GOROOT/src/runtime/xgo_trap.go

	FUNCTAB_FILE             = "functab.go"
	RUNTIME_FUNCTAB_REGISTER = "RegisterFunc"

	VERSION_FILE = "version.go"
	FLAG_FILE    = "flags.go"
	TRACE_FILE   = "trace.go"
)

const (
	XGO_REAL_NOW   = "XgoRealNow"
	XGO_REAL_SLEEP = "XgoRealSleep"
)

var RUNTIME_LINK_TEMPLATE_PATH = []string{"internal", "runtime", RUNTIME_LINK_TEMPLATE_FILE}
var RUNTIME_XGO_TRAP_TEMPLATE_PATH = []string{"internal", "runtime", XGO_TRAP_TEMPLATE_FILE}
var __GO_RUNTIME_XGO_TRAP_PATH = []string{"src", "runtime", XGO_TRAP_FILE}

func GetGoRuntimeXgoTrapFile(goroot string) string {
	return filepath.Join(goroot, filepath.Join(__GO_RUNTIME_XGO_TRAP_PATH...))
}
