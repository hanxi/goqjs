package qjs

/*
#include "bridge.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"os"
	"sync"
	"unsafe"
)

// Context wraps a QuickJS JSContext.
type Context struct {
	ctx   *C.JSContext
	rt    *Runtime
	fns   map[int64]struct{} // tracks registered function IDs for cleanup
	fnsMu sync.Mutex
}

// Close frees the context and unregisters all associated Go functions.
func (c *Context) Close() {
	if c.ctx != nil {
		// Unregister all Go functions associated with this context
		c.fnsMu.Lock()
		for id := range c.fns {
			unregisterFunction(id)
		}
		c.fns = nil
		c.fnsMu.Unlock()

		// Unregister context from registry
		ctxMu.Lock()
		delete(ctxRegistry, c.ctx)
		ctxMu.Unlock()

		C.JS_FreeContext(c.ctx)
		c.ctx = nil
	}
}

// Eval evaluates JavaScript code and returns the result.
func (c *Context) Eval(code, filename string) (Value, error) {
	c.rt.UpdateStackTop()
	cCode := C.CString(code)
	defer C.free(unsafe.Pointer(cCode))
	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))

	result := C.goqjs_eval(c.ctx, cCode, C.size_t(len(code)), cFilename, C.int(EvalGlobal))
	if C.goqjs_is_exception(result) != 0 {
		return Value{ctx: c, val: result}, c.Exception()
	}
	return Value{ctx: c, val: result}, nil
}

// EvalModule evaluates JavaScript code as a module.
func (c *Context) EvalModule(code, filename string) (Value, error) {
	c.rt.UpdateStackTop()
	cCode := C.CString(code)
	defer C.free(unsafe.Pointer(cCode))
	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))

	result := C.goqjs_eval(c.ctx, cCode, C.size_t(len(code)), cFilename, C.int(EvalModule))
	if C.goqjs_is_exception(result) != 0 {
		return Value{ctx: c, val: result}, c.Exception()
	}
	return Value{ctx: c, val: result}, nil
}

// EvalFile reads and evaluates a JavaScript file.
func (c *Context) EvalFile(path string) (Value, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return c.Undefined(), fmt.Errorf("qjs: read file: %w", err)
	}
	return c.Eval(string(data), path)
}

// GlobalObject returns the global object.
func (c *Context) GlobalObject() Value {
	return Value{ctx: c, val: C.JS_GetGlobalObject(c.ctx)}
}

// NewObject creates a new empty JavaScript object.
func (c *Context) NewObject() Value {
	return Value{ctx: c, val: C.JS_NewObject(c.ctx)}
}

// NewArray creates a new empty JavaScript array.
func (c *Context) NewArray() Value {
	return Value{ctx: c, val: C.JS_NewArray(c.ctx)}
}

// NewString creates a new JavaScript string.
func (c *Context) NewString(s string) Value {
	cstr := C.CString(s)
	defer C.free(unsafe.Pointer(cstr))
	return Value{ctx: c, val: C.JS_NewStringLen(c.ctx, cstr, C.size_t(len(s)))}
}

// NewInt32 creates a new JavaScript int32.
func (c *Context) NewInt32(v int32) Value {
	return Value{ctx: c, val: C.goqjs_new_int32(c.ctx, C.int32_t(v))}
}

// NewInt64 creates a new JavaScript int64.
func (c *Context) NewInt64(v int64) Value {
	return Value{ctx: c, val: C.goqjs_new_int64(c.ctx, C.int64_t(v))}
}

// NewFloat64 creates a new JavaScript float64.
func (c *Context) NewFloat64(v float64) Value {
	return Value{ctx: c, val: C.goqjs_new_float64(c.ctx, C.double(v))}
}

// NewBool creates a new JavaScript boolean.
func (c *Context) NewBool(v bool) Value {
	val := 0
	if v {
		val = 1
	}
	return Value{ctx: c, val: C.goqjs_new_bool(c.ctx, C.int(val))}
}

// NewFunction creates a new JavaScript function backed by a Go function.
func (c *Context) NewFunction(name string, fn GoFunction, argc int) Value {
	fnID := registerFunction(fn)
	c.fnsMu.Lock()
	c.fns[fnID] = struct{}{}
	c.fnsMu.Unlock()

	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return Value{ctx: c, val: C.goqjs_new_function(c.ctx, cname, C.int(argc), C.int64_t(fnID))}
}

// NewPromise creates a new Promise and returns (promise, resolve, reject).
func (c *Context) NewPromise() (promise, resolve, reject Value) {
	var resolveFuncs [2]C.JSValue
	p := C.goqjs_new_promise(c.ctx, &resolveFuncs[0])
	return Value{ctx: c, val: p}, Value{ctx: c, val: resolveFuncs[0]}, Value{ctx: c, val: resolveFuncs[1]}
}

// NewArrayBuffer creates a new ArrayBuffer from a byte slice.
func (c *Context) NewArrayBuffer(data []byte) Value {
	if len(data) == 0 {
		return Value{ctx: c, val: C.JS_NewArrayBufferCopy(c.ctx, nil, 0)}
	}
	return Value{ctx: c, val: C.JS_NewArrayBufferCopy(c.ctx, (*C.uint8_t)(unsafe.Pointer(&data[0])), C.size_t(len(data)))}
}

// Null returns JS null.
func (c *Context) Null() Value {
	return Value{ctx: c, val: C.goqjs_null()}
}

// Undefined returns JS undefined.
func (c *Context) Undefined() Value {
	return Value{ctx: c, val: C.goqjs_undefined()}
}

// Exception returns the current pending exception as a Go error.
func (c *Context) Exception() error {
	exc := C.goqjs_get_exception(c.ctx)
	val := Value{ctx: c, val: exc}
	defer val.Free()

	msg := val.String()
	stack := val.Get("stack")
	defer stack.Free()
	if !stack.IsUndefined() {
		return fmt.Errorf("%s\n%s", msg, stack.String())
	}
	return fmt.Errorf("%s", msg)
}

// ThrowError throws a JavaScript error with the given message.
func (c *Context) ThrowError(msg string) Value {
	cmsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cmsg))
	return Value{ctx: c, val: C.goqjs_throw_error(c.ctx, cmsg)}
}

// ThrowTypeError throws a JavaScript TypeError.
func (c *Context) ThrowTypeError(format string, args ...interface{}) Value {
	msg := fmt.Sprintf(format, args...)
	cmsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cmsg))
	return Value{ctx: c, val: C.goqjs_throw_error(c.ctx, cmsg)}
}

// JSONStringify converts a value to a JSON string.
func (c *Context) JSONStringify(val Value) string {
	result := C.JS_JSONStringify(c.ctx, val.val, C.goqjs_undefined(), C.goqjs_undefined())
	v := Value{ctx: c, val: result}
	defer v.Free()
	return v.String()
}

// ParseJSON parses a JSON string and returns the result.
func (c *Context) ParseJSON(jsonStr string) (Value, error) {
	cstr := C.CString(jsonStr)
	defer C.free(unsafe.Pointer(cstr))
	cfn := C.CString("<json>")
	defer C.free(unsafe.Pointer(cfn))
	result := C.JS_ParseJSON(c.ctx, cstr, C.size_t(len(jsonStr)), cfn)
	if C.goqjs_is_exception(result) != 0 {
		return Value{ctx: c, val: result}, c.Exception()
	}
	return Value{ctx: c, val: result}, nil
}
