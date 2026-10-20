# Mio M17e：纯 Mio Opcode Lowering 与 Native Runtime 集成（持续开发中）

## 2026-10-20 数组 / NativeFS / Enum 检查点（最新状态）

**M17e 未验收，M18 未开始。** 以下为当前状态；后文带 2026-10-19 日期的段落保留为历史记录，
不作为当前阻塞位置。没有生成或执行完整 Compiler A 原生镜像，没有 Compiler B/C/D 或 M18 Stage 2/3
产物，也没有将局部 fixture 的成功替代固定点验收。

Compiler A SHA256：`b5904e8fd51e96be16183826177504ed395be67143e0c78d7a4b5d304cfd00a1`，377 个函数。
最新探针输出错误码 `2`（UnsupportedOpcode）、detail `20006745`：

```text
function 19: kiso/io::write_all
byte offset 67: OpCallValue 3
stack: [writer.write, source, offset + written, length - written]
```

已越过 `kiso/bytes::write@16 OpArrayAppendAll` 和 `kiso/io::write_all@19 OpEnum`，没有跳过任何 Opcode。
`CallValue` 的目标来自 Writer 的函数字段；需要真正的 Closure / Environment / 共享 Cell、捕获变量
类型传播、间接调用返回类型和所有相关安全点的精确 Root。当前纯 Mio 后端未迁移 M15b 的
103/104/105 对象协议，因而继续 fail closed，不能把此调用替换为猜测的直接调用。`GetEnumValue`
还需要 variant/tag 敏感的 payload 类型约束，不能把不同变体的字段无条件合并。这些是继续 Compiler A
和高层 `fs.read_all` / `io.write_all` 的真实技术阻塞；本检查点没有宣称完整 Typed Array 或 NativeFS 验收。

### CallValue 阻塞的具体审计结果

Compiler A function 28 `kiso/fs::writer` 创建 function 376 `<closure:376>`，capture 为 `{Local:true, Index:0}`，
即捕获 File。function 19 从 Writer.field0 取出函数值，用 3 个参数调用并读取 WriteResult。
现有 Go 黄金实现位于 `internal/aot/darwinarm64/m15b_closure.go`：

- type 103，48-byte payload `{kind=4, functionID, parameters, returns, codeAddress, environment}`；
- type 104，`{kind=5, count, cells...}`；
- type 105，16-byte `{kind=6 | reference<<8, value}`，多个 closure 必须共享同一 Cell；
- x25 是隐藏 Environment 参数，调用方另留 16-byte slot 保存 x25，9+ 参数仍从 sp+0 开始；
- 创建 Cell / Environment / Closure 都可能触发 GC，中间对象和捕获变量必须逐一发布为精确临时 Root。

当前纯 Mio 类型图没有 function target 集合、参数/返回签名边和 capture 边，也没有 Cell slot 规划。
纯 Mio main 还使用 x25 保存顶层表达式结果，因而 ABI 迁移必须保护此状态，不能只加一个 BLR。
后续需要先迁移 `internal/aot/m6.go` 的 functionTargets / deferred call / captures 约束，再迁移
Cell 化 local、Get/SetUpvalue、三类 scanner 和返回/调用帧 Root，之后重新运行本文件中的探针。
本轮没有为绕过这一阻塞生成不完整 Closure 对象或空 Root Map。

### AppendAll 语义与对象生命周期审计

黄金实现：`internal/vm/vm.go` 的 OpArrayAppendAll、`internal/value/value.go` 的 AppendArray / ReserveArray /
grownArrayCapacity，以及 `internal/aot/darwinarm64/m12_array.go` 的 append/reserve runtime。

- 栈 `[... destination, source] -> [...]`，无返回值；通过共享 Descriptor 修改目标，元素浅复制。
- 源长度在 reserve 前冻结；空源不修改目标，自追加通过同一 Descriptor 的新 Storage 读取。
- type 101 Descriptor：`{flags, length, capacity, 1, 1, storage}`，48-byte payload；type 102 Storage：
  `{flags, capacity, 1, reference, data...}`，32-byte prefix。两者继续使用 16-byte 对象头。
- 非 nullable uint8 用 1-byte 存储；其他固定宽度值按黄金 AOT 放在 8-byte canonical slot：窄有符号数
  已符号扩展，uint8/16/32 零扩展。引用复制的是托管值；nullable scalar 使用 type 106 box。
