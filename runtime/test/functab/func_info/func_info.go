package main

import (
	"fmt"

	"github.com/omniaura/xgo/runtime/functab"
)

// go run ./cmd/xgo run --project-dir ./runtime ./test/func_info
func main() {
	funcInfo := functab.InfoFunc(example)
	if funcInfo == nil {
		panic(fmt.Errorf("func not found"))
	}

	fmt.Printf("identityName: %s\n", funcInfo.IdentityName)
	fmt.Printf("args: %v\n", funcInfo.ArgNames)
}

func example(a string) {

}
