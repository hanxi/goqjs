package qjs

import (
	"sync"
	"sync/atomic"
)

// GoFunction is the signature for Go functions callable from JavaScript.
type GoFunction func(ctx *Context, this Value, args []Value) Value

var (
	// fnRegistry maps function IDs to Go functions
	fnRegistry = make(map[int64]GoFunction)
	fnMu       sync.RWMutex
	fnCounter  atomic.Int64
)

// registerFunction registers a Go function and returns its unique ID.
func registerFunction(fn GoFunction) int64 {
	id := fnCounter.Add(1)
	fnMu.Lock()
	fnRegistry[id] = fn
	fnMu.Unlock()
	return id
}

// unregisterFunction removes a Go function from the registry.
func unregisterFunction(id int64) {
	fnMu.Lock()
	delete(fnRegistry, id)
	fnMu.Unlock()
}
