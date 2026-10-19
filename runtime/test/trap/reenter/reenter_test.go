package reenter

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/xhd2015/xgo/runtime/core"
	"github.com/xhd2015/xgo/runtime/mock"
	"github.com/xhd2015/xgo/runtime/trap"
)

// When an interceptor has a post, an instrumented function hands over to a
// helper that defers post and calls the function again with the trap
// skipped once, instead of deferring post itself.

var bodyRuns int32

func work(s string, n int) (string, error) {
	atomic.AddInt32(&bodyRuns, 1)
	if n < 0 {
		return "", errors.New("negative")
	}
	return strings.Repeat(s, n), nil
}

func boom(msg string) int {
	panic(msg)
}

func sum(base int, xs ...int) int {
	for _, x := range xs {
		base += x
	}
	return base
}

type val struct{ n int }

func (v val) get(k int) int  { return v.n + k }
func (v *val) inc(k int) int { v.n += k; return v.n }
func shadow(shadow int) int  { return shadow * 2 }
func fact(n int) int {
	if n <= 1 {
		return 1
	}
	return n * fact(n-1)
}
func noArgs() { atomic.AddInt32(&bodyRuns, 1) }
func caller() string {
	_, file, line, _ := runtime.Caller(1)
	return fmt.Sprintf("%s:%d", file[strings.LastIndex(file, "/")+1:], line)
}

func addPost(t *testing.T, fn interface{}, post func(args, results core.Object)) *int32 {
	var calls int32
	t.Cleanup(trap.AddFuncInterceptor(fn, &trap.Interceptor{
		Post: func(ctx context.Context, f *core.FuncInfo, args core.Object, results core.Object, data interface{}) error {
			atomic.AddInt32(&calls, 1)
			if post != nil {
				post(args, results)
			}
			return nil
		},
	}))
	return &calls
}

func TestBodyRunsOnceAndPostSeesResults(t *testing.T) {
	var got string
	calls := addPost(t, work, func(args, results core.Object) {
		got = fmt.Sprintf("%v %v %v", args.GetFieldIndex(0).Value(), args.GetFieldIndex(1).Value(), results.GetFieldIndex(0).Value())
	})
	atomic.StoreInt32(&bodyRuns, 0)
	if s, err := work("ab", 2); s != "abab" || err != nil {
		t.Fatalf("work = %q, %v", s, err)
	}
	if n := atomic.LoadInt32(&bodyRuns); n != 1 {
		t.Fatalf("body ran %d times, want 1", n)
	}
	if *calls != 1 || got != "ab 2 abab" {
		t.Fatalf("post calls %d, saw %q", *calls, got)
	}
}

func TestPostChangesResults(t *testing.T) {
	addPost(t, work, func(args, results core.Object) {
		results.GetFieldIndex(0).Set("changed")
		results.GetFieldIndex(1).Set(error(nil))
	})
	if s, err := work("x", -1); s != "changed" || err != nil {
		t.Fatalf("work = %q, %v, want changed, nil", s, err)
	}
}

func TestStopWithPost(t *testing.T) {
	mock.Patch(work, func(s string, n int) (string, error) { return "mocked", nil })
	var seen interface{}
	calls := addPost(t, work, func(args, results core.Object) { seen = results.GetFieldIndex(0).Value() })
	atomic.StoreInt32(&bodyRuns, 0)
	if s, _ := work("a", 1); s != "mocked" {
		t.Fatalf("work = %q, want mocked", s)
	}
	if n := atomic.LoadInt32(&bodyRuns); n != 0 {
		t.Fatalf("mocked body ran %d times", n)
	}
	if *calls != 1 || seen != "mocked" {
		t.Fatalf("post calls %d, saw %v", *calls, seen)
	}
}

