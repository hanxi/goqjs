package qjs

/*
#include "quickjs.h"
*/
import "C"

// Runtime wraps a QuickJS JSRuntime.
type Runtime struct {
	rt *C.JSRuntime
}

// NewRuntime creates a new QuickJS runtime.
func NewRuntime() *Runtime {
	rt := C.JS_NewRuntime()
	if rt == nil {
		panic("qjs: failed to create runtime")
	}
	return &Runtime{rt: rt}
}

// Close frees the runtime. All contexts must be freed before calling this.
func (r *Runtime) Close() {
	if r.rt != nil {
		C.JS_FreeRuntime(r.rt)
		r.rt = nil
	}
}

// SetMemoryLimit sets the memory limit for the runtime.
// Use 0 to disable the limit.
func (r *Runtime) SetMemoryLimit(limit int64) {
	C.JS_SetMemoryLimit(r.rt, C.size_t(limit))
}

// SetMaxStackSize sets the maximum stack size for the runtime.
// Use 0 to disable the check.
func (r *Runtime) SetMaxStackSize(size int64) {
	C.JS_SetMaxStackSize(r.rt, C.size_t(size))
}

// UpdateStackTop updates the stack top pointer for stack overflow detection.
// Must be called before JS execution when using CGO, as the C stack pointer
// may differ between calls.
func (r *Runtime) UpdateStackTop() {
	C.JS_UpdateStackTop(r.rt)
}

// RunGC triggers garbage collection.
func (r *Runtime) RunGC() {
	C.JS_RunGC(r.rt)
}

// NewContext creates a new QuickJS context in this runtime.
func (r *Runtime) NewContext() *Context {
	ctx := C.JS_NewContext(r.rt)
	if ctx == nil {
		panic("qjs: failed to create context")
	}
	goCtx := &Context{
		ctx: ctx,
		rt:  r,
		fns: make(map[int64]struct{}),
	}
	// Register context for C->Go callback lookup
	ctxMu.Lock()
	ctxRegistry[ctx] = goCtx
	ctxMu.Unlock()
	return goCtx
}

// ExecutePendingJob executes a pending job (microtask).
// Returns true if a job was executed, false if no jobs are pending.
// Returns an error if the job threw an exception.
func (r *Runtime) ExecutePendingJob() (bool, error) {
	var pctx *C.JSContext
	ret := C.JS_ExecutePendingJob(r.rt, &pctx)
	if ret < 0 {
		// An exception occurred
		if pctx != nil {
			ctxMu.RLock()
			goCtx, ok := ctxRegistry[pctx]
			ctxMu.RUnlock()
			if ok {
				return false, goCtx.Exception()
			}
		}
		return false, ErrException
	}
	return ret > 0, nil
}