- reserve 从至少 4 开始，低于 1024 加倍，其后增长 50%；复制旧元素后才发布新 Storage / capacity。
  所有新容量先清零，避免 free-list 复用产生伪引用。truncate 清除被移除的字节/引用槽。
- 调用方的精确 Shadow Frame 同时保护源/目标 Descriptor，GC scanner 传递保护两个 Storage 和活跃元素。
  分配后从 Descriptor 重新加载 Storage；新 Storage 初始化到发布之间没有分配安全点。
- 复制使用无分配的重叠安全循环（memmove 方向），不保留跨 GC 的旧 Storage 裸指针。对于可由语言
  产生的共享别名，自追加测试实际覆盖同一 Descriptor。语言没有创建两个独立 Descriptor 指向同一
  Storage 的 API；该人为布局情形尚无独立可执行差分测试，不冒称已覆盖。
- 先检查负容量、length+additional 的有符号/无符号溢出和乘法上界，再分配。非法参数 exit 1，OOM exit 2。
  纯 Mio 仍使用 16 KiB 数据页内的有界 Heap（扣除 globals/metadata）；Go runtime 的单 Storage 上界为
  1 MiB、VM 的数组分配限制为 256 MiB。这是资源上限差异，不能声称大数组 OOM 阈值完全相同。
- 显式 descriptor 的宽度/引用标志必须与求解类型一致；无法证明的泛型/混合 nullability 布局 fail closed。
  不通过忽略 Root 或把未知元素当整数来通过编译。

Compiler A 实际数组 constructor descriptor 扫描（全部主块/函数静态扫描，非动态执行次数）：

| descriptor | 元素类型 | constructor 数 |
|---:|---|---:|
| 1 | int64 | 38 |
| 4 | bool | 3 |
| 5 | string | 16 |
| 8 | Struct | 48 |
| 15 | uint8 | 24 |

### 本轮回归与 GC 安全

新增 `internal/aot/m17e_array_test.go` 和 `internal/aot/m17e_nativefs_test.go`。正常行为 fixture 都比较
VM / Go AOT（GCStress）/ 纯 Mio AOT 的真实 stdout、退出状态；纯 Mio Builder 在 VM 与 Go 引导的
Native Seed 上生成未签名镜像，再逐字节比较。目标镜像实际经过 ad-hoc codesign 和 strict verify 后
在本机 Darwin ARM64 执行。Go AOT 只构建一次性执行 Seed 和独立参考程序；目标机器码与 Mach-O
由纯 Mio 的 Decoder / Lowering / Encoder / Writer 生成。

覆盖空数组、0→4→8 等容量边界、Descriptor 别名/自追加、8 种元素（uint8、int8/16/32/64、
uint16/32、Struct）、浅复制引用存活、500 次后续对象分配及逐元素验证、完整引用尾槽清零、nullable
None/Some(0)、literal / make / clone 的独立性、SetIndex / SetNullableIndex、4096-byte 数据完整校验和、
负容量、容量乘法/长度加法溢出及有界 OOM。Enum type 107 分配、tag 和精确 bitmap 扫描另有引用/GC fixture；
尚未开放 enum payload 读取。

新增 `lower.compile_gc_stress(file)`：在相同 allocator 的每次分配入口执行精确 collection，同时保留
有界 Heap、普通重试路径和 32-bin free list。该模式没有关闭 GC，也没有把 scratch stack 当保守 Root。
动态字符串槽现在作为 tagged-union Root 发布，scanner 明确忽略 odd tagged ID，动态 Descriptor 则继续
追踪 Storage。修复 NativeFS `dir_name` 在 allocator 后误用被覆盖的长度寄存器；目录 GC 测试先真实复现
bus error，再验证修复。

### NativeFS 支持 / 验证矩阵

| 能力 | 纯 Mio bridge | 本轮真实执行状态 |
|---|---|---|
| open / file_status / close | 已生成 | UTF-8/CJK 文件；40 次开关复用超过 32-slot 数；新 handle 有效、旧 generation 拒绝 |
| read / write | 已生成 | 空文件、NUL/0xff/CJK 二进制内容、分段写入、短读、EOF、负 offset 与 offset+length 溢出 |
| open_dir / dir_next | 已生成 | 单文件真实目录遍历、末尾 false |
| dir_name / dir_is_file / dir_is_dir | 已生成 | UTF-8/CJK 名字、两次分配及后续 300 次分配均可 GC，原名仍可读取 |
| dir_status / dir_close | 已生成 | 真实目录成功状态与显式关闭 |
| stat / mkdir | 已生成 | 文件/目录分类、含 NUL 路径拒绝、真实目录创建 |
| 正常退出 cleanup | 已接入 | fixture 执行正常退出路径；未独立量化 fd/mmap 泄漏 |
| 无效/过期 handle | 已拒绝 | 原生两端 Closed=4；VM 在 MMA ResourceManager 边界以 closed_resource 错误停止 |
| 错误消息字符串 | 仍沿用 Go bridge 的空字符串约定 | 未证明与 VM 的详细错误文本一致 |
| kiso/bytes::write | AppendAll lowering 已越过 | 尚未完成独立高层端到端验收 |
| kiso/fs::read_all / io.write_all | 仍受 Closure/CallValue 阻塞 | 未通过，不以低层 bridge 测试替代 |

