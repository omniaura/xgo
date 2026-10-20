package sub_a

import "github.com/omniaura/xgo/runtime/test/build/modules_without_trap_all/sub_b"

func SubA() {
	sub_b.SubB()
}
