package struct_field

import "github.com/omniaura/xgo/runtime/test/patch/patch_third_party_func/third/struct_field/other"

type SomeStruct struct {
	MyField *other.Other
}
