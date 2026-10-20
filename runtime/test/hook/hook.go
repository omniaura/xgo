package hook

import (
	"github.com/omniaura/xgo/runtime/test/hook/id"
	_ "github.com/omniaura/xgo/runtime/test/hook/pkg"
)

var mainInitID int

func init() {
	mainInitID = id.Next()
}
