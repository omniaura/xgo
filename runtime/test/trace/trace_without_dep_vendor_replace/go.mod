module github.com/omniaura/xgo/runtime/test/trace_without_dep_vendor_replace

go 1.14

require github.com/omniaura/xgo/runtime/test/trace_without_dep_vendor_replace/lib v1.0.0

replace github.com/omniaura/xgo/runtime/test/trace_without_dep_vendor_replace/lib => ./lib
