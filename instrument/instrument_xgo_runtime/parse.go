package instrument_xgo_runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/omniaura/xgo/instrument/constants"
	"github.com/omniaura/xgo/instrument/load"
	"github.com/omniaura/xgo/instrument/overlay"
	"github.com/omniaura/xgo/support/goinfo"
	"github.com/omniaura/xgo/support/strutil"
)

func CheckRuntimeLegacyVersion(goroot string, projectDir string, overlayFS overlay.Overlay, mod string, modfile string, overlayFile string, goVersion *goinfo.GoVersion) (bool, string, error) {
	opts := load.LoadOptions{
		Dir:         projectDir,
		Overlay:     overlayFS,
		Mod:         mod,
		ModFile:     modfile,
		OverlayFile: overlayFile,
		Goroot:      goroot,
		GoVersion:   goVersion,
	}
	pkgs, err := load.LoadPackages([]string{
		constants.RUNTIME_CORE_PKG,
	}, opts)
	if err != nil {
		return false, "", err
	}
	var foundCorePkg *load.Package
	for _, pkg := range pkgs.Packages {
		if pkg.GoPackage.ImportPath == constants.RUNTIME_CORE_PKG {
			foundCorePkg = pkg
			break
		}
	}
	if foundCorePkg == nil || foundCorePkg.GoPackage.Incomplete {
		return false, "", nil
	}
	var foundFile *load.File
	for _, file := range foundCorePkg.Files {
		if file.Name == constants.VERSION_FILE {
			foundFile = file
			break
		}
	}
	if foundFile == nil {
		return false, "", nil
	}
	coreVersion, err := ParseCoreVersion(strutil.ToReadonlyString(foundFile.Content))
	if err != nil {
		return false, "", err
	}
	if !isDeprecatedCoreVersion(coreVersion) {
		return false, coreVersion, nil
	}

	return true, coreVersion, nil
}

// MIN_CORE_VERSION is the oldest xgo/runtime core xgo links as is. Older
// runtimes are swapped for the embedded one: their xgo_trap_template.go still
// needs instantiating (go-version placeholders, appended trap-skip and
// function name helpers, entry guards), which core 1.3.0 templates carry
// already. Keep it at or below CORE_VERSION in cmd/xgo/version.go.
const MIN_CORE_VERSION = "1.3.0"

func isDeprecatedCoreVersion(coreVersion string) bool {
	return goinfo.CompareSemVer("v"+coreVersion, "v"+MIN_CORE_VERSION) < 0
}

func ReplaceActualXgoVersion(versionCode string, xgoVersion string, xgoRevision string, xgoNumber int) string {
	versionCode = replaceByLine(versionCode, `const XGO_VERSION = `, `const XGO_VERSION = "`+xgoVersion+`"`)
	versionCode = replaceByLine(versionCode, `const XGO_REVISION = `, `const XGO_REVISION = "`+xgoRevision+`"`)
	versionCode = replaceByLine(versionCode, `const XGO_NUMBER = `, `const XGO_NUMBER = `+strconv.Itoa(xgoNumber))
	return versionCode
}

func ParseCoreVersion(versionCode string) (string, error) {
	anchor := `const VERSION =`
	idx := strings.Index(versionCode, anchor)
	if idx < 0 {
		return "", fmt.Errorf("VERSION not found")
	}
	idx += len(anchor)
	endIdx := strings.Index(versionCode[idx:], "\n")
	if endIdx < 0 {
		return "", fmt.Errorf("VERSION not found")
	}
	endIdx += idx
	versionStr := strings.TrimSpace(versionCode[idx:endIdx])
	ver, err := strconv.Unquote(versionStr)
	if err != nil {
		return "", fmt.Errorf("parse VERSION: %w", err)
	}
	if ver == "" {
		return "", fmt.Errorf("VERSION is empty")
	}
	return ver, nil
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
