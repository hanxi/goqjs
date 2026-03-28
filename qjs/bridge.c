#include "bridge.h"
#include <string.h>
#include <pthread.h>
#include <stdlib.h>

// Macro-to-function wrappers
JSValue goqjs_null(void) { return JS_NULL; }
JSValue goqjs_undefined(void) { return JS_UNDEFINED; }
JSValue goqjs_exception(void) { return JS_EXCEPTION; }
JSValue goqjs_uninitialized(void) { return JS_UNINITIALIZED; }
JSValue goqjs_true(void) { return JS_TRUE; }
JSValue goqjs_false(void) { return JS_FALSE; }

// JSValue tag access
int32_t goqjs_get_tag(JSValue v) { return JS_VALUE_GET_TAG(v); }
int32_t goqjs_get_int(JSValue v) { return JS_VALUE_GET_INT(v); }
int goqjs_get_bool(JSValue v) { return JS_VALUE_GET_BOOL(v); }
double goqjs_get_float64(JSValue v) { return JS_VALUE_GET_FLOAT64(v); }
void *goqjs_get_ptr(JSValue v) { return JS_VALUE_GET_PTR(v); }

// JSValue type checks
int goqjs_is_null(JSValue v) { return JS_IsNull(v); }
int goqjs_is_undefined(JSValue v) { return JS_IsUndefined(v); }
int goqjs_is_exception(JSValue v) { return JS_IsException(v); }
int goqjs_is_number(JSValue v) { return JS_IsNumber(v); }
int goqjs_is_string(JSValue v) { return JS_IsString(v); }
int goqjs_is_bool(JSValue v) { return JS_IsBool(v); }
int goqjs_is_object(JSValue v) { return JS_IsObject(v); }
int goqjs_is_symbol(JSValue v) { return JS_IsSymbol(v); }
int goqjs_is_error(JSValue v) { return JS_IsError(v); }
int goqjs_is_function(JSContext *ctx, JSValue v) { return JS_IsFunction(ctx, v); }
int goqjs_is_array(JSValue v) { return JS_IsArray(v); }
int goqjs_is_promise(JSValue v) { return JS_IsPromise(v); }

// JSValue constructors
JSValue goqjs_new_bool(JSContext *ctx, int val) { return JS_NewBool(ctx, val); }
JSValue goqjs_new_int32(JSContext *ctx, int32_t val) { return JS_NewInt32(ctx, val); }
JSValue goqjs_new_int64(JSContext *ctx, int64_t val) { return JS_NewInt64(ctx, val); }
JSValue goqjs_new_float64(JSContext *ctx, double val) { return JS_NewFloat64(ctx, val); }
JSValue goqjs_new_uint32(JSContext *ctx, uint32_t val) { return JS_NewUint32(ctx, val); }

// Go function callback proxy
JSValue goqjs_proxy_fn(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv, int magic, JSValueConst *func_data) {
    (void)magic;
    int64_t fnID;
    JS_ToInt64(ctx, &fnID, func_data[0]);
    return goInvokeFunction(ctx, this_val, argc, argv, fnID);
}

// Create a new JS function that proxies to Go
JSValue goqjs_new_function(JSContext *ctx, const char *name, int argc, int64_t fnID) {
    JSValue data[1];
    data[0] = JS_NewInt64(ctx, fnID);
    return JS_NewCFunctionData2(ctx, goqjs_proxy_fn, name, argc, 0, 1, data);
}

// Property helpers
int goqjs_define_property_value_str(JSContext *ctx, JSValueConst this_obj, const char *prop, JSValue val, int flags) {
    return JS_DefinePropertyValueStr(ctx, this_obj, prop, val, flags);
}

// Promise helpers
JSValue goqjs_new_promise(JSContext *ctx, JSValue *resolve_funcs) {
    return JS_NewPromiseCapability(ctx, resolve_funcs);
}

// Eval on a large-stack pthread to avoid SIGBUS with large/obfuscated scripts.
// CGO goroutine C stacks are limited (~8MB on macOS arm64), which is not enough
// for QuickJS to parse deeply nested or large scripts.

