package instrument_xgo_runtime

import (
	"errors"
	"fmt"
	"go/token"
	"os"

	"github.com/xhd2015/xgo/instrument/config"
	"github.com/xhd2015/xgo/instrument/constants"
	"github.com/xhd2015/xgo/instrument/edit"
	"github.com/xhd2015/xgo/instrument/instrument_func"
	"github.com/xhd2015/xgo/instrument/load"
	"github.com/xhd2015/xgo/instrument/overlay"
	"github.com/xhd2015/xgo/instrument/patch"
	"github.com/xhd2015/xgo/support/goinfo"
	"github.com/xhd2015/xgo/support/strutil"
)

var ErrLinkFileNotFound = errors.New("xgo: link file not found")
var ErrLinkFileNotRequired = errors.New("xgo: link file not required")
var ErrRuntimeVersionDeprecated = errors.New("runtime version deprecated")

type LinkOptions struct {
	Fset                *token.FileSet
	Mod                 string
	Modfile             string
	XgoVersion          string
	XgoRevision         string
	XgoNumber           int
	CollectTestTrace    bool
	CollectTestTraceDir string
	XgoRaceSafe         bool
	// OverlayFile is the caller go -overlay JSON (e.g. doctest vendor-gomod
	// bridges). Forwarded to go list so nested vendor go.mod replaces resolve.
	OverlayFile string

	ReadRuntimeGenFile func(path []string) ([]byte, error)
}

