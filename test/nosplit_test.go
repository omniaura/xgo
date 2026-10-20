package test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xhd2015/xgo/support/cmd"
	"github.com/xhd2015/xgo/support/strutil"
)

// go test -run TestLongFuncNoSplitShouldNotCompileWithDebugFlags -v ./test
func TestLongFuncNoSplitShouldNotCompileWithDebugFlags(t *testing.T) {
	var errBuf bytes.Buffer
	buildErr := cmd.Dir(filepath.Join("testdata", "nosplit", "longfunc")).
		Stderr(&errBuf).
		Stdout(&errBuf).
		Run("go", "build", "-gcflags=all=-N -l", "-o", "/dev/null", "./")

	if buildErr == nil {
		t.Fatalf("expect build fail")
	}
	errOutput := errBuf.String()

	// t.Logf("output: %s", errOutput)
	expects := []string{"main.longFunc: nosplit stack over 792 byte limit"}

	idx := strutil.IndexSequence(errOutput, expects)
	if idx < 0 {
		t.Fatalf("expect contains %v, actual: %s", expects, errOutput)
	}
}

// go test -run TestSmallFuncNoSplitShouldCompileWithDebugFlagsWithGo -v ./test
func TestSmallFuncNoSplitShouldCompileWithDebugFlagsWithGo(t *testing.T) {
	output, err := cmd.Dir(filepath.Join("testdata", "nosplit", "shortfunc")).
		Output("go", "run", "-gcflags=all=-N -l", "./")
	if err != nil {
		t.Fatal(err)
	}

	output = strings.TrimSpace(output)
	expect := "hello nosplit:<nil>"
	if output != expect {
		t.Fatalf("expect output: %q, actual: %q", expect, output)
	}
}
