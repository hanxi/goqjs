package polyfill

import (
	"sync"
	"sync/atomic"
	"time"

	"goqjs/qjs"
)

// timerEntry represents a pending timer.
type timerEntry struct {
	id       int64
	fn       qjs.Value
	interval bool
	delay    time.Duration
	timer    *time.Timer
	ticker   *time.Ticker
	canceled bool
}

// TimerManager manages setTimeout/setInterval timers.
type TimerManager struct {
	mu      sync.Mutex
	timers  map[int64]*timerEntry
	counter atomic.Int64
	pending chan func()
	closed  atomic.Bool
	done    chan struct{}
	wg      sync.WaitGroup
}

// NewTimerManager creates a new TimerManager.
func NewTimerManager() *TimerManager {
	return &TimerManager{
		timers:  make(map[int64]*timerEntry),
		pending: make(chan func(), 256),
		done:    make(chan struct{}),
	}
}

// InjectTimers injects setTimeout, clearTimeout, setInterval, clearInterval.
func InjectTimers(ctx *qjs.Context, tm *TimerManager) {
	global := ctx.GlobalObject()
	defer global.Free()

	global.SetFunction("setTimeout", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.NewInt32(0)
		}
		fn := args[0].Dup()
		delay := 0
		if len(args) > 1 {
			delay = int(args[1].Int32())
		}
		if delay < 0 {
			delay = 0
		}

		id := tm.counter.Add(1)
		entry := &timerEntry{
			id:       id,
			fn:       fn,
			interval: false,
			delay:    time.Duration(delay) * time.Millisecond,
		}

		tm.mu.Lock()
		tm.timers[id] = entry
		tm.mu.Unlock()

		entry.timer = time.AfterFunc(entry.delay, func() {
			select {
			case tm.pending <- func() {
				tm.mu.Lock()
				e, ok := tm.timers[id]
				if ok {
					delete(tm.timers, id)
				}
				tm.mu.Unlock()
				if ok {
					if !e.canceled && !tm.closed.Load() {
						result, _ := e.fn.Call(ctx.Undefined())
						result.Free()
					}
					e.fn.Free()
				}
			}:
			case <-tm.done:
			}
		})

		return ctx.NewInt64(id)
	}, 2)

	global.SetFunction("clearTimeout", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.Undefined()
		}
		id := args[0].Int64()
		tm.mu.Lock()
		if entry, ok := tm.timers[id]; ok {
			entry.canceled = true
			if entry.timer != nil {
				entry.timer.Stop()
			}
			// Do not Free fn here; it may still be referenced by pending callbacks.
			// fn will be freed in Close().
		}
		tm.mu.Unlock()
		return ctx.Undefined()
	}, 1)

	global.SetFunction("setInterval", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.NewInt32(0)
		}
		fn := args[0].Dup()
		delay := 0
		if len(args) > 1 {
			delay = int(args[1].Int32())
		}
		if delay <= 0 {
			delay = 1
		}

		id := tm.counter.Add(1)
		entry := &timerEntry{
			id:       id,
			fn:       fn,
			interval: true,
			delay:    time.Duration(delay) * time.Millisecond,
		}

		tm.mu.Lock()
		tm.timers[id] = entry
		tm.mu.Unlock()

		entry.ticker = time.NewTicker(entry.delay)
		tm.wg.Add(1)
		go func() {
			defer tm.wg.Done()
			for {
				select {
				case <-tm.done:
					return
				case _, ok := <-entry.ticker.C:
					if !ok || tm.closed.Load() {
						return
					}
					tm.mu.Lock()
					_, exists := tm.timers[id]
					tm.mu.Unlock()
					if !exists {
						return
					}
					select {
					case tm.pending <- func() {
						if tm.closed.Load() {
							return
						}
						tm.mu.Lock()
						e, exists := tm.timers[id]
						tm.mu.Unlock()
						if exists && !e.canceled {
							result, _ := e.fn.Call(ctx.Undefined())
							result.Free()
						}
					}:
					case <-tm.done:
						return
					}
				}
			}
		}()

		return ctx.NewInt64(id)
	}, 2)

	global.SetFunction("clearInterval", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.Undefined()
		}
		id := args[0].Int64()
		tm.mu.Lock()
		if entry, ok := tm.timers[id]; ok {
			entry.canceled = true
			if entry.ticker != nil {
				entry.ticker.Stop()
			}
			// Do not Free fn here; it may still be referenced by pending callbacks.
			// fn will be freed in Close().
		}
		tm.mu.Unlock()
		return ctx.Undefined()
	}, 1)
}

// ProcessPending processes pending timer callbacks. Returns true if any were processed.
func (tm *TimerManager) ProcessPending() bool {
	if tm.closed.Load() {
		return false
	}
	processed := false
	for {
		select {
		case fn := <-tm.pending:
			if !tm.closed.Load() {
				fn()
			}
			processed = true
		default:
			return processed
		}
	}
}

// HasPending returns true if there are pending timers.
func (tm *TimerManager) HasPending() bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return len(tm.timers) > 0 || len(tm.pending) > 0
}

// Close cancels all pending timers and waits for all goroutines to exit.
func (tm *TimerManager) Close() {
	tm.closed.Store(true)
	close(tm.done)

	// Wait for all interval goroutines to exit first,
	// so no new callbacks are sent to pending.
	tm.wg.Wait()

	// Drain any remaining pending callbacks without executing them.
	// Some callbacks may Free their own fn (setTimeout), so we must
	// not double-free those entries.
	for {
		select {
		case <-tm.pending:
		default:
			goto drained
		}
	}
drained:

	// Free fn for all entries still in the map.
	// setTimeout callbacks that executed successfully already removed
	// themselves from the map and freed their fn.
	tm.mu.Lock()
	for id, entry := range tm.timers {
		entry.canceled = true
		if entry.timer != nil {
			entry.timer.Stop()
		}
		if entry.ticker != nil {
			entry.ticker.Stop()
		}
		entry.fn.Free()
		delete(tm.timers, id)
	}
	tm.mu.Unlock()
}
