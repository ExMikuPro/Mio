# MioCore 内置 LSP 架构方案

- 阶段：Language Support 0.3.0（统一语义索引 + 语义导航 + 实时诊断 + VS Code 客户端接入）
- 状态：0.2.0 协议与诊断层之上完成语义导航阶段
- 范围：stdio JSON-RPC、生命周期、文档同步、实时诊断、统一 Semantic Index、Hover / Definition / References / Document Symbols / Completion / Signature Help、UTF-16 位置、版本校验、取消请求、未保存缓冲区、缓存与并发
- 非范围：Rename / Semantic Tokens / Formatting / Code Action / Workspace Symbols（不 advertise，见第 14 节）

## 1. MioCore 与 `cmd/mio` 的真实关系（先确认，不重命名 CLI）

仓库中不存在名为 `MioCore` 的可执行程序，也不存在 `cmd/miocore`。真实的层次是：

| 层 | 物理位置 | 说明 |
| --- | --- | --- |
| CLI 入口 | `cmd/mio` | `mio` 可执行文件，负责参数解析、`check` / `run` / `build` / `test` 等子命令 |
| 编译器核心（本方案称 MioCore） | `internal/lexer`、`internal/parser`、`internal/resolver`、`internal/typecheck`、`internal/constant`、`internal/bunko`、`internal/diagnostic`、`internal/compiler`、`internal/vm`、`internal/value`、`internal/stdlib` | 与 CLI 解耦的 Go 包 |
| 可嵌入宿主 API | 根包 `mio`（`mio.go`） | `Environment` / `Program` 的嵌入式编译执行接口 |

因此本阶段的落地方式是：**在 `cmd/mio` 中新增 `mio lsp` 子命令，直接调用 `internal/lsp`**。不新增 `cmd/mio-lsp`，不改名 `mio`，不改变现有子命令行为。`mio lsp` 与 `mio check` 共用同一套 Lexer、Parser、Resolver、TypeChecker，只是输出协议不同。

## 2. 工作区隔离

- M12～M15（Native AOT、GC）的未提交改动位于 `/Volumes/Sector0/AppData/Goland/Mio`（分支 `Mio-2`）。
- 本阶段全部改动只发生在独立工作区 `/Volumes/Sector0/AppData/Goland/Mio-vscode-worktree`（分支 `feature/vscode-language-support`）。
- 不修改 `internal/aot/**`、`internal/mochi/**`、`internal/mma/**`、`mma/**`、`internal/vm/**` 中与 GC / AOT / 字节码格式相关的文件。
- 不修改 Mochi v14 格式，不修改 Native AOT 流程，不修改 GC。
- 不执行 `git commit` / `push` / `tag` / `release`。

## 3. 总体架构

```text
VS Code
  └── mio-language-support (TypeScript)
        └── vscode-languageclient 9.x
              │  JSON-RPC 2.0 over stdio
              ▼
        mio lsp  (cmd/mio 子命令)
          └── internal/lsp
                ├── transport   Content-Length 帧 / JSON-RPC 编解码
                ├── server      initialize / shutdown / exit / $/cancelRequest
                ├── document    内存文档仓 + 版本 + 增量编辑
                ├── position    UTF-8 字节偏移 ⇄ UTF-16 (line, character)
                ├── analysis    语义快照 + 缓存 + 依赖失效 + 防抖/代际/取消
                ├── semantic    统一符号/引用/作用域/字段/变体索引
                ├── features    hover / definition / references / symbols /
                │               completion / signatureHelp
                └── diagnostics compiler pipeline → LSP Diagnostic
                      ├── internal/bunko（含 overlay）
                      ├── internal/constant
                      ├── internal/resolver
                      └── internal/typecheck
```

关键原则：

1. **单一语义来源**：诊断只能来自编译器，不用正则或猜测位置。
2. **stdout 只传协议**：任何日志写入 stderr 或 `window/logMessage`，绝不污染 JSON-RPC 通道。
3. **零第三方 Go 依赖**：协议层手写 JSON-RPC，不引入 `gopls` 或 LSP SDK，保持 `go.mod` 无依赖。
4. **能力诚实**：只 advertise 已实现且有测试覆盖的 Hover / Definition / References / DocumentSymbol / Completion / SignatureHelp；Rename、Semantic Tokens、Formatting 不进入 capabilities。

