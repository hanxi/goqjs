package polyfill

import (
	"net/url"
	"strings"

	"goqjs/qjs"
)

// InjectURL injects URL and URLSearchParams into the global scope.
func InjectURL(ctx *qjs.Context) {
	global := ctx.GlobalObject()
	defer global.Free()

	// URL constructor function
	global.SetFunction("URL", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError("URL requires at least 1 argument")
		}

		urlStr := args[0].String()
		if len(args) > 1 {
			baseStr := args[1].String()
			baseURL, err := url.Parse(baseStr)
			if err == nil {
				ref, err := url.Parse(urlStr)
				if err == nil {
					urlStr = baseURL.ResolveReference(ref).String()
				}
			}
		}

		parsed, err := url.Parse(urlStr)
		if err != nil {
			return ctx.ThrowError("Invalid URL: " + urlStr)
		}

		obj := ctx.NewObject()
		obj.Set("href", ctx.NewString(parsed.String()))
		obj.Set("protocol", ctx.NewString(parsed.Scheme+":"))
		obj.Set("host", ctx.NewString(parsed.Host))
		obj.Set("hostname", ctx.NewString(parsed.Hostname()))
		obj.Set("port", ctx.NewString(parsed.Port()))
		obj.Set("pathname", ctx.NewString(parsed.Path))
		obj.Set("search", ctx.NewString(func() string {
			if parsed.RawQuery != "" {
				return "?" + parsed.RawQuery
			}
			return ""
		}()))
		obj.Set("hash", ctx.NewString(func() string {
			if parsed.Fragment != "" {
				return "#" + parsed.Fragment
			}
			return ""
		}()))
		obj.Set("origin", ctx.NewString(parsed.Scheme+"://"+parsed.Host))
		obj.Set("username", ctx.NewString(parsed.User.Username()))
		pw, _ := parsed.User.Password()
		obj.Set("password", ctx.NewString(pw))

		// searchParams
		sp := buildSearchParams(ctx, parsed.Query())
		obj.Set("searchParams", sp)

		// toString method
		href := parsed.String()
		obj.SetFunction("toString", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
			return ctx.NewString(href)
		}, 0)

		return obj
	}, 2)

	// URLSearchParams constructor
	global.SetFunction("URLSearchParams", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		var params url.Values
		if len(args) > 0 {
			qs := args[0].String()
			qs = strings.TrimPrefix(qs, "?")
			var err error
			params, err = url.ParseQuery(qs)
			if err != nil {
				params = url.Values{}
			}
		} else {
			params = url.Values{}
		}
		return buildSearchParams(ctx, params)
	}, 1)
}

func buildSearchParams(ctx *qjs.Context, params url.Values) qjs.Value {
	obj := ctx.NewObject()

	obj.SetFunction("get", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.Null()
		}
		key := args[0].String()
		val := params.Get(key)
		if val == "" && !params.Has(key) {
			return ctx.Null()
		}
		return ctx.NewString(val)
	}, 1)

	obj.SetFunction("set", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) >= 2 {
			params.Set(args[0].String(), args[1].String())
		}
		return ctx.Undefined()
	}, 2)

	obj.SetFunction("has", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) < 1 {
			return ctx.NewBool(false)
		}
		return ctx.NewBool(params.Has(args[0].String()))
	}, 1)

	obj.SetFunction("delete", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) >= 1 {
			params.Del(args[0].String())
		}
		return ctx.Undefined()
	}, 1)

	obj.SetFunction("toString", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		return ctx.NewString(params.Encode())
	}, 0)

	obj.SetFunction("append", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
		if len(args) >= 2 {
			params.Add(args[0].String(), args[1].String())
		}
		return ctx.Undefined()
	}, 2)

	return obj
}
