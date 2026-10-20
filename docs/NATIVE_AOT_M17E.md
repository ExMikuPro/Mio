# Mio M17e：纯 Mio Opcode Lowering 与 Native Runtime 集成（本次自举验收通过）

## 2026-10-20 Heap 内存模型修复与真实原生固定点（最新状态）

本节覆盖下方全部历史状态。**本次列出的 M17e 验收条件已满足：纯 Mio Native A→B→C→D 已实际完成，B=C=D；M18 未启动。**
最终全量检查结果见本节验证表；不能把下方保留的历史 OOM / 未生成 B 记录当成当前状态。

### 32800 字节的来源与两个真实分配器问题

沿源码核对：`selfhost/bootstrap/bootstrap.mio` 读取 config → `kiso/fs::read_all` →
`array.with_capacity[uint8](32768)`。`read_all` 另外创建一个 32768-byte read buffer，按 32 KiB
调用 read，再逐字节 push 到输出数组，EOF 后返回准确长度；扩容遵循 4 / 2x / 1.5x 策略。
32 KiB 是普通分块 I/O 的容量策略，虽对空文件浪费容量，但不是非法对象，也不能靠缩小它解决大文件。
本轮没有修改 fs 源码、输入 A 或文件内容；保留原自举黄金哈希。

Storage payload 为四个 8-byte 字段 `{kind/width flags, capacity, version, reference flag}`，
随后是 `capacity * elementWidth` 个字节。因此字节数组请求 **32768 + 32 = 32800**；
对象头 `{type/mark flags, physical payload size}` 为 16 bytes，分配公式
`align16(payload + 16)`，这次为 **32816 bytes**，没有每页 GC bitmap 或隐含的页间对象头。
Array descriptor 是独立的 type-101 对象，48-byte payload + 16-byte header = 64 bytes；
Storage 是 type 102，字节元素 width=1、reference=false，引用元素 width=8、reference=true。

原托管区和 globals 共用一个 16384-byte Mach-O `__DATA` 页。A 的两个 globals 占 16 bytes，
runtime metadata 占 288 bytes（bump/end/GC count、32 个 bin heads 及对齐），托管对象区仅
16080 bytes；原 LLDB 32816-byte 申请即使 GC 清空全堆也不可能成功。
Mach-O 的 16 KiB 是映射/文件布局对齐，**不是 GC 的单对象上限**。mark 的范围来自 arena
起点和 bump，sweep 通过每个对象的 physical payload size 前进；既没有页内边界判断，也没有
跨页 relocation 表。原实现没有独立大对象机制，根本原因是把整个 managed arena 限死在一个映像页。

先接通 128 MiB 独立 arena 后，实际 A 又复现 exit 2。LLDB 在 `runtime_alloc_fail`
`0x1002265a8` 得到 type 100、payload 104、physical 128、已 GC 重试一次；heap walk 显示
134217728 bytes 全部走完、GC=4，**95576192 bytes 已空闲但最大空闲块只有 112 bytes**。
旧 allocator 不合并相邻死对象，并把较大的空闲块整块交给较小请求，存在严重的外部/内部碎片。
证据：`heap-stats.log`、`lldb-heap.log`。这证明单纯增大容量并不够。

### 最终内存管理方案

`bunko/kiso/aot/lower/lower.mio`：

- Mach-O Writer、16 KiB 映像对齐、globals 地址 Patch、对象/数组/Shadow Frame ABI 保持不变。
  runtime 启动使用 Darwin `mmap(NULL, capacity+288, RW, MAP_PRIVATE|MAP_ANON, -1, 0)`，
  将 x23 指向独立零初始化 metadata；arena 紧随其后且对齐 16 bytes。整个 arena 受精确 GC 管理，
  没有绕过 GC 的大对象旁路，也不为每个大对象创建独立 mmap。
- `compile` / `compile_gc_stress` 默认目标 arena 为 128 MiB；新增
  `compile_with_heap(file, capacity, gc_stress)`，要求 64 bytes..1 GiB 且 16-byte 对齐，非法参数拒绝。
  原生 bootstrap 显式选 **1 GiB**，与 `scripts/check-native-bootstrap.sh` 的目标资源协议一致。
  Seed Builder 的已有堆参数与它独立；本轮成功不依赖扩大宿主进程/Seed 的内存设置。
- logical heap end 精确等于对象区起点 + capacity，不使用内核向页取整后多出的映射字节。
  mmap 系统调用失败明确 exit 2；mapping 在进程退出时由 OS 统一释放。
- bump 分配与 32-bin 空闲表继续共用同一对象协议。sweep 合并最大连续死对象区，尾部死区退回
  bump frontier；分配较大空闲块时拆分，至少保留 32-byte remainder（16-byte header + link 空间）。
  更新 physical size 后才发布 bin，存活对象不移动，旧死对象内部 header 不再作为边界访问。
- payload 上界先于加法检查，防止负数/巨型请求导致 wrap。Storage 构造和增长原先不一致的
  1 MiB 硬上界统一到 1 GiB payload 协议（数据上界 1 GiB-32）；实际是否成功仍受 header、
  descriptor、其他存活对象、连续空间和配置容量限制，不承诺上界大小的任意对象一定可分配。
- 分配失败后保留一次真实 collect + retry；无合适空闲块且 bump 空间不足才返回 NULL，调用方
  转 `runtime_oom` exit 2。未吞错、未截断文件、未关闭边界检查。非移动堆仍可能因资源/碎片限制
  OOM；不会把资源失败当 EOF 或 I/O 成功。

大 Storage 可直接连续跨越任意多个 OS 页。输出数组继续按统一 reserve 协议增长，旧 Storage
在复制和 descriptor 切换完成前受 root 保护；复制无 safepoint。GC 中 Array descriptor 精确追踪
Storage；Storage 根据 reference flag 扫描引用槽，字节内容、整数和 padding 不当指针。
unused 引用槽仍显式清零。Struct bitmap、Enum bitmap、nullable box、Closure/Cell 扫描未放宽。
分配、合并和拆分都不会移动存活对象，因此无需 relocation 或更新现有对象引用。

VM 的 Array 使用 Go 管理的切片、由宿主 GC 回收，数组资源上限为 256 MiB；现有 Go Native
同样使用独立 mmap arena 和 type-101/102 原始布局，默认/可配置目标容量不等于 VM 宿主内存。
纯 Mio 原来的一页总堆是移植缺口，不是语言语义要求。Compiler A 还合理地需要 AST、Token、
类型表、字节码、常量池等大于一页的数组，不能专门处理 read_all；本次真实自编译也验证了这些负载。

### 自举推进时发现并修复的后续问题

1. 1 GiB 下 A 越过 OOM，但在 `selfhost/codegen::same_constant @199 OpDiv` exit 1。
   LLDB 精确分支 PC `0x100203974`，x9=`0x3ff0000000000000`（1.0），x10=0；后续是 SDIV。
   泛型 `kiso/math::is_inf` 的共享参数槽将所有调用方具体 float width 合并为 `float_any`，
   lowering 错走整数分支。完整的 `GetLocal 0; Math(predicate,1); ReturnValue` 转发体现在按每个
   直接调用点的具体 float32/float64 宽度展开，`is_nan/is_inf/is_finite` 不再污染调用者表示。
   不依赖函数名；未知宽度拒绝，暂不支持将这些多态转发体经普通间接 ABI 当函数值调用。