## 4. `internal/lsp` 模块划分

| 文件 | 职责 |
| --- | --- |
| `protocol.go` | JSON-RPC 2.0 消息、LSP 3.17 参数/结果子集、错误码 |
| `transport.go` | `Content-Length` 帧读写、大小上限、脏数据行 |
| `server.go` | 生命周期、方法分发、请求取消、`window/logMessage` |
| `document.go` | 文档打开/变更/关闭、版本校验、增量应用 |
| `position.go` | UTF-8 字节 ↔ UTF-16 坐标、越界夹取、CRLF、非法 UTF-8 |
| `uri.go` | `file:` URI ↔ 本地路径（含百分号解码） |
| `analysis.go` | 语义快照构建、单飞缓存、依赖 mtime 失效、并发限制 |
| `semantic.go` | 统一语义索引模型、声明收集、常量折叠前捕获 |
| `semantic_walk.go` | AST 遍历：引用、字段、枚举变体、调用、作用域 |
| `hover.go` / `definition.go` / `references.go` | 语义导航请求 |
| `symbols.go` / `completion.go` / `signature.go` | 文档符号、补全、签名提示 |
| `features.go` | Markdown 渲染、位置换算、请求解析 |
| `diagnostics.go` | 编译流水线、错误分类、LSP 诊断映射 |
| `bunkoRoot`（在 `diagnostics.go`） | 与 CLI 一致的 `MIO_BUNKO` → 向上查找 `bunko/` → `bunko` |

`internal/bunko` 增加**只增不改**的 overlay 支持：

```go
type Overlay struct { /* 规范化绝对路径 → 内存内容 */ }
func NewOverlay() Overlay
func (o *Overlay) Set(path string, content []byte)
func (o Overlay) Lookup(path string) ([]byte, bool)
func LoadProgramWithOverlay(inputPath, bunkoRoot string, registry *mma.ModuleRegistry, overlay Overlay) (*Program, error)
func NewWithOverlay(root string, overlay Overlay) *Loader
```

0.3.0 在此基础上只增不改地增加语句来源映射：

```go
type Import struct { Alias, Path string }
func (p *Program) SourceOf(statement ast.Statement) (string, bool)
// Program.StatementSources / Files / Imports
```

- 根文件、同目录依赖、Bunko 包源码读取时优先命中 overlay（键为规范化绝对路径）。
- 现有 `LoadProgram` / `LoadProgramWithRegistry` 行为完全不变。
- overlay 只覆盖文件内容，不新增目录项；目录枚举仍以磁盘为准。这是"未保存缓冲区"的边界，后续可扩展为完整虚拟文件系统。

## 5. 协议层

### 5.1 传输

- 仅支持 stdio（`mio lsp` 默认 `--stdio`；保留参数以便未来切换）。
- 帧格式：`Content-Length: <n>\r\n\r\n<payload>`，容忍 `Content-Type` 与未知头部。
- 默认单帧上限 64 MiB；超限返回 `-32700` 并断开，防止内存耗尽。
- 非法 UTF-8 / JSON 返回 `-32700 Parse error`；无 `method` 且无 `id` 的消息返回 `-32600`。

### 5.2 生命周期

| 方法 | 类型 | 行为 |
| --- | --- | --- |
| `initialize` | request | 只能一次；返回 capabilities + `serverInfo{name:"MioCore"}`；缓存 `rootUri`、`workspaceFolders`、`initializationOptions` |
| `initialized` | notification | 标记就绪，允许后续文档通知 |
| `shutdown` | request | 停止接收新请求与诊断，返回 `null` |
| `exit` | notification | 在 `shutdown` 之后退出码 0，否则 1 |
| 其他未实现请求 | request | `-32601 Method not found` |
| 其他通知 | notification | 静默忽略（协议要求） |

未 `initialize` 前收到请求返回 `-32002 Server not initialized`（`initialize` 自身除外）。`shutdown` 之后除 `exit` 外的请求返回 `-32600`。

### 5.3 并发与写入

- 单读循环解析消息；request 在独立 goroutine 中执行，notification 在文档顺序上串行处理（保证增量编辑顺序）。
- 所有响应/通知通过同一把写锁串行写帧。
- 每个 request 携带可取消的 `context.Context`。

