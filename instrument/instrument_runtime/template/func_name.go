package template

import (
	"github.com/xhd2015/xgo/support/goinfo"
)

func AppendGetFuncNameImpl(goVersion *goinfo.GoVersion, content string) string {
	// func name patch
	if goVersion.Major > goinfo.GO_MAJOR_1 || goVersion.Minor > goinfo.GO_VERSION_28 {
		panic("should check the implementation of runtime.FuncForPC(pc).Name() to ensure __xgo_get_pc_name is not wrapped in print format above go1.28,it is confirmed that in go1.21~go1.28 the name is wrapped in funcNameForPrint(...).")
	}
	return content + RuntimeGetFuncName_Go121
}
