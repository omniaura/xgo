module github.com/omniaura/xgo/runtime/test/build/modules_with_trap_all/sub_a

go 1.18

require github.com/omniaura/xgo/runtime/test/build/modules_with_trap_all/sub_b v0.0.0

replace github.com/omniaura/xgo/runtime/test/build/modules_with_trap_all/sub_b => ../sub_b