### 5.4 取消请求

- `$/cancelRequest { id }` 取消对应 request。
- 若取消先于 request 分发到达，记录"预取消"集合，request 启动时立即取消。
- 被取消 request 返回 `-32800 Request cancelled`，不写结果。
- 诊断不是 request，但同一文档的旧分析会被新的变更取消（第 7 节）。

### 5.5 服务能力（0.3.0）

```json
{
  "positionEncoding": "utf-16",
  "textDocumentSync": {
    "openClose": true,
    "change": 2,
    "save": { "includeText": false }
  },
  "hoverProvider": true,
  "definitionProvider": true,
  "referencesProvider": true,
  "documentSymbolProvider": true,
  "completionProvider": { "triggerCharacters": ["."] },
  "signatureHelpProvider": {
    "triggerCharacters": ["(", ","],
    "retriggerCharacters": [","]
  }
}
```

`positionEncoding` 固定 `utf-16`（LSP 默认）。若客户端只支持 `utf-8`，仍按规范默认 `utf-16` 并在 stderr 记录。

**能力诚实原则**：只 advertise 已实现并有测试覆盖的方法。Rename、Semantic Tokens、
Formatting、Code Action、Workspace Symbols 均未实现，因此不出现在 capabilities 中。

## 6. 文档同步

### 6.1 通知

- `textDocument/didOpen`：记录 URI、languageId、version、全文。
- `textDocument/didChange`：支持 `Full` 与 `Incremental`（本阶段 advertise Incremental，同时兼容 Full 作为单条无 range 变更）。
- `textDocument/didSave`：立即重新诊断；若客户端附带 text（`includeText` 未启用，一般不附带）则先更新缓冲区。
- `textDocument/didClose`：删除文档、取消分析、发布空诊断以清除波浪线。

### 6.2 增量应用

每条 change 按 UTF-16 `range` 换算为字节区间，校验 `start <= end <= len(text)` 后替换。`rangeLength` 只作为诊断信息，不参与替换（兼容旧客户端）。越界 range 夹取到文档边界并记录 warning，不崩溃。

### 6.3 版本校验

- 文档保存 `version`；`didChange.version <= 当前 version` 的变更整条丢弃并记录 warning（防止回滚）。
- 每次分析捕获 `(version, text)` 快照；发布诊断时若版本已变化则丢弃结果。
- `publishDiagnostics` 携带 `version`，客户端据此丢弃过期诊断。

## 7. 实时诊断

### 7.1 流水线

与 `mio check` 完全一致：

1. `bunko.LoadProgramWithOverlay`（根文件来自编辑器缓冲区）。
2. `constant.Fold`。
3. `resolver.ResolveProgram`。
4. `typecheck.NewWithResolver(r).CheckProgram`。
5. 首个错误 → 一条 LSP 诊断。

文件不存在或 URI 不是 `file:`（如 `untitled:`）时退化为直接 `lexer.NewChecked` + `parser.ParseProgram` + 后续阶段（无 Bunko 合并），保证未保存草稿也有真实语法/类型诊断。

### 7.2 错误分类

| 错误类型 | 诊断 |
| --- | --- |
| `*diagnostic.Diagnostic` | 使用其 `Code`、`Severity`、主标签 `Span`、`notes`/`helps` |
| `*lexer.LexicalError` | `E0001`，位置到位置 +1 字节 |
| 其他（包加载、文件读取等） | `E0401`，范围 `0:0`，消息保留错误链 |

- `source` 固定 `mio`，`severity` 映射 error/warning。
- 无 Span 时不猜测行号，落在文件开头并在消息中说明。
- 若诊断 `Path` 指向其他文件，消息前缀 `reported in <path>`；本阶段只向当前文档发布，避免误标其他 URI。

### 7.3 调度

- `didOpen` / `didChange`：防抖 150 ms（`initializationOptions.diagnosticsDelayMs` 可覆盖，测试用 0～20 ms）。
- `didSave`：立即执行。
- 每次调度递增代际号并 `cancel` 上一轮；分析 goroutine 发布前再次校验代际与版本。
- 同时分析文档数上限 4，超出排队，避免大型包导入拖垮服务器。
- 诊断发布失败（客户端断开）只记录日志，不 panic。