过期 handle 的 VM/Native 差异由 `TestM17ePureMioNativeFSStaleHandleBoundary` 显式断言：VM 输出 `0`
后在旧 handle 调用前报 closed_resource；两种 Native 输出 `0,4` 并退出 0。这是已有接口边界差异，
不是三端完全相同的结果。低层 IO 边界检查原先使用 `offset+length`，可整数溢出；本轮在 Go AOT 和
纯 Mio bridge 同步改为 `offset<=size && length<=size-offset`，并用 MaxInt64 输入锁定回归。

### 仓库回归（最终轮）

- `go test ./internal/aot/... -count=1`：通过，AOT 主包 155.207 s。
- `go test ./... -count=1 -timeout 30m`：通过，包括现有 LSP / 编辑器关联代码。
- `go vet ./internal/aot/...`：通过。
- `git diff --check`：通过。
- 纯 Mio 包测试：arm64 2 项、macho 2 项、lower 7 项均通过。
- `go test -race ./internal/aot/... -count=1 -timeout 30m -v`：最终轮通过，AOT 主包 388.243 s，未报告 race。
  此前未扩展 timeout 的原命令也已通过（387.307 s）。
- Compiler A 探针：VM 和真实 Native Mio Builder 均输出错误码 2 / detail 20006745。

### 可复现成果与性能边界

运行以下命令可重建证据目录，含输入 Mochi、可重新运行的 Builder Mio、未签名镜像、签名镜像、
SHA256、构建与执行 JSON（Darwin Rusage.Maxrss 单位为 byte）。NativeFS 输入内含测试临时路径，
其端到端重现需运行对应测试重新建立真实文件/目录；bulk fixture 无外部数据依赖，已从保存的 Builder
单独重建、cmp、strict verify 和再次执行通过：

```sh
MIO_M17E_EVIDENCE_DIR="$PWD/build/m17e-20261020" go test ./internal/aot \
  -run 'TestM17ePureMio(ArrayConstructCloneNullableAndBulk|NativeFSReadWrite|NativeFSDirectoryAndPaths|NativeFSStaleHandleBoundary|EnumAllocationAndTagGC)' \
  -count=1 -v
```

本轮单次数据（含进程启动与签名后首次执行开销，不是 M19 基准）：

| fixture | 纯 Mio VM decode/lower/write | 目标执行 wall | Peak RSS | 未签名镜像 |
|---|---:|---:|---:|---:|
| Array nullable / clone / bulk | 0.068 s | 0.366 s | 1,097,728 B | 32,768 B |
| NativeFS directory / GC | 1.336 s | 0.301 s | 1,196,032 B | 131,072 B |

Array fixture 未签名 SHA256：`a39686a4de8741f411de7d730371e84cdc6c9f0406b00951293a84e0ae5fa468`。
`go_seed_build_sign_and_native_mio_run_seconds` 包含 Go Seed 构建/签名，不能当纯 Mio Native 构建耗时。
GC 次数、GC 耗时和累计分配量暂未导出，JSON 明确标记 not exported，不填假零。原生 Compiler A 的
执行耗时/RSS、Compiler B SHA256 和原生 A→B→C→D 均无本轮数据；固定点目标仍为
`b44a55bf3f9020e6b0ff83ad137b25774814a32dfe8acaf070abcd167bfd1465`。

建议中文 Conventional Commit（不自动提交）：

```text
feat(aot): 扩展纯 Mio 数组 lowering 并验证 NativeFS GC 安全
```

## 2026-10-19 NativeFS MMA Bridge 检查点

