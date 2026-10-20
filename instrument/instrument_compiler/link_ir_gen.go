package instrument_compiler

// RuntimeExtraDef declares the GOROOT/src/runtime/xgo_trap.go functions the
// compiler may link to (see mkbuiltin).
const RuntimeExtraDef = `
// xgo
func XgoTrap(info interface{}, recvPtr interface{}, args []interface{}, results []interface{}) (func(), bool)
func XgoTrapOn(flag *uint32) bool
func XgoTrapVar(info interface{}, varAddr interface{}, res interface{})
func XgoTrapVarPtr(info interface{}, varAddr interface{}, res interface{})
func XgoRegister(fn interface{})

func XgoSetTrap(trap func(info unsafe.Pointer, recvPtr interface{}, args []interface{}, results []interface{}) (func(), bool))
func XgoSetTrapSkip(skip bool)
func XgoSetVarTrap(trap func(info unsafe.Pointer, varAddr interface{}, res interface{}))
func XgoSetVarPtrTrap(trap func(info unsafe.Pointer, varAddr interface{}, res interface{}))
func XgoSetupRegisterHandler(register func(fn unsafe.Pointer))
func XgoGetCurG() unsafe.Pointer
func XgoPeekPanic() (interface{}, uintptr)
func XgoGetFullPCName(pc uintptr) string
func XgoOnCreateG(callback func(g unsafe.Pointer, childG unsafe.Pointer))
func XgoOnExitG(callback func())
func XgoOnInitFinished(callback func())
func XgoInitFinished() bool
`
