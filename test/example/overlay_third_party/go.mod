module github.com/omniaura/xgo/test/example/overlay_third_party

go 1.27.0

require (
	github.com/omniaura/xgo v0.0.0
	golang.org/x/example/hello v0.0.0-20250407153444-dd59d6852dfb
)

replace github.com/omniaura/xgo => ../../..