纯 Mio lowering 已迁入 M16b 的 Darwin/ARM64 NativeFS bridge 机器码生成路径。识别使用
`$mma::mio/fs/runtime::<operation>::<signature fingerprint>` 的完整 binding key，并再次校验参数数目
与有返回值标志；同名、错误 hash 或任意其他 body-less 函数仍 fail closed，不能作为空函数通过。

当前 bridge 已生成 `open/read/write/close/file_status/open_dir/dir_next/dir_name/dir_is_file/
dir_is_dir/dir_status/dir_close/stat/mkdir` 全部入口以及共享 helper。句柄继续使用 32 个、每槽 128 字节的
私有 mmap 表，Mio 可见值为 `(generation << 8) | (slot + 1)`，不会暴露 fd 或宿主指针。路径通过私有
mmap 制作 NUL 结尾副本，嵌入 NUL fail closed；errno 分类、目录 buffer 和 Darwin syscall 编号与
M16b 相同。`dir_name` 使用 type 102 byte storage 和 type 101 array descriptor，并在 descriptor 分配前
把 storage 发布为一项临时 Shadow Root；GC scanner 已增加 descriptor/storage 精确路径。

真实 ARM64 回归使用完整 `open` binding key 打开含 NUL 字节内容的临时文件。纯 Mio Builder 生成
Mach-O，经过 ad-hoc codesign 与 `codesign --verify --strict` 后执行，输出 resource token `257`，与
Go AOT 一致。该测试验证的是 bridge 识别、静态字符串路径、mmap 路径副本、open syscall、代次句柄
分配和返回 ABI；尚不等价于完整 ReadAllResult 验收。

Compiler A 已从 function 0 的 body-less NativeFS stub 推进，随后补齐运行时分派的 `OpLen`、
`OpIndex`、byte-array `OpArrayWithCapacity` 和扩容式 `OpArrayPush`。最新首个阻塞为 function 17
`kiso/bytes::write` byte offset 16 的 `OpArrayAppendAll`，detail `18001664`。因此 NativeFS 入口已接通，
但完整 `read_all` 仍等待 AppendAll/ArrayTyped 等可变数组路径；M17e 与 M18 仍未完成。

## 2026-10-19 Phase 3 类型 / Nullable / Struct / GC 闭环

纯 Mio lowering 现在先运行独立的约束图分析。每个可达 byte offset 都有 operand-stack
entry state；global、local、直接调用参数/返回值和 Struct 字段通过 union 节点相连，跳转目标会按
完全相同的栈深合流。表示类别明确区分 integer、float32、float64、bool、tagged string ID、Array、
Struct、Enum、Function、Nil 和 unknown。unknown 不会被当作 Root；一旦分配/调用安全点需要发布一个
unknown local/global/operand，lowering 返回 `UnsafeType(function,offset,opcode)`。

本检查点新增：

- `OpNil` 和类型驱动的 `OpIsNil`。nullable Heap/Struct/Array/Enum/Function 与 nullable tagged string
  使用零表示 None；nullable scalar producer 在离开指令时分配 M13 type 106、16-byte payload box。
  因而 `None`、`Some(0)`、`Some(false)` 和 `Some(-0.0)` 已经在原生差分中分开。
- `OpStruct` 使用 M16b payload：16-byte object header 后为
  `{kind, fieldCount, fieldCount, bitmapWords, bitmap..., fields...}`。bitmap 由已求解字段类型生成，
  tagged string 不伪装成 Heap Root；`OpGetField` 同时验证 kind、fieldCount、预期 bitmapWords 和物理
  payload 边界，再按动态 bitmap 宽度取字段。
- main 和函数调用/分配安全点会物化 `{previous,count,roots...}` Shadow Frame。调用返回和分配返回后
  恢复链；函数 local 区先清零，避免尚未初始化的引用槽含有栈垃圾。
- 受限 16 KiB managed data page、递归精确 mark、线性 sweep、32-bin free list、一次 GC 后重试和
  明确 OOM exit 2 已接通。scanner 只接受精确 payload start，并验证 type 100 Struct / type 106 box。

真实 Apple Silicon 回归构造了 field 64 位于第二个 bitmap word 的 65-field Struct、嵌套 Struct、
main/global roots、调用方 operand/local roots、nullable scalar box，并在 main 和被调函数中各连续分配
400 个对象。该负载不可能在 16 KiB managed 区内仅靠 bump allocation 完成；程序经多次 GC/free-list
复用后仍读回原对象，stdout、退出码与 VM 相同，codesign strict 验证通过。