2. A 已输出黄金 B 后，B 保留了 selfhost 的泛型数组转发体（例如 `push`），不能把不同 []T 的
   调用全 union 到一个函数签名。按完整参数加载、操作、返回、captures/local metadata 和泛型
   descriptor=0 匹配八种转发体，静态调用使用已有 Array lowering；保持原 Mochi byte offset/width、
   CFG 和 safepoint 映射，**没有改写输入字节码**。泛型构造的 Storage 宽度延迟到元素约束确定。
   构造、push、capacity、reserve、clone、truncate、append_all 都有混合字节/整数真实执行回归。
3. B 后续出现 `format_finite_float32 @19` 的 float64/float32 冲突，已由全部三类谓词宽度隔离解决；
   随后未调用的泛型 `equal @45` 因 unknown 报错。加入与 Go 后端相同方向的保守函数可达性：
   main 起点、Call 和所有 Function/Closure 引用保留目标，递归/间接调用目标不遗漏；只有整个程序
   不可达的函数体不参与表示约束。保留 function ID/label，以 trap stub 占位，不伪造成功返回值。
   body-less NativeFS 的签名检查仍保留；所有可达未知表示仍 fail closed。

中间失败日志为 `bootstrap.log`、`bootstrap-v2.log`、`bootstrap-1g.log`、`bootstrap-B*.log`；
最终成功以 `bootstrap-A-final.log`、`bootstrap-B5.log`、`bootstrap-C.log` 为准。
`B-type-and-generic-probe.log` 记录原冲突两侧类型；临时诊断源码未留在产品树。

### 新回归和真实边界

`internal/aot/m17e_heap_test.go`：

- 空、1、127、16320、16321、16336、16337、16383、16384、16385、32768、65536、262144、
  2097152-byte 文件，每个返回值比较长度和**全部字节**；内容 `(i*37) mod 256`，包含 NUL 和所有
  byte 值。多次连续读取，保留 Storage 后分配垃圾，普通 GC 与逐次分配 GC stress 都运行。
  VM / Go Native / 纯 Mio Native 差分，纯 Mio 目标仅 8 MiB，文件超过旧 1 MiB 上界也成功。
- missing 文件为 NotFound；读取目录触发真实 read(2) 错误，返回 IOFailure，空数据；与正常 EOF
  分开比较错误类别和长度。
- 实际分配边界 `align16(n+48)+64`：Storage n=16320/16321/16336/16337/32768，配置 arena 恰好
  容纳时验证数据；少 16 bytes 则精确 exit 2。这里验证 header、padding、descriptor 和 logical end，
  不是仅以文件大小猜测 allocator 边界。
- 320 KiB 堆保留 3000 个 Box 的 24 KiB reference Storage，100 次跨页分配/回收后读取全部字段
  校验和；普通与 GCStress 都通过，生命周期和 pointer layout 与字节 Storage 分开覆盖。
- 104 KiB 堆中先创建 500 个临时数组，再用 live Marker 钉住尾部。只有合并内部死区才够分配
  90000-byte Storage，只有正确拆分 remainder 才能继续分配 10000-byte Storage；验证 Marker
  和两块数据内容，另单独执行 race。
- 混合宽度谓词覆盖 ±0、±Inf、NaN、有限值；特别将有限 float64 次正规数的原始 bits 设为
  `0x7f800000`，防止用同时猜测两种宽度的检查错误识别为 float32 +Inf。
- 八种通用数组转发操作，以及不可达泛型函数不污染可达函数、function ID 调用仍正确。

旧 `TestM17ePureMioReadAllBoundedHeap` 已从接受预期 OOM 改为要求真实输出 `1\n`；旧 Array
bounded OOM fixture 显式选 16 KiB 目标堆继续验证真实耗尽，不因默认堆变大而删除错误断言。
已有数组、nullable、checked unbox、整数伪 Heap 地址、Closure 和精确 GC 回归继续运行。

### 真实产物、阶段与协议

A 输入保持 377 functions、SHA256
`b5904e8fd51e96be16183826177504ed395be67143e0c78d7a4b5d304cfd00a1`。
Go 仅构建 Native Mio Builder 的 Seed，所有 A/B/C 镜像的 Decoder、lowering、ARM64 assembler 和
Mach-O Writer 都在 Mio 中执行；每个镜像都在 VM Builder 与 Native Mio Builder 之间逐字节比较，
随后解析 Mach-O、ad-hoc 签名并 strict verify，才启动该镜像执行编译。

| 阶段 | machine words / 未签名 bytes | 未签名 SHA256 | 实际运行 |
|---|---|---|---|
| Compiler A Native | 565061 / 2293760 | `e186e9d3074b836eeed7c80925153f81faa6dc97ba7c22cfd9eac81cb9d3820d` | exit 0，10.514 s，生成 B |
| Compiler B Native | 561435 / 2277376 | `5cbcd48cc904f7faafc209f172973754bf75cb2e6b568df9b34c1047d0c3aeb6` | exit 0，9.639 s，生成 C |
| Compiler C Native | 561435 / 2277376 | 同 B，原始镜像逐字节一致 | exit 0，10.057 s，生成 D |

另在 LLDB 下复跑最终 A，断在正常退出前 `0x100000864`：metadata x23=`0x300000000`，
logical end=`0x340000120`，bump=`0x33fffffe0`（余 320 bytes），**GC counter=1**，x24=0。
继续执行真实 exit 0，额外生成的 B 仍与黄金一致。这是目标 runtime 确实进行 GC 并复用空闲空间的
实际证据，不把未导出的计数写成零；详见 `lldb-final-gc.log`。

三次 stdout 均为 `bootstrap: ok\n120707\n477\n497\n1\n`，stderr 为空。
本次新生成 B/C/D Mochi **各 120707 bytes，逐字节相同**，SHA256 均为
`b44a55bf3f9020e6b0ff83ad137b25774814a32dfe8acaf070abcd167bfd1465`。
D 是本次生成的 Mochi；没有声称另外生成或执行 D Native。B/C 的原生执行已真实证实。
机器镜像签名后的 bytes/hash 受 codesign 影响，固定点比较使用 Mochi 与未签名镜像。

`internal/aot/m17e_probe_test.go` 默认仍为 A；新增 `MIO_M17E_BOOTSTRAP_STAGE=B|C` 以及显式
`MIO_M17E_BOOTSTRAP_INPUT`，B/C 输入须校验黄金 SHA。每次拒绝覆盖已有输出，并准确恢复原 config。
本轮没有调用会覆盖仓库已有黄金文件的旧脚本；按照其黄金哈希、真实原生连续运行、B=C=D 协议，
在独立证据目录实施纯 Mio 后端自举。最终证据：`build/m17e-heap-20261020/fixed-point.json`。

复现命令（证据目录必须不存在同名输出；B/C 阶段不能与其他 config 写入任务并行）：

