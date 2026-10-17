# Mochi Native AOT M8：macOS ARM64 原生堆分配器

M0–M7 在 Apple M4 Pro 上已完成实机验收。M8 首次增加**真正由目标进程在运行时向 macOS 申请的可写内存**，而不是使用原来 Mach-O 的静态全局槽位。

当前 M8 只完成独立原生 Allocator 及对象布局的验证，**尚未将堆对象接入 Mio/Mochi 普通字节码**。没有增加任何 Mochi Opcode，也未添加 GC。

## 原生 API

M8 在 `internal/aot/darwinarm64/heap.go` 中用原始 ARM64 指令编码三个函数。`M8AllocatorRuntimeWords()` 会导出可直接拼接的机器码片段和三个入口指令偏移，便于后续与普通 Mochi 程序链接。调用使用私有的当前 AOT 约定：

| 入口 | 参数 | 返回 |
| --- | --- | --- |
| `heap.init` | `x0=capacity`，按字节计的可用容量 | `x0=Heap*`，失败返回 0 |
| `heap.alloc` | `x0=Heap*`, `x1=typeID`, `x2=payloadSize` | `x0=payload*`，失败返回 0 |
| `heap.destroy` | `x0=Heap*` | `x0=1` 成功、`0` 失败 |

- `heap.init` 使用 Darwin `SYS_mmap=197`，参数为 `mmap(NULL, capacity+64, PROT_READ|PROT_WRITE, MAP_PRIVATE|MAP_ANON, -1, 0)`。
- `heap.destroy` 使用 Darwin `SYS_munmap=73`；不能在释放后继续使用旧指针，也不能二次释放。
- `capacity` 必须在 `1..1048576` 范围内，`typeID` 必须在 `1..65535` 范围内。
- `heap.alloc` 采用 Bump Allocator，单个对象不释放；容量满时返回 0，不能意外写出 `Heap` 边界。
- `Heap*` 参数必须是本模块成功创建、且尚未销毁的地址；不对任意伪造指针进行安全解引用。
- 返回的 payload 地址保证 **16 字节对齐**；0 字节 payload 仍需要完整对象头。
- 分配失败必须保留 `cursor` 和 `allocationCount`，不能留下半初始化对象。
- x9..x15 可被调用方视为临时寄存器；不修改 x19..x29；原生栈始终 16 字节对齐。

这只是研究性质的 Darwin 专用后端。直接 BSD syscall 不具备跨系统的稳定兼容承诺；将来可把底层页分配替换为独立的平台适配层。

## Heap 布局

Heap 起始地址为 mmap 返回的内存基址：

| 字节偏移 | 内容 | 类型 |
| ---: | --- | --- |
| 0 | Magic：`MIOHEAP8` | uint64 原始字节 |
| 8 | cursor | uint64 |
| 16 | end | uint64 |
| 24 | requested capacity | uint64 |
| 32 | mmap 长度 | uint64 |
| 40 | allocationCount | uint64 |
| 48 | payload region start | uint64 |
| 56 | reserved | uint64 |

对象占用 `align16(16 + payloadSize)` 字节，其前 16 字节固定：

| 相对 payload 地址 | 内容 | 类型 |
| ---: | --- | --- |
| -16 | typeID | uint32 |
| -12 | flags（M8 为 0） | uint32 |
| -8 | payloadSize | uint64 |
| 0 | 对象载荷起始 | `uint8*` |

未来精确 GC 可使用 `typeID` 查类型描述符，但 M8 尚未实现对象引用扫描、Root Map、GC 标记或对象释放。

## Apple Silicon 实机验收

先把 M8 增量 ZIP 解压至 Mio 仓库根目录。**无需 .sh 文件**。在仓库根目录执行：

```bash
set -e

go test ./internal/aot/... -count=1
mkdir -p build

for mode in smoke limits metadata; do
  go run ./examples/aot_m8/make_allocator_probe.go -mode "$mode" -o "build/m8-$mode"
  codesign --force --sign - "build/m8-$mode"
  codesign --verify --verbose=2 "build/m8-$mode"
  "./build/m8-$mode"
done
```

预期三个原生程序分别打印：

```text
M8 smoke PASS
M8 limits PASS
M8 metadata PASS
```

其中 `smoke` 在原生地址上创建不同大小的对象、写入载荷标记并检查覆盖；`limits` 检查零容量、非法类型、容量耗尽和失败后状态；`metadata` 运行一个原生循环，分配 32 个对象并逐次检查对象头与地址间距。原生程序发生任何断言失败均以状态码 1 退出。

**注意：** 这些是独立的原生 Allocator 自检程序，不是 Mio 源码的 VM/AOT 差分测试。M8 不应被误报成已经支持动态 Mio 字符串、数组或普通对象。

## 下一阶段

M9 应先设计 `typeID` 到精确指针布局的映射、GC Root/Stack Map、可达对象遍历与安全点。只有验证所有引用可准确定位之后，再进入 M10 的精确 Mark-Sweep GC。

M8 不实现逐对象 `free`，避免在未确定精确 GC Root 和对象生命周期模型前引入错误的内存释放接口。
