module github.com/omniaura/xgo/runtime/test/mock/mock_rule_include_as_main_module/with_flag_two_modules

go 1.18

require (
	github.com/omniaura/xgo/runtime v0.0.0
	github.com/omniaura/xgo/runtime/test/mock/mock_rule_include_as_main_module/app v0.0.0
	github.com/omniaura/xgo/runtime/test/mock/mock_rule_include_as_main_module/lib v0.0.0
	github.com/omniaura/xgo/runtime/test/mock/mock_rule_include_as_main_module/other v0.0.0
)

replace github.com/omniaura/xgo/runtime => ../../../../

replace github.com/omniaura/xgo/runtime/test/mock/mock_rule_include_as_main_module/app => ../app

replace github.com/omniaura/xgo/runtime/test/mock/mock_rule_include_as_main_module/lib => ../lib

replace github.com/omniaura/xgo/runtime/test/mock/mock_rule_include_as_main_module/other => ../other