```sh
E="$PWD/build/m17e-heap-20261020"
MIO_M17E_EVIDENCE_DIR="$E/bootstrap-A-final" MIO_M17E_BOOTSTRAP_RUN=1 \
  go test ./internal/aot -run '^TestM17eCompilerAProgressAfterClosures$' -count=1 -v -timeout 30m
MIO_M17E_EVIDENCE_DIR="$E/bootstrap-B5" MIO_M17E_BOOTSTRAP_RUN=1 MIO_M17E_BOOTSTRAP_STAGE=B \
  MIO_M17E_BOOTSTRAP_INPUT="$E/bootstrap-A-final/TestM17eCompilerAProgressAfterClosures/compiler-B.mochi" \
  go test ./internal/aot -run '^TestM17eCompilerAProgressAfterClosures$' -count=1 -v -timeout 30m
MIO_M17E_EVIDENCE_DIR="$E/bootstrap-C" MIO_M17E_BOOTSTRAP_RUN=1 MIO_M17E_BOOTSTRAP_STAGE=C \
  MIO_M17E_BOOTSTRAP_INPUT="$E/bootstrap-B5/TestM17eCompilerAProgressAfterClosures/compiler-C.mochi" \
  go test ./internal/aot -run '^TestM17eCompilerAProgressAfterClosures$' -count=1 -v -timeout 30m
cmp "$E/bootstrap-A-final/TestM17eCompilerAProgressAfterClosures/compiler-B.mochi" \
    "$E/bootstrap-B5/TestM17eCompilerAProgressAfterClosures/compiler-C.mochi"
cmp "$E/bootstrap-B5/TestM17eCompilerAProgressAfterClosures/compiler-C.mochi" \
    "$E/bootstrap-C/TestM17eCompilerAProgressAfterClosures/compiler-D.mochi"
cmp "$E/bootstrap-B5/TestM17eCompilerAProgressAfterClosures/compiler-B.unsigned" \
    "$E/bootstrap-C/TestM17eCompilerAProgressAfterClosures/compiler-C.unsigned"
```

实际 B5 启动时 A-final 尚未重建，输入取 `bootstrap-float/.../compiler-B.mochi`；随后 A-final
重新生成的 B 与它逐字节比较一致。该说明保留真实执行顺序，不把未运行命令伪称为原日志。

### 最终验证

以下命令都在本轮实际运行，最终全部通过；没有以前轮结果替代。

| 检查 | 本轮最终结果 / 日志 |
|---|---|
| Apple Silicon 三端差分、数组/nullable、checked unbox、精确 GC、Heap 新测试 | 通过；`final-special.log`，93.051 s |
| 更紧的 104 KiB 合并/拆分实机测试与 race | 通过；`coalesce-verified-final.log`，3.146 s |
| Native AOT 全量 | 通过，主包 247.139 s；`aot-final.log` |
| Native AOT race | 通过，无竞态报告，主包 852.229 s；`race-final.log` |
| 全仓 Go 测试 | 通过，AOT 246.235 s、selfhost/codegen 304.403 s；`go-final.log` |
| go vet | 通过；`vet-final-clean.log` |
| 纯 Mio 全包 / AOT 子包 | 74 / 18 项通过；`mio-verified-final.log`、`mio-aot-verified-final.log` |
| strict codesign | 本轮 111 个落盘原生镜像独立复验全部通过；`codesign.json`；测试内亦逐个签名验证 |
| 现有纯 Mio Decoder Golden | 通过；`decoder-golden.log`，4.055 s |
| 真实原生 A→B→C→D / 黄金哈希 / 固定点 | 通过；三个最终 bootstrap 日志及 `fixed-point.json` |
| git diff --check | 通过 |

```sh
MIO_M17E_EVIDENCE_DIR="$PWD/build/m17e-heap-20261020/final-special" \
  go test ./internal/aot \
  -run '^TestM17e(PureMio(ReadAll|Heap|GenericArray|FloatPredicate|FunctionReachability|Array|MutableCapture|Nullable)|CompilerAArrayTypeProvenance)' \
  -count=1 -v -timeout 30m
go test -race ./internal/aot -run '^TestM17ePureMioHeapCoalesceAndSplit$' -count=1 -v
go test ./internal/aot/... -count=1 -timeout 30m
go test -race ./internal/aot/... -count=1 -timeout 30m
go test ./... -count=1 -timeout 30m
go vet ./...
go run ./cmd/mio test ./...
go run ./cmd/mio test ./bunko/kiso/aot/...
go test ./internal/aot -run '^TestM17PureMioDecoderGoldenVMAndNative$' -count=1 -v -timeout 30m
git diff --check
```

最终全量重跑前曾发现两个测试设施问题：保守函数可达性裁掉旧 far fixture 未调用的 padding，
已增加无副作用静态调用保留它；诊断目录两个 `.go` 的多个 main 曾被全仓测试发现，已改存
`.go.txt`，没有留诊断包。修复后远错误分支真实镜像 1818624 bytes，专项 race 通过（172.004 s），
日志 `far-verified-final.log`。失败/中止的中间 full/race 日志不能充当最终通过；上表仅列最终重跑。

30m 全仓测试按既定规则跳过昂贵 VM `TestBootstrapSelfCompilation`，没有声称额外执行 VM 的
60/80 分钟连续自举。Decoder Golden 使用仓库已有黄金路径，但新 B/C/D 已与该黄金逐字节一致；
新产物自身的纯 Mio Decoder 在 B/C 的 VM 和 Native Builder 中都已真实运行。

本轮开始的工作树 diff/status 保存在 `/tmp/m17e-start.diff`、`/tmp/m17e-start.status`（另复制为证据目录的 `task-start.diff/status`）；保留全部
既有 ARM64、类型、Closure、数组改动和外部 CI 提交 `8b53a25`。没有 Commit、Push、Tag、Release、
git clean、hard reset 或启动 M18。当前无新的自举运行阻塞；本次要求的内存模型、原生连续自举及完整验证条件均已满足。任意多态函数值、Map 等未声明支持的语言面不因此自动变成已支持，也没有启动 M18。




## 2026-10-20 imm19 远条件分支修复与真实 Compiler A 执行（最新状态）

**M17e 仍未验收，M18 未启动。** 本节取代后文历史章节的当前状态。
本轮已经完整生成、签名并实际启动纯 Mio Native Compiler A；**A→B 未完成**，没有新的 B、
固定点或自举成功结论。新的运行期阻塞为目标有界 Heap 无法满足 `fs.read_all` 的首个 32 KiB
Storage 申请，详见下文。没有修改输入 Mochi、放宽类型、关闭 GC 或绕过错误分支。

### 原始失败的复现与根因

先保存原工作树 diff，再在改动前执行：

```sh
go test ./internal/aot -run '^TestM17eCompilerAProgressAfterClosures$' -count=1 -v -timeout 30m
```

VM 与 Native Seed 均复现 `10\n262496\n`。临时只读诊断（已移除）在失败 fixup 输出
kind、operand、源/目标 word、字节位移和邻近 label；两端结果完全相同：

| 项目 | 本次实际数据 |
|---|---|
| fixup kind / operand | `3 / 9`，即 **CBZ X9**，不是 B.eq |
| 源 / 目标 | word `262496` → `318`，目标 `runtime_error` |
| word 位移 | `-262178`；imm19 有效范围 `[-262144,262143]` |
| 字节位移 | `-1048712`，超过负向范围 136 bytes |
| 相对 text 的源 / 目标 PC | `0x100580` / `0x4f8` |
| 按 Writer image base / text offset 计算的 VA | `0x100100880` → `0x1000007f8` |
| 编码结果 | `EncodeError.BranchOutOfRange`，Instruction.word=0；未写回机器指令 |
| 当次总 words | `534770` |
| 来源 | `fn_225_34`，function 225 `selfhost/typecheck::compare_float32` 的 `OpLessEqual@34` |

