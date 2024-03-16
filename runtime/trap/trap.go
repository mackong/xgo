package trap

import (
	"context"
	"runtime"
	"sync"
	"unsafe"

	"github.com/xhd2015/xgo/runtime/core"
	"github.com/xhd2015/xgo/runtime/functab"
)

var setupOnce sync.Once

func ensureInit() {
	setupOnce.Do(func() {
		__xgo_link_set_trap(trapImpl)
	})
}

func __xgo_link_set_trap(trapImpl func(funcName string, genericName string, pc uintptr, recv interface{}, args []interface{}, results []interface{}) (func(), bool)) {
	panic("failed to link __xgo_link_set_trap")
}

func Skip() {
	// this is intenionally leave empty
	// as trap.Skip() is a function used
	// to mark the caller should not be trapped.
	// one can also use trap.Skip() in
	// the non-interceptor context
}

// link to runtime
// xgo:notrap
func trapImpl(funcName string, genericName string, pc uintptr, recv interface{}, args []interface{}, results []interface{}) (func(), bool) {
	type intf struct {
		_  uintptr
		pc *uintptr
	}
	interceptors := GetAllInterceptors()
	n := len(interceptors)
	if n == 0 {
		return nil, false
	}
	if false {
		// check if the calling func is an interceptor, if so, skip
		// UPDATE: don't do manual check
		for i := 0; i < n; i++ {
			if interceptors[i].Pre == nil {
				continue
			}
			ipc := (**uintptr)(unsafe.Pointer(&interceptors[i].Pre))
			pcName := runtime.FuncForPC(**ipc).Name()
			_ = pcName
			if **ipc == pc {
				return nil, false
			}
		}
	}
	// NOTE: this may return nil for generic template
	f := functab.InfoPC(pc)
	if f == nil && genericName != "" {
		f = functab.InfoGeneric(genericName)
	}
	if f == nil {
		// may be generic
		// fallback to default
		pkgPath, recvType, recvPtr, funcShortName := core.ParseFuncName(funcName, true)
		f = &core.FuncInfo{
			Pkg:      pkgPath,
			RecvType: recvType,
			RecvPtr:  recvPtr,
			Name:     funcShortName,
			FullName: funcName,
		}
	}

	// TODO: set FirstArgCtx and LastResultErr
	req := make(object, 0, len(args))
	result := make(object, 0, len(results))
	if f.RecvType != "" {
		req = append(req, field{
			name:   f.RecvName,
			valPtr: recv,
		})
	}
	if !f.FirstArgCtx {
		req = appendFields(req, args, f.ArgNames)
	} else {
		argNames := f.ArgNames
		if argNames != nil {
			argNames = argNames[1:]
		}
		req = appendFields(req, args[1:], argNames)
	}
	if !f.LastResultErr {
		result = appendFields(result, results, f.ResNames)
	} else {
		resNames := f.ResNames
		if resNames != nil {
			resNames = resNames[:len(resNames)-1]
		}
		result = appendFields(result, results[:len(results)-1], resNames)
	}

	// TODO: what about inlined func?
	// funcArgs := &FuncArgs{
	// 	Recv:    recv,
	// 	Args:    args,
	// 	Results: results,
	// }

	// TODO: will results always have names?
	var perr *error
	if len(results) > 0 {
		if errPtr, ok := results[len(results)-1].(*error); ok {
			perr = errPtr
		}
	}

	// NOTE: ctx may
	var ctx context.Context
	if len(args) > 0 {
		// TODO: is *HttpRequest a *Context?
		if argCtxPtr, ok := args[0].(*context.Context); ok {
			ctx = *argCtxPtr
		}
	}
	// NOTE: context.TODO() is a constant
	if ctx == nil {
		ctx = context.TODO()
	}

	abortIdx := -1
	dataList := make([]interface{}, n)
	for i := n - 1; i >= 0; i-- {
		interceptor := interceptors[i]
		if interceptor.Pre == nil {
			continue
		}
		// if
		data, err := interceptor.Pre(ctx, f, req, result)
		dataList[i] = data
		if err != nil {
			if err == ErrAbort {
				abortIdx = i
				// aborted
				break
			}
			// handle error gracefully
			if perr != nil {
				*perr = err
				return nil, true
			} else {
				panic(err)
			}
		}
	}
	if abortIdx >= 0 {
		// run Post immediately
		for i := abortIdx; i < n; i++ {
			interceptor := interceptors[i]
			if interceptor.Post == nil {
				continue
			}
			err := interceptor.Post(ctx, f, req, result, dataList[i])
			if err != nil {
				if err == ErrAbort {
					return nil, true
				}
				if perr != nil {
					*perr = err
					return nil, true
				} else {
					panic(err)
				}
			}
		}
		return nil, true
	}

	return func() {
		for i := 0; i < n; i++ {
			interceptor := interceptors[i]
			if interceptor.Post == nil {
				continue
			}
			err := interceptor.Post(ctx, f, req, result, dataList[i])
			if err != nil {
				if err == ErrAbort {
					return
				}
				if perr != nil {
					*perr = err
					return
				} else {
					panic(err)
				}
			}
		}
	}, false
}
