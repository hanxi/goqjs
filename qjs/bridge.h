#ifndef GOQJS_BRIDGE_H
#define GOQJS_BRIDGE_H

#include "quickjs.h"

// Macro-to-function wrappers (CGO cannot use C macros)
JSValue goqjs_null(void);
JSValue goqjs_undefined(void);
JSValue goqjs_exception(void);
JSValue goqjs_uninitialized(void);
JSValue goqjs_true(void);
JSValue goqjs_false(void);

// JSValue tag access
int32_t goqjs_get_tag(JSValue v);
int32_t goqjs_get_int(JSValue v);
int goqjs_get_bool(JSValue v);
double goqjs_get_float64(JSValue v);
void *goqjs_get_ptr(JSValue v);

// JSValue type checks
int goqjs_is_null(JSValue v);
int goqjs_is_undefined(JSValue v);
int goqjs_is_exception(JSValue v);
int goqjs_is_number(JSValue v);
int goqjs_is_string(JSValue v);
int goqjs_is_bool(JSValue v);
int goqjs_is_object(JSValue v);
int goqjs_is_symbol(JSValue v);
int goqjs_is_error(JSValue v);
int goqjs_is_function(JSContext *ctx, JSValue v);
int goqjs_is_array(JSValue v);
int goqjs_is_promise(JSValue v);

// JSValue constructors
JSValue goqjs_new_bool(JSContext *ctx, int val);
JSValue goqjs_new_int32(JSContext *ctx, int32_t val);
JSValue goqjs_new_int64(JSContext *ctx, int64_t val);
JSValue goqjs_new_float64(JSContext *ctx, double val);
JSValue goqjs_new_uint32(JSContext *ctx, uint32_t val);

// Go function callback proxy
JSValue goqjs_proxy_fn(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv, int magic, JSValueConst *func_data);

// Defined in Go (bridge.go), called from C
extern JSValue goInvokeFunction(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv, int64_t fnID);

// Helper to create a CFunction with data (fnID stored as data)
JSValue goqjs_new_function(JSContext *ctx, const char *name, int argc, int64_t fnID);

// Property helpers
int goqjs_define_property_value_str(JSContext *ctx, JSValueConst this_obj, const char *prop, JSValue val, int flags);

// Promise helpers
JSValue goqjs_new_promise(JSContext *ctx, JSValue *resolve_funcs);

// Eval helpers (runs on 64MB-stack pthread)
JSValue goqjs_eval(JSContext *ctx, const char *input, size_t input_len, const char *filename, int eval_flags);

// Call helpers (runs on 64MB-stack pthread to prevent SIGBUS in timer/fetch callbacks)
JSValue goqjs_call(JSContext *ctx, JSValueConst func_obj, JSValueConst this_obj, int argc, JSValueConst *argv);

// Exception helpers
JSValue goqjs_get_exception(JSContext *ctx);
JSValue goqjs_throw_error(JSContext *ctx, const char *msg);

// Length helper
int goqjs_get_length(JSContext *ctx, JSValueConst obj, int64_t *pres);

// OwnPropertyNames helper
int goqjs_get_own_property_names(JSContext *ctx, JSPropertyEnum **ptab, uint32_t *plen, JSValueConst obj, int flags);

#endif // GOQJS_BRIDGE_H
