package ctxt

import "cmd/compile/internal/types"

func GetPkgPath() string {
	return types.LocalPkg.Path
}
