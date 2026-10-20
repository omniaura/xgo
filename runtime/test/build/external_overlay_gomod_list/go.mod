module github.com/omniaura/xgo/runtime/test/build/external_overlay_gomod_list

go 1.18

require (
	example.com/dep v0.0.0
	github.com/omniaura/xgo/runtime v0.0.0
)

replace example.com/dep => ./vendor_dep

replace github.com/omniaura/xgo/runtime => ../../../
