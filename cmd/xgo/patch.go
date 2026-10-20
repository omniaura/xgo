package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/omniaura/xgo/cmd/xgo/asset"
	embedutil "github.com/omniaura/xgo/instrument/embed"
	"github.com/omniaura/xgo/instrument/instrument_compiler"
	"github.com/omniaura/xgo/instrument/patch"
	"github.com/omniaura/xgo/support/filecopy"
	"github.com/omniaura/xgo/support/goinfo"
	"github.com/omniaura/xgo/support/osinfo"
)

// patchRuntime applies the file-based patches for goVersion to goroot.
// the patch should be idempotent
// the origGoroot is used to generate runtime defs, see https://github.com/xhd2015/xgo/issues/4#issuecomment-2017880791
func patchRuntime(origGoroot string, goroot string, xgoSrc string, goVersion *goinfo.GoVersion, resetOrCoreRevisionChanged bool, skipRebuildCompilerAndGo bool) error {
	if goroot == "" {
		return fmt.Errorf("requires goroot")
	}
	if isDevelopment && xgoSrc == "" {
		return fmt.Errorf("requires xgoSrc")
	}
	if !isDevelopment && !resetOrCoreRevisionChanged {
		return nil
	}

	var patchDir string
	if isDevelopment {
		patchDir = filepath.Join(xgoSrc, "patches", fmt.Sprintf("go%d.%d", goVersion.Major, goVersion.Minor))
	} else {
		var err error
		patchDir, err = os.MkdirTemp("", "xgo-patches-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(patchDir)
		if err := embedutil.CopyDir(asset.PatchesFS, asset.Patches, patchDir, embedutil.CopyOptions{}); err != nil {
			return fmt.Errorf("extract patches: %w", err)
		}
		patchDir = filepath.Join(patchDir, fmt.Sprintf("go%d.%d", goVersion.Major, goVersion.Minor))
		filepath.WalkDir(patchDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".go.txt") {
				return os.Rename(path, strings.TrimSuffix(path, ".txt"))
			}
			return nil
		})
		xgoSrc, err = os.MkdirTemp("", "xgo-src-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(xgoSrc)
		if err := embedutil.CopyDir(asset.CompilerPatchGenFS, asset.CompilerPatchGen, filepath.Join(xgoSrc, "patch"), embedutil.CopyOptions{}); err != nil {
			return fmt.Errorf("extract compiler patch: %w", err)
		}
	}
	if _, err := os.Stat(patchDir); os.IsNotExist(err) {
		return fmt.Errorf("file-based patch directory not found: %s", patchDir)
	}

	extraEnv := map[string]string{
		"XGO_SRC":           xgoSrc,
		"INSTRUMENT_GOROOT": goroot,
		"ORIG_GOROOT":       origGoroot,
		"GO_VERSION":        fmt.Sprintf("go%d.%d.%d", goVersion.Major, goVersion.Minor, goVersion.Patch),
		"GOOS":              runtime.GOOS,
		"GOARCH":            runtime.GOARCH,
	}
	var skipKinds []string
	if skipRebuildCompilerAndGo {
		skipKinds = []string{"rebuild-compiler", "rebuild-go"}
	}
	return patch.ApplyPatches(patchDir, goroot, xgoSrc, extraEnv, skipKinds, mkbuiltinHandler)
}

func checkRevisionChanged(coreRevisionFile string, currentCoreRevision string) (bool, error) {
	savedCoreRevision, err := readOrEmpty(coreRevisionFile)
	if err != nil {
		return false, err
	}
	logDebug("current core revision: %s, last core revision: %s from file %s", currentCoreRevision, savedCoreRevision, coreRevisionFile)
	if savedCoreRevision == "" || savedCoreRevision != currentCoreRevision {
		return true, nil
	}
	return false, nil
}

