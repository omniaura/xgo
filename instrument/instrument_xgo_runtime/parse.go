package instrument_xgo_runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ReplaceActualXgoVersion(versionCode string, xgoVersion string, xgoRevision string, xgoNumber int) string {
	versionCode = replaceByLine(versionCode, `const XGO_VERSION = `, `const XGO_VERSION = "`+xgoVersion+`"`)
	versionCode = replaceByLine(versionCode, `const XGO_REVISION = `, `const XGO_REVISION = "`+xgoRevision+`"`)
	versionCode = replaceByLine(versionCode, `const XGO_NUMBER = `, `const XGO_NUMBER = `+strconv.Itoa(xgoNumber))
	return versionCode
}

func InjectFlags(flagsCode string, collectTestTrace bool, collectTestTraceDir string, xgoRaceSafe bool) string {
	flagsCode = replaceByLine(flagsCode, `const COLLECT_TEST_TRACE = `, fmt.Sprintf(`const COLLECT_TEST_TRACE = %t`, collectTestTrace))
	flagsCode = replaceByLine(flagsCode, `const COLLECT_TEST_TRACE_DIR = `, fmt.Sprintf(`const COLLECT_TEST_TRACE_DIR = %q`, collectTestTraceDir))
	flagsCode = replaceByLine(flagsCode, `const XGO_RACE_SAFE = `, fmt.Sprintf(`const XGO_RACE_SAFE = %t`, xgoRaceSafe))
	return flagsCode
}

// replaceByLine allows re-entrant replacement
func replaceByLine(code string, linePattern string, replacement string) string {
	idx := strings.Index(code, linePattern)
	if idx == -1 {
		return code
	}
	base := idx + len(linePattern)
	endIdx := strings.Index(code[base:], "\n")
	if endIdx == -1 {
		return code
	}
	endIdx += base
	// this will include the \n
	return code[:idx] + replacement + code[endIdx:]
}

func hasFile(dir string, fileName string) string {
	filePath := filepath.Join(dir, fileName)
	fi, err := os.Stat(filePath)
	if err == nil && !fi.IsDir() {
		return filePath
	}
	return ""
}
