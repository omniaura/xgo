package main

import (
	"testing"

	"github.com/omniaura/xgo/instrument/instrument_xgo_runtime"
	"github.com/omniaura/xgo/support/goinfo"
)

// the embedded runtime carries CORE_VERSION, and xgo swaps runtimes older
// than MIN_CORE_VERSION for it, so it must never be older itself
func TestCoreVersionNotBelowMinCoreVersion(t *testing.T) {
	if goinfo.CompareSemVer("v"+CORE_VERSION, "v"+instrument_xgo_runtime.MIN_CORE_VERSION) < 0 {
		t.Fatalf("CORE_VERSION %s is older than MIN_CORE_VERSION %s", CORE_VERSION, instrument_xgo_runtime.MIN_CORE_VERSION)
	}
}
