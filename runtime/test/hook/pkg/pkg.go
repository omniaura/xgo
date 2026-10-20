package pkg

import (
	"github.com/omniaura/xgo/runtime/test/hook/id"
)

var ID int

func init() {
	ID = id.Next()
}
