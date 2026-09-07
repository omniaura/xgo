package main

import (
	"path/filepath"
	"testing"
)

func TestResolveBuildCacheDir(t *testing.T) {
	def := resolveBuildCacheDir("/tmp/xgo/go-instrument/go1.26.0_abc", "", "go1.26.0_abc")
	if def != filepath.Join("/tmp/xgo/go-instrument/go1.26.0_abc", "build-cache") {
		t.Fatalf("default: got %q", def)
	}
	got := resolveBuildCacheDir("/tmp/xgo/go-instrument/go1.26.0_abc", "/var/cache/go-build/xgo", "go1.26.0_abc")
	if got != filepath.Join("/var/cache/go-build/xgo", "go1.26.0_abc") {
		t.Fatalf("override: got %q", got)
	}
}
