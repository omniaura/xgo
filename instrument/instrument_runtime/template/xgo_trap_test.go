package template

import (
	"strings"
	"testing"

	"github.com/xhd2015/xgo/support/goinfo"
)

func TestAppendTrapSkipImpl(t *testing.T) {
	const tmpl = "// comment\n\npackage runtime\n\nimport (\n\t\"unsafe\"\n)\n\nfunc XgoTrap() {}\n"
	for _, c := range []struct {
		minor  int
		atomic string
	}{
		{17, `import __xgo_atomic "runtime/internal/atomic"`},
		{22, `import __xgo_atomic "runtime/internal/atomic"`},
		{23, `import __xgo_atomic "internal/runtime/atomic"`},
		{27, `import __xgo_atomic "internal/runtime/atomic"`},
	} {
		got, err := AppendTrapSkipImpl(&goinfo.GoVersion{Major: 1, Minor: c.minor}, tmpl)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "package runtime\n\n"+c.atomic+"\n") {
			t.Errorf("go1.%d: atomic import missing after the package clause:\n%s", c.minor, got)
		}
		if strings.Count(got, "func XgoTrapOn(") != 1 || strings.Count(got, "func XgoSetTrapSkip(") != 1 {
			t.Errorf("go1.%d: expected XgoTrapOn and XgoSetTrapSkip once:\n%s", c.minor, got)
		}
		// a template that already defines them is kept as is
		again, err := AppendTrapSkipImpl(&goinfo.GoVersion{Major: 1, Minor: c.minor}, got)
		if err != nil || again != got {
			t.Errorf("go1.%d: appended twice", c.minor)
		}
	}
	if _, err := AppendTrapSkipImpl(&goinfo.GoVersion{Major: 1, Minor: 27}, "package main\n"); err == nil {
		t.Errorf("expected an error without the runtime package clause")
	}
	legacy := tmpl + "\nfunc XgoTrapOn() bool { return true }\n"
	if _, err := AppendTrapSkipImpl(&goinfo.GoVersion{Major: 1, Minor: 27}, legacy); err == nil {
		t.Errorf("expected an error for XgoTrapOn without the flag parameter")
	}
}
