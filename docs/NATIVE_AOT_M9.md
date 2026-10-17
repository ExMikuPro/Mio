# Mochi Native AOT M9：精确 Root Map 与原生对象可达性标记

M8 的原生 `mmap` / Bump Allocator / `munmap` 已在 Apple M4 Pro 实机通过。
M9 在**不更改 M8 对象 ABI** 的前提下，让 ARM64 程序根据显式类型元数据和 Root Map 遍历对象引用，并在对象头保存标记状态。

**本阶段不是完整 GC，也没有给 Mio 增加新的 Mochi Opcode。** M9 只实现能够复用的原生标记例程与独立的 Mach-O 自检程序；不能回收单个对象，更没有自动生成所有 Mio 函数的 Stack Map。

## 1. 堆和对象布局

继续沿用 M8：

```text
Heap* + 0     uint64 magic = "MIOHEAP8"
Heap* + 8     uint64 cursor
Heap* + 16    uint64 end
Heap* + 24    uint64 capacity
Heap* + 32    uint64 mmapLength
Heap* + 40    uint64 allocationCount
Heap* + 48    uint64 origin

ObjectHeader + 0  uint32 typeID
ObjectHeader + 4  uint32 flags       // M9 bit0=marked，bit1=scanned
ObjectHeader + 8  uint64 payloadSize
ObjectHeader +16  payload[]        // 用户的 ObjectRef 指向这里
```

M9 不移动、压缩或回收对象。每次标记先清除上一轮 flags，然后从 Root Map 出发追踪可达对象，避免旧 Root 消失后标记残留。

## 2. 只读 Type Layout 与 Root Map

通过 `M9EncodeMetadata` 在编译期验证并序列化元数据，放入 Mach-O `__TEXT,__const`：

- 每个 Type Layout 为 **32 字节**：`typeID(u32)`、`payloadBytes(u32)`、`refCount(u32)`、保留字段、最多 4 个 8 字节对齐的引用字段偏移（`u32`）。
- 一个 Root Map 为 **80 字节**：`frameBytes(u32)`、`frameRefCount(u32)`、`globalSlots(u32)`、`globalRefCount(u32)`、最多 8 个 frame 字节偏移及最多 8 个 global 槽位索引。
- 类型 ID 必须唯一；引用偏移必须对齐、位于载荷内，不能重复；Root Map 的偏移也必须落在对应帧与全局槽位内。

例如模拟 Safepoint 的帧如下：

```text
frameBase + 0   ObjectRef(A)     <- Root Map 登记
frameBase + 8   int64(G.address)  <- 普通整数，不登记
frameBase +16   ObjectRef(D)     <- Root Map 登记

__DATA global[0]   ObjectRef(H)     <- Root Map 登记
__DATA global[1]   int64(F.address)  <- 普通整数，不登记
```

**精确**的意义是只有登记在 Map 中的根，以及 Type Layout 中标明的引用字段会被扫描。即使普通整数内容等于真实对象地址，也不得因为“看起来像指针”就保留目标对象。内嵌偏移地址（Interior Pointer）不会被接受为新的 ObjectRef。

目前仅提供**一个明确的 Safepoint Map 实例**供原生探针使用。真正接入 Mio 编译器前，还需为各函数返回点、调用点和可能触发分配的 Safepoint 构造 PC → Root Map 索引，并处理寄存器中的活跃引用值（可以预先溢出到已描述的帧槽位）。

## 3. 原生标记例程

入口 `M9MarkRuntimeWords()` 返回可搬移的 ARM64 指令数组；
`appendM9Mark` 生成主例程 `gc.mark` 与 `gc.trymark`：

```text
x0 = Heap*
x1 = TypeLayoutTable*
x2 = typeCount
x3 = frameBase
x4 = RootMap*
x5 = globalsBase

return x0 = reachableObjectCount
return x0 = UINT64_MAX  // 类型缺失、对象布局损坏或非法 Heap
```

