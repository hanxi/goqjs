package polyfill

import (
	"fmt"
	"os"
	"strings"

	"goqjs/qjs"
)

// InjectConsole injects the console object into the global scope.
// Output goes to stderr to avoid polluting stdout JSON Lines protocol.
func InjectConsole(ctx *qjs.Context) {
	global := ctx.GlobalObject()
	defer global.Free()

	console := ctx.NewObject()

	makeLogFn := func(prefix string) qjs.GoFunction {
		return func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
			parts := make([]string, len(args))
			for i, arg := range args {
				parts[i] = arg.String()
			}
			msg := strings.Join(parts, " ")
			if prefix != "" {
				fmt.Fprintf(os.Stderr, "[%s] %s\n", prefix, msg)
			} else {
				fmt.Fprintf(os.Stderr, "%s\n", msg)
			}
			return ctx.Undefined()
		}
	}

	console.SetFunction("log", makeLogFn(""), 0)
	console.SetFunction("info", makeLogFn("INFO"), 0)
	console.SetFunction("warn", makeLogFn("WARN"), 0)
	console.SetFunction("error", makeLogFn("ERROR"), 0)
	console.SetFunction("debug", makeLogFn("DEBUG"), 0)
	console.SetFunction("trace", makeLogFn("TRACE"), 0)

	global.Set("console", console)
}