### 7.4 诊断开关

- 客户端通过 `initializationOptions.diagnosticsEnabled` 传入 `mio.diagnostics.enabled`。
- 运行期 `workspace/didChangeConfiguration` 支持
  `{ settings: { mio: { diagnostics: { enabled: bool } } } }` 与扁平 `{ settings: { diagnosticsEnabled: bool } }`。
- 关闭时发布空诊断并跳过分析，不停止服务器。

## 8. 位置与编码

Mio 编译器位置：

- `Offset`：UTF-8 字节偏移（0 起）。
- `Line` / `Column`：1 起，`Column` 以 Unicode 标量计数，仅用于展示。

LSP 位置：`line` 0 起，`character` 以 UTF-16 code unit 计数。

实现：

- `PositionAt(byteOffset)`：扫描文本建立行起点；`character = utf16Len(text[lineStart:offset])`；`\r\n` 的 `\r` 不计入行内容；offset 夹取到 `[0, len]`。
- `OffsetAt(Position)`：按行起点 + 逐 rune 累计 UTF-16 宽度，遇到行终止符停止；越界夹取。
- 非法 UTF-8 字节按 1 字节 / 1 code unit 处理（与 Go `utf8.DecodeRune` 的 `RuneError,1` 对齐）。
- 代理对（emoji）计 2 个 code unit；CJK 计 1 个。
- 单元测试覆盖 ASCII、CJK、emoji、组合字符、CRLF、越界、反向互转。

## 9. 未保存缓冲区

- 文档内容始终以内存缓冲区为准；`didChange` 后不需要写盘。
- 分析时把缓冲区全文注入 Bunko overlay，根文件路径保持不变，因此：
  - `import` 解析、`MIO_BUNKO` 查找、同目录包语义与磁盘一致。
  - 不会在磁盘产生临时文件，不需要路径映射。
- 当前只把正在分析的文档注入 overlay；同一目录其他未保存文件仍按磁盘内容参与导入分析。后续可把全部打开文档合并成 workspace overlay。

## 10. CLI 回退与避免双重诊断

VS Code 客户端状态机：

```text
disabled ──(设置关闭)──▶ 不启动 server，CLI 诊断生效
starting ──(initialize 成功)──▶ ready：server 拥有诊断，CLI 诊断停用并清空
starting ──(启动/握手失败)──▶ failed：CLI 诊断生效，输出升级提示
ready ──(进程退出/崩溃)──▶ stopped：自动回到 CLI 诊断
                          └─ vscode-languageclient 重启成功 ▶ ready：交还 server
```

- `mio.languageServer.enabled`（默认 `true`）控制是否尝试启动。
- `mio.diagnostics.enabled` 在 LSP ready 时通过 `workspace/didChangeConfiguration` 控制 server 诊断；在回退模式下控制 CLI 诊断。
- `Mio: Check Current File` 在 LSP ready 时提示"诊断由语言服务器提供"，不再启动 CLI，避免两套诊断打架。
- 服务器启动失败（旧版 `mio` 无 `lsp` 子命令）属于预期路径：记录日志、回退 CLI，不弹阻断式错误。
- `vscode-languageclient` 内置崩溃重启：重启期间保持 CLI 回退，收到 `State.Running` 后重新切换到 server，避免两套诊断并存。
- 手动 `run` / `build` / `version` 命令仍走 CLI，不受影响。

## 11. VS Code 客户端

- 依赖：`vscode-languageclient@^9.0.1`（CommonJS、LSP 3.17.5，兼容 VS Code 1.85+）。
- `serverOptions`：`command = mio.executablePath 或 PATH 中的 mio`，`args = ["lsp"]`，`options.env` 注入 `MIO_BUNKO`（与现有 `findBunkoRoot` 一致）。
- `documentSelector`：`{ language: "mio", scheme: "file" }` 与 `{ language: "mio", scheme: "untitled" }`。
- `synchronize.configurationSection = "mio"`，另在启动后主动发送一次 `workspace/didChangeConfiguration`，消除客户端实现差异。
- 输出复用现有 `Mio` OutputChannel；server 的 `window/logMessage` 与客户端状态转换都写入该通道。
- 配置变更（`executablePath` / `languageServer.enabled`）触发客户端重启；`diagnostics.enabled` 只发配置通知。
- `.vscodeignore` 精确放行 `vscode-languageclient`、`vscode-languageserver-protocol`、`vscode-languageserver-types`、`vscode-jsonrpc`、`semver`、`minimatch`，其余 `node_modules/**` 继续排除，保持 VSIX 体积。

