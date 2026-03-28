package qjs

/*
#cgo CFLAGS: -D_GNU_SOURCE -DCONFIG_VERSION="0.8.0" -DNDEBUG -pthread
#cgo LDFLAGS: -lm -lpthread
#include "quickjs.h"
#include "quickjs-libc.h"
#include "bridge.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"unsafe"
)

// Tags from quickjs.h
const (
	TagBigInt        = C.JS_TAG_BIG_INT
	TagSymbol        = C.JS_TAG_SYMBOL
	TagString        = C.JS_TAG_STRING
	TagObject        = C.JS_TAG_OBJECT
	TagInt           = C.JS_TAG_INT
	TagBool          = C.JS_TAG_BOOL
	TagNull          = C.JS_TAG_NULL
	TagUndefined     = C.JS_TAG_UNDEFINED
	TagException     = C.JS_TAG_EXCEPTION
	TagFloat64       = C.JS_TAG_FLOAT64
	TagUninitialized = C.JS_TAG_UNINITIALIZED
)

// Eval flags
const (
	EvalGlobal = C.JS_EVAL_TYPE_GLOBAL
	EvalModule = C.JS_EVAL_TYPE_MODULE
	EvalStrict = C.JS_EVAL_FLAG_STRICT
	EvalAsync  = C.JS_EVAL_FLAG_ASYNC
)

// Property flags
const (
	PropConfigurable = C.JS_PROP_CONFIGURABLE
	PropWritable     = C.JS_PROP_WRITABLE
	PropEnumerable   = C.JS_PROP_ENUMERABLE
	PropCWE          = C.JS_PROP_C_W_E
)

// GetOwnPropertyNames flags
const (
	GPNStringMask = C.JS_GPN_STRING_MASK
	GPNEnumOnly   = C.JS_GPN_ENUM_ONLY
	GPNSetEnum    = C.JS_GPN_SET_ENUM
)

//export goInvokeFunction
func goInvokeFunction(ctx *C.JSContext, thisVal C.JSValue, argc C.int, argv *C.JSValue, fnID C.int64_t) C.JSValue {
	id := int64(fnID)

	// Look up the Go function in the registry
	fnMu.RLock()
	fn, ok := fnRegistry[id]
	fnMu.RUnlock()

	if !ok {
		cmsg := C.CString("goqjs: unknown function ID")
		defer C.free(unsafe.Pointer(cmsg))
		return C.goqjs_throw_error(ctx, cmsg)
	}

	// Find the Context wrapper
	ctxMu.RLock()
	goCtx, ctxOk := ctxRegistry[ctx]
	ctxMu.RUnlock()

	if !ctxOk {
		cmsg := C.CString("goqjs: context not found")
		defer C.free(unsafe.Pointer(cmsg))
		return C.goqjs_throw_error(ctx, cmsg)
	}

	// Convert args
	goArgs := make([]Value, int(argc))
	if argc > 0 {
		argSlice := unsafe.Slice(argv, int(argc))
		for i := 0; i < int(argc); i++ {
			goArgs[i] = Value{ctx: goCtx, val: argSlice[i]}
		}
	}

	thisObj := Value{ctx: goCtx, val: thisVal}
	result := fn(goCtx, thisObj, goArgs)
	return result.val
}
