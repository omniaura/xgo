package auto_collect

import (
	"testing"
	// NOTE: we expect xgo to auto add
	// the following blank import to enable trace
	// _ "github.com/omniaura/xgo/runtime/trace"
)

func TestTraceAutoCollect(t *testing.T) {
	h()
}

func h() {
	A()
	B()
	C()
}

func A() string {
	return "A"
}

func B() string {
	C()
	return "B"
}

func C() string {
	return "C"
}