## 12. 测试策略

### 12.1 Go

| 测试 | 覆盖 |
| --- | --- |
| `internal/lsp/transport_test.go` | 帧读写、大小上限、非法头、EOF |
| `internal/lsp/protocol_test.go` | 请求/通知/响应编解码、错误码、ID 原样回传 |
| `internal/lsp/position_test.go` | UTF-8 ↔ UTF-16、CJK/emoji/CRLF/越界/非法 UTF-8 |
| `internal/lsp/document_test.go` | 增量编辑、版本回退丢弃、越界 range、关闭清理 |
| `internal/lsp/server_test.go` | initialize → didOpen → 诊断 → shutdown → exit；未初始化错误；method not found；取消请求 |
| `internal/lsp/diagnostics_test.go` | 真实 Mio 语义：语法错误、未定义变量、类型不匹配、合法程序；未保存缓冲区；overlay 依赖 |
| `internal/lsp/semantic_test.go` | Hover/Definition/References：变量、参数、函数、遮蔽、结构体字段、枚举变体、泛型、闭包捕获、`teisu`、递归与互相调用、文档注释、跨文件、真实 Bunko 源码 |
| `internal/lsp/feature_test.go` | 补全作用域/遮蔽/字段/变体/模块导出/字面量/注释抑制/同名去重；签名提示嵌套调用、泛型、枚举构造、未完成调用、非调用上下文 |
| `internal/lsp/tolerance_test.go` | 缺少右括号、未闭合代码块、未终止字符串、未完成泛型/导入、未知符号、注释与字符串不参与引用 |
| `internal/lsp/navigation_test.go` | JSON-RPC：initialize capabilities（含不 advertise 的项）、hover/definition/references/documentSymbol/completion/signatureHelp、版本更新、取消、并发、Unicode、跨打开文档引用 |
| `internal/lsp/cache_test.go` | 单飞、版本缓存、取消、依赖 mtime 失效、淘汰、大程序性能（120 函数约 25ms） |
| `internal/lsp/real_project_test.go` | 仓库真实 `bunko/kiso/path/path.mio`：无诊断、跨包泛型调用 hover/definition、文档符号 |
| `internal/bunko/overlay_test.go` | overlay 根文件与依赖，默认行为不变 |
| `cmd/mio` | `mio lsp` 子命令存在、usage 更新 |

### 12.2 VS Code

| 测试 | 覆盖 |
| --- | --- |
| 单元（纯 Node） | 服务器参数/环境构造、状态机、URI/path 纯函数 |
| 集成（Extension Host） | 激活后 server 进入 ready；打开错误文件收到 LSP 诊断；调试后诊断清除；悬停、跳转定义、查找引用、大纲、补全、签名提示；文档修改后结果更新；CLI 回退关闭与恢复；设置变更重启；卸载后无残留 |

### 12.3 命令

```bash
# Go
go test ./internal/lsp/... ./internal/bunko/... ./cmd/mio/...
# 扩展
cd editors/vscode && npm run compile && npm run test:unit
npm run test:integration
```

## 13. 统一语义索引与语义导航（0.3.0）

### 13.1 分层

```text
internal/lsp
  ├── analysis.go       每文档版本的语义快照 + 缓存 + 依赖失效 + 单飞构建
  ├── semantic.go       索引模型、声明收集、常量折叠前捕获、查询辅助
  ├── semantic_walk.go  AST 遍历：引用、字段、枚举变体、调用、作用域
  ├── hover.go / definition.go / references.go
  ├── symbols.go / completion.go / signature.go
  └── features.go       Markdown、位置换算、请求参数解析
```

### 13.2 符号身份

