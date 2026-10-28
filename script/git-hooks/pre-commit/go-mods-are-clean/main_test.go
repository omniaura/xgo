package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckoutNamedRuntimeStillClassifiesModulesByRelativePath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime")
	if err := os.MkdirAll(filepath.Join(root, "runtime"), 0700); err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(root, "go.mod")
	rt := filepath.Join(root, "runtime", "go.mod")
	dependency := []byte("module example.com/test\ngo 1.27.0\nrequire github.com/spf13/cobra v1.10.2\n")
	if err := os.WriteFile(cli, dependency, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rt, dependency, 0600); err != nil {
		t.Fatal(err)
	}
	if err := checkGoMod(root, cli); err != nil {
		t.Fatal(err)
	}
	if err := checkGoMod(root, rt); err == nil {
		t.Fatal("runtime accepted tooling dependencies")
	}
}

func TestCLIToolingDependenciesDoNotLeakIntoRuntime(t *testing.T) {
	tooling := "module github.com/omniaura/xgo\ngo 1.27.0\nrequire github.com/spf13/cobra v1.10.2\n"
	if err := checkCLIGoModContent(tooling); err != nil {
		t.Fatal(err)
	}
	if err := checkGoModContent("runtime/go.mod", tooling); err == nil {
		t.Fatal("runtime accepted CLI dependencies")
	}
	for _, directive := range []string{"replace foo => ../foo", "exclude foo v1.0.0", "require github.com/omniaura/xgo/runtime v1.6.0"} {
		if err := checkCLIGoModContent(tooling + directive + "\n"); err == nil {
			t.Fatalf("CLI accepted %s", directive)
		}
	}
}

func TestCheckGoModContent_Clean(t *testing.T) {
	cases := []string{
		"module github.com/omniaura/xgo\n\ngo 1.18\n",
		"module github.com/omniaura/xgo/runtime\n\ngo 1.14\n",
		"module foo\n\n",
		"module foo\ngo 1.21\n",
		"module foo\ngo 1.21\ntoolchain go1.22.0\n",
	}

	for i, content := range cases {
		err := checkGoModContent("go.mod", content)
		if err != nil {
			t.Errorf("case %d: expected clean, got error: %v\ncontent:\n%s", i, err, content)
		}
	}
}

func TestCheckGoModContent_Dirty_Require_SingleLine(t *testing.T) {
	content := "module github.com/omniaura/xgo\n\ngo 1.18\n\nrequire github.com/foo/bar v1.0.0\n"
	err := checkGoModContent("go.mod", content)
	if err == nil {
		t.Fatal("expected error for require directive")
	}
	if !strings.Contains(err.Error(), "require") {
		t.Fatalf("error should mention require, got: %v", err)
	}
}

func TestCheckGoModContent_Dirty_Require_Block(t *testing.T) {
	content := `module github.com/omniaura/xgo

go 1.18

require (
	github.com/foo/bar v1.0.0
)
`
	err := checkGoModContent("go.mod", content)
	if err == nil {
		t.Fatal("expected error for require block directive")
	}
	if !strings.Contains(err.Error(), "require") {
		t.Fatalf("error should mention require, got: %v", err)
	}
}

func TestCheckGoModContent_Dirty_Replace(t *testing.T) {
	content := `module github.com/omniaura/xgo

go 1.18

replace github.com/foo/bar => ../bar
`
	err := checkGoModContent("go.mod", content)
	if err == nil {
		t.Fatal("expected error for replace directive")
	}
	if !strings.Contains(err.Error(), "replace") {
		t.Fatalf("error should mention replace, got: %v", err)
	}
}

func TestCheckGoModContent_Dirty_Exclude(t *testing.T) {
	content := `module github.com/omniaura/xgo

go 1.18

exclude github.com/foo/bar v1.0.0
`
	err := checkGoModContent("go.mod", content)
	if err == nil {
		t.Fatal("expected error for exclude directive")
	}
	if !strings.Contains(err.Error(), "exclude") {
		t.Fatalf("error should mention exclude, got: %v", err)
	}
}

func TestCheckGoModContent_Dirty_Retract(t *testing.T) {
	content := `module github.com/omniaura/xgo

go 1.18

retract (
	v1.0.0
)
`
	err := checkGoModContent("go.mod", content)
	if err == nil {
		t.Fatal("expected error for retract directive")
	}
	if !strings.Contains(err.Error(), "retract") {
		t.Fatalf("error should mention retract, got: %v", err)
	}
}

func TestCheckGoModContent_Empty(t *testing.T) {
	err := checkGoModContent("go.mod", "")
	if err != nil {
		t.Fatalf("empty file should be clean, got: %v", err)
	}
}

func TestCheckGoModContent_OnlyWhitespace(t *testing.T) {
	err := checkGoModContent("go.mod", "\n\n  \n\t\n")
	if err != nil {
		t.Fatalf("whitespace-only file should be clean, got: %v", err)
	}
}

func TestCLIRuntimeModuleBoundaries(t *testing.T) {
	for _, module := range []string{"github.com/omniaura/xgo/runtime", "github.com/omniaura/xgo/runtime/submodule"} {
		if err := checkCLIGoModContent("require " + module + " v1.0.0"); err == nil {
			t.Fatalf("accepted %s", module)
		}
	}
	for _, line := range []string{"require github.com/omniaura/xgo/runtimeclient v1.0.0", "require example.com/foo v1.0.0 // github.com/omniaura/xgo/runtime"} {
		if err := checkCLIGoModContent(line); err != nil {
			t.Fatal(err)
		}
	}
}
