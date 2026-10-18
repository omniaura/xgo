package main

import (
	"reflect"
	"testing"

	"github.com/xhd2015/xgo/support/goinfo"
)

func TestTargetPatterns(t *testing.T) {
	tests := []struct {
		cmd  string
		args []string
		want []string
	}{
		{"test", nil, []string{"."}},
		{"test", []string{"-short", "./a", "./b/..."}, []string{"./a", "./b/..."}},
		{"test", []string{"./a", "-run", "TestX", "-v"}, []string{"./a"}},
		{"test", []string{"-count=1", "-p", "2", "./a"}, []string{"./a"}},
		{"test", []string{"--race", "-test.short", "./a"}, []string{"./a"}},
		{"build", []string{"-o", "out", "./cmd/x"}, []string{"./cmd/x"}},
		{"run", []string{"./cmd/x", "arg", "-flag"}, []string{"./cmd/x"}},
		{"run", []string{"main.go"}, nil},
		// an unknown flag may or may not take the next argument
		{"test", []string{"-somenewflag", "./a"}, nil},
	}
	for _, tt := range tests {
		if got := targetPatterns(tt.cmd, tt.args); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("targetPatterns(%q, %q) = %q, want %q", tt.cmd, tt.args, got, tt.want)
		}
	}
}

func TestNeededPackages(t *testing.T) {
	list := []*goinfo.Package{
		{ImportPath: "m/a", Match: []string{"m/...", "./a"}, Imports: []string{"m/b", "fmt"}, XTestImports: []string{"m/testutil"}},
		{ImportPath: "m/b", Match: []string{"m/..."}, Imports: []string{"m/c"}},
		{ImportPath: "m/c", Match: []string{"m/..."}},
		{ImportPath: "m/testutil", Match: []string{"m/..."}, Imports: []string{"m/d"}},
		{ImportPath: "m/d", Match: []string{"m/..."}},
		{ImportPath: "m/unrelated", Match: []string{"m/..."}, Imports: []string{"m/b"}},
		{ImportPath: "fmt", DepOnly: true},
	}
	targets, needed := neededPackages(list, []string{"./a"})
	if !reflect.DeepEqual(targets, map[string]bool{"m/a": true}) {
		t.Errorf("targets = %v", targets)
	}
	want := map[string]bool{"m/a": true, "m/b": true, "m/c": true, "m/testutil": true, "m/d": true, "fmt": true}
	if !reflect.DeepEqual(needed, want) {
		t.Errorf("needed = %v, want %v", needed, want)
	}
	if targets, needed := neededPackages(list, []string{"./a", "./missing"}); targets != nil || needed != nil {
		t.Errorf("a pattern matching nothing must disable the sets, got %v %v", targets, needed)
	}
	if targets, needed := neededPackages(list, nil); targets != nil || needed != nil {
		t.Errorf("no patterns must disable the sets")
	}
}