| 来源 | symbolID | 说明 |
| --- | --- | --- |
| Resolver 绑定（变量/参数/函数/类型） | `binding:<resolver.Symbol.ID>` | 遮蔽、闭包、递归、互相调用都以真实绑定为准 |
| 结构体字段 | `field:<类型稳定名>#<字段名>` | Resolver 不建模成员，用类型限定身份 |
| 枚举变体 | `variant:<类型稳定名>#<变体名>` | 构造与 `baai` 模式共用同一身份 |
| `teisu` 常量 | `const:<文件键>:<字节偏移>` | 常量折叠会移除声明，因此在折叠前捕获 |
| 无 Resolver 绑定的声明 | `decl:<文件键>:<字节偏移>` | 仅用于悬停/大纲，不伪造引用 |

绝不使用裸字符串名称判断两个引用相同。跨分析合并引用时使用"物理声明位置
（文件 URI + 原始字节范围）"，因为同一文件作为入口或被导入时 `pkg::` 前缀不同。

### 13.3 源码位置与 Bunko 改写

- `internal/bunko` 的 rewriter 只改写 `Identifier.Name`，保留原始 `Range`。
- `bunko.Program.StatementSources` 记录合并后每个顶层语句的来源文件；
  `Files` 列出全部参与文件；`Imports` 暴露根文件的 `yobu` 别名 → 路径。
- LSP 的所有 Range 都来自原始 `ast.Span`，显示名来自源码切片，
  不会展示 `pkg::name`，也不会用改写后的名字替换编辑器内范围。
- 文件身份使用规范化路径（`EvalSymlinks`）作为内部键，位置 URI 优先使用客户端
  打开的 URI，避免 `/var` 与 `/private/var` 之类的符号链接别名拆出两个符号身份。

### 13.4 语义快照与缓存

`analysis` 保存版本、AST、Resolver、Checker、文件文本、诊断与只读索引。
构建完成后不再修改，可被并发请求读取。

- **单飞**：同一 (URI, version, text-hash) 只构建一次；其它请求等待同一次构建。
- **版本关联**：请求返回前重新读取文档快照，版本或文本变化则丢弃旧结果并重试，
  最多 3 次；诊断发布携带 version，代际号不匹配则丢弃。
- **依赖失效**：记录依赖文件的 size/mtime；请求命中缓存前检查依赖是否变化。
  `didChange` / `didSave` / `didClose` 会失效该文档以及依赖其磁盘文件的其它文档。
- **取消**：每个请求与诊断任务都有 `context.Context`；共享构建被取消时，
  其它调用方用自己的 context 重新构建。
- **限制**：并发构建数由 `Options.MaxConcurrentAnalysis` 控制（默认 4），
  每文档缓存项上限 16，超出时淘汰最久未使用项。

### 13.5 未完成源码容错

| 机制 | 位置 | 行为 |
| --- | --- | --- |
| `Parser.ParseProgramRecovering` | `internal/parser` | 单条语句失败时记录并跳过；代码块内按 `;`/`}` 恢复，函数不再整体丢失 |
| `Resolver.ResolveProgramLenient` | `internal/resolver` | 收集可恢复错误并继续；已建立绑定仍然可信 |
| `Checker.CheckProgramLenient` | `internal/typecheck` | 单语句/单函数失败不阻止后续检查；已推断类型保留 |
| `parseRecovering` | `internal/lsp/analysis.go` | 严格解析失败时改用容错 AST |
| 严格诊断流水线 | `analyzer.analyze` | 诊断始终来自与 `mio check` 相同的严格路径，容错结果不参与诊断 |

无法可靠解析的符号一律返回 `null` / 空集合，不构造猜测类型。

### 13.6 各方法语义来源

| 方法 | 实现要点 |
| --- | --- |
| Hover | 声明名称 > 引用绑定 > 表达式类型；Markdown 行内代码用双反引号包裹含反引号内容，文档注释放进 `text` 代码围栏避免 HTML/Markdown 注入 |
| Definition | 一律返回声明名称位置；导入符号返回真实包文件 URI 与原始范围 |
| References | 按 symbolID 反向索引；字段/变体按类型限定身份；顶层符号额外合并工作区其它打开文档的引用（按物理声明位置匹配） |
| DocumentSymbol | 只返回真实 AST 声明；`range` 为完整声明，`selectionRange` 为名称；结构体字段、枚举变体、函数参数作为 children |
| Completion | 作用域可见集合（内层遮蔽外层）→ 关键字 → 内置；`.` 后按模块别名导出 / 接收者类型字段 / 枚举变体；结构体字面量内只列未初始化字段；注释与字符串内不返回语义候选；函数插入片段 `${1:param}` 符合 Mio 调用语法 |
| SignatureHelp | 最内层 `CallExpression` 的实参 span 计算当前参数下标，不数逗号；未闭合调用用真实 Token 回退定位最内层未闭合括号；名称退化只在"唯一顶层同名函数"时使用 |

