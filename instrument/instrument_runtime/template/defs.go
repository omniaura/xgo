package template

// start with go1.21, the runtime.FuncForPC(pc).Name()
// is wrapped in funcNameForPrint(...), we unwrap it.
// it is confirmed that in go1.21~go1.28,
// the name is wrapped.
// NOTE: when upgrading to go1.29, should check
// the implementation again
const RuntimeGetFuncName_Go121 = `
func __xgo_get_pc_name_impl(pc uintptr) string {
	return FuncForPC(pc).__xgo_no_print_name()
}

func (f *Func) __xgo_no_print_name() string {
	if f == nil {
		return ""
	}
	fn := f.raw()
	if fn.isInlined() { // inlined version
		fi := (*funcinl)(unsafe.Pointer(fn))
		return fi.name
	}
	return funcname(f.funcInfo())
}
`
