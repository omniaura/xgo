package embed_other_struct

import "github.com/omniaura/xgo/runtime/test/patch/patch_third_party_func/third/embed_other_struct/other"

type EmbedStruct struct {
	*other.Other
}