真实 Compiler A 已越过 `main@15 OpIsNil`、`main@34 OpIsNil` 和 `main@55 OpChant(string)`。
当前 fail-closed 阻塞是 `function 0` 的无函数体 NativeFS MMA stub，诊断 detail 为 `1000000`
（function 0、offset/opcode sentinel 0）。NativeFS bridge、Array/Enum/Closure 的纯 Mio runtime 和完整
Compiler A 自举仍未完成，因此本文件继续标记 **M17e 进行中、M18 未通过**。

## 2026-10-19 Phase 3 检查点

本检查点解决了纯 Mio lowering 对 `OpGetField` 的首个编译期阻塞，但**尚未完成对象分配、
精确 GC 或 M17e 验收**。真实 Compiler A 重新执行 lowering 后，第一个 fail-closed 阻塞已从
`main byte offset 12: OpGetField 1` 推进到 `main byte offset 15: OpIsNil`。

`main@12` 的数据流已由 Go M16b 的统一类型分析回归锁定：

- `OpCall 29` 调用 `kiso/fs::read_all(string) -> ReadAllResult`；
- 接收者是 `ReadAllResult { data: []uint8; error: FsError? }`，不是 NativeFS resource；
- operand stack 在指令前为 `[ReadAllResult]`，指令后为 `[FsError?]`；
- field index 为 1，两个字段都是精确引用（field 1 为 nullable Enum 引用）；
- M16b payload 是 `{kind, fieldCount, fieldCount, bitmapWords, bitmap..., fields...}`，
  2 字段对象有 1 个 bitmap word，因此该实例的 field 1 位于 payload `+48`；payload 前另有
  16 字节对象头；
- 这个读取不分配也不调用 helper，不是 GC safepoint；结果在后续进入 global/local 并跨越可分配
  调用时，必须由精确 root 分类发布到 Shadow Frame。

纯 Mio lowering 不把 `+48` 写死。它先检查非 nil、struct kind 和 fieldCount，再读取
`bitmapWords`，按 `payload + 32 + bitmapWords*8 + index*8` 计算地址。新增的 u16 index 回归使用
field 65，确保跨 64 字段位图边界时仍读取正确位置。错误诊断现在保留 function、byte offset 和
opcode；当前 Compiler A detail 为 `1534`，即 main offset 15、opcode 34 (`OpIsNil`)。

不能在此处把所有 `OpIsNil` 简化为 raw-slot 等零：nullable heap reference 使用零指针，nullable
scalar 则使用 M13 type 106 的 16 字节 payload box。Phase 3 的下一步是先迁入 M6/M13 类型传播、
精确 root 分类和 Shadow Frame，再同时开放 `Nil`/`IsNil`、Struct 分配和自动 GC；在这之前保持
fail closed。

## 结论

本轮已建立第一条不调用 Go lowering/encoder/writer 的真实路径：

```text
Mochi 文件 -> 纯 Mio v1-v14 Decoder/Validator -> 纯 Mio scalar lowering
           -> M17cd ARM64 Encoder/Fixup -> M17cd Mach-O Writer -> 原生程序
```

阶段一的整数、分支、循环、全局、局部、静态函数及 9+ 参数已经实际执行通过。Phase 2 又完成
了全部 Mochi v14 常量标签、窄整数符号扩展、浮点原始位和静态字符串常量区。
这不是完整 M17e：M11-M16b 精确 GC Runtime、对象分配、Closure 和 NativeFS 尚未迁入纯
Mio 后端。纯 Mio 编译 Compiler A 已越过此前的 string `UnsupportedConstant` 和首个
`OpGetField 1` lowering 阻塞；当前第一个阻塞是主块 byte offset 15 的 `OpIsNil`，它需要
nullable 类型分类和精确 root/GC 集成。没有跳过、修改 Mochi 或回退 Go AOT，因此 Compiler A
自举仍未通过。

## 模块清单

- `bunko/kiso/aot/lower/lower.mio`：Mochi block lowering、运行时 operand area、函数帧、
  Mio raw-slot ABI、全局地址 Patch、int64 输出 helper、元数据驱动 Struct field 读取。
- `bunko/kiso/aot/lower/lower_test.mio`：空 v14 输入、全部常量标签、字符串常量区、跨 bitmap
  边界的 GetField 地址计算及 fail-closed 回归。
- `bunko/kiso/aot/arm64/arm64.mio`：本轮补充 `MOVZ W`、`ADD W immediate`、`MSUB X`、
  `CSET W`，仍复用原有 Encoder/Fixup。
