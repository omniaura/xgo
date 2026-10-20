package exec_tool

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func isCompileCommand(cmd string) bool {
	// on Windows, cmd ends with .exe
	baseName := filepath.Base(cmd)
	if baseName == "compile" {
		return true
	}
	if runtime.GOOS == "windows" && baseName == "compile.exe" {
		return true
	}
	return false
}

func hasFlag(args []string, flag string) bool {
	flagEq := flag + "="
	for _, arg := range args {
		if arg == flag || strings.HasPrefix(arg, flagEq) {
			return true
		}
	}
	return false
}

func findArgAfterFlag(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag {
			if i+1 < len(args) {
				return args[i+1]
			}
		}
	}
	return ""
}

func runCommandExit(name string, args []string) {
	runCustomizeCommandExit(name, args, nil)
}
func runCustomizeCommandExit(name string, args []string, f func(cmd *exec.Cmd)) {
	err := runCommand(name, args, true, f)
	if err != nil {
		panic(fmt.Errorf("unexpected err: %w", err))
	}
}
func runCommand(name string, args []string, exit bool, f func(cmd *exec.Cmd)) error {
	// invoke the process as is
	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if f != nil {
		f(cmd)
	}

	err := cmd.Run()
	if err != nil {
		if exit {
			os.Exit(cmd.ProcessState.ExitCode())
		}
		return err
	}
	return nil
}
