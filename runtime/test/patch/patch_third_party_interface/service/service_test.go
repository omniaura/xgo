package service

import (
	"testing"

	"github.com/omniaura/xgo/runtime/mock"
	"github.com/omniaura/xgo/runtime/test/patch/patch_third_party_interface/third"
)

func TestInterfaceMethodShouldNotPanic(t *testing.T) {
	isvc := third.NewGreetService()

	mock.Patch(isvc.Greet, func(name string) string {
		return "mock " + name
	})
	res := isvc.Greet("world")
	if res != "mock world" {
		t.Fatalf("expect patched result to be %q, actual: %q", "mock world", res)
	}
}