- `bunko/kiso/aot/macho/macho.mio`：未经修改的最终 Writer。
- `examples/m17e_aot/build.mio`：按当前 CLI/文件 API 使用固定输入输出路径的构建入口；没有虚构 argv。
- `examples/m17e_aot/stage1.mio`：循环、全局和 10 参数调用验收程序。
- `internal/aot/m17e_pure_mio_lowering_test.go`：VM/Native Builder 差分、全部常量位模式、字符串去重、
  Compiler A 首个字段的真实 M16b 类型/布局、codesign 和真实 ARM64 执行。
- `internal/aot/m17e_probe_test.go`：真实 Compiler A 的下一阻塞诊断固定为 main offset 15
  `OpIsNil`，避免只凭反汇编手工判断推进位置。

## Opcode 覆盖矩阵（2026-10-20 更新）

“A 使用”来自 Compiler A (`b5904e...`) 的实际 Mochi v14 静态扫描。Go 状态描述现有
M16b 后端；“纯 Mio”只描述本轮新 lowering，不能用 Go 列替代纯 Mio 完成度。

| # | Opcode | A 使用 | 现有 Go AOT | 纯 Mio M17e | Runtime / 限制 |
|---:|---|---:|---|---|---|
| 1 | Constant | 4597 | 有限实现 | 全部 v14 常量已实现 | float 运算仍需专用 lowering；静态字符串使用 `(id<<1)\|1` |
| 2 | Add | 357 | 有限实现 | int64 已实现 | float/复合语义未迁移 |
| 3 | Pop | 80 | 有限实现 | int64 顶层结果/函数丢弃 | 顶层动态类型标签未迁移 |
| 4 | Sub | 149 | 有限实现 | int64 已实现 | 同数值限制 |
| 5 | Mul | 58 | 有限实现 | int64 已实现 | 同数值限制 |
| 6 | Div | 60 | 有限实现 | int64 + 除零退出已实现 | MinInt/-1 精确错误尚未迁移 |
| 7 | Neg | 268 | 有限实现 | int64 已实现 | float 未迁移 |
| 8 | True | 436 | 已实现 | 已实现 | raw 0/1 |
| 9 | False | 460 | 已实现 | 已实现 | raw 0/1 |
| 10 | Not | 163 | 已实现 | 已实现 | scalar bool |
| 11 | Equal | 1180 | 有限实现 | scalar 已实现 | 尚无完整静态类型分析 |
| 12 | NotEqual | 225 | 有限实现 | scalar 已实现 | 同上 |
| 13 | Less | 398 | 有限实现 | int64 已实现 | float/string 未迁移 |
| 14 | LessEqual | 51 | 有限实现 | int64 已实现 | 同上 |
| 15 | Greater | 88 | 有限实现 | int64 已实现 | 同上 |
| 16 | GreaterEqual | 142 | 有限实现 | int64 已实现 | 同上 |
| 17 | Jump | 1091 | 已实现 | Label/Fixup 已实现 | 26-bit 范围检查由 Encoder 执行 |
| 18 | JumpIfFalse | 2302 | 已实现 | 已实现 | 19-bit CBZ 范围检查 |
| 19 | SetGlobal | 2 | 有限实现 | scalar 已实现 | layout 后 ADRP/ADD Patch |
| 20 | GetGlobal | 4 | 有限实现 | scalar 已实现 | 同上 |
| 21 | Nil | 32 | 有限实现 | 已实现 | 零 Nil / type 106 nullable scalar box |
| 22 | Chant | 14 | 有限实现 | int64/bool/string/nil 已实现 | 含动态 tagged-union string 输出 |
| 23 | SetLocal | 1586 | 有限实现 | scalar 已实现 | 256 local 上限 |
| 24 | GetLocal | 9539 | 有限实现 | scalar 已实现 | 同上 |
| 25 | Call | 2350 | 有限实现 | 无捕获静态调用已实现 | 1-8 寄存器、9+ 栈参数；MMA 未接入 |
| 26 | Return | 137 | 已实现 | 已实现 | 16-byte SP 对齐 |
| 27 | ReturnValue | 1272 | 已实现 | scalar 已实现 | x0 raw-slot 返回 |
| 28 | IntToFloat | 2 | 有限实现 | 未实现 | FP lowering/helper |
| 29 | FloatToInt | 3 | 有限实现 | 未实现 | FP 边界语义 |
| 30 | Array | 26 | 有限实现 | 已实现（可求解元素） | 8-byte word literal，未知元素 fail closed |
| 31 | Index | 896 | 有限实现 | 已实现 | Array / tagged 或动态 string，边界检查；元素类型延迟约束 |
| 32 | Len | 537 | 有限实现 | 已实现 | Array / tagged 或动态 string |
| 33 | SetIndex | 86 | 有限实现 | 已实现 | 1/8-byte 存储，unsigned bounds 检查 |
| 34 | IsNil | 37 | 有限实现 | 已实现 | nullable reference / boxed scalar |
| 35 | SetNullableIndex | 0 | 有限实现 | 已实现（已验证 None/Some(0)） | 使用 type 106 存储；未宣称任意 nullable 运算闭环 |
| 36 | Struct | 234 | 有限实现 | 已实现 | M16b bitmap、精确 Root 和真实 GC |
| 37 | GetField | 3025 | 有限实现 | 已实现 | 变量 bitmap 宽度与对象边界检查 |
| 38 | Enum | 410 | 有限实现 | 已实现 | M13 type 107；每实例 bitmap |
| 39 | EnumTag | 182 | 有限实现 | 已实现 | nil / kind 检查后读取 tag |
| 40 | GetEnumValue | 16 | 有限实现 | 未实现 | 需要 variant/tag 敏感的 payload 类型约束 |
| 41 | Map | 0 | 不支持 | 未实现 | VM 本身尚无完整 Native map 路径 |
| 42 | MapGet | 0 | 不支持 | 未实现 | 同上 |
| 43 | MapSet | 0 | 不支持 | 未实现 | 同上 |
| 44 | Function | 0 | 有限实现 | 未实现 | 函数值对象 |
| 45 | CallValue | 1 | 有限实现 | 未实现；当前首个阻塞 | function 19 kiso/io::write_all@67，3 参数 |
| 46 | Closure | 1 | 有限实现 | 未实现 | Cell/Upvalue/GC |
| 47 | GetUpvalue | 1 | 有限实现 | 未实现 | Cell/Upvalue |
| 48 | SetUpvalue | 0 | 有限实现 | 未实现 | Cell/Upvalue |
| 49 | DefineLocal | 292 | 有限实现 | scalar 已实现 | 捕获 local 尚未 Cell 化 |
| 50 | IntToFloat32 | 2 | 有限实现 | 未实现 | FP lowering |
| 51 | Float32ToInt | 2 | 有限实现 | 未实现 | FP 边界语义 |
| 52 | Float32ToFloat64 | 2 | 有限实现 | 未实现 | FP lowering |
| 53 | Float64ToFloat32 | 38 | 有限实现 | 未实现 | FP rounding |
| 54 | Math | 3 | 已实现 | 未实现 | Math primitive helper |
| 55 | IntegerConvert | 214 | 已实现 | 已实现 | canonical 整数转换的 checked 范围验证 |
| 56 | ArrayTyped | 17 | 有限实现 | 已实现（布局可证明） | uint8 / word literal；descriptor 必须匹配类型 |
| 57 | ArrayMake | 10 | 有限实现 | 已实现（布局可证明） | 填充与原始元素 Root、Storage 临时 Root |
| 58 | ArrayWithCapacity | 102 | 有限实现 | 已实现（布局可证明） | 溢出检查、清零 Storage、Descriptor 临时 Root |
| 59 | ArrayPush | 230 | 有限实现 | 已实现 | 共享 reserve；4/2x/1.5x 增长、GC 后重载指针 |
| 60 | ArrayCapacity | 0 | 有限实现 | 已实现 | Descriptor capacity |
| 61 | ArrayReserve | 0 | 有限实现 | 已实现 | 事务式扩容，保留旧数据 |
| 62 | ArrayClone | 7 | 有限实现 | 已实现（布局可证明） | 独立 Descriptor/Storage，引用元素浅复制 |
| 63 | ArrayTruncate | 13 | 有限实现 | 已实现 | 保留 capacity，清除截断槽 |
| 64 | ArrayAppendAll | 20 | 有限实现 | 已实现（同布局） | 冻结源长度、重叠安全复制、精确 Root |
| 65 | DataConvert | 27 | 已实现 | 未实现 | string/bytes 复制与 UTF-8 |