func readOrEmpty(file string) (string, error) {
	version, err := os.ReadFile(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	s := string(version)
	s = strings.TrimSuffix(s, "\n")
	s = strings.TrimSuffix(s, "\r")
	return s, nil
}

// syncGoroot copies goroot to instrumentGoroot
// NOTE: flagA never cause goroot to reset
// syncGoroot mirrors goroot into instrumentGoroot. With link set, files are
// hard links to goroot instead of copies, which saves the size of a whole
// GOROOT per instrumented toolchain. Only use it when every later write into
// instrumentGoroot first unshares its target, as patch.ApplyPatches does.
func syncGoroot(goroot string, instrumentGoroot string, fullSyncRecordFile string, link bool) error {
	// check if src goroot has src/runtime
	srcRuntimeDir := filepath.Join(goroot, "src", "runtime")
	err := assertDir(srcRuntimeDir)
	if err != nil {
		return err
	}
	srcGoBin := filepath.Join(goroot, "bin", "go"+osinfo.EXE_SUFFIX)
	dstGoBin := filepath.Join(instrumentGoroot, "bin", "go"+osinfo.EXE_SUFFIX)

	srcFile, err := os.Stat(srcGoBin)
	if err != nil {
		return err
	}
	if srcFile.IsDir() {
		return fmt.Errorf("bad goroot: %s", goroot)
	}

	dstFile, statErr := os.Stat(dstGoBin)
	if statErr != nil {
		if !os.IsNotExist(statErr) {
			return statErr
		}
	}

	var dstGoCopied bool
	if dstFile != nil && !dstFile.IsDir() {
		dstGoCopied = true
	}

	var onlyCopySrc bool
	if dstGoCopied && statNoErr(fullSyncRecordFile) {
		// full sync record does not yet exist
		onlyCopySrc = true
	}
	if onlyCopySrc {
		// do partial copy
		// keeps bin/ and pkg/tool, which may hold rebuilt tools that
		// --skip-rebuild-compiler-and-go would not rebuild again
		logDebug("partial copy $GOROOT/src")
		var err error
		if link {
			err = filecopy.LinkReplaceDir(filepath.Join(goroot, "src"), filepath.Join(instrumentGoroot, "src"))
		} else {
			err = copyGorootSrc(goroot, instrumentGoroot)
		}
		if err != nil {
			return err
		}
	} else {
		logDebug("fully copy $GOROOT and write %s", fullSyncRecordFile)
		rmErr := os.Remove(fullSyncRecordFile)
		if rmErr != nil {
			if !errors.Is(rmErr, os.ErrNotExist) {
				return rmErr
			}
		}

		// need copy, delete target dst dir first
		// TODO: use git worktree add if .git exists
		if link {
			err = filecopy.LinkReplaceDir(goroot, instrumentGoroot)
		} else {
			err = filecopy.NewOptions().
				Concurrent(1). // 10 is too much
				CopyReplaceDir(goroot, instrumentGoroot)
		}
		if err != nil {
			return err
		}

		// record this full sync
		copyTime := time.Now().Format("2006-01-02T15:04:05Z07:00")
		err = os.WriteFile(fullSyncRecordFile, []byte(copyTime), 0755)
		if err != nil {
			return err
		}
	}
	// change binary executable
	return nil
}

func copyGorootSrc(goroot string, instrumentGoroot string) error {
	return filecopy.CopyReplaceDir(filepath.Join(goroot, "src"), filepath.Join(instrumentGoroot, "src"), false)
}

func statNoErr(f string) bool {
	_, err := os.Stat(f)
	return err == nil
}

func mkbuiltinHandler(kind string, extraEnv map[string]string) error {
	switch kind {
	case "mkbuiltin":
		goVersion, err := goinfo.ParseGoVersion("go version " + extraEnv["GO_VERSION"] + " darwin/amd64")
		if err != nil {
			return fmt.Errorf("parse go version: %w", err)
		}
		return instrument_compiler.MkBuiltin(extraEnv["ORIG_GOROOT"], extraEnv["INSTRUMENT_GOROOT"], goVersion, instrument_compiler.RuntimeExtraDef)
	case "rebuild-compiler", "rebuild-go":
		return fmt.Errorf("internal: %q must be handled via cmd execution, not handler", kind)
	default:
		return fmt.Errorf("unknown generate kind: %q", kind)
	}
}