VA 使用现有 base `0x100000000`、text file offset `768`；原镜像未生成，这一行是从真实 word
索引和 Writer 布局推算的 PC，并非声称曾执行原始未完成镜像。
`OpLessEqual` 的 nullable scalar 输入经 checked unbox，`emit_unbox_register` 的 nil guard 发射
`assembler_cbz_x(code, 9, "runtime_error")`。`lower` 的错误 detail 返回 `fix.at_word`；
`assembler_assemble` 独立计算 `target - fix.at_word`，因此 `262496` 不是位移，也不是 Mochi offset。
原始 label index 正确解析到 word 318，encoder 正确执行范围检查；根因为 **assembler 仅为每个
条件 fixup 预留一条指令，缺少远跳转布局协议**，不是 imm19 符号编码或 Mach-O relocation 错误。
证据：`build/m17e-branch-20261020/{baseline,diagnostic}.log`。

### 实现与布局协议

- `bunko/kiso/aot/arm64/arm64.mio`：发射 B.cond / CBZ / CBNZ fixup 时立即固定预留两个 words，
  此后所有 label、局部跳转、BL、函数地址 AddressPatch 都基于这一最终大小。
- 近分支保留原条件指令编码，第二个 word 为 ARM64 NOP (`0xd503201f`)。
  远分支使用 `inverse-condition +2; B target`；CBZ/CBNZ 互换，普通条件反转最低位。
  两条指令都不改 NZCV 或 GPR。AL/NV 对 B.cond 都是 always，远跳时直接用第一 word 的 B，
  不错误地把 AL/NV 当相反条件。
- B 从自己的 PC 编码，位移为 `target - (at_word + 1)`；仍调用原 `b` 编码器检查 imm26。
  AL/NV 的 B 则使用第一 word 的 PC。超出 imm26 明确拒绝，未引入间接跳转寄存器或静默截断。
- assemble 只回填 clone，不插入、不移动、不迭代布局，不改变原 words 或 labels；重复 assemble
  结果一致。Writer 按最终 word 下标处理 ADRP/ADD；无需调整任何已记录的 patch。
- `lower/lower.mio` 仅同步剩余超距诊断：条件 fixup 不再因原 imm19 距离本身被误报失败，
  检查真正剩余 B/BL 的 imm26 距离。此前类型、CFG、checked unbox、GC scanner 和布局未更改。
- 代价：每个条件 fixup 多占一个 word；本次 A 从 `534770` 增为 `564930` words（约 +5.64%）。
  这是确定性固定布局的显式空间开销，不是 branch relaxation 的临时膨胀。

### 新增回归与既有语义

`arm64_test.mio` 新增三项测试，覆盖 imm19 正负边界与两端越界、近分支保留、正反向长分支、
14 个普通条件反转、AL/NV、CBZ/CBNZ、NOP、label 不移动、可重复 assemble、imm26 边界及拒绝。
imm26 大距离单测用虚拟 label 避免分配 128 MiB；实际正反向 imm19 测试使用真实 padding。

`internal/aot/m17e_branch_test.go`：

1. 六组真实 >1 MiB 的 B.cond / CBZ / CBNZ × 前向/后向 Mach-O；VM 与 Native Seed Builder
   输出逐字节一致。每组实际运行 input 0 / 1，分别验证 taken / fallthrough 的独立退出码，
   并在目的地验证原 NZCV 与 X9 未被长跳序列破坏。ADRP/ADD/BR entry patch 也跨越最终布局，
   两种输入的镜像均签名和 strict verify。
2. 1,818,624-byte 真实 lowering 镜像，调用位于远端的除法函数；先输出 `42`，随后零除数
   走远 `runtime_error` 并 exit 1。VM 与 Go Native 参考亦验证成功前缀与错误；不会把错跳或
   提前退出误当成“encoder 没报错”。
3. `fs.read_all` 一字节文件的独立资源边界复现：VM / Go Native 输出 `1`，当前纯 Mio
   16 KiB 目标 exit 2。**该测试通过只表示已准确复现资源限制，不表示自举或行为差分通过。**

`lower_test.mio` 更新五个预留 NOP 带来的 word 下标，原 Struct metadata / bitmap 断言全部保留。
`m17e_array_test.go` 提取带测试资源参数的 helper，默认仍为 500M VM instructions / 128 MiB Seed；
大型 far-error fixture 单独使用 1.5B instructions / 512 MiB Seed，目标 Heap 没有改变。
`m17e_probe_test.go` 不再把预期编码失败当进度成功，而实际执行完整 lower + Writer、解析 Mach-O、
比较 VM/Native 原始产物并签名；显式环境开关运行 A→B，且精确恢复原 bootstrap.config 内容。

### Compiler A 与新的准确阻塞

输入仍为 377 functions、SHA256 `b5904e8fd51e96be16183826177504ed395be67143e0c78d7a4b5d304cfd00a1`。
本次完整纯 Mio A：

- `564930` words；未签名 Mach-O `2293760` bytes；ARM64 `__text` / `__const` / `__DATA` 结构解析成功。
- 未签名 SHA256：`0299e0199a517455ccafa3bf05b41e5c71884acc0cc2d1589400fe922d01b3c8`。
- VM Builder / Native Mio Builder 输出逐字节相同。Go 仅生成执行 Builder 的 Seed；A 的 lowering、
  encoder、Writer 都是 Mio，不是 Go AOT 目标替代品。strict codesign verify 通过。
- 实际 A→B 尝试：wall `0.263s`、exit `2`、stdout/stderr 均为空；没有生成目标 B。

复现完整 A 并单独尝试 B（不能与其他会写 bootstrap.config 的任务并行）：

```sh
MIO_M17E_EVIDENCE_DIR="$PWD/build/m17e-branch-20261020/bootstrap" MIO_M17E_BOOTSTRAP_RUN=1 \
  go test ./internal/aot -run '^TestM17eCompilerAProgressAfterClosures$' -count=1 -v -timeout 30m
```

测试拒绝覆盖已有 B；预期必须真实成功并符合仓库 B 黄金 SHA256 才通过，因此本次这条命令
**失败**，不会把 OOM 接受为自举成功。去掉 `MIO_M17E_BOOTSTRAP_RUN` 可单独验证完整 A 构建。
产物/Builder 源码与 execution.json 位于证据目录内对应测试名子目录。

LLDB 使用 `process launch --stop-at-entry` 后按 image address 设置断点，确认：

```text
runtime_alloc_fail PC = 0x100226550（本次完整 A 的 word 563348）
x19 = 0x8020 = 32800     // requested payload
x20 = 0x66 = 102         // byte Storage type
x21 = 1                 // 已做 GC 重试
x22 = 0x8030 = 32816     // 含对象头、对齐后的 allocation
x23 = 0x10022c010        // globals 后的 runtime metadata
[x23 + 0] = 0x10022c240  // current bump
[x23 + 8] = 0x100230000  // heap end
caller frames = 0x10000eac8, 0x1000003d4
```