func LinkXgoRuntime(goroot string, projectDir string, xgoRuntimeModuleDir string, goVersion *goinfo.GoVersion, overlayFS overlay.Overlay, overrideContent func(absFile overlay.AbsFile, content string), linkOpts LinkOptions) (*edit.Packages, error) {
	fset := linkOpts.Fset
	mod := linkOpts.Mod
	modfile := linkOpts.Modfile
	xgoVersion := linkOpts.XgoVersion
	xgoRevision := linkOpts.XgoRevision
	xgoNumber := linkOpts.XgoNumber
	collectTestTrace := linkOpts.CollectTestTrace
	collectTestTraceDir := linkOpts.CollectTestTraceDir
	xgoRaceSafe := linkOpts.XgoRaceSafe
	readRuntimeGenFile := linkOpts.ReadRuntimeGenFile

	var opts load.LoadOptions

	if xgoRuntimeModuleDir != "" {
		// xgo runtime is replaced in a separate module
		// so we need to load packages from the separate module
		opts = load.LoadOptions{
			Dir: xgoRuntimeModuleDir,
			// files are instrumented from the contents overlayFS gives
			// them, e.g. the pristine trace.go
			Overlay:     overlayFS,
			Fset:        fset,
			OverlayFile: linkOpts.OverlayFile,
			Goroot:      goroot,
			GoVersion:   goVersion,
		}
	} else {
		opts = load.LoadOptions{
			Dir:         projectDir,
			Overlay:     overlayFS,
			Mod:         mod,
			ModFile:     modfile,
			Fset:        fset,
			OverlayFile: linkOpts.OverlayFile,
			Goroot:      goroot,
			GoVersion:   goVersion,
		}
	}
	packages, err := load.LoadPackages([]string{
		constants.RUNTIME_INTERNAL_RUNTIME_PKG,
		constants.RUNTIME_CORE_PKG,
		constants.RUNTIME_TRAP_FLAGS_PKG,
		constants.RUNTIME_FUNCTAB_PKG,
		constants.RUNTIME_MOCK_PKG,
		constants.RUNTIME_TRACE_PKG,
		constants.RUNTIME_TRAP_PKG,
	}, opts)
	if err != nil {
		// TODO: handle the case where error indicates the package is not found
		return nil, err
	}
	editPackages := edit.New(packages)
	var foundLink bool
	var foundMock bool
	var foundTrace bool
	var foundTrap bool
	var foundFunctabPkg bool
	var traceFile *edit.File
	var runtimeCoreVersion string

	// find version first
	var corePkg *edit.Package
	for _, pkg := range editPackages.Packages {
		goPkg := pkg.LoadPackage.GoPackage
		if goPkg.Incomplete {
			continue
		}
		if goPkg.ImportPath == constants.RUNTIME_CORE_PKG {
			corePkg = pkg
			break
		}
	}
	var versionFile *edit.File
	if corePkg != nil {
		for _, file := range corePkg.Files {
			if file.File.Name == constants.VERSION_FILE {
				versionFile = file
				break
			}
		}
	}

	if versionFile != nil {
		content := versionFile.File.Content
		absFile := overlay.AbsFile(versionFile.File.AbsPath)
		var err error
		runtimeCoreVersion, err = ParseCoreVersion(strutil.ToReadonlyString(content))
		if err != nil {
			return nil, err
		}
		if isDeprecatedCoreVersion(runtimeCoreVersion) {
			return nil, fmt.Errorf("%w: %s", ErrRuntimeVersionDeprecated, runtimeCoreVersion)
		}
		versionContent := ReplaceActualXgoVersion(strutil.ToReadonlyString(content), xgoVersion, xgoRevision, xgoNumber)
		versionContent = checkBypassVersionCheck(versionContent, runtimeCoreVersion)
		overrideContent(absFile, versionContent)
	}

	for _, pkg := range editPackages.Packages {
		goPkg := pkg.LoadPackage.GoPackage
		if goPkg.Incomplete {
			continue
		}
		importPath := goPkg.ImportPath
		suffixPkg, ok := goinfo.PkgWithinModule(importPath, constants.RUNTIME_MODULE)
		if !ok {
			continue
		}
		n := len(constants.RUNTIME_MODULE) + 1
		switch suffixPkg {
		case constants.RUNTIME_MOCK_PKG[n:]:
			foundMock = true
		case constants.RUNTIME_TRACE_PKG[n:]:
			foundTrace = true
		case constants.RUNTIME_TRAP_PKG[n:]:
			foundTrap = true
		case constants.RUNTIME_FUNCTAB_PKG[n:]:
			foundFunctabPkg = true
		}
		if suffixPkg == constants.RUNTIME_FUNCTAB_PKG[n:] ||
			suffixPkg == constants.RUNTIME_MOCK_PKG[n:] ||
			suffixPkg == constants.RUNTIME_TRAP_PKG[n:] {
			// only for lookup
			continue
		}
		if suffixPkg == constants.RUNTIME_INTERNAL_RUNTIME_PKG[n:] {
			var runtimeLinkFile *edit.File
			for _, file := range pkg.Files {
				switch file.File.Name {
				case constants.RUNTIME_LINK_FILE:
					foundLink = true
					runtimeLinkFile = file
				}
				if foundLink {
					break
				}
			}

			err := linkRuntimeTemplates(goroot, overlayFS, pkg.LoadPackage.GoPackage.Dir, runtimeLinkFile, readRuntimeGenFile, overrideContent)
			if err != nil {
				return nil, err
			}
			continue
		}
		for _, file := range pkg.Files {
			loadFile := file.File
			content := loadFile.Content
			absFile := overlay.AbsFile(loadFile.AbsPath)
			switch loadFile.Name {
			case constants.FLAG_FILE:
				if suffixPkg == constants.RUNTIME_TRAP_FLAGS_PKG[n:] && (collectTestTrace || collectTestTraceDir != "" || xgoRaceSafe) {
					flagsContent := InjectFlags(strutil.ToReadonlyString(content), collectTestTrace, collectTestTraceDir, xgoRaceSafe)
					overrideContent(absFile, flagsContent)
				}
			case constants.TRACE_FILE:
				if suffixPkg == constants.RUNTIME_TRACE_PKG[n:] {
					traceFile = file
				}
			}
		}
	}
	// found any usage of xgo public API, but does not found
	// link file, it means the runtime is not instrumented
	if !foundLink {
		if foundMock || foundTrace || foundTrap {
			// mock, trace and trap are public APIs of xgo/runtime
			return editPackages, ErrLinkFileNotFound
		}
		return editPackages, ErrLinkFileNotRequired
	}
	if foundFunctabPkg && traceFile != nil {
		// trap trace.go
		edit := traceFile.Edit
		funcInfos, extraFuncs := instrument_func.TrapFuncs(edit, constants.RUNTIME_TRACE_PKG, traceFile.File.Syntax, traceFile.Index, instrument_func.Options{
			// trap all funcs inside trace.go,
			// in reality there is only one func: Trace
			InstrumentMode: config.InstrumentMode_All,
			// force in place edit, which uses overlay
			ForceInPlace: true,
		})
		if len(extraFuncs) > 0 {
			panic(fmt.Errorf("instrument %s.%s: unexpected extra compiler-assisted func: %d", constants.RUNTIME_TRACE_PKG, "Trace", len(extraFuncs)))
		}
		if edit.HasEdit() {
			overrideContent(overlay.AbsFile(traceFile.File.AbsPath), edit.Buffer().String())
		}
		traceFile.TrapFuncs = funcInfos
	}
	return editPackages, nil
}

