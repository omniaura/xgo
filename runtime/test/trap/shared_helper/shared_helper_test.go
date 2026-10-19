package shared_helper

import (
	"context"
	"strings"
	"testing"

	"github.com/xhd2015/xgo/runtime/core"
	"github.com/xhd2015/xgo/runtime/mock"
	"github.com/xhd2015/xgo/runtime/trap"
)

// Functions of one file whose receiver, parameters and results have the same
// types share one trap helper. Each must still be intercepted on its own,
// with its own parameters and results.

type counter struct{ n int }

func (c *counter) add(k int) (int, error) { c.n += k; return c.n, nil }
func (c *counter) sub(k int) (int, error) { c.n -= k; return c.n, nil }

func double(s string, n int) (r string) {
	for i := 0; i < n; i++ {
		r += s
	}
	return
}

func triple(s string, n int) (r string) {
	return double(s, 3*n)
}

func TestSharedHelperMocksEachFunction(t *testing.T) {
	mock.Patch(double, func(s string, n int) string { return "mocked" })
	if got := double("a", 2); got != "mocked" {
		t.Fatalf("double = %q, want mocked", got)
	}
	mock.Patch(triple, func(s string, n int) string { return "tripled:" + s })
	if got := triple("b", 1); got != "tripled:b" {
		t.Fatalf("triple = %q, want tripled:b", got)
	}
}

func TestSharedHelperKeepsReceivers(t *testing.T) {
	c := &counter{}
	mock.Patch(c.sub, func(k int) (int, error) { return -100, nil })
	if n, _ := c.add(5); n != 5 {
		t.Fatalf("add = %d, want 5", n)
	}
	if n, _ := c.sub(1); n != -100 {
		t.Fatalf("sub = %d, want -100", n)
	}
	if c.n != 5 {
		t.Fatalf("c.n = %d, want 5", c.n)
	}
}

func TestSharedHelperPostChangesResults(t *testing.T) {
	var seen []string
	trap.AddFuncInterceptor(double, &trap.Interceptor{
		Post: func(ctx context.Context, f *core.FuncInfo, args core.Object, result core.Object, data interface{}) error {
			seen = append(seen, args.GetFieldIndex(0).Value().(string)+"="+result.GetFieldIndex(0).Value().(string))
			result.GetFieldIndex(0).Set("post")
			return nil
		},
	})
	if got := double("x", 2); got != "post" {
		t.Fatalf("double = %q, want post", got)
	}
	// triple shares the helper but has no interceptor: its call to double
	// is intercepted, its own result is not
	if got := triple("y", 1); got != "post" {
		t.Fatalf("triple = %q, want post", got)
	}
	if want := "x=xx,y=yyy"; strings.Join(seen, ",") != want {
		t.Fatalf("post saw %v, want %s", seen, want)
	}
}
