package goinfo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// nestedGoModOverlayFixture builds a replace-target that has sources but no
// on-disk go.mod; the synthesized go.mod is only available via overlayJSON.
//
// Overlay Replace keys include both the raw and EvalSymlinks spellings so
// macOS /var vs /private/var does not miss the go.mod lookup (same class of
// issue as runtime/test/build/external_overlay_path_identity).
func nestedGoModOverlayFixture(t *testing.T) (appDir, overlayJSON string) {
	t.Helper()
	tmp := t.TempDir()
	appDir = filepath.Join(tmp, "app")
	depDir := filepath.Join(tmp, "vendor", "example.com", "dep")
	overlayDir := filepath.Join(tmp, "overlay")
	for _, dir := range []string{appDir, depDir, overlayDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(appDir, "go.mod"), []byte(`module example.com/app

go 1.18

require example.com/dep v0.0.0

replace example.com/dep => ../vendor/example.com/dep
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.go"), []byte("package app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(depDir, "hello.go"), []byte("package dep\n\nfunc Hello() string { return \"real\" }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	depModPath := filepath.Join(overlayDir, "dep.mod")
	if err := os.WriteFile(depModPath, []byte("module example.com/dep\n\ngo 1.18\n"), 0644); err != nil {
		t.Fatal(err)
	}

	replace := map[string]string{}
	addOverlayPath(replace, filepath.Join(depDir, "go.mod"), depModPath)

	overlayJSON = filepath.Join(overlayDir, "overlay.json")
	payload, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlayJSON, payload, 0644); err != nil {
		t.Fatal(err)
	}
	return appDir, overlayJSON
}

func addOverlayPath(replace map[string]string, from, to string) {
	realTo := to
	if r, err := filepath.EvalSymlinks(to); err == nil {
		realTo = r
	}
	replace[from] = realTo
	// from may not exist on disk; resolve via parent dir.
	if parent, err := filepath.EvalSymlinks(filepath.Dir(from)); err == nil {
		replace[filepath.Join(parent, filepath.Base(from))] = realTo
	}
}
