//go:build !go1.19
// +build !go1.19

package goinfo

import "testing"

// On Go ≤1.18, `go list -e` exits 1 when a replace-target go.mod is missing
// (no Incomplete JSON). OverlayFile must still make the package listable —
// that is the doctest vendor-gomod fix surface on these versions too.
func TestListPackagesOverlayFileSynthesizesNestedGoMod(t *testing.T) {
	appDir, overlayJSON := nestedGoModOverlayFixture(t)

	_, err := ListPackages([]string{"example.com/dep"}, LoadPackageOptions{Dir: appDir})
	if err == nil {
		t.Fatalf("without overlay: go ≤1.18 must fail listing missing replace go.mod")
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
