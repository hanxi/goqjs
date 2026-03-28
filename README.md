# goqjs

基于 [quickjs-ng](https://github.com/nicbarker/quickjs) 的 Go JavaScript 运行时库，通过 CGO 直接编译 amalgamated 源码，提供 Web API polyfill 和 stdin/stdout JSON Lines 交互协议。

## 特性

- **quickjs-ng 引擎**: 支持 ES2023+，活跃维护的 QuickJS 分支
- **CGO 直接编译**: 无需预编译静态库，amalgamated 源码自动编译
- **Web API Polyfill**: console、fetch、crypto、zlib、Buffer、URL、Timer、TextEncoder/TextDecoder
- **lx 对象**: JS 层实现的 `globalThis.lx` 对象，提供 request/send/on 等接口
- **JSON Lines 协议**: 通过 stdin/stdout 进行进程间通信
- **Go 函数注入**: 支持将 Go 函数注入到 JS 全局作用域

## 项目结构

```
goqjs/
├── qjs/          # Go CGO 绑定层（核心库）
├── polyfill/     # Web API Polyfill（Go 实现）
├── js/           # JS 层脚本（lx_prelude.js）
├── cmd/goqjs/    # 独立程序入口
└── README.md
```

## 快速开始

### 编译

```bash
cd goqjs
go build ./cmd/goqjs/
```

### 运行

```bash
# 执行 JS 文件
./goqjs --file script.js

# 带参数运行
./goqjs --file script.js --memory-limit 128 --stack-size 8 --timeout 30

# 交互模式（stdin JSON Lines）
echo '{"id":"1","type":"eval","code":"1+1"}' | ./goqjs
```

### 作为库使用

```go
package main

import (
    "fmt"
    "goqjs/qjs"
    "goqjs/polyfill"
)

func main() {
    rt := qjs.NewRuntime()
    defer rt.Close()

    ctx := rt.NewContext()
    defer ctx.Close()

    // 注入 Web API polyfill
    tm := polyfill.InjectAll(ctx)
    defer tm.Close()

    // 执行 JS 代码
    result, err := ctx.Eval("1 + 2", "test.js")
    if err != nil {
        panic(err)
    }
    defer result.Free()
    fmt.Println(result.String()) // "3"

    // 注入 Go 函数
    global := ctx.GlobalObject()
    defer global.Free()
    global.SetFunction("myFunc", func(ctx *qjs.Context, this qjs.Value, args []qjs.Value) qjs.Value {
        return ctx.NewString("hello from Go!")
    }, 0)

    result2, _ := ctx.Eval("myFunc()", "test.js")
    defer result2.Free()
    fmt.Println(result2.String()) // "hello from Go!"
}
```

## JSON Lines 协议

### 请求格式（stdin）

```json
{"id":"1", "type":"eval", "code":"1+1", "filename":"test.js"}
{"id":"2", "type":"eval_file", "path":"/path/to/script.js"}
{"id":"3", "type":"dispatch", "event":"request", "data":"{\"action\":\"search\"}"}
{"id":"4", "type":"callMusicUrl", "source":"wy", "songInfo":"{\"name\":\"歌曲\",\"singer\":\"歌手\",\"songmid\":\"123\"}", "quality":"320k"}
{"id":"5", "type":"exit"}
```

### 响应格式（stdout）

```json
{"id":"1", "type":"result", "value":"2"}
{"id":"2", "type":"error", "message":"SyntaxError: ..."}
{"type":"event", "name":"inited", "data":{"sources":{}}}
```

**异步 dispatch 机制**：`dispatch` 和 `callMusicUrl` 请求会异步等待 JS handler 的 Promise 结果。JS 侧通过 `lx._dispatch(requestId, eventName, data)` 触发事件，handler 返回的 Promise resolve/reject 后，结果会通过 `dispatchResult`/`dispatchError` 内部事件回传到 Go 侧，最终返回给调用方。

## Web API Polyfill

| API | 说明 |
|-----|------|
| `console.log/warn/error/info/debug/trace` | 输出到 stderr |
| `setTimeout/clearTimeout` | 定时器 |
| `setInterval/clearInterval` | 周期定时器 |
| `fetch(url, options)` | HTTP 请求，返回 Promise |
| `crypto.md5/aesEncrypt/rsaEncrypt/randomBytes` | 加密工具 |
| `zlib.inflate/deflate` | 压缩/解压，返回 Promise |
| `Buffer.from/isBuffer/toString` | Buffer 操作 |
| `atob/btoa` | Base64 编解码 |
| `TextEncoder/TextDecoder` | 文本编码 |
| `URL/URLSearchParams` | URL 解析 |

## lx 对象

`globalThis.lx` 在 JS 层实现（`js/lx_prelude.js`），提供：

- `lx.request(url, options, callback)` - HTTP 请求（基于 fetch，回调式 API）
- `lx.send(eventName, data)` - 发送事件到 Go 侧
- `lx.on(eventName, handler)` - 注册事件处理器
- `lx._dispatch(requestId, eventName, data)` - 内部方法，支持 Promise 异步结果回传
- `lx._getSources()` - 获取已注册的音乐源列表
- `lx.utils.crypto.*` - 加密工具
- `lx.utils.zlib.*` - 压缩工具
- `lx.utils.buffer.*` - Buffer 工具

## 依赖

- Go 1.26+
- C 编译器（gcc/clang）
- 不支持交叉编译为 WASM

## 许可证

[MIT](LICENSE)
