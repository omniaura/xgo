package main

import (
	"testing"

	"github.com/xhd2015/xgo/support/goinfo"
)

func TestFormatGoDirectiveVersion(t *testing.T) {
	version := formatGoDirectiveVersion(&goinfo.GoVersion{
		Major: 1,
		Minor: 26,
		Patch: 0,
	})

	if version != "1.26.0" {
		t.Fatalf("formatGoDirectiveVersion() = %q, want %q", version, "1.26.0")
	}
}
