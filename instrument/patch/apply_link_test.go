package patch

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/xhd2015/xgo/support/filecopy"
)

// TestApplyPatches_HardLinkedGorootKeepsOriginal checks that applying patches
// to a GOROOT whose files are hard links of the original never writes
// through to the original: patched files, copied files and generate outputs
// must all be unshared first.
func TestApplyPatches_HardLinkedGorootKeepsOriginal(t *testing.T) {
	origGoroot := t.TempDir()
	patchDir := t.TempDir()
	xgoSrc := t.TempDir()
	goroot := filepath.Join(t.TempDir(), "goroot")

	const patchedSrc = "package p\nfunc f() {\n}\n"
	const toolBin = "original compiler"
	mustMkdirAll(t, filepath.Join(origGoroot, "src", "p"))
	mustMkdirAll(t, filepath.Join(origGoroot, "pkg", "tool"))
	writeTestFile(t, filepath.Join(origGoroot, "src", "p", "f.go"), patchedSrc)
	writeTestFile(t, filepath.Join(origGoroot, "src", "p", "replaced.go"), "package p\n")
	writeTestFile(t, filepath.Join(origGoroot, "pkg", "tool", "compile"), toolBin)

	mustMkdirAll(t, filepath.Join(patchDir, "src", "p"))
	writeTestFile(t, filepath.Join(patchDir, "src", "p", "f.go.xgo.patch"), "<patch test>\ngoto func f\nmatch {\ninsert_after println()\n</patch>\n")
	writeTestFile(t, filepath.Join(patchDir, "src", "p", "replaced.go"), "package p // replaced\n")

	if runtime.GOOS == "windows" {
		t.Skip("generate command uses sh")
	}
	writeTestFile(t, filepath.Join(patchDir, "__config__.json"), `{
		"version": "test",
		"generate": [{"cmd": ["sh", "-c", "echo rebuilt > ${INSTRUMENT_GOROOT}/pkg/tool/compile"], "outputs": ["pkg/tool/compile"]}]
	}`)

	if err := filecopy.LinkReplaceDir(origGoroot, goroot); err != nil {
		t.Fatal(err)
	}
	assertSameFile(t, filepath.Join(origGoroot, "src", "p", "f.go"), filepath.Join(goroot, "src", "p", "f.go"), true)

	err := ApplyPatches(patchDir, goroot, xgoSrc, map[string]string{"INSTRUMENT_GOROOT": goroot}, nil, nil)
	if err != nil {
		t.Fatalf("ApplyPatches: %v", err)
	}

	for rel, want := range map[string]string{
		"src/p/f.go":        patchedSrc,
		"src/p/replaced.go": "package p\n",
		"pkg/tool/compile":  toolBin,
	} {
		got, err := os.ReadFile(filepath.Join(origGoroot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("original %s was modified: %q", rel, got)
		}
		assertSameFile(t, filepath.Join(origGoroot, filepath.FromSlash(rel)), filepath.Join(goroot, filepath.FromSlash(rel)), false)
	}
	got, err := os.ReadFile(filepath.Join(goroot, "pkg", "tool", "compile"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "rebuilt\n" {
		t.Errorf("generate output not written: %q", got)
	}
}

func assertSameFile(t *testing.T, a string, b string, want bool) {
	t.Helper()
	ai, err := os.Stat(a)
	if err != nil {
		t.Fatal(err)
	}
	bi, err := os.Stat(b)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(ai, bi) != want {
		t.Fatalf("SameFile(%s, %s) = %v, want %v", a, b, !want, want)
	}
}
