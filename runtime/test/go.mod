module github.com/omniaura/xgo/runtime/test

go 1.27.0

require (
	github.com/omniaura/xgo v0.0.0
	github.com/omniaura/xgo/runtime v1.1.0
)

replace (
	github.com/omniaura/xgo => ../..
	github.com/omniaura/xgo/runtime => ../
)
