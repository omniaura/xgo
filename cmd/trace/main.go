package main

import (
	"os"

	"github.com/omniaura/xgo/cmd/xgo/trace"
)

func main() {
	args := os.Args[1:]
	trace.Main(args)
}
