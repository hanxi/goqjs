package polyfill

import (
	"encoding/base64"
	"encoding/hex"

	"goqjs/qjs"
)

// InjectBuffer injects a simplified Buffer global object.
func InjectBuffer(ctx *qjs.Context) {
	global := ctx.GlobalObject()
	defer global.Free()

	buffer := ctx.NewObject()

	// Buffer.from(data, encoding)
	buffer.SetFunction("from", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.NewArrayBuffer(nil)
		}

		encoding := "utf8"
		if len(args) > 1 {
			encoding = args[1].String()
		}

		// If first arg is already an ArrayBuffer, return as-is
		data := args[0].ToByteArray()
		if data != nil {
			return ctx.NewArrayBuffer(data)
		}

		// String input
		str := args[0].String()
		switch encoding {
		case "base64":
			decoded, err := base64.StdEncoding.DecodeString(str)
			if err != nil {
				decoded, _ = base64.RawStdEncoding.DecodeString(str)
			}
			return ctx.NewArrayBuffer(decoded)
		case "hex":
			decoded, err := hex.DecodeString(str)
			if err != nil {
				return ctx.NewArrayBuffer(nil)
			}
			return ctx.NewArrayBuffer(decoded)
		default: // utf8, utf-8, ascii, latin1, binary
			return ctx.NewArrayBuffer([]byte(str))
		}
	}, 2)

	// Buffer.isBuffer(obj)
	buffer.SetFunction("isBuffer", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.NewBool(false)
		}
		data := args[0].ToByteArray()
		return ctx.NewBool(data != nil)
	}, 1)

	// Buffer.toString(buf, encoding) - standalone helper
	buffer.SetFunction("toString", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.NewString("")
		}
		data := args[0].ToByteArray()
		if data == nil {
			return ctx.NewString(args[0].String())
		}

		encoding := "utf8"
		if len(args) > 1 {
			encoding = args[1].String()
		}

		switch encoding {
		case "base64":
			return ctx.NewString(base64.StdEncoding.EncodeToString(data))
		case "hex":
			return ctx.NewString(hex.EncodeToString(data))
		default: // utf8, utf-8, ascii
			return ctx.NewString(string(data))
		}
	}, 2)

	global.Set("Buffer", buffer)
}
