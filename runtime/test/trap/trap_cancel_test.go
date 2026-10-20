package trap

import (
	"context"
	"testing"

	"github.com/omniaura/xgo/runtime/core"
	"github.com/omniaura/xgo/runtime/trap"
)

var cancel func()

func init() {
	cancel = trap.AddInterceptor(&trap.Interceptor{
		Pre: func(ctx context.Context, f *core.FuncInfo, args, result core.Object) (data interface{}, err error) {
			return
		},
	})
}

func TestCancelGlobalTrapShouldError(t *testing.T) {
	var panicked bool
	func() {
		defer func() {
			panicked = recover() != nil
		}()
		cancel()
	}()
	if !panicked {
		t.Fatalf("expect cancelling global interceptor should panic, actually not")
	}
}
