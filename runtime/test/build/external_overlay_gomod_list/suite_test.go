package external_overlay_gomod_list

import (
	"testing"

	"example.com/dep"
	"github.com/xhd2015/xgo/runtime/mock"
)

// Caller -overlay synthesizes vendor_dep/go.mod. Instrumentation discovery must
// pass that overlay to go list; otherwise example.com/dep is Incomplete with
// empty GoFiles and mock.Patch panics ERR_NOT_INSTRUMENTED.
func TestOverlayGoModDiscoverableForPatch(t *testing.T) {
	if got, want := dep.Hello(), "real"; got != want {
		t.Fatalf("dep.Hello() = %q, want %q (module graph / overlay broken before mock?)", got, want)
	}

	mock.Patch(dep.Hello, func() string { return "mocked" })
	if got, want := dep.Hello(), "mocked"; got != want {
		t.Fatalf("after mock dep.Hello() = %q, want %q (go list missed overlay go.mod?)", got, want)
	}
}
