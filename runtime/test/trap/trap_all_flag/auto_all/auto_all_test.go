package auto_all

import (
	"context"
	"testing"

	"github.com/omniaura/xgo/runtime/core"
	"github.com/omniaura/xgo/runtime/trap"
	"golang.org/x/example/hello/reverse"
)

func TestAutoAll(t *testing.T) {
	var trapped bool
	trap.AddInterceptor(&trap.Interceptor{
		Pre: func(ctx context.Context, f *core.FuncInfo, args, result core.Object) (data interface{}, err error) {
			if f.Pkg == "golang.org/x/example/hello/reverse" && f.IdentityName == "String" {
				trapped = true
			}
			return nil, nil
		},
	})
	reverse.String("hello")
	if !trapped {
		t.Errorf("expect trapped, actually not trapped")
	}
}
