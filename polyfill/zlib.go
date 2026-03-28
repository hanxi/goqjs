package polyfill

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"fmt"
	"io"

	"goqjs/qjs"
)

// InjectZlib injects the zlib utility object into the global scope.
func InjectZlib(ctx *qjs.Context) {
	global := ctx.GlobalObject()
	defer global.Free()

	zlibObj := ctx.NewObject()

	// zlib.inflate(buffer) -> Promise<ArrayBuffer>
	zlibObj.SetFunction("inflate", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError("inflate requires 1 argument")
		}

		var data []byte
		if buf := args[0].ToByteArray(); buf != nil {
			data = buf
		} else {
			data = []byte(args[0].String())
		}

		promise, resolve, reject := ctx.NewPromise()

		// Try zlib first, then raw deflate
		var result []byte
		var err error

		// Try zlib format
		reader, zlibErr := zlib.NewReader(bytes.NewReader(data))
		if zlibErr == nil {
			result, err = io.ReadAll(reader)
			reader.Close()
		}

		if zlibErr != nil || err != nil {
			// Try raw deflate
			flateReader := flate.NewReader(bytes.NewReader(data))
			result, err = io.ReadAll(flateReader)
			flateReader.Close()
		}

		if err != nil {
			errVal := ctx.NewString(fmt.Sprintf("inflate error: %v", err))
			reject.Call(ctx.Undefined(), errVal)
			errVal.Free()
		} else {
			buf := ctx.NewArrayBuffer(result)
			resolve.Call(ctx.Undefined(), buf)
			buf.Free()
		}

		resolve.Free()
		reject.Free()
		return promise
	}, 1)

	// zlib.deflate(buffer) -> Promise<ArrayBuffer>
	zlibObj.SetFunction("deflate", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError("deflate requires 1 argument")
		}

		var data []byte
		if buf := args[0].ToByteArray(); buf != nil {
			data = buf
		} else {
			data = []byte(args[0].String())
		}

		promise, resolve, reject := ctx.NewPromise()

		var buf bytes.Buffer
		writer := zlib.NewWriter(&buf)
		_, err := writer.Write(data)
		if err != nil {
			writer.Close()
			errVal := ctx.NewString(fmt.Sprintf("deflate error: %v", err))
			reject.Call(ctx.Undefined(), errVal)
			errVal.Free()
		} else {
			writer.Close()
			resultBuf := ctx.NewArrayBuffer(buf.Bytes())
			resolve.Call(ctx.Undefined(), resultBuf)
			resultBuf.Free()
		}

		resolve.Free()
		reject.Free()
		return promise
	}, 1)

	global.Set("zlib", zlibObj)
}