剩余空间只有 `15808` bytes；即使没有已分配对象，总可用 managed 区也只有
`16384 - 2*8 - 288 = 16080` bytes，无法满足这一个申请。
源码链为 `selfhost/bootstrap/bootstrap.mio` 首次读 config → `kiso/fs::read_all` 的
`array.with_capacity[uint8](32768)`（`bunko/kiso/fs/fs.mio:129`）→ type-102 Storage allocator。
因此它甚至不依赖 config 文件内容或后续源码规模。最小复现命令：

```sh
go test ./internal/aot -run '^TestM17ePureMioReadAllBoundedHeap$' -count=1 -v
```

这与 branch 编码无关，后续需要为纯 Mio 目标单独实现并验证可配置的较大 Heap / Writer 映射与
GC 边界协议；只扩 Seed Heap 无效。本轮不通过缩小 fs buffer、修改输入、自行关闭/放宽 GC 或
绕过分配失败来掩盖这一阻塞。详细日志：`compiler-A-B.log`、`compiler-A-oom-lldb.log`。

### 本次验证结果

所有下列结果来自本次重新执行，日志在 `build/m17e-branch-20261020/`：

| 本次命令 / 检查 | 结果与证据 |
|---|---|
| 远分支与 Heap 边界专项（完整命令见下） | 通过；far-final.log。随后给 far-error 增加 VM/Go M6 参考断言后，单独复验通过，far-runtime-final.log（22.80 s） |
| Apple Silicon 专项差分（命令见下） | Apple Silicon VM / Go AOT / 纯 Mio Native 差分通过，49.151 s；含数组原始/boxed layout、CFG non-nil、Heap 地址整数、GCStress、Closure、nil 错误；differential.log |
| `go test ./internal/aot/... -count=1 -timeout 30m` | 最终全量通过，主包 199.308 s；aot-full-final.log |
| `go test ./... -count=1 -timeout 30m` | 最终全仓通过；AOT 198.722 s、selfhost/codegen 311.374 s；go-full-final.log |
| `go test -race ./internal/aot/... -count=1 -timeout 30m` | 最终全量通过，无 race 报告；主包 781.507 s；aot-race-final.log |
| `go vet ./...` | 通过；vet.log 为空 |
| `go run ./cmd/mio test ./...` | 74 项通过；mio-full.log。AOT 子包另单独复验，18 项通过 |
| `codesign --verify --strict` | 本次测试中签名验证均通过；另独立复验落盘的 21 个当前镜像（含完整 A），codesign.log |
| `git diff --check` | 通过 |
| Compiler A lower + Writer + VM/Native 字节一致性 + Mach-O + 签名 | 通过；compiler-A-final-build.log，23.80 s |
| 实际启动纯 Mio Compiler A 编译 B | **失败：exit 2 / OOM**；compiler-A-B.log / execution.json / LLDB；没有 B |
| 本次纯 Mio B/C/D 与固定点 | **尚未执行**，受上述 A→B 阻塞 |

专项测试与差分的完整命令：

```sh
go test ./internal/aot -run '^TestM17ePureMio(Far|ReadAllBounded)' -count=1 -v -timeout 30m
```

```sh
MIO_M17E_EVIDENCE_DIR="$PWD/build/m17e-branch-20261020/differential" \
  go test ./internal/aot \
  -run '^TestM17e(PureMioArray|PureMioMutableCapture|PureMioNullableFunctionRuntimeError|CompilerAArrayTypeProvenance)' \
  -count=1 -v -timeout 30m
```

开发中的首次全量检查曾因新增测试把纯 scalar fixture 送入默认 Go M4 参考入口而失败
（M4 无法建立该 fixture 的返回类型）；已改为现有 `CompileDarwinARM64WithGCStress` M6
参考入口并通过针对性复验，随后重新跑上述最终全量检查。首次 race 也因该旧 fixture 停止，
不能记作通过；结果以上表最终重跑为准。Go 参考入口的选择只影响测试，没有在 Go 增加任何
绕过纯 Mio assembler 的逻辑。

`go test ./... -timeout 30m` 沿用现有 deadline 规则跳过昂贵的 VM
`TestBootstrapSelfCompilation`；本次没有另跑 60/80 分钟 VM 连续自举，也没有把此前的 Go M16b
B=C=D 当成本次结果。`scripts/check-native-bootstrap.sh` 面向 Go Native AOT，且当前已有黄金
产物，其拒绝覆盖协议继续保留；本轮未运行该脚本来冒充纯 Mio 验收。

本轮未启动 M18；**M17e 未满足验收**：A 完整构建与可启动已证实，A→B 失败，B/C/D 及固定点
尚未执行。此前 Go M16b 原生固定点产物不算本次纯 Mio A→B 的结果。本任务没有执行 Commit、
Push、Tag 或 Release，没有清理/重置工作树；保留任务开始时已有的源码修改。
执行期间观察到外部 HEAD 更新为 `8b53a25`（原有 CI / TESTING / bootstrap 脚本改动）；
该提交不是本任务执行，也未回滚。初始工作树完整 diff/status 已保存；原 types 与 lower README
的 diff 经逐段比较仍与开始时完全相同。



## 2026-10-20 Array 类型来源与精确布局修复（当前状态）

**M17e 尚未完整验收，M18 未启动。** 本节取代后文 Closure 检查点的当前阻塞描述。
Compiler A 输入未改动，仍是 377 个函数、SHA256
`b5904e8fd51e96be16183826177504ed395be67143e0c78d7a4b5d304cfd00a1`。
没有生成完整 Compiler A 镜像，因此没有原生 A→B、Compiler B SHA256 或固定点通过结论。

### operand 与源码约束核对

`internal/code/code.go` 的 v14 定义为单个 u8 operand；
`internal/compiler/builtin.go` 的 `$array.with_capacity` 从泛型类型生成 descriptor；
`internal/mochi/validate.go` 与 Mio decoder 检验 descriptor。
VM 的 `arrayDescriptor` 和 Go AOT 的 `m12DynamicDescriptorKind` 均取低 7 位为元素 Kind、
bit 7 为元素 nullable。**容量来自栈上的 int64，而不是此 operand。** `1` 确实是
非 nullable int64；`129` 是 nullable int64；`0` 是需要后续约束求解的动态元素类型。
这项结论来自三端实现核对，不能把所有单字节 operand 一律当成元素类型。

`selfhost/parser/cursor.mio` 的实际声明是：

```mio
waza empty_children(capacity: int64) -> []int64 {
    kaesu array.with_capacity[int64](capacity);
}
```

固定输入中的 function 117 无 capture，1 个参数、1 个 Local Slot，返回一个值：

```text
0000 OpGetLocal 0
0003 OpArrayWithCapacity 1
0005 OpReturnValue
```

Mochi v14 函数元数据只存 parameter/local 数量、returns-value 标志、capture 和函数名，
没有序列化 `[]int64` 的完整签名。源码签名与 constructor descriptor 必须结合实际调用/返回图核验，
不能从 `returns_value=true` 推出元素的 nullable 表示。

### 首次 nullable 来源、CFG 路径与错误合并

新增 `analyze_types_traced(file, true)`；默认分析不收集来源事件。
事件为 `[instruction detail, Value node, peer node, reason]`，记录 nullable 传播、真实 nil alternative
和显式 nullable descriptor。`TestM17eCompilerAArrayTypeProvenance` 解码真实 A，输出函数元数据、
相关字节码、Go Storage 分类以及纯 Mio 的 Return / Value / Local Slot 与来源。

