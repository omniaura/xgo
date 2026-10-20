package main

import (
	"context"
	"fmt"
	"os"

	"github.com/omniaura/xgo/runtime/core"
	"github.com/omniaura/xgo/runtime/trap"
)

var hasInstrument = os.Getenv("XGO_TEST_HAS_INSTRUMENT") != "false"

func init() {
	if hasInstrument {
		trap.AddInterceptor(&trap.Interceptor{
			Pre: func(ctx context.Context, f *core.FuncInfo, args core.Object, results core.Object) (interface{}, error) {
				trap.Skip()
				fmt.Printf("global trap: %s\n", f.Name)
				return nil, nil
			},
		})
	}
}

func main() {
	A()
	B()
}

func A() {
	// non instrument mode cannot get curg
	if hasInstrument {
		cancel := trap.AddInterceptor(&trap.Interceptor{
			Pre: func(ctx context.Context, f *core.FuncInfo, args core.Object, results core.Object) (interface{}, error) {
				trap.Skip()
				fmt.Printf("local trap from A: %s\n", f.Name)
				return nil, nil
			},
		})
		defer cancel()
	}
	printHello()
}

func B() {
	if hasInstrument {
		cancel := trap.AddInterceptor(&trap.Interceptor{
			Pre: func(ctx context.Context, f *core.FuncInfo, args core.Object, results core.Object) (interface{}, error) {
				trap.Skip()
				fmt.Printf("local trap from B: %s\n", f.Name)
				return nil, nil
			},
		})
		defer cancel()
	}
	printHello()
}

func printHello() {
	fmt.Printf("hello\n")
}