typedef struct {
    JSContext *ctx;
    const char *input;
    size_t input_len;
    const char *filename;
    int eval_flags;
    JSValue result;
} goqjs_eval_args;

static void *goqjs_eval_thread(void *arg) {
    goqjs_eval_args *a = (goqjs_eval_args *)arg;
    // Update stack top so QuickJS sees the new thread's stack
    JSRuntime *rt = JS_GetRuntime(a->ctx);
    JS_UpdateStackTop(rt);
    a->result = JS_Eval(a->ctx, a->input, a->input_len, a->filename, a->eval_flags);
    return NULL;
}

// goqjs_eval runs JS_Eval on a dedicated pthread with a 64MB stack.
JSValue goqjs_eval(JSContext *ctx, const char *input, size_t input_len, const char *filename, int eval_flags) {
    goqjs_eval_args args = {
        .ctx = ctx,
        .input = input,
        .input_len = input_len,
        .filename = filename,
        .eval_flags = eval_flags,
        .result = JS_UNDEFINED,
    };

    pthread_t thread;
    pthread_attr_t attr;
    pthread_attr_init(&attr);
    // 64 MB stack — enough for deeply nested/obfuscated scripts
    pthread_attr_setstacksize(&attr, 64 * 1024 * 1024);

    int err = pthread_create(&thread, &attr, goqjs_eval_thread, &args);
    pthread_attr_destroy(&attr);

    if (err != 0) {
        // Fallback: run on current thread if pthread_create fails
        JS_UpdateStackTop(JS_GetRuntime(ctx));
        return JS_Eval(ctx, input, input_len, filename, eval_flags);
    }

    pthread_join(thread, NULL);

    // Restore stack top for the CGO thread
    JS_UpdateStackTop(JS_GetRuntime(ctx));

    return args.result;
}

// Call on a large-stack pthread (same approach as goqjs_eval).
// Timer callbacks and fetch resolves can trigger deeply nested JS execution,
// which overflows the CGO goroutine's limited C stack.

typedef struct {
    JSContext *ctx;
    JSValueConst func_obj;
    JSValueConst this_obj;
    int argc;
    JSValueConst *argv;
    JSValue result;
} goqjs_call_args;

static void *goqjs_call_thread(void *arg) {
    goqjs_call_args *a = (goqjs_call_args *)arg;
    JSRuntime *rt = JS_GetRuntime(a->ctx);
    JS_UpdateStackTop(rt);
    a->result = JS_Call(a->ctx, a->func_obj, a->this_obj, a->argc, a->argv);
    return NULL;
}

JSValue goqjs_call(JSContext *ctx, JSValueConst func_obj, JSValueConst this_obj, int argc, JSValueConst *argv) {
    goqjs_call_args args = {
        .ctx = ctx,
        .func_obj = func_obj,
        .this_obj = this_obj,
        .argc = argc,
        .argv = argv,
        .result = JS_UNDEFINED,
    };

    pthread_t thread;
    pthread_attr_t attr;
    pthread_attr_init(&attr);
    pthread_attr_setstacksize(&attr, 64 * 1024 * 1024);

    int err = pthread_create(&thread, &attr, goqjs_call_thread, &args);
    pthread_attr_destroy(&attr);

    if (err != 0) {
        JS_UpdateStackTop(JS_GetRuntime(ctx));
        return JS_Call(ctx, func_obj, this_obj, argc, argv);
    }

    pthread_join(thread, NULL);
    JS_UpdateStackTop(JS_GetRuntime(ctx));
    return args.result;
}

// Exception helpers
JSValue goqjs_get_exception(JSContext *ctx) {
    return JS_GetException(ctx);
}

JSValue goqjs_throw_error(JSContext *ctx, const char *msg) {
    return JS_ThrowInternalError(ctx, "%s", msg);
}

// Length helper
int goqjs_get_length(JSContext *ctx, JSValueConst obj, int64_t *pres) {
    return JS_GetLength(ctx, obj, pres);
}

// OwnPropertyNames helper
int goqjs_get_own_property_names(JSContext *ctx, JSPropertyEnum **ptab, uint32_t *plen, JSValueConst obj, int flags) {
    return JS_GetOwnPropertyNames(ctx, ptab, plen, obj, flags);
}
