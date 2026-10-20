package link

import (
	"cmd/compile/internal/ir"
)

func typecheckLinkedName(sym *ir.Name) {
	// go1.22 and above does not need to call typecheck
}