- 每次重置 mark/scanned 位，读取显式的 frame/global Root Map。
- 先在真实已分配对象列表中匹配“载荷首地址”，匹配成功后才写 flags；不会直接解引用未知地址。
- 根据 `typeID` 查找元数据，仅扫描 `refOffsets` 指定的字段，支持有环对象图。
- 同一个对象仅累计一次；所有可达对象最终 flags 为 `0b11`，不可达对象保持 `0`。
- 不使用 `x18`，保存并恢复 `x19..x28` 与 `x30`，SP 入口与出口保持 16 字节对齐。
- 不调用系统库、Go Runtime、Mochi VM；只使用 M8 的 Heap 结构。

当前标记采用 **固定点重扫描**：内存 O(1)，最坏耗时约 O(n²)，方便优先验证根与引用元数据的正确性。M10 应改成有界的显式 Mark Worklist，并引入统一的失败处理和对象生存期管理。

## 4. 实机验证（Apple M4 Pro / macOS ARM64）

源代码 ZIP 仅包含 `.go` 和 `.md` 文件，不包含 `.sh`。
把增量 ZIP 解压到 Mio 仓库根目录，在仓库根目录执行：

```bash
set -euo pipefail

go test ./internal/aot/... -count=1
mkdir -p build

for mode in roots cycles reset interior badtype fanout; do
  go run ./examples/aot_m9/make_gc_probe.go -mode "$mode" -o "build/m9-$mode"
  codesign --force --sign - "build/m9-$mode"
  codesign --verify --verbose=2 "build/m9-$mode"
  actual="$(./build/m9-$mode)"
  printf '%s\n' "$actual"
  test "$actual" = "M9 $mode PASS"
done
```

六组结果应为：

```text
M9 roots PASS
M9 cycles PASS
M9 reset PASS
M9 interior PASS
M9 badtype PASS
M9 fanout PASS
```

注意：上述 `PASS` 来自原生程序**读取并检查每个对象标记位**、验证分配数量与载荷后才打印，并非编译期写死的数学答案。

| 模式 | 真正验证的内容 |
| --- | --- |
| `roots` | Frame 与 __DATA 根、循环引用、普通整数伪指针不参与追踪，6/8 对象可达 |
| `cycles` | 只保留 B/C 组成的环，2/8 对象可达 |
| `reset` | 第一次 6/8 可达，根变更后重新标记为 F/G 两个对象，其余 flags 清零 |
| `interior` | 只有 A+8 内部地址，不能误认为合法的对象引用，0/8 可达 |
| `badtype` | 被 Root 引用的对象 Type ID 不存在时，返回明确失败哨兵 |
| `fanout` | 四个引用字段都被追踪，最终 8/8 对象可达 |

同时可执行 `go test -race ./internal/aot/...`、`go vet ./internal/aot/...` 验证 Go 侧生成器。
这些 Go/Linux 结果与指令仿真**不能代替 Apple M4 Pro 实机测试**。

## 5. 边界与后续

M9 明确暂不支持：

- 不新增 Mochi Opcode；普通 Mio 代码仍不会自动创建 GC 对象或 Root Map。
- 暂不支持动态类型大小；每个静态类型限 4 个引用字段，最多 8 个类型描述符。
- 每个 Probe 的单个 Safepoint Map 最多 8 个 Frame Roots 与 8 个 Global Roots。
- M8 Heap 最大 1 MiB，所有对象仍由 Bump Allocator 分配；不会逐对象回收或复用。
- 暂不扫描寄存器中的活跃指针；未来 Safepoint 前须将其 spill 到已登记的根槽位。
- 不实现对象终结器、写屏障、并发、多线程、跨线程根、移动压缩 GC。

M10 的合理路线是先把 Type Layout 与 Root Map 接入 Native AOT 的真实对象模型，建立分配器与标记器共享的元数据 ABI，再实现**精确、非移动、单线程 Stop-the-World Mark-Sweep**，最后才逐步优化为分代或并发 GC。
