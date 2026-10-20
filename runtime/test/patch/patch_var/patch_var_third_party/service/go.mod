module github.com/omniaura/xgo/runtime/test/patch/patch_var/patch_var_third_party/service

go 1.18

require (
	github.com/omniaura/xgo/runtime/test/patch/patch_var/patch_var_third_party/third v0.0.0
	github.com/omniaura/xgo/runtime v0.0.0
)

replace github.com/omniaura/xgo/runtime/test/patch/patch_var/patch_var_third_party/third => ../third

replace github.com/omniaura/xgo/runtime => ../../../../..
