package hook

import "github.com/omniaura/xgo/runtime/internal/runtime"

func OnInitFinished(f func()) {
	runtime.XgoOnInitFinished(f)
}

func InitFinished() bool {
	return runtime.XgoInitFinished()
}
