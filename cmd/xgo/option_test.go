package main

import (
	"testing"
)

func TestReleaseXgoHomeIsNotForwardedToTestBinary(t *testing.T) {
	opts, err := parseOptions("test", []string{"--xgo-home", "/tmp/isolated-xgo", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	if opts.xgoHome != "/tmp/isolated-xgo" || len(opts.remainArgs) != 1 || opts.remainArgs[0] != "./..." {
		t.Fatalf("xgo-home must be consumed by xgo: %+v", opts)
	}
}

func TestUseFilePatchesFlagIsAcceptedForCompatibility(t *testing.T) {
	for _, args := range [][]string{
		{"--use-file-patches", "./..."},
		{"--use-file-patches=true", "./..."},
	} {
		opts, err := parseOptions("build", args)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if len(opts.remainArgs) != 1 || opts.remainArgs[0] != "./..." {
			t.Fatalf("%v: flag must be consumed by xgo: %+v", args, opts.remainArgs)
		}
	}
	for _, arg := range []string{"--use-file-patches=false", "--use-file-patches=bad"} {
		if _, err := parseOptions("build", []string{arg, "./..."}); err == nil {
			t.Fatalf("%s: expected error", arg)
		}
	}
}