Compiler A 的无函数体 NativeFS MMA entry 已按完整签名 key 接入；同名伪造 / hash 不符仍拒绝。
表中的“已实现”只覆盖注明的表示和路径，不能用 48/65 个入口存在宣称 Compiler A 完整覆盖。

## 地址、ABI 与 Runtime

- Mach-O 仍按 16 KiB 布局；全局 ADRP 由 Writer 在最终布局后按 ARM64 4 KiB 页计算。
- 分支/调用先记录 Label/Fixup，最终统一回填并执行 19/26 位范围检查。
- 每个函数保存 FP/LR 与 x19-x22；SP 调整均为 16 字节倍数。
- 前 8 个参数放 x0-x7，第 9 个起按 8 字节 slot 放调用方栈区，整个参数区向 16 字节取整。
- 当前 scalar tier 每帧预留 4,080 字节，其中 local 最多 256 个 slot，operand 最多 254 个 slot。
- int64 输出 helper 是可审计纯 Mio 指令构造，支持 0、负数和 MinInt64。
- 16 字节对象头、Shadow Frame、Root/Type Map、Mark-Sweep、Free List、Heap 上限和 NativeFS
  helper 仍只存在于已验证 Go AOT Runtime；本轮没有关闭 GC，也没有伪造空 Root Map，而是对所有
  需要对象 Runtime 的 opcode fail closed。