// linkRuntimeTemplates links the runtime templates:
//
//	runtime_link_template.go -> runtime_link.go
//	xgo_trap_template.go -> GOROOT/src/runtime/xgo_trap.go
//
// why these two files cannot be found by list?
// because they have special build tag:
//
//	'//go:build ignore'
//
// The templates are complete Go files; linking only drops that tag.
func linkRuntimeTemplates(goroot string, overlayFS overlay.Overlay, internalRuntimeDir string, runtimeLinkFile *edit.File, readRuntimeGenFile func(path []string) ([]byte, error), overrideContent func(absFile overlay.AbsFile, content string)) error {
	if runtimeLinkFile != nil {
		content, err := readRuntimeTemplate(internalRuntimeDir, constants.RUNTIME_LINK_TEMPLATE_PATH, overlayFS, readRuntimeGenFile)
		if err != nil {
			return err
		}
		// override: runtime_link_template.go -> runtime_link.go
		overrideContent(overlay.AbsFile(runtimeLinkFile.File.AbsPath), content)
	}

	// override file under GOROOT
	// by always use the one bind with runtime ensures the trap file's
	// API always matches the one that's expected by xgo/runtime
	content, err := readRuntimeTemplate(internalRuntimeDir, constants.RUNTIME_XGO_TRAP_TEMPLATE_PATH, overlayFS, readRuntimeGenFile)
	if err != nil {
		return err
	}
	// override: xgo_trap_template.go -> GOROOT/src/runtime/xgo_trap.go
	overlayFS.OverrideContent(overlay.AbsFile(constants.GetGoRuntimeXgoTrapFile(goroot)), content)
	return nil
}

func readRuntimeTemplate(internalRuntimeDir string, path []string, overlayFS overlay.Overlay, readRuntimeGenFile func(path []string) ([]byte, error)) (string, error) {
	content, err := readRuntimeFileFromDirOrGen(internalRuntimeDir, path, overlayFS, readRuntimeGenFile)
	if err != nil {
		return "", err
	}
	return patch.RemoveBuildIgnore(content)
}

func readRuntimeFileFromDirOrGen(internalRuntimeDir string, path []string, overlayFS overlay.Overlay, readRuntimeGenFile func(path []string) ([]byte, error)) (string, error) {
	templateFile := hasFile(internalRuntimeDir, path[len(path)-1])
	_, templateContent, readErr := overlayFS.Read(overlay.AbsFile(templateFile))
	if readErr == nil {
		// fmt.Fprintf(os.Stderr, "DEBUG readRuntimeFileFromDirOrGen read from fs: %s\n", templateFile)
		return templateContent, nil
	}
	if !os.IsNotExist(readErr) {
		return "", readErr
	}

	// fallback to the one that xgo embeds(this only happens in when transition from v1.1.0 to v1.1.1)
	templateContentBytes, err := readRuntimeGenFile(path)
	if err != nil {
		return "", err
	}
	// fmt.Fprintf(os.Stderr, "DEBUG readRuntimeFileFromDirOrGen read from embedded: %s\n", path)
	return string(templateContentBytes), nil
}
