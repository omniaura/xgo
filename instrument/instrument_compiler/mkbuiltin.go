package instrument_compiler

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/omniaura/xgo/instrument/build"
	"github.com/omniaura/xgo/instrument/patch"
	"github.com/omniaura/xgo/support/goinfo"
)

var CompilerRuntimeDefFile = patch.FilePath{"src", "cmd", "compile", "internal", "typecheck", "_builtin", "runtime.go"}

// cd src/cmd/compile/internal/typecheck && go run mkbuiltin.go
func MkBuiltin(origGoroot string, outputGoroot string, goVersion *goinfo.GoVersion, extraDefs string) error {
	err := PrepareRuntimeDefs(outputGoroot, goVersion, extraDefs)
	if err != nil {
		return err
	}

	// run mkbuiltin
	cmd := exec.Command(filepath.Join(origGoroot, "bin", "go"), "run", "mkbuiltin.go")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = GetMkBuiltinDir(outputGoroot, goVersion)
	cmd.Env = build.EnvForNative(os.Environ(), origGoroot)

	err = cmd.Run()
	if err != nil {
		return err
	}

	// Clean up .xgo directory that xgo (as drop-in go) may have created
	os.RemoveAll(filepath.Join(cmd.Dir, ".xgo"))

	return nil
}

func GetMkBuiltinDir(goroot string, goVersion *goinfo.GoVersion) string {
	return filepath.Join(goroot, "src", "cmd", "compile", "internal", "typecheck")
}

func PrepareRuntimeDefs(goRoot string, goVersion *goinfo.GoVersion, extraDef string) error {
	fullFile := CompilerRuntimeDefFile.JoinPrefix(goRoot)

	return patch.EditFile(fullFile, func(content string) (string, error) {
		content = patch.UpdateContent(content,
			`/*<begin extra_runtime_func>*/`, `/*<end extra_runtime_func>*/`,
			[]string{
				`var x86HasFMA bool`,
				`var armHasVFPv4 bool`,
				`var arm64HasATOMICS bool`,
			},
			2,
			patch.UpdatePosition_After,
			extraDef+"\n",
		)
		return content, nil
	})
}
