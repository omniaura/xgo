package funcs

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
)

func ForEach(callback func(fn *ir.Func) bool) {
	for _, fn := range typecheck.Target.Funcs {
		if !callback(fn) {
			return
		}
	}
}