- 首次连入旧 children 元素分量的 nil 是 function 80 `kiso/convert::parse_negative`，
  `@166 OpReturnValue`，detail `81016627`，涉及 Value `443` 与 `8415`。
  `@82`、`@134` 返回 nil，成功路径返回 Local Slot 3。
  这是合法的 `int64?` 函数返回约束，本身不是原始字节码冲突。
- function 82 `kiso/convert::parse_int` 在 `@123 OpCall 80` 后写 Local Slot 5，
  `@132 OpIsNil`，非 nil 路径 `@147 OpReturnValue` 或 `@151 OpNeg` 后返回。
  旧分析使 nil-return、该数值局部 Slot、运算输入和结果共享一个表示分量。
- function 125 `selfhost/parser::parse_prefix` 的 `@40 OpCall 82` 写 Slot 3；
  Slot 4 在 `@49` 先写入普通 int64 默认值。`@52 GetLocal 3; @55 IsNil; @56 Not;
  @57 JumpIfFalse 66` 的 nil 路径直接到 `66`，非 nil 路径在 `@60 GetLocal 3; @63 SetLocal 4`
  赋值。**旧分析在这个已证明非 nil 的 GetLocal 仍复用 boxed Value，首次错误地污染了
  源码非 nullable 的 Slot 4。** 真实边 `Slot3 → 非 nil 值 → Slot4` 被错误当作表示完全相等。
- 此后 Slot 4 经 `@90 Call 106`、`@93 Call 119` 等签名/Struct 字段/数值约束继续传播。
  运算和比较又把输入双方、运算结果无条件 union，进一步扩大分量。
  `OpIsNil` 还错误地对被观察值调用 `type_mark_nullable`，即使原值为普通 0 也会要求装箱。
- 最后 constructor 的显式 descriptor `1` 与被污染的 inferred `129` 首次在布局检查处
  报 `118000358`。旧纯 Mio Return Value `621` → root `49685`，元素 Value `57944` →
  root `51715`，nullable=true；不是 constructor operand 解码错误。

修复后同一探针的 Return Value `621`、root `49685`，元素 Value `58410`、root `51715`，
nullable=false。parse_prefix 的 Slot 3 为 Value `664`、nullable=true；Slot 4 为 Value `665`、
nullable=false；`@63` entry 顶部 Value `11252`、nullable=false。
Value 编号是该次确定性分析的审计快照，不作为公共 ABI；测试断言实际类型/表示而非依赖编号。

### 修复协议与 Go 对照边界

1. `OpIsNil` 只产生 bool，不向输入添加 nullable 约束。
2. 非 nil 分支采用 CFG must-analysis：所有前驱事实取交集，循环求固定点；
   `SetLocal` / `DefineLocal` 清除目标事实，调用与 Upvalue 写入清除可能过期的 Cell 事实。
   只识别有指令边界证明的编译器 nil-test 形式，其余条件不猜测收窄。
3. scalar 非 nil GetLocal 使用独立 Value view；只有基类型约束相连，raw / boxed 表示不 union。
   从 boxed Local / Shared Cell 读取 raw view 时生成 checked unbox；若 Value 流又要求 boxed 表示，
   沿用既有 type-106 分配与精确临时 Root 协议。引用对象继续保留原 shape、variant、target union。
4. 数值运算、取负和比较建立基类型关系；运算结果单独求表示。
   真实 Slot、返回值、capture、Array 元素与 CFG 值流仍用严格 union。
   未知运算类型、未知数组元素以及显式/推断布局冲突继续 UnsafeType；没有覆盖 descriptor 或放宽检查。
5. 比较的 nil 分支只检查可能为 nil 的操作数；普通 scalar 0 不会因另一侧 nullable 而被当成 nil。

Go 的独立 M6 分析在此次输入上仍得到 Return Value `599` / root `72712`，元素 Value `72715` /
root `72827`，int64 nullable=true，Storage 元素宽 8、reference=true。
它把整个相关分量装箱，因此实际行为差分可以通过，**但它的过度合并布局不能作为纯 Mio 的正确性依据**。
本轮没有修改 Go AOT 求解器；Go 只作 Seed 与独立行为参考，不替代目标 lowering。

| 对象 / 表示 | 纯 Mio 修复后布局与扫描 |
|---|---|
| `[]int64`，descriptor 1 | type-101 Descriptor 扫描 Storage；type-102 的 data slot 宽 8、reference=0，不扫描 int64 bits |
| `[]int64?`，descriptor 129 | 相同 Descriptor ABI；Storage 宽 8、reference=1，扫描 nil / type-106 box 引用 |
| type-106 scalar box | `{kind=7, scalarBits}`，scalar payload 不扫描；Some(0) 与 nil 不同 |
| nullable Struct 数组 | Storage reference=1；对象继续使用原 Struct bitmap 逐字段追踪 |

Array Descriptor 的 Storage 指针、Storage 的元素 reference 位与 Struct / Enum bitmap 各司其职；
本轮未改动 100–107 对象 ABI、scanner、Closure Environment、Shared Cell 或 raw-slot 调用约定。
这里的 GC Pointer Bitmap 对应 Go/纯 Mio type-102 的统一元素扫描位（不是每个 int64 按地址猜测）。
同为 8 bytes 的 raw 与 boxed Storage 不因宽度相同而兼容；AppendAll/共享签名合并后仍须通过
constructor 的精确 GC 布局验证。

### Apple Silicon 差分与继续 Compiler A

`internal/aot/m17e_array_types_test.go` 新增真实源码和手工 v14 fixture：

- `[]int64` / `[]int64?` 容量 0、make 长度 0、nil / Some(0)、索引、普通 / nullable 写入、
  reserve、push 扩容、AppendAll、独立 Clone；数组经过函数返回、Closure 与间接调用后保持布局。
- 两种数组在 300 次额外分配及逐分配 GC 后仍保留值；nullable Struct 数组经 Clone、AppendAll、
  reserve 与 400 次分配后，原动态 UTF-8 字符串仍可读取。
- 将普通 int64 和 nullable box 的 scalarBits 都设成目标镜像 Heap 内部地址 `0x10001c148`。
  350 次额外 Struct 分配后再次取值；nullable 路径通过 checked unbox / 数值运算读取，
  不把 box 指针直接当整数输出。镜像二次解析确认 __DATA / Heap 地址未改变。
- descriptor 0 无约束的空数组、descriptor 1 与 129 经 AppendAll 混合均明确拒绝；
  Mio 单测另验证 nil 观察、非 nil / nil CFG 两条路径、汇合、赋值和调用后事实失效。

成功 fixture 比较 VM、Go AOT GCStress 与纯 Mio GCStress 的 stdout、stderr、exit；
两种 Native 镜像实际在 Darwin ARM64 执行并 strict codesign verify。
纯 Mio Builder 分别在 VM 与 Native Seed 运行，目标未签名镜像逐字节相同。
可重建证据目录：`build/m17e-array-types-20261020/`（按测试名分目录）。

修复后立即继续真实 Compiler A，越过原 array 阻塞。静态字符串 ID 原先每次查询都嵌套重扫
全部前缀，构建触及 VM 指令预算；现在一次建立去重 ID 表，指令、常量镜像、NativeFS 空字符串
共用同一 first-occurrence 顺序。函数地址 Patch 也使用现有 Assembler label index。

