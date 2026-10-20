package template

import (
	"strings"
	"testing"

	"github.com/xhd2015/xgo/support/goinfo"
)

func TestAppendGetFuncNameImpl(t *testing.T) {
	for minor := 27; minor <= 28; minor++ {
		got := AppendGetFuncNameImpl(&goinfo.GoVersion{Major: 1, Minor: minor}, "")
		if !strings.Contains(got, RuntimeGetFuncName_Go121) {
			t.Fatalf("go1.%d: missing function name implementation", minor)
		}
	}
}

func TestAppendGetFuncNameImplRequiresNextVersionAudit(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("go1.29 must require a runtime function name audit")
		}
	}()
	AppendGetFuncNameImpl(&goinfo.GoVersion{Major: 1, Minor: 29}, "")
}