func TestPostRunsOnPanic(t *testing.T) {
	calls := addPost(t, boom, nil)
	func() {
		defer func() {
			if r := recover(); r != "bang" {
				t.Fatalf("recovered %v, want bang", r)
			}
		}()
		boom("bang")
	}()
	if *calls != 1 {
		t.Fatalf("post ran %d times on panic, want 1", *calls)
	}
	// the goroutine must not be left skipping its next trap
	calls2 := addPost(t, sum, nil)
	if sum(1, 2) != 3 || *calls2 != 1 {
		t.Fatalf("after panic: post calls %d", *calls2)
	}
}

func TestVariadicAndReceivers(t *testing.T) {
	addPost(t, sum, nil)
	if got := sum(1, 2, 3); got != 6 {
		t.Fatalf("sum = %d", got)
	}
	if got := sum(1); got != 1 {
		t.Fatalf("sum() = %d", got)
	}
	v := &val{n: 10}
	addPost(t, val.get, nil)
	addPost(t, (*val).inc, func(args, results core.Object) {
		results.GetFieldIndex(0).Set(results.GetFieldIndex(0).Value().(int) + 100)
	})
	if got := v.get(1); got != 11 {
		t.Fatalf("get = %d", got)
	}
	if got := v.inc(5); got != 115 || v.n != 15 {
		t.Fatalf("inc = %d, v.n = %d", got, v.n)
	}
}

func TestShadowedNameKeepsDefer(t *testing.T) {
	addPost(t, shadow, func(args, results core.Object) { results.GetFieldIndex(0).Set(-1) })
	if got := shadow(3); got != -1 {
		t.Fatalf("shadow = %d, want -1", got)
	}
}

func TestRecursion(t *testing.T) {
	calls := addPost(t, fact, nil)
	if got := fact(5); got != 120 {
		t.Fatalf("fact = %d", got)
	}
	if *calls != 5 {
		t.Fatalf("post ran %d times, want 5", *calls)
	}
}

func TestNoArgs(t *testing.T) {
	calls := addPost(t, noArgs, nil)
	atomic.StoreInt32(&bodyRuns, 0)
	noArgs()
	if atomic.LoadInt32(&bodyRuns) != 1 || *calls != 1 {
		t.Fatalf("body %d, post %d", bodyRuns, *calls)
	}
}

func TestConcurrentGoroutines(t *testing.T) {
	calls := addPost(t, work, nil)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if s, _ := work("z", i%4); len(s) != i%4 {
					t.Errorf("work len %d, want %d", len(s), i%4)
				}
			}
		}(i)
	}
	wg.Wait()
	if *calls != 3200 {
		t.Fatalf("post ran %d times, want 3200", *calls)
	}
}

func TestCallerWithoutPost(t *testing.T) {
	mock.Patch(work, func(s string, n int) (string, error) { return "m", nil })
	if got, want := caller(), "reenter_test.go:"+fmt.Sprint(lineOfCall()); got != want {
		t.Fatalf("caller = %s, want %s", got, want)
	}
}

func lineOfCall() int {
	_, _, line, _ := runtime.Caller(1)
	return line
}

func frames() []string {
	pcs := make([]uintptr, 16)
	n := runtime.Callers(1, pcs)
	fs := runtime.CallersFrames(pcs[:n])
	var names []string
	for {
		f, more := fs.Next()
		names = append(names, f.Function[strings.LastIndex(f.Function, ".")+1:])
		if !more {
			break
		}
	}
	return names
}

// The helper that calls a function again is a wrapper, left out of stacks
// like the compiler's defer wrappers: the body sees itself called by the
// function, then by its real caller, so t.Helper and the like still find
// the caller.
func TestReenterHelperNotInStack(t *testing.T) {
	addPost(t, frames, nil)
	names := frames()
	for _, n := range names {
		if strings.Contains(n, "__xgo") {
			t.Fatalf("stack has %s: %v", n, names)
		}
	}
	if len(names) < 3 || names[0] != "frames" || names[1] != "frames" || names[2] != "TestReenterHelperNotInStack" {
		t.Fatalf("stack = %v, want frames, frames, TestReenterHelperNotInStack, ...", names)
	}
}
