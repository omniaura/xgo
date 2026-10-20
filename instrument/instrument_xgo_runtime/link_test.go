package instrument_xgo_runtime

import (
	"testing"

	"github.com/omniaura/xgo/support/goinfo"
)

// runtime/internal/runtime/xgo_trap_template.go is linked into GOROOT as is,
// so it must be valid for every supported Go version. It relies on runtime
// internals that change across releases: _panic.retpc, the
// internal/runtime/atomic package, getg/m.curg/m.locks, and FuncForPC(pc).Name()
// wrapping names in funcNameForPrint. Before raising MAX_SUPPORTED_MINOR,
// check those against the new release, then raise auditedMaxMinor.
const auditedMaxMinor = 28

func TestTrapTemplateAuditedForMaxGoVersion(t *testing.T) {
	if goinfo.MAX_SUPPORTED_MINOR > auditedMaxMinor {
		t.Fatalf("go1.%d is supported but xgo_trap_template.go was audited up to go1.%d only", goinfo.MAX_SUPPORTED_MINOR, auditedMaxMinor)
	}
}
