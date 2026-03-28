package qjs

import (
	"strings"
	"testing"
)

func TestEvalBasic(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	val, err := ctx.Eval("1 + 2", "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val.Free()

	if !val.IsNumber() {
		t.Fatalf("expected number, got tag %d", val.val.tag)
	}
	if val.Int32() != 3 {
		t.Fatalf("expected 3, got %d", val.Int32())
	}
}

func TestEvalString(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	val, err := ctx.Eval(`"hello" + " " + "world"`, "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val.Free()

	if !val.IsString() {
		t.Fatalf("expected string")
	}
	if val.String() != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", val.String())
	}
}

func TestEvalBool(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	val, err := ctx.Eval("true", "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val.Free()

	if !val.IsBool() {
		t.Fatalf("expected bool")
	}
	if !val.Bool() {
		t.Fatalf("expected true")
	}
}

func TestEvalNull(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	val, err := ctx.Eval("null", "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val.Free()

	if !val.IsNull() {
		t.Fatalf("expected null")
	}
}

func TestEvalUndefined(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	val, err := ctx.Eval("undefined", "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val.Free()

	if !val.IsUndefined() {
		t.Fatalf("expected undefined")
	}
}

func TestEvalFloat64(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	val, err := ctx.Eval("3.14", "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val.Free()

	if !val.IsNumber() {
		t.Fatalf("expected number")
	}
	f := val.Float64()
	if f < 3.13 || f > 3.15 {
		t.Fatalf("expected ~3.14, got %f", f)
	}
}

func TestEvalError(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	_, err := ctx.Eval("throw new Error('test error')", "test.js")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "test error") {
		t.Fatalf("expected 'test error' in error, got: %v", err)
	}
}

func TestEvalSyntaxError(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	_, err := ctx.Eval("function(", "test.js")
	if err == nil {
		t.Fatalf("expected syntax error")
	}
}

func TestNewValues(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test NewString
	s := ctx.NewString("hello")
	defer s.Free()
	if s.String() != "hello" {
		t.Fatalf("expected 'hello', got '%s'", s.String())
	}

	// Test NewInt32
	i := ctx.NewInt32(42)
	defer i.Free()
	if i.Int32() != 42 {
		t.Fatalf("expected 42, got %d", i.Int32())
	}

	// Test NewFloat64
	f := ctx.NewFloat64(2.718)
	defer f.Free()
	if f.Float64() < 2.71 || f.Float64() > 2.72 {
		t.Fatalf("expected ~2.718, got %f", f.Float64())
	}

	// Test NewBool
	b := ctx.NewBool(true)
	defer b.Free()
	if !b.Bool() {
		t.Fatalf("expected true")
	}

	// Test Null/Undefined
	n := ctx.Null()
	if !n.IsNull() {
		t.Fatalf("expected null")
	}

	u := ctx.Undefined()
	if !u.IsUndefined() {
		t.Fatalf("expected undefined")
	}
}

func TestObjectProperties(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	obj := ctx.NewObject()
	defer obj.Free()

	obj.Set("name", ctx.NewString("test"))
	obj.Set("value", ctx.NewInt32(123))

	name := obj.Get("name")
	defer name.Free()
	if name.String() != "test" {
		t.Fatalf("expected 'test', got '%s'", name.String())
	}

	value := obj.Get("value")
	defer value.Free()
	if value.Int32() != 123 {
		t.Fatalf("expected 123, got %d", value.Int32())
	}
}

func TestArrayOperations(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	arr := ctx.NewArray()
	defer arr.Free()

	arr.SetByIndex(0, ctx.NewString("a"))
	arr.SetByIndex(1, ctx.NewString("b"))
	arr.SetByIndex(2, ctx.NewString("c"))

	if arr.Len() != 3 {
		t.Fatalf("expected length 3, got %d", arr.Len())
	}

	elem := arr.GetByIndex(1)
	defer elem.Free()
	if elem.String() != "b" {
		t.Fatalf("expected 'b', got '%s'", elem.String())
	}
}

func TestGoFunction(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Register a Go function
	global := ctx.GlobalObject()
	defer global.Free()

	global.SetFunction("add", func(ctx *Context, this Value, args []Value) Value {
		if len(args) < 2 {
			return ctx.NewInt32(0)
		}
		a := args[0].Int32()
		b := args[1].Int32()
		return ctx.NewInt32(a + b)
	}, 2)

	val, err := ctx.Eval("add(3, 4)", "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val.Free()

	if val.Int32() != 7 {
		t.Fatalf("expected 7, got %d", val.Int32())
	}
}

func TestGoFunctionWithStrings(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	global := ctx.GlobalObject()
	defer global.Free()

	global.SetFunction("greet", func(ctx *Context, this Value, args []Value) Value {
		if len(args) < 1 {
			return ctx.NewString("hello")
		}
		name := args[0].String()
		return ctx.NewString("hello " + name)
	}, 1)

	val, err := ctx.Eval(`greet("world")`, "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val.Free()

	if val.String() != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", val.String())
	}
}

func TestObjectKeys(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	val, err := ctx.Eval(`({a: 1, b: 2, c: 3})`, "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val.Free()

	keys := val.Keys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}

	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}
	for _, expected := range []string{"a", "b", "c"} {
		if !keyMap[expected] {
			t.Fatalf("missing key '%s'", expected)
		}
	}
}

func TestFunctionCall(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	val, err := ctx.Eval(`(function(a, b) { return a * b; })`, "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val.Free()

	if !val.IsFunction() {
		t.Fatalf("expected function")
	}

	result, err := val.Call(ctx.Undefined(), ctx.NewInt32(6), ctx.NewInt32(7))
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}
	defer result.Free()

	if result.Int32() != 42 {
		t.Fatalf("expected 42, got %d", result.Int32())
	}
}

func TestJSONStringify(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	val, err := ctx.Eval(`({name: "test", value: 42})`, "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val.Free()

	json := ctx.JSONStringify(val)
	if !strings.Contains(json, `"name"`) || !strings.Contains(json, `"test"`) {
		t.Fatalf("unexpected JSON: %s", json)
	}
}

func TestParseJSON(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	val, err := ctx.ParseJSON(`{"name":"hello","value":123}`)
	if err != nil {
		t.Fatalf("ParseJSON error: %v", err)
	}
	defer val.Free()

	name := val.Get("name")
	defer name.Free()
	if name.String() != "hello" {
		t.Fatalf("expected 'hello', got '%s'", name.String())
	}

	value := val.Get("value")
	defer value.Free()
	if value.Int32() != 123 {
		t.Fatalf("expected 123, got %d", value.Int32())
	}
}

func TestES2023Features(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Test Array.prototype.at()
	val, err := ctx.Eval(`[1,2,3,4,5].at(-1)`, "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val.Free()
	if val.Int32() != 5 {
		t.Fatalf("expected 5, got %d", val.Int32())
	}

	// Test Object.hasOwn
	val2, err := ctx.Eval(`Object.hasOwn({a: 1}, "a")`, "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val2.Free()
	if !val2.Bool() {
		t.Fatalf("expected true")
	}
}

func TestMemoryLimit(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	rt.SetMemoryLimit(1024 * 1024) // 1MB

	ctx := rt.NewContext()
	defer ctx.Close()

	// This should work fine
	val, err := ctx.Eval("1 + 1", "test.js")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	defer val.Free()
}

func TestRunGC(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Create some objects and run GC
	for i := 0; i < 100; i++ {
		val, _ := ctx.Eval(`({a: 1, b: [1,2,3]})`, "test.js")
		val.Free()
	}
	rt.RunGC()

	// Should still work after GC
	val, err := ctx.Eval("42", "test.js")
	if err != nil {
		t.Fatalf("Eval error after GC: %v", err)
	}
	defer val.Free()
	if val.Int32() != 42 {
		t.Fatalf("expected 42, got %d", val.Int32())
	}
}