## 14. 下一阶段

1. 工作区符号索引：索引工作区内全部 `.mio` 文件，而不是仅打开文档和导入图。
2. `baai` 分支与类型标注中的类型参数补全。
3. 签名标签中的参数高亮范围（`ParameterInformation.label` 使用 `[start, end]`）。
4. 多诊断收集（需要编译器错误收集管线，当前仍只发布第一条）。
5. 大型 Bunko 树的索引持久化与冷启动延迟测量。

## 15. 不变式与验收

- `mio check` / `mio run` / `mio build` 输出与退出码不变。
- 无 LSP 时扩展行为与 0.1.0 一致。
- LSP 活跃时同一文件只有一套诊断（source `mio`，owner 为语言服务器/客户端）。
- 任何时刻 stdout 只包含 JSON-RPC 帧。
- Mochi v14、Native AOT、GC 代码零改动。
- 不自动 Commit / Push / Tag / Release。

## 16. 本阶段验证记录（0.3.0，2026-10-18）

全部命令在 worktree `Mio-vscode-worktree`（分支 `feature/vscode-language-support`）执行。

| 范围 | 命令 | 结果 |
| --- | --- | --- |
| LSP 测试 | `go test ./internal/lsp/... -count=1` | 通过 |
| 数据竞争 | `go test -race ./internal/lsp/... -count=1` | 通过 |
| 静态检查 | `go vet ./internal/lsp/... ./internal/bunko/... ./cmd/mio` | 通过 |
| Go 全仓库 | `go test ./... -count=1 -timeout 30m` | 通过（`selfhost/codegen` 276.4s、`selfhost/typecheck` 110.7s） |
| 扩展依赖 | `npm ci` | 338 packages |
| 编译 / Lint | `npm run compile`、`npm run lint` | 通过 |
| 扩展单元测试 | `npm run test:unit` | 67 passing |
| Extension Host | `npm test` | 11 passing（含语义导航、实时诊断、CLI 回退、资源释放） |
| VSIX | `npx @vscode/vsce package` + `npm run check:vsix` | 0.3.0，347 files / 644.49 KB，内容校验通过 |
| 大程序性能 | `go test ./internal/lsp -run TestSemanticAnalysisLargeProgram -v` | 120 函数 / 34.3 KiB：约 23 ms，堆增量约 3.7 MiB |

测试覆盖补充：

- Hover / Definition / References：变量、参数、函数、遮蔽、结构体字段、枚举变体、泛型、
  闭包捕获、`teisu`、递归、互相调用、文档注释。
- 跨文件：仓库真实 `bunko/kiso/path/path.mio`（跨包泛型 hover/definition）、临时
  Bunko 包、跨打开文档引用（函数与字段）。
- Unicode：CJK、非 BMP emoji（UTF-16 代理对）、CRLF、行列往返。
- 容错：缺少右括号、未闭合代码块、未终止字符串、未完成泛型/导入、未知符号。
- 协议：capabilities 真实性（不 advertise Rename/SemanticTokens/Formatting）、
  客户端 contentFormat/snippetSupport 降级、版本更新、取消、并发、增量编辑。
- 缓存：单飞、版本缓存、依赖 mtime 失效、反向依赖失效、容量淘汰、取消。

测试覆盖：

- 协议帧、并发写入、大小上限、非法头、请求/通知分类。
- initialize / initialized / shutdown / exit、重复 initialize、未初始化请求、未知方法、退出码。
- 增量编辑、UTF-16 ↔ UTF-8（CJK、emoji、CRLF、单独 CR、非法 UTF-8、越界）、版本回退丢弃。
- `$/cancelRequest`、预取消、分析上下文取消。
- 真实 Mio 语义：E0001、E0101、E0201、E0304、E0301、Bunko 导入、未保存缓冲区、文件不存在。
- VS Code：languageclient 启动、实时诊断、修改后清除、禁用后 CLI 回退、规避双重诊断、资源释放。