最新 VM 与 Native Mio Builder 探针一致返回：

```text
error_code = 10 (BranchOutOfRange)
detail = 262496 (machine word index, not Mochi byte offset)
conditional branch target = runtime_error
delta = -262178 words; imm19 valid range = [-262144, 262143]
```

诊断时共生成 `534770` words，约 2.14 MiB text；实际尚不能完成 assembler fixup。
新增 BranchOutOfRange 只细化原 EncodeFailure 的错误，仍由原编码器检查范围后拒绝，
没有截断位移、跳过指令或关闭安全检查。Native Seed 的引导堆由 128 MiB 调为 512 MiB：
前者返回 `M12 allocation failed`，后者与 VM 完全相同地抵达上述编码阻塞。
这只改变执行 Builder 的 Go Seed 资源，**目标纯 Mio Heap 仍为原有有界布局**。
下一项真实工作是保留所有 label / AddressPatch / PC 语义的条件长分支协议；本轮在此新阻塞停止，
不宣称已经完成 Compiler A 原生运行或 M17e 验收。

### 本轮工程检查

- `go test ./internal/aot/... -count=1 -timeout 30m`：通过，AOT 主包 138.515 s。
- `go vet ./...`：通过。
- `go run ./cmd/mio test ./bunko/kiso/aot/...`：15 项通过（arm64 3、lower 7、types 3、macho 2）。
- 新增数组证据集：通过，60.019 s；额外的来源断言复验通过，3.871 s。
- 18 个落盘纯 Mio 镜像再次执行 `codesign --verify --strict`：全部通过。
- Compiler A 输入 SHA256 再核验未改变；`git diff --check` 通过。
- `go test ./... -count=1 -timeout 30m`：全部通过；AOT 主包 137.638 s，selfhost codegen 286.420 s。
- `go test -race ./internal/aot/... -count=1 -timeout 30m`：全部通过；AOT 主包 407.488 s，未报告数据竞争。

重建本轮证据：

```sh
MIO_M17E_EVIDENCE_DIR="$PWD/build/m17e-array-types-20261020" go test ./internal/aot \
  -run 'TestM17e(PureMioArray|CompilerAArrayTypeProvenance|CompilerAProgressAfterClosures)' -count=1 -v
```

验证边界：源码 `chant` 不接受 nullable scalar；Heap 地址 fixture 的 nullable 路径先通过数值
操作生成 checked unbox，再与 VM / Go AOT 比较数值，不把 pointer 的十进制输出误当成数值成功。
没有以扩大的 Seed Heap 声称纯 Mio 目标 Heap 已扩容，也没有把 assembler 的拒绝当成镜像生成成功。
本轮未 Commit / Push / Tag / Release，保留已有用户改动。


## 2026-10-20 Closure / Shared Cell / CallValue 检查点（当前状态）

**M17e 未验收，M18 Stage 2/3 未开始。** 本节取代后文历史检查点的当前状态。
未生成完整 Compiler A 原生镜像，未执行原生 A→B，也没有 B/C/D 或新的固定点 SHA256。
没有关闭 GC、跳过指令、修改 Compiler A Mochi 或把 Go AOT 输出冒充纯 Mio 目标镜像。

Compiler A 输入仍为 377 个函数、SHA256
`b5904e8fd51e96be16183826177504ed395be67143e0c78d7a4b5d304cfd00a1`。
VM 与真实 ARM64 Native Mio Builder 的最新探针一致输出：

```text
error_code = 8 (UnsafeType)
detail = 118000358
function 117: selfhost/parser::empty_children
byte offset 3: OpArrayWithCapacity 1
```

此处显式 descriptor 为 `1`（非 nullable int64），完整约束图的元素表示则求得 nullable int64，
即 inferred descriptor `129`，需要 type-106 box 和引用 Storage。纯 Mio constructor 仍拒绝两种
Root/layout 分类的冲突。下一步需要迁移 Go M13 的 nullable 元素布局求解与相关 scalar unbox /
调用、索引安全点协议，不能仅改 reference bitmap 或忽略显式 descriptor。
本轮已经连续越过原 CallValue、Enum payload、Math is_inf / error、DataConvert、六类 FP 转换和
nullable Struct 字段检查；这不代表完整 Compiler A 数值/字符串/nullable 运行语义已经验收。

### 真实函数值来源与 v14 栈语义

`TestM17eCompilerAWriterClosureOrigin` 直接解码固定 Compiler A 并运行独立 Go M6 类型分析，锁定：

- function 28 `kiso/fs::writer` 创建 function 376，capture 为 `{Local:true, Index:0}`，来源是 File；审计另外断言 File 为精确引用 Struct，其唯一 handle 字段为不扫描的 Resource；
- `write_all@67` 的 entry stack 为 `[Function(target=376), []uint8, int64, int64]`，不是资源句柄或裸 code pointer；
- `OpCallValue` 的 u16 operand 是可见参数数目，消耗函数值及其后 N 个参数；此处返回一个 2-field WriteResult Struct；
- unit 目标不压入返回值。返回形状先按 M15b 的 branchless literal/local/global/factory hints、消费位置或
  同 arity 一致签名建立栈深，再由最终 target 集合逐一验证；不能解析/签名不一致时拒绝编译。

VM 的普通 Function 仅含函数身份，Closure 另持有共享 `*value.Cell` 列表；Native 表示沿用 M15b：

| type | payload | 扫描规则 |
|---:|---|---|
| 103 | 48 bytes `{4, functionID, parameters, returns, codeAddress, environment}` | 只追踪 environment；无捕获时为 0 |
| 104 | `{5, count, cells...}` | 逐 Cell 引用扫描 |
| 105 | 16 bytes `{6 \| reference<<8, value}` | 仅 reference 标志为真时扫描 value；string 值走 tagged-union 规则 |

函数、签名参数/返回、capture、Struct/Array 和 tag-sensitive Enum payload 的约束参与同一个固定点。
`DefineLocal` 清除当前 Cell 绑定，`SetLocal` 更新已有 Cell，`GetLocal` 从 Cell 读取最新值；
`SetUpvalue` 修改同一 Cell，嵌套 capture 转发原 Cell，不能复制 value 取代共享绑定。
Cell 区独立于已有 local/operand 区且先清零。当前最多 256 locals；为 constructor 临时值留边界，
函数最多 252 个 entry operand、main 最多 508 个，超出明确返回 FrameTooLarge。

### ABI 与精确 Root 生命周期

仍使用原 raw-slot Native ABI：x0–x7 为前 8 个可见参数，第 9 个从 caller `sp+0` 起。
x25 是隐藏 Environment，间接 caller 在 stack arguments 后额外保留 16-byte slot 保存原 x25；
其位置不会挤占第 9 个参数。函数保存/恢复 x25/x26，main 的最后表达式状态随调用保留。
函数地址由 Mach-O text AddressPatch 解析；没有把函数 ID 当地址或改用 Go trampoline。
静态/间接调用均遵守 1024 的既有调用深度上限；array constructor 使用 x22 暂存 capacity 时单独保护调用深度。

