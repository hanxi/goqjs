package polyfill

import (
	"goqjs/qjs"
)

// PolyfillManagers holds all managers returned by polyfill injection.
type PolyfillManagers struct {
	TimerManager *TimerManager
	FetchManager *FetchManager
}

// Close releases resources held by all managers.
func (pm *PolyfillManagers) Close() {
	if pm.FetchManager != nil {
		pm.FetchManager.Close()
	}
	if pm.TimerManager != nil {
		pm.TimerManager.Close()
	}
}

// InjectAll injects all Web API polyfills into the given context.
// Returns PolyfillManagers that must be used in the event loop.
func InjectAll(ctx *qjs.Context) *PolyfillManagers {
	InjectConsole(ctx)
	tm := NewTimerManager()
	InjectTimers(ctx, tm)
	InjectEncoding(ctx)
	InjectBuffer(ctx)
	InjectURL(ctx)
	InjectCrypto(ctx)
	InjectZlib(ctx)
	fm := InjectFetch(ctx, nil)

	return &PolyfillManagers{
		TimerManager: tm,
		FetchManager: fm,
	}
}

// InjectAllWithOptions injects all polyfills with custom fetch options.
func InjectAllWithOptions(ctx *qjs.Context, fetchOpts *FetchOptions) *PolyfillManagers {
	InjectConsole(ctx)
	tm := NewTimerManager()
	InjectTimers(ctx, tm)
	InjectEncoding(ctx)
	InjectBuffer(ctx)
	InjectURL(ctx)
	InjectCrypto(ctx)
	InjectZlib(ctx)
	fm := InjectFetch(ctx, fetchOpts)

	return &PolyfillManagers{
		TimerManager: tm,
		FetchManager: fm,
	}
}
