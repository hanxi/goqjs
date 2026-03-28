package polyfill

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"goqjs/qjs"
)

// FetchOptions configures the fetch polyfill.
type FetchOptions struct {
	Client  *http.Client
	Timeout time.Duration
}

// DefaultFetchOptions returns default fetch options.
func DefaultFetchOptions() *FetchOptions {
	return &FetchOptions{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		Timeout: 30 * time.Second,
	}
}

// httpResult holds the raw HTTP response data (pure Go, no JS values).
// This is produced by the goroutine and consumed by the event loop.
type httpResult struct {
	statusCode int
	statusText string
	urlStr     string
	headers    http.Header
	body       []byte
	errMsg     string
	isError    bool
}

// fetchPending holds a pending fetch: the JS promise handles + the HTTP result channel.
type fetchPending struct {
	resolve qjs.Value
	reject  qjs.Value
	done    chan httpResult // capacity 1, written by goroutine
}

// FetchManager manages async fetch requests.
// HTTP requests run in goroutines. The event loop polls for completed requests
// and resolves/rejects promises on the main thread (safe for QuickJS).
type FetchManager struct {
	mu      sync.Mutex
	pending []*fetchPending
	notify  chan struct{} // non-blocking signal when a fetch completes
}

// NewFetchManager creates a new FetchManager.
func NewFetchManager() *FetchManager {
	return &FetchManager{
		notify: make(chan struct{}, 64),
	}
}

// NotifyChan returns a channel that receives a signal when any fetch completes.
// The event loop should select on this to wake up promptly.
func (fm *FetchManager) NotifyChan() <-chan struct{} {
	return fm.notify
}

// addPending registers a new in-flight fetch.
func (fm *FetchManager) addPending(p *fetchPending) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.pending = append(fm.pending, p)
}

// ProcessPending checks all in-flight fetches; for any that have completed,
// builds the JS Response object and resolves/rejects the promise.
// Returns true if any were processed.
func (fm *FetchManager) ProcessPending(ctx *qjs.Context) bool {
	fm.mu.Lock()
	if len(fm.pending) == 0 {
		fm.mu.Unlock()
		return false
	}
	// Snapshot current list
	snapshot := fm.pending
	fm.pending = nil
	fm.mu.Unlock()

	processed := false
	var stillPending []*fetchPending

	for _, p := range snapshot {
		select {
		case result := <-p.done:
			// This fetch is complete — resolve or reject
			if result.isError {
				errVal := ctx.NewString(result.errMsg)
				p.reject.Call(ctx.Undefined(), errVal)
				errVal.Free()
			} else {
				respObj := buildResponseObject(ctx, result.statusCode, result.statusText, result.urlStr, result.headers, result.body)
				p.resolve.Call(ctx.Undefined(), respObj)
				respObj.Free()
			}
			p.resolve.Free()
			p.reject.Free()
			processed = true
		default:
			// Not done yet, keep it
			stillPending = append(stillPending, p)
		}
	}

	if len(stillPending) > 0 {
		fm.mu.Lock()
		fm.pending = append(fm.pending, stillPending...)
		fm.mu.Unlock()
	}

	return processed
}

// HasPending returns true if there are in-flight fetch requests.
func (fm *FetchManager) HasPending() bool {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	return len(fm.pending) > 0
}

// Close frees any remaining pending JS values.
func (fm *FetchManager) Close() {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	for _, p := range fm.pending {
		// Drain channel if completed
		select {
		case <-p.done:
		default:
		}
		p.resolve.Free()
		p.reject.Free()
	}
	fm.pending = nil
}

