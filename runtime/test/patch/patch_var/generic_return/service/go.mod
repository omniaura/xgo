module github.com/omniaura/xgo/runtime/test/patch/patch_var/generic_return/service

go 1.18

require (
	github.com/omniaura/xgo/runtime v0.0.0
	github.com/omniaura/xgo/runtime/test/patch/patch_var/generic_return/third v0.0.0
)

replace github.com/omniaura/xgo/runtime/test/patch/patch_var/generic_return/third => ../third

replace github.com/omniaura/xgo/runtime => ../../../../..
