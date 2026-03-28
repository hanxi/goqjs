package polyfill

import (
	"encoding/base64"

	"goqjs/qjs"
)

// InjectEncoding injects atob, btoa, TextEncoder, TextDecoder into the global scope.
func InjectEncoding(ctx *qjs.Context) {
	global := ctx.GlobalObject()
	defer global.Free()

	// atob: decode base64 string
	global.SetFunction("atob", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError("atob requires 1 argument")
		}
		encoded := args[0].String()
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			// Try URL-safe and raw variants
			decoded, err = base64.RawStdEncoding.DecodeString(encoded)
			if err != nil {
				return ctx.ThrowError("atob: invalid base64 string")
			}
		}
		return ctx.NewString(string(decoded))
	}, 1)

	// btoa: encode string to base64
	global.SetFunction("btoa", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError("btoa requires 1 argument")
		}
		str := args[0].String()
		encoded := base64.StdEncoding.EncodeToString([]byte(str))
		return ctx.NewString(encoded)
	}, 1)

	// TextEncoder
	textEncoder := ctx.NewObject()
	textEncoder.SetFunction("encode", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.NewArrayBuffer(nil)
		}
		str := args[0].String()
		return ctx.NewArrayBuffer([]byte(str))
	}, 1)
	global.Set("TextEncoder", textEncoder)

	// TextDecoder
	textDecoder := ctx.NewObject()
	textDecoder.SetFunction("decode", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.NewString("")
		}
		data := args[0].ToByteArray()
		if data == nil {
			// Try string fallback
			return ctx.NewString(args[0].String())
		}
		return ctx.NewString(string(data))
	}, 1)
	global.Set("TextDecoder", textDecoder)
}
