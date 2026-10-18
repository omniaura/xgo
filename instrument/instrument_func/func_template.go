package instrument_func

import (
	"fmt"
	"strings"
)

// funcInfo, recvPtr, paramPtrs, resultPtrs
const trapTemplate = "__xgo_post_%d, __xgo_stop_%d := __xgo_trap_%d(%s,%s,[]interface{}{%s},[]interface{}{%s});if __xgo_post_%d!=nil { defer __xgo_post_%d(); };if __xgo_stop_%d { return; };"

// guardedTrap builds a trap statement that only calls the trap while
// __xgo_trap_on_<fileIndex>() reports true, and passes it pointers to copies
// of the receiver, parameters and results instead of the variables
// themselves. Taking the address of a variable that is passed to the trap
// moves it to the heap for the whole function, so trapTemplate costs
// several allocations on every call even when nothing intercepts it; with
// copies, only the guarded branch allocates. The copies are written back
// after the trap returns, and synced in both directions around post.
//
// vars lists the receiver (if any), parameters and results, in that order.
func guardedTrap(line int, fileIndex int, funcInfo string, recv string, params []string, results []string) string {
	var vars []string
	if recv != "" {
		vars = append(vars, recv)
	}
	vars = append(vars, params...)
	nIn := len(vars)
	vars = append(vars, results...)

	copies := make([]string, len(vars))
	for i := range vars {
		copies[i] = fmt.Sprintf("__xgo_c_%d_%d", line, i)
	}
	addrs := func(c []string) string {
		a := make([]string, len(c))
		for i, name := range c {
			a[i] = "&" + name
		}
		return strings.Join(a, ",")
	}
	assign := func(dst []string, src []string) string {
		if len(dst) == 0 {
			return ""
		}
		return strings.Join(dst, ",") + "=" + strings.Join(src, ",") + ";"
	}

	recvAddr := "nil"
	paramCopies := copies[:nIn]
	if recv != "" {
		recvAddr = "&" + copies[0]
		paramCopies = copies[1:nIn]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "if __xgo_trap_on_%d() {", fileIndex)
	if len(vars) > 0 {
		fmt.Fprintf(&b, "%s:=%s;", strings.Join(copies, ","), strings.Join(vars, ","))
	}
	fmt.Fprintf(&b, "__xgo_post_%d, __xgo_stop_%d := __xgo_trap_%d(%s,%s,[]interface{}{%s},[]interface{}{%s});", line, line, fileIndex, funcInfo, recvAddr, addrs(paramCopies), addrs(copies[nIn:]))
	b.WriteString(assign(vars[:nIn], copies[:nIn]))
	fmt.Fprintf(&b, "if __xgo_post_%d!=nil { defer func(){%s__xgo_post_%d();%s}() };", line, assign(copies, vars), line, assign(vars[nIn:], copies[nIn:]))
	fmt.Fprintf(&b, "if __xgo_stop_%d { %sreturn; };", line, assign(vars[nIn:], copies[nIn:]))
	b.WriteString("};")
	return b.String()
}
