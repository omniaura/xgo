package instrument_func

import (
	"fmt"
	"go/ast"
	"go/token"
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
// after the trap returns, so the parameters and results the interceptor may
// have changed are visible to the body.
//
// The copying lives in helper, a top-level function shared by the
// functions of a file whose receiver, parameters and results have the same
// types (see guardHelperKey): spelled out in every function, it made each
// one several times larger, in the binary and in the build cache. The trap
// itself is still called from the instrumented function, because the
// runtime identifies the function by its caller's PC. Called with neither
// post nor copies, helper copies the variables and returns the copies and
// the trap's arguments; with only the copies, it writes them back; with
// post, it syncs the copies around it.
//
// When the trap returns post, it must run when the function returns, even
// by panicking. A defer for it in every instrumented function made each
// one larger still (the deferred call needs a wrapper and code at every
// return) and kept small functions from being inlined. With fnRef, the
// function instead hands over to reenter, which defers post and calls the
// function again (fnRef names it, a method expression for methods) with
// the copies as arguments, after __xgo_trap_on_<fileIndex>(nil) made the
// nested call skip the trap: only while something intercepts the function
// with a post does its body run one call deeper. Without fnRef (fnRef would
// not compile, see reenterFuncRef) the function keeps the defer.
//
// A function literal would shift the numbering of the closures in the
// body (F.func1 becoming F.func2), and pointers passed to a function that
// does not keep them do not move the variables to the heap. helperDecl and
// reenterDecl are single lines, so they do not shift line numbers; the
// caller inserts each once per name.
func guardedTrap(line int, fileIndex int, helper string, reenter string, fnRef string, funcInfo string, recv *guardVar, params []*guardVar, results []*guardVar) (stmt string, helperDecl string, reenterDecl string) {
	vars, nIn := guardHelperVars(recv, params, results)
	l := func(name string) string { return fmt.Sprintf("__xgo_%s_%d", name, line) }
	var b strings.Builder
	fmt.Fprintf(&b, "if __xgo_trap_on_%d(&%s) {", fileIndex, trapOnVar(funcInfo))
	if len(vars) == 0 {
		fmt.Fprintf(&b, "%s, %s := __xgo_trap_%d(%s,nil,[]interface{}{},[]interface{}{});", l("post"), l("stop"), fileIndex, funcInfo)
		if fnRef != "" {
			fmt.Fprintf(&b, "if %s!=nil { %s(%s,%s,%s); return; };", l("post"), reenter, l("post"), l("stop"), fnRef)
			reenterDecl = fmt.Sprintf(";func %s(__xgo_post func(),__xgo_stop bool,__xgo_fn func()) {defer __xgo_post();if !__xgo_stop {__xgo_trap_on_%d(nil);__xgo_fn()}}", reenter, fileIndex)
		} else {
			fmt.Fprintf(&b, "if %s!=nil { defer %s() };", l("post"), l("post"))
		}
		fmt.Fprintf(&b, "if %s { return; };};", l("stop"))
		return b.String(), "", reenterDecl
	}

	addrs := make([]string, len(vars))
	for i, v := range vars {
		addrs[i] = "&" + v.name
	}
	vs := strings.Join(addrs, ",")
	fmt.Fprintf(&b, "%s, %s, %s, %s := %s(nil,nil,%s);", l("c"), l("r"), l("a"), l("s"), helper, vs)
	fmt.Fprintf(&b, "%s, %s := __xgo_trap_%d(%s,%s,%s,%s);", l("post"), l("stop"), fileIndex, funcInfo, l("r"), l("a"), l("s"))
	if fnRef != "" {
		resultNames := make([]string, 0, len(vars)-nIn)
		for _, v := range vars[nIn:] {
			resultNames = append(resultNames, v.name)
		}
		assignResults := ""
		if len(resultNames) > 0 {
			assignResults = strings.Join(resultNames, ",") + " = "
		}
		fmt.Fprintf(&b, "if %s!=nil { %s%s(%s,%s,%s,%s); return; };", l("post"), assignResults, reenter, l("post"), l("stop"), l("c"), fnRef)
		fmt.Fprintf(&b, "%s(nil,%s,%s);", helper, l("c"), vs)
	} else {
		fmt.Fprintf(&b, "%s(nil,%s,%s);", helper, l("c"), vs)
		fmt.Fprintf(&b, "if %s!=nil { defer %s(%s,%s,%s) };", l("post"), helper, l("post"), l("c"), vs)
	}
	fmt.Fprintf(&b, "if %s { return; };", l("stop"))
	b.WriteString("};")

	fields := make([]string, len(vars))
	hParams := make([]string, len(vars))
	hVars := make([]string, len(vars))
	hCopies := make([]string, len(vars))
	hAddrs := make([]string, len(vars))
	for i, v := range vars {
		fields[i] = fmt.Sprintf("c%d %s", i, v.typ)
		hParams[i] = fmt.Sprintf("__xgo_v%d *%s", i, v.typ)
		hVars[i] = fmt.Sprintf("*__xgo_v%d", i)
		hCopies[i] = fmt.Sprintf("__xgo_c.c%d", i)
		hAddrs[i] = fmt.Sprintf("&__xgo_c.c%d", i)
	}
	assign := func(dst []string, src []string) string {
		if len(dst) == 0 {
			return ""
		}
		return strings.Join(dst, ",") + "=" + strings.Join(src, ",") + ";"
	}
	recvAddr := "nil"
	paramAddrs := hAddrs[:nIn]
	if recv != nil {
		recvAddr = hAddrs[0]
		paramAddrs = hAddrs[1:nIn]
	}
	// the trap's argument slices are arrays in the copies, so that calling
	// the trap allocates once
	fields = append(fields, fmt.Sprintf("a [%d]interface{}", len(paramAddrs)), fmt.Sprintf("r [%d]interface{}", len(vars)-nIn))
	copiesType := "*struct{" + strings.Join(fields, ";") + "}"
	var h strings.Builder
	fmt.Fprintf(&h, ";func %s(__xgo_post func(),__xgo_c %s,%s) (%s,interface{},[]interface{},[]interface{}) {", helper, copiesType, strings.Join(hParams, ","), copiesType)
	fmt.Fprintf(&h, "if __xgo_post!=nil {%s__xgo_post();%sreturn nil,nil,nil,nil};", assign(hCopies, hVars), assign(hVars[nIn:], hCopies[nIn:]))
	fmt.Fprintf(&h, "if __xgo_c!=nil {%sreturn nil,nil,nil,nil};", assign(hVars, hCopies))
	fmt.Fprintf(&h, "__xgo_c=new(%s);%s", copiesType[1:], assign(hCopies, hVars))
	fmt.Fprintf(&h, "__xgo_c.a=[%d]interface{}{%s};__xgo_c.r=[%d]interface{}{%s};", len(paramAddrs), strings.Join(paramAddrs, ","), len(vars)-nIn, strings.Join(hAddrs[nIn:], ","))
	fmt.Fprintf(&h, "return __xgo_c,%s,__xgo_c.a[:],__xgo_c.r[:]}", recvAddr)

	if fnRef != "" {
		// reenter: results start from the copies, which hold what the
		// interceptor set when it stopped the call, and post sees and may
		// change the results the nested call returned
		inTypes := make([]string, nIn)
		args := make([]string, nIn)
		for i, v := range vars[:nIn] {
			inTypes[i] = v.typ
			args[i] = hCopies[i]
			if v.variadic && i == nIn-1 {
				inTypes[i] = "..." + strings.TrimPrefix(v.typ, "[]")
				args[i] += "..."
			}
		}
		outTypes := make([]string, 0, len(vars)-nIn)
		outs := make([]string, 0, len(vars)-nIn)
		outDecls := make([]string, 0, len(vars)-nIn)
		for i, v := range vars[nIn:] {
			outTypes = append(outTypes, v.typ)
			outs = append(outs, fmt.Sprintf("__xgo_r%d", i))
			outDecls = append(outDecls, fmt.Sprintf("__xgo_r%d %s", i, v.typ))
		}
		var r strings.Builder
		fmt.Fprintf(&r, ";func %s(__xgo_post func(),__xgo_stop bool,__xgo_c %s,__xgo_fn func(%s) (%s)) (%s) {", reenter, copiesType, strings.Join(inTypes, ","), strings.Join(outTypes, ","), strings.Join(outDecls, ","))
		r.WriteString(assign(outs, hCopies[nIn:]))
		fmt.Fprintf(&r, "defer func(){%s__xgo_post();%s}();", assign(hCopies[nIn:], outs), assign(outs, hCopies[nIn:]))
		call := fmt.Sprintf("__xgo_fn(%s)", strings.Join(args, ","))
		if len(outs) > 0 {
			call = strings.Join(outs, ",") + "=" + call
		}
		fmt.Fprintf(&r, "if !__xgo_stop {__xgo_trap_on_%d(nil);%s};return}", fileIndex, call)
		reenterDecl = r.String()
	}
	return b.String(), h.String(), reenterDecl
}

// reenterFuncRef returns how the instrumented function refers to itself
// for guardedTrap's reenter: its name, or a method expression. It is empty
// when that would not compile: init cannot be referred to, and a receiver,
// parameter or result may shadow the name or the receiver's type.
func reenterFuncRef(funcDecl *ast.FuncDecl, recv *guardVar, params []*guardVar, results []*guardVar) string {
	name := funcDecl.Name.Name
	if name == "init" || name == "_" {
		return ""
	}
	ref, shadowed := name, name
	if recv != nil {
		base := strings.TrimPrefix(recv.typ, "*")
		if !token.IsIdentifier(base) {
			return ""
		}
		ref, shadowed = "("+recv.typ+")."+name, base
	}
	vars, _ := guardHelperVars(recv, params, results)
	for _, v := range vars {
		if v.name == shadowed {
			return ""
		}
	}
	return ref
}

// guardHelperKey identifies the helper of guardedTrap: functions whose
// receiver, parameters and results have the same types share it. It is
// empty when the function needs no helper.
func guardHelperKey(recv *guardVar, params []*guardVar, results []*guardVar) string {
	vars, nIn := guardHelperVars(recv, params, results)
	if len(vars) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%t;%d", recv != nil, nIn)
	for _, v := range vars {
		b.WriteString(";")
		if v.variadic {
			b.WriteString("...")
		}
		b.WriteString(v.typ)
	}
	return b.String()
}

// guardHelperVars lists the receiver (if any), parameters and results, in
// that order, and how many of them are inputs.
func guardHelperVars(recv *guardVar, params []*guardVar, results []*guardVar) ([]*guardVar, int) {
	var vars []*guardVar
	if recv != nil {
		vars = append(vars, recv)
	}
	vars = append(vars, params...)
	nIn := len(vars)
	vars = append(vars, results...)
	return vars, nIn
}

// guardedDirectTrap is trapTemplate behind the __xgo_trap_on_<fileIndex>()
// guard, for functions whose signature guardedTrap cannot spell outside of
// them (generic functions and methods). It still skips the trap call, but
// the variables whose addresses it takes live on the heap.
func guardedDirectTrap(line int, fileIndex int, funcInfo string, recvAddr string, paramAddrs []string, resultAddrs []string) string {
	return fmt.Sprintf("if __xgo_trap_on_%d(&%s) {", fileIndex, trapOnVar(funcInfo)) + fmt.Sprintf(trapTemplate,
		line, line,
		fileIndex,
		funcInfo,
		recvAddr,
		strings.Join(paramAddrs, ","),
		strings.Join(resultAddrs, ","),
		line, line, line,
	) + "};"
}

// guardVar is a receiver, parameter or result of an instrumented function,
// with its type as source text.
type guardVar struct {
	name string
	typ  string
	// variadic: the last parameter, declared ...T and spelled []T in typ
	variadic bool
}

// trapOnVar is the name of the flag that the guard of the function with
// info variable funcInfo passes to __xgo_trap_on_. The runtime sets it
// when something is registered for that function.
func trapOnVar(funcInfo string) string {
	return funcInfo + "_on"
}
