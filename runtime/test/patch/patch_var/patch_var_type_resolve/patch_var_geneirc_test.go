package patch_var_type_resolve

import (
	"testing"

	"github.com/xhd2015/xgo/runtime/mock"
)

type GreetService[T string] struct {
}

func (s *GreetService[T]) Greet(name T) string {
	return "hello " + string(name)
}

func TestPatchVarGeneric(t *testing.T) {
	mock.Patch(&varRecv, func() *Service {
		return &Service{
			Hello: "mock",
		}
	})
}
