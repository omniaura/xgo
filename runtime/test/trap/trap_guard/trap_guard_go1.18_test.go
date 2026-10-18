//go:build go1.18
// +build go1.18

package trap_guard

import (
	"testing"

	"github.com/xhd2015/xgo/runtime/mock"
)

func TestUnmockedGeneric(t *testing.T) {
	if v, ok := First(3, 4); !ok || v != 3 {
		t.Errorf("First: %v %v", v, ok)
	}
	if _, ok := First[int](); ok {
		t.Errorf("First of nothing")
	}
}

func TestPatchGenericAndVariadic(t *testing.T) {
	mock.Patch(First[string], func(items ...string) (string, bool) {
		return "generic", true
	})
	mock.Patch(Join, func(sep string, parts ...string) string {
		return "joined " + sep
	})
	if v, ok := First("a"); !ok || v != "generic" {
		t.Errorf("First: %q %v", v, ok)
	}
	if v, ok := First(1); !ok || v != 1 {
		t.Errorf("First[int] must not be mocked: %v %v", v, ok)
	}
	if got := Join("-", "a"); got != "joined -" {
		t.Errorf("Join: %q", got)
	}
}
