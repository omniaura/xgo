module github.com/omniaura/xgo/runtime/test/build/modules_without_trap_all/service

go 1.18

require (
	github.com/omniaura/xgo/runtime v1.1.0
	github.com/omniaura/xgo/runtime/test/build/modules_without_trap_all/sub_a v0.0.0
)

require github.com/omniaura/xgo/runtime/test/build/modules_without_trap_all/sub_b v0.0.0 // indirect

replace github.com/omniaura/xgo/runtime/test/build/modules_without_trap_all/sub_a => ../sub_a

replace github.com/omniaura/xgo/runtime/test/build/modules_without_trap_all/sub_b => ../sub_b
