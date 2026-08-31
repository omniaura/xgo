//go:build go1.19
// +build go1.19

package goinfo

import "testing"

// Regression: nested replace targets may expose go.mod only through -overlay
// (doctest vendor-gomod). ListPackages must forward OverlayFile to go list or
// the package is Incomplete with empty GoFiles.
//
// Go 1.19+ returns Incomplete JSON for a missing replace-target go.mod under
// `go list -e` (≤1.18 exits 1 instead — see list_overlay_before_go1.19_test.go).
func TestListPackagesOverlayFileSynthesizesNestedGoMod(t *testing.T) {
	appDir, overlayJSON := nestedGoModOverlayFixture(t)

	without, err := ListPackages([]string{"example.com/dep"}, LoadPackageOptions{Dir: appDir})
	if err != nil {
		t.Fatalf("list without overlay: %v", err)
	}
	if len(without) != 1 {
		t.Fatalf("without overlay: got %d pkgs, want 1", len(without))
	}
	if !without[0].Incomplete || len(without[0].GoFiles) != 0 {
		t.Fatalf("without overlay: Incomplete=%v nGoFiles=%d, want Incomplete with 0 GoFiles", without[0].Incomplete, len(without[0].GoFiles))
	}

	with, err := ListPackages([]string{"example.com/dep"}, LoadPackageOptions{
		Dir:         appDir,
		OverlayFile: overlayJSON,
	})
	if err != nil {
		t.Fatalf("list with overlay: %v", err)
	}
	if len(with) != 1 {
		t.Fatalf("with overlay: got %d pkgs, want 1", len(with))
	}
	if with[0].Incomplete || len(with[0].GoFiles) == 0 {
		t.Fatalf("with overlay: Incomplete=%v nGoFiles=%d Err=%v, want complete package with GoFiles", with[0].Incomplete, len(with[0].GoFiles), with[0].Error)
	}
}