每个间接调用的 Shadow Frame 包含当前 globals、locals、完整 entry operand 中的精确引用，
因此被调用的 type-103 对象、引用参数及需要跨调用保留的 operand 都仍可达；captured 函数还发布 x25，
并发布所有已规划 Cell slot（尚未创建者为 0）。创建后续 Cell / Environment 时会重新物化已创建 Cell；
Environment 在 type-103 分配前单独放入一个精确临时 Root frame。没有在 allocator 后继续使用未保护的临时环境。

| 值类别 | Root 分类 |
|---|---|
| 普通 integer / float32 / float64 / codeAddress / functionID | 不作为 Root |
| Tagged static string `(id<<1)\|1` | string tagged-union slot；mark 明确跳过 odd ID |
| Dynamic string、Array、Struct、Enum、nullable scalar box | 对应精确 Heap 引用 |
| Native Function Value | type-103 Heap 引用；codeAddress 不扫描 |
| Closure Environment / Cell | 分别按 type-104 / type-105 扫描 |
| unknown | 不可发布；必要安全点返回 UnsafeType |

nullable scalar producer 装箱时，新 raw bits 留在独立非 Root slot，只发布消费后仍存活的 operand prefix；
修复了按旧 entry 类型把刚产生的 raw scalar 当 boxed Root 发布的窗口。Heap 仍为既有 16 KiB data page
扣除 globals/metadata 后的有界区域，保留逐分配 GCStress、正常 GC/retry、mark/sweep 和 free list。
未导出 GC 次数/耗时/累计分配量，不能据此填写零或宣称 Compiler A 的资源需求已经满足。

### Apple Silicon 验证覆盖矩阵

新增 `internal/aot/m17e_closure_test.go`。成功 fixture 比较 VM stdout、Go AOT GCStress 与纯 Mio AOT
的独立 stdout/stderr 和 exit 0；两种 Native 镜像均经过 ad-hoc codesign 和 strict verify，实际在 Darwin ARM64 执行。
纯 Mio Builder 分别在 VM / Go 引导的 Native Seed 执行，输出未签名镜像逐字节一致。
Go 只承担一次性 Seed 与独立参考程序；目标 lowering / encoder / writer 均由 Mio 执行。

| 能力 | 真实执行结果 |
|---|---|
| 无捕获 Function / 间接调用 | 通过 |
| 可变捕获 / 多复合捕获 | 通过，integer、string、Array、Struct |
| 两 Closure 共享 Cell 与别名修改 | 通过，交替 increment/read；原函数 SetLocal 与 Closure SetUpvalue 双向可见 |
| 嵌套 capture 转发 / Closure 返回 Closure | 通过，返回后重复调用 |
| Closure 作为函数参数 | 通过 |
| 递归 / 互递归 Closure | 通过，64/80 层测试 |
| 16 个可见参数的间接 Closure 调用 | 通过，另有 Closure→16 参数静态函数测试 |
| DefineLocal 循环重绑定 | 通过，三个独立 Cell 保留各次循环值 |
| 间接调用中的活跃 operand Root | 通过，Struct 留在函数值下方，callee 逐分配 GC 后仍读回 42 |
| 返回 Closure 跨 GC、共享引用替换 | 通过，返回后两轮各 400 次垃圾 Struct 分配；调用内替换引用并分配，读回更新值 |
| Float / tagged / dynamic string 捕获 | 通过，3.25 不扫描，静态和动态 CJK string 存活 |
| 普通 allocator / GC retry | 通过，GCStress 关闭时保留同一个旧 Closure，额外创建并覆盖 400 个 Closure，超过有界 Heap 后仍可调用 |
| Closure 回收与 free-list 复用 | 通过，12,000 次创建/丢弃后最后 Closure 仍返回 42 |
| 非函数值 / 无目标 / 参数数目错误 | 编译期 UnsafeType；独立 Go AOT 也拒绝 |
| nullable Function 实际为 nil | 两 Native 均 exit 1，stdout/stderr 空；VM API 返回带位置的 `requires function value` 错误 |
| `kiso/io::write_all` 真实 Writer/File | 通过，7-byte NUL/0xff/CJK 内容逐字节校验，成功与非法范围错误分支 |
| Enum 不同 tag 的 int/string payload | 通过，按编译器 awase 结构收窄，读取时另验证实际 tag，不合并不同变体 |
| DataConvert 与 UTF-8 | 通过，复制独立性路径、CJK/emoji、空输入、surrogate/overlong/truncated 返回 nil |
| FP 转换及基本运算/比较 | 通过定向 fixture，包括 f32↔f64、float→int、负值、±0、NaN 比较；不是完整数值库验收 |

错误输出边界并非三端完全同文：VM API 提供详细错误对象，现有 Native checked-error ABI 只以 exit 1
停止且 stderr 空。本轮保留这一 Go AOT 约定，没有伪造 VM 错误文本一致的结论。

真实 IO fixture 还发现并修复 `OpArray` 形式的源码 `[]uint8` literal 被错误当 word storage 的问题：
类型图现在保留整数 descriptor、NativeFS byte 参数边与未显式指定数组的求解宽度。
修复前文件为 `[77,0,0,0,0,0,0]`，修复后为 `[77,0,255,228,184,173,10]`。
ARM64 Assembler 的 label index 改为开放寻址表；collision/growth 和 fixup 单测通过。
此优化解决 lowering 插入标签时的线性查找预算耗尽，不代表已完成所有 Compiler A 函数。

### 本轮回归与证据

- `go test ./internal/aot/... -count=1`：通过，AOT 主包 128.294 s。
- `go test ./... -count=1 -timeout 30m`：通过，含 AOT / selfhost / LSP / 编辑器关联包；AOT 主包 127.404 s。
- `go vet ./internal/aot/...`：通过。
- `git diff --check`：通过。
- `go test -race ./internal/aot/... -count=1`：最终轮通过，AOT 主包 297.241 s，未报告 race。
- Mio 包测试：arm64 3 项、lower 7 项、lower/types 1 项、macho 2 项通过。
- 最新 Compiler A VM / Native Builder 探针、真实 Writer 来源审计及 23 项新增验证通过。
  补充的 IO fixture 会在 VM、Go AOT 和纯 Mio AOT 各次运行后分别核验真实文件内容。

可重建 Closure 验证证据：

```sh
MIO_M17E_EVIDENCE_DIR="$PWD/build/m17e-closure-20261020" go test ./internal/aot \
  -run 'TestM17ePureMio(MutableCapture|UncapturedFunctionValue|MultipleCompositeCaptures|SharedCell|NestedCapture|Closure|RecursiveClosure|CallValueRejects|DefineLocal|UnitClosure|MutuallyRecursive|UnreachableClosures|IOWriteAll|DataConversion|FloatConversions|EnumVariantPayloads|NullableFunctionRuntimeError|LocalAndUpvalueAliasUpdates|IndirectCallPreservesOperandRoot)' \
  -count=1 -v
```

使用通用差分 helper 的成功 fixture 目录保存 input.mochi、可重建 builder.mio、签名前/后镜像、build.json 以及两个 Native
stdout/stderr/exit JSON；普通 allocator / 错误 fixture 保存的证据范围以其文件为准，不冒称导出完整 VM CLI 诊断。
固定点目标仍为 `b44a55bf3f9020e6b0ff83ad137b25774814a32dfe8acaf070abcd167bfd1465`，尚无原生 A→B 验证结果。


## 历史检查点（以下当前状态描述已被上文取代）

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
