module github.com/omniaura/xgo/runtime/test/build/overlay_build_cache_ok_with_xgo

go 1.27.0

require golang.org/x/example/hello v0.0.0-20250407153444-dd59d6852dfb

require github.com/omniaura/xgo/runtime/test v0.0.0

require (
	github.com/omniaura/xgo v0.0.0 // indirect
	github.com/omniaura/xgo/runtime v1.1.0 // indirect
)

replace (
	github.com/omniaura/xgo => ../../../..
	github.com/omniaura/xgo/runtime => ../../..
	github.com/omniaura/xgo/runtime/test => ../../
)
