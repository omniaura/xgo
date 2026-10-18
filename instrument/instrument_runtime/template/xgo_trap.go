package template

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/xhd2015/xgo/instrument/patch"
	"github.com/xhd2015/xgo/support/goinfo"
)

func InstantiateXgoTrap(xgoTrapTemplate string, goVersion *goinfo.GoVersion) (string, error) {
	// instrument xgo_trap.go
	xgoTrapContent, err := patch.RemoveBuildIgnore(xgoTrapTemplate)
	if err != nil {
		return "", fmt.Errorf("remove build ignore: %w", err)
	}
	// type _panic has a major upgrade from go1.21 to go1.22
	// go1.21 and before use pc, go1.22 and after use retpc
	retpc := "retpc"
	if goVersion.Major == 1 && goVersion.Minor <= 21 {
		retpc = "pc"
	}
	xgoTrapContent = strings.ReplaceAll(xgoTrapContent, "__RETPC__", retpc)
	xgoTrapContent = AppendGetFuncNameImpl(goVersion, xgoTrapContent)
	xgoTrapContent, err = AppendTrapSkipImpl(goVersion, xgoTrapContent)
	if err != nil {
		return "", err
	}

	return xgoTrapContent, nil
}

var runtimePkgClause = regexp.MustCompile(`(?m)^package runtime[ \t]*\r?\n`)

// AppendTrapSkipImpl adds XgoTrapOn and XgoSetTrapSkip to xgo_trap.go.
// They are generated here rather than kept in the template because the
// template comes from the xgo/runtime version of the project being built,
// which can predate them, while the instrumented code always links
// __xgo_trap_on_ to runtime.XgoTrapOn. Such a runtime never sets the skip,
// so its calls keep going through the trap. The runtime's atomic package
// moved in go1.23.
func AppendTrapSkipImpl(goVersion *goinfo.GoVersion, content string) (string, error) {
	if strings.Contains(content, "func XgoTrapOn()") {
		return content, nil
	}
	atomicPkg := "internal/runtime/atomic"
	if goVersion.Major == 1 && goVersion.Minor <= 22 {
		atomicPkg = "runtime/internal/atomic"
	}
	loc := runtimePkgClause.FindStringIndex(content)
	if loc == nil {
		return "", fmt.Errorf("xgo_trap.go: package clause not found")
	}
	importDecl := fmt.Sprintf("\nimport __xgo_atomic %q\n", atomicPkg)
	return content[:loc[1]] + importDecl + content[loc[1]:] + trapSkipImpl, nil
}

const trapSkipImpl = `
// __xgo_trap_skip is 1 while instrumented functions can skip calling
// XgoTrap, and building its arguments, because nothing could intercept the
// call. Only an xgo/runtime that turns it off again on first registration
// sets it. It is read on every instrumented call while registrations may
// run concurrently, hence atomic.
var __xgo_trap_skip uint32

// XgoTrapOn reports whether instrumented functions should call XgoTrap.
func XgoTrapOn() bool {
	return __xgo_atomic.Load(&__xgo_trap_skip) == 0
}

func XgoSetTrapSkip(skip bool) {
	var v uint32
	if skip {
		v = 1
	}
	__xgo_atomic.Store(&__xgo_trap_skip, v)
}
`