// buildResponseObject creates a JS Response object from HTTP response data.
func buildResponseObject(ctx *qjs.Context, statusCode int, statusText string, urlStr string, respHeaders http.Header, respBody []byte) qjs.Value {
	respObj := ctx.NewObject()
	respObj.Set("ok", ctx.NewBool(statusCode >= 200 && statusCode < 300))
	respObj.Set("status", ctx.NewInt32(int32(statusCode)))
	respObj.Set("statusText", ctx.NewString(statusText))
	respObj.Set("url", ctx.NewString(urlStr))

	// Headers object with get()/has() methods (browser-compatible)
	headersObj := ctx.NewObject()
	headerMap := make(map[string]string)
	for k, vals := range respHeaders {
		lowerKey := strings.ToLower(k)
		joinedVal := strings.Join(vals, ", ")
		headersObj.Set(lowerKey, ctx.NewString(joinedVal))
		headerMap[lowerKey] = joinedVal
	}
	headersObj.SetFunction("get", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.Null()
		}
		key := strings.ToLower(args[0].String())
		if val, ok := headerMap[key]; ok {
			return ctx.NewString(val)
		}
		return ctx.Null()
	}, 1)
	headersObj.SetFunction("has", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.NewBool(false)
		}
		key := strings.ToLower(args[0].String())
		_, ok := headerMap[key]
		return ctx.NewBool(ok)
	}, 1)
	respObj.Set("headers", headersObj)

	// Body as string (captured for closures)
	bodyStr := string(respBody)
	bodyBytes := respBody

	// text() method
	respObj.SetFunction("text", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		p, res, rej := ctx.NewPromise()
		textVal := ctx.NewString(bodyStr)
		res.Call(ctx.Undefined(), textVal)
		textVal.Free()
		res.Free()
		rej.Free()
		return p
	}, 0)

	// json() method
	respObj.SetFunction("json", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		p, res, rej := ctx.NewPromise()
		parsed, parseErr := ctx.ParseJSON(bodyStr)
		if parseErr != nil {
			errVal := ctx.NewString(fmt.Sprintf("json parse error: %v", parseErr))
			rej.Call(ctx.Undefined(), errVal)
			errVal.Free()
		} else {
			res.Call(ctx.Undefined(), parsed)
			parsed.Free()
		}
		res.Free()
		rej.Free()
		return p
	}, 0)

	// arrayBuffer() method
	respObj.SetFunction("arrayBuffer", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		p, res, rej := ctx.NewPromise()
		buf := ctx.NewArrayBuffer(bodyBytes)
		res.Call(ctx.Undefined(), buf)
		buf.Free()
		res.Free()
		rej.Free()
		return p
	}, 0)

	return respObj
}

// InjectFetch injects the fetch function into the global scope.
// HTTP requests run asynchronously in goroutines. The returned FetchManager
// must be polled in the event loop to resolve/reject promises.
func InjectFetch(ctx *qjs.Context, opts *FetchOptions) *FetchManager {
	if opts == nil {
		opts = DefaultFetchOptions()
	}

	fm := NewFetchManager()

	global := ctx.GlobalObject()
	defer global.Free()

	global.SetFunction("fetch", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError("fetch requires at least 1 argument")
		}

		urlStr := args[0].String()

		// Parse options (extract all values from JS before spawning goroutine)
		method := "GET"
		var body string
		var headers map[string]string

		if len(args) > 1 && args[1].IsObject() {
			options := args[1]

			methodVal := options.Get("method")
			if !methodVal.IsUndefined() && !methodVal.IsNull() {
				method = strings.ToUpper(methodVal.String())
			}
			methodVal.Free()

			bodyVal := options.Get("body")
			if !bodyVal.IsUndefined() && !bodyVal.IsNull() {
				body = bodyVal.String()
			}
			bodyVal.Free()

			headersVal := options.Get("headers")
			if headersVal.IsObject() {
				headers = make(map[string]string)
				keys := headersVal.Keys()
				for _, key := range keys {
					v := headersVal.Get(key)
					headers[key] = v.String()
					v.Free()
				}
			}
			headersVal.Free()
		}

		promise, resolve, reject := ctx.NewPromise()

		doneCh := make(chan httpResult, 1)
		fp := &fetchPending{
			resolve: resolve,
			reject:  reject,
			done:    doneCh,
		}
		fm.addPending(fp)

		// Launch HTTP request in a goroutine (off the CGO stack)
		client := opts.Client
		go func() {
			var bodyReader io.Reader
			if body != "" {
				bodyReader = strings.NewReader(body)
			}

			req, err := http.NewRequest(method, urlStr, bodyReader)
			if err != nil {
				doneCh <- httpResult{errMsg: fmt.Sprintf("fetch error: %v", err), isError: true}
				fm.notify <- struct{}{}
				return
			}

			for k, v := range headers {
				req.Header.Set(k, v)
			}

			resp, err := client.Do(req)
			if err != nil {
				doneCh <- httpResult{errMsg: fmt.Sprintf("fetch error: %v", err), isError: true}
				fm.notify <- struct{}{}
				return
			}
			defer resp.Body.Close()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				doneCh <- httpResult{errMsg: fmt.Sprintf("fetch read body error: %v", err), isError: true}
				fm.notify <- struct{}{}
				return
			}

			doneCh <- httpResult{
				statusCode: resp.StatusCode,
				statusText: resp.Status,
				urlStr:     urlStr,
				headers:    resp.Header,
				body:       respBody,
				isError:    false,
			}
			fm.notify <- struct{}{}
		}()

		return promise
	}, 2)

	return fm
}