## 差分与真实执行

`examples/m17e_aot/stage1.mio` 的 VM 期望输出为 `15\n55\n`。纯 Mio Builder 在 VM 和
Native 上生成的未签名 Mach-O `cmp` 完全一致；文件 32,768 字节，原始 SHA-256 为
`96660f2da38453cd61f4a942e6123103a398b65aee5d1ac33e63d45db8de763e`。文件经 ad-hoc
codesign 后 `codesign --verify --strict` 通过，真实 Apple Silicon 执行 stdout 与 VM、Go AOT
逐字节一致，退出码 0。

Go AOT 与纯 Mio AOT 的未签名 Mach-O 都是 32,768 字节，但机器码/帧策略不同，原始文件不相同；
这里不把布局差异误报为错误，而是同时验证 Writer 布局、VM/Native Builder 确定性、签名、退出码和
stdout。M17cd 已继续覆盖同布局策略下 Writer 的逐字节 Go/Mio 黄金比较。

## 单次性能记录

2026-10-19，Apple Silicon，本轮 stage-one fixture，`/usr/bin/time -l` 单次测量：

| 项目 | 时间 | 最大 RSS | 说明 |
|---|---:|---:|---|
| Go AOT 构建 | 0.04 s | 26,001,408 B | 含 `go run` 调度开销 |
| 纯 Mio AOT / VM | 0.09 s | 28,147,712 B | 解码、lowering、Writer、落盘 |
| 纯 Mio AOT / Native | <0.01 s | 2,015,232 B | 已构建 Native Builder；stdout 为镜像大小 |
| 生成程序执行 | 0.11 s | 1,081,344 B | 含进程启动；输出 2 行 |

该 scalar tier 不分配 GC 对象，因此生成程序的 managed Heap 容量、累计对象分配、GC 次数、存活量和
GC 耗时均为 0/不适用。Compiler A 的 M16b 原生执行基线仍为约 51-54 秒，没有以微型 fixture
替代该基线。

## 自举阻塞与 M18 剩余工作

使用 Native 纯 Mio Builder 读取真实 Compiler A 可完成 v14 解码及完整常量规划；此前的
`UnsupportedConstant` 已消失。Phase 3 的 `OpGetField 1` 元数据 lowering 也已通过单元与真实
Compiler A 编译推进测试。当前 lowering 仍返回错误码 2 (`UnsupportedOpcode`)，首个阻塞为
主块 byte offset 15 的 `OpIsNil`。尚未生成 Compiler A 原生镜像，因此没有 Compiler B Mochi、
SHA256 或 `cmp` 结果，不能宣称自举成功。

后续必须按现有 Go 后端语义迁移，而不是另建 Runtime：

1. 完整类型/控制流固定点分析与 Root 生命周期；
2. float 运算、动态 string/bytes 与 Math（常量位模式和静态字符串表已完成）；
3. M12/M13 对象、Array、Struct、Enum、Nullable 及精确 Type Map；
4. M15b Closure/Cell/Upvalue 与 safepoint；
5. M11 Mark-Sweep/Free List/Shadow Frame 机器码模板和可配置 Heap；
6. M14 NativeFS 12 个 MMA bridge；
7. Compiler A -> B -> C 的真实原生连续自举及固定点比较。

建议提交信息（本轮不自动提交）：

```text
feat(aot): 完成纯 Mio AOT 常量与静态字符串 lowering
```
