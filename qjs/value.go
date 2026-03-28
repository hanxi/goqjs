package qjs

/*
#include "bridge.h"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

var (
	ErrException = errors.New("qjs: exception")

	// Context registry for C->Go callback lookup
	ctxRegistry = make(map[*C.JSContext]*Context)
	ctxMu       sync.RWMutex
)

// Value wraps a QuickJS JSValue.
type Value struct {
	ctx *Context
	val C.JSValue
}

// Free decrements the reference count of the value.
func (v Value) Free() {
	if v.ctx != nil {
		C.JS_FreeValue(v.ctx.ctx, v.val)
	}
}

// Dup increments the reference count and returns a new Value.
func (v Value) Dup() Value {
	return Value{ctx: v.ctx, val: C.JS_DupValue(v.ctx.ctx, v.val)}
}

// --- Type checks ---

func (v Value) IsNull() bool      { return C.goqjs_is_null(v.val) != 0 }
func (v Value) IsUndefined() bool  { return C.goqjs_is_undefined(v.val) != 0 }
func (v Value) IsException() bool  { return C.goqjs_is_exception(v.val) != 0 }
func (v Value) IsNumber() bool     { return C.goqjs_is_number(v.val) != 0 }
func (v Value) IsString() bool     { return C.goqjs_is_string(v.val) != 0 }
func (v Value) IsBool() bool       { return C.goqjs_is_bool(v.val) != 0 }
func (v Value) IsObject() bool     { return C.goqjs_is_object(v.val) != 0 }
func (v Value) IsSymbol() bool     { return C.goqjs_is_symbol(v.val) != 0 }
func (v Value) IsError() bool      { return C.goqjs_is_error(v.val) != 0 }
func (v Value) IsArray() bool      { return C.goqjs_is_array(v.val) != 0 }
func (v Value) IsPromise() bool    { return C.goqjs_is_promise(v.val) != 0 }

func (v Value) IsFunction() bool {
	if v.ctx == nil {
		return false
	}
	return C.goqjs_is_function(v.ctx.ctx, v.val) != 0
}

// --- Type conversions ---

// String returns the string representation of the value.
func (v Value) String() string {
	if v.ctx == nil {
		return ""
	}
	var plen C.size_t
	cstr := C.JS_ToCStringLen2(v.ctx.ctx, &plen, v.val, C.bool(false))
	if cstr == nil {
		return ""
	}
	defer C.JS_FreeCString(v.ctx.ctx, cstr)
	return C.GoStringN(cstr, C.int(plen))
}

// Int32 returns the int32 value.
func (v Value) Int32() int32 {
	var res C.int32_t
	C.JS_ToInt32(v.ctx.ctx, &res, v.val)
	return int32(res)
}

// Int64 returns the int64 value.
func (v Value) Int64() int64 {
	var res C.int64_t
	C.JS_ToInt64(v.ctx.ctx, &res, v.val)
	return int64(res)
}

// Float64 returns the float64 value.
func (v Value) Float64() float64 {
	var res C.double
	C.JS_ToFloat64(v.ctx.ctx, &res, v.val)
	return float64(res)
}

// Bool returns the boolean value.
func (v Value) Bool() bool {
	return C.JS_ToBool(v.ctx.ctx, v.val) == 1
}

// ToByteArray returns the ArrayBuffer contents as a byte slice.
func (v Value) ToByteArray() []byte {
	var size C.size_t
	ptr := C.JS_GetArrayBuffer(v.ctx.ctx, &size, v.val)
	if ptr == nil {
		return nil
	}
	return C.GoBytes(unsafe.Pointer(ptr), C.int(size))
}

// --- Property access ---

// Get returns a property by name.
func (v Value) Get(name string) Value {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return Value{ctx: v.ctx, val: C.JS_GetPropertyStr(v.ctx.ctx, v.val, cname)}
}

// GetByIndex returns a property by index.
func (v Value) GetByIndex(idx uint32) Value {
	return Value{ctx: v.ctx, val: C.JS_GetPropertyUint32(v.ctx.ctx, v.val, C.uint32_t(idx))}
}

// Set sets a property by name. Takes ownership of propVal.
func (v Value) Set(name string, propVal Value) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	C.JS_SetPropertyStr(v.ctx.ctx, v.val, cname, propVal.val)
}

// SetByIndex sets a property by index. Takes ownership of propVal.
func (v Value) SetByIndex(idx uint32, propVal Value) {
	C.JS_SetPropertyUint32(v.ctx.ctx, v.val, C.uint32_t(idx), propVal.val)
}

// SetFunction sets a function property by name.
func (v Value) SetFunction(name string, fn GoFunction, argc int) {
	fnID := registerFunction(fn)
	if v.ctx != nil {
		v.ctx.fnsMu.Lock()
		v.ctx.fns[fnID] = struct{}{}
		v.ctx.fnsMu.Unlock()
	}
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	fnVal := C.goqjs_new_function(v.ctx.ctx, cname, C.int(argc), C.int64_t(fnID))
	C.JS_SetPropertyStr(v.ctx.ctx, v.val, cname, fnVal)
}

// DefineProperty defines a property with flags.
func (v Value) DefineProperty(name string, propVal Value, flags int) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	C.goqjs_define_property_value_str(v.ctx.ctx, v.val, cname, propVal.val, C.int(flags))
}

// Len returns the length property of the value.
func (v Value) Len() int64 {
	var res C.int64_t
	C.goqjs_get_length(v.ctx.ctx, v.val, &res)
	return int64(res)
}

// Keys returns the enumerable own property names.
func (v Value) Keys() []string {
	var ptab *C.JSPropertyEnum
	var plen C.uint32_t
	ret := C.goqjs_get_own_property_names(v.ctx.ctx, &ptab, &plen, v.val, C.int(GPNStringMask|GPNEnumOnly|GPNSetEnum))
	if ret != 0 {
		return nil
	}
	defer C.JS_FreePropertyEnum(v.ctx.ctx, ptab, plen)

	keys := make([]string, int(plen))
	for i := 0; i < int(plen); i++ {
		entry := (*C.JSPropertyEnum)(unsafe.Pointer(uintptr(unsafe.Pointer(ptab)) + uintptr(i)*unsafe.Sizeof(*ptab)))
		atom := entry.atom
		val := C.JS_AtomToString(v.ctx.ctx, atom)
		keys[i] = Value{ctx: v.ctx, val: val}.String()
		C.JS_FreeValue(v.ctx.ctx, val)
	}
	return keys
}

// Call calls the value as a function.
func (v Value) Call(thisObj Value, args ...Value) (Value, error) {
	v.ctx.rt.UpdateStackTop()
	cArgs := make([]C.JSValue, len(args))
	for i, a := range args {
		cArgs[i] = a.val
	}
	var argv *C.JSValue
	if len(cArgs) > 0 {
		argv = &cArgs[0]
	}
	// Use goqjs_call which runs JS_Call on a 64MB-stack pthread,
	// preventing SIGBUS when timer/fetch callbacks trigger deep JS execution.
	result := C.goqjs_call(v.ctx.ctx, v.val, thisObj.val, C.int(len(args)), argv)
	if C.goqjs_is_exception(result) != 0 {
		return Value{ctx: v.ctx, val: result}, v.ctx.Exception()
	}
	return Value{ctx: v.ctx, val: result}, nil
}

// Error returns the error representation of the value.
func (v Value) Error() error {
	if v.IsError() || v.IsException() {
		msg := v.String()
		stack := v.Get("stack")
		defer stack.Free()
		if !stack.IsUndefined() {
			return fmt.Errorf("%s\n%s", msg, stack.String())
		}
		return errors.New(msg)
	}
	return nil
}
