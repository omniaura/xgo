module github.com/omniaura/xgo/runtime/test/core/instrumented

go 1.18

require (
	example.com/xgo-initprobe v0.0.0
	github.com/omniaura/xgo/runtime v1.1.0
)

replace github.com/omniaura/xgo/runtime => ../../../

replace example.com/xgo-initprobe => ./initprobe
