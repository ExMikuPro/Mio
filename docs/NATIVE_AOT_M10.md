# Mochi Native AOT M10 — 原生 Precise Mark-Sweep GC 原型

M9 的 `gc.mark` 已在 Apple M4 Pro 实机验证，本阶段在此基础上添加 `gc.validate`、`gc.sweep`、`gc.alloc` 和 `gc.collect`，实现：

**Allocate → Mark → Sweep → Reuse**

这些函数是可拼接到 Mach-O 的 ARM64 原生运行时片段，不使用 libc、Go Runtime、Mochi VM 或 LLVM。M10 仍然是独立的原生内存管理自检：**普通 Mio 源码不会自动触发 GC，也尚未拥有由 AOT 自动生成的精确 Stack Map。**

## ABI 与数据布局

继续使用 M8 的 64 字节 Heap 头和 16 字节对象头，**不修改 Mochi v14 格式和已有 Opcode**：

```text
Heap Header (64 B)
  +0  magic
  +8  bump cursor
  +16 end
  +24 requested capacity
  +32 mmap byte length
  +40 allocated live object count
  +48 origin
  +56 free-list head index （M8 预留字段，初始为 0）

Object Header (16 B)
  +0  TypeID uint32   （0 = Free Block）
  +4  Flags uint32
  +8  Payload Size uint64
  +16 Payload address （16 B 对齐）
```

M10 的 Free List 用 **1-based 16 字节单位相对偏移**表示节点，不保存进程绝对地址，也不需要新增字段。

- `Heap[56] == 0` 表示空链表，否则表示 `origin + (index-1)*16` 处的空闲块头。
- 空闲块 `TypeID == 0`；`Flags & 7 == 4`；`Flags >> 3` 为下一个节点的 index。
- 分配对象的 `Flags == 0`；M9 标记时允许 `1`（待扫描）、`3`（已扫描）。
- 空闲块大小仍按 `align16(16+payloadSize)` 计算，因此连 0 字节对象也能够加入 Free List。
- M10 `heap[40]` 统计当前已分配对象数，Sweep 回收时减小，再分配时增加；历史 M8/M9 不触发回收时行为不变。

## 原生接口

```text
heap.init(x0=capacity)                            -> x0=Heap* 或 0

gc.alloc(x0=Heap*,x1=typeID,x2=payloadSize)      -> x0=payload* 或 0

gc.validate(x0=Heap*,x1=types,x2=typeCount)     -> x0=0 或 UINT64_MAX

gc.mark(x0=Heap*,x1=types,x2=typeCount,
        x3=frameBase,x4=rootMap,x5=globalsBase) -> x0=可达数或 UINT64_MAX

gc.sweep(x0=Heap*,x1=types,x2=typeCount)        -> x0=回收数或 UINT64_MAX

gc.collect(x0=Heap*,x1=types,x2=typeCount,
           x3=frameBase,x4=rootMap,x5=globalsBase)
                                                   -> x0=回收数或 UINT64_MAX

heap.destroy(x0=Heap*)                          -> x0=1 成功、0 失败
```

`M10RuntimeWords()` 返回全部可搬移机器码和这七个入口相对代码片段开头的指令索引（注意：`gc.validate` 也有独立入口）。调用者通过 `BL` 的 PC 相对偏移链接目标。x19..x29 以及 x18 不被 GC 例程修改，栈指针维持 16 字节对齐。出现损坏时通过错误返回，不将半完成的 Sweep 假装为成功。

`gc.alloc` 先按空闲链表顺序 First-Fit，必要时从前端拆分较大块；耗尽 Free List 后尾调用 M8 Bump Allocator。复用时清零整个对齐的载荷区域，防止旧引用残留。`gc.sweep` 经过全堆只读校验后再回收未标记对象，合并相邻空闲区域并重建链表。

**重要调用约束：** GC 是单线程 Stop-the-World 原型。只有在持有准确 Root Map、活跃对象引用已写回受保护槽位、没有其他线程修改对象图的 Safepoint 才能调用 `gc.collect`。手动调用 `gc.sweep` 必须紧接一次成功的 `gc.mark`。目前还没有自动 Safepoint、Write Barrier、运行时类型注册、全程序 GC Root 发现或动态扩容。

本阶段限制：Heap 请求容量不超过 1 MiB、类型表不超过 8 个类型、每个类型最多 4 个引用字段。M9 的原型 Mark 使用固定点迭代，因此时间最坏为 O(n²)，M10 暂不追求吞吐和延迟优化。

## M10 原生自检

`examples/aot_m10/make_gc_probe.go` 接收 `-mode`、`-o`，生成不依赖外部动态库的 ARM64 Mach-O；在 Apple Silicon 上要先用 `codesign --force --sign -` 临时签名。

| 模式 | 验收内容 |
| --- | --- |
| `smoke` | A→B 可达、C 不可达；回收 C、零初始化复用；第二轮 GC |
| `cycles` | 活着的 A↔B 保留、不可达的 C↔D 回收；整数槽位的伪指针被忽略 |
| `split` | 80 字节对象回收后，拆分给多个较小对象 |
| `coalesce` | 相邻四个空闲块合并后可满足更大的分配 |
| `reset` | 两轮 GC 的 Root 变化和标记位重置 |
| `badtype` | 未知 Type ID 校验失败，Heap 未进入 Sweep |
| `badsize` | 损坏的载荷长度被安全拒绝，不发生越界访问 |
| `badlink` | 超出 Heap 范围的空闲表索引被拒绝，不改写分配状态 |
| `oom` | Bump 空间耗尽后，回收一块并复用 |
| `interior` | 内部指针与仅存于整数槽的伪指针不能保活对象 |
| `holes` | 空闲链表包含多个离散节点，并可跳过不够大的节点 |
| `zerobyte` | 零载荷对象也可回收和复用 |
| `stale` | 相邻空闲块合并后，旧的 payload 地址不会错误保活新对象 |
| `stress` | 连续 64 次分配、回收、重复利用，原有可达引用保持完整 |

每组程序真正执行 `mmap → gc.alloc → gc.collect → gc.alloc → munmap`，在 ARM64 上检查对象头、地址与计数，并输出 `M10 <mode> PASS`。自检结果由原生指令判断，不在 Go 中提前预计算。

## 本机测试

在 Mio 仓库根目录，将 ZIP 解压覆盖后执行以下命令（无需创建 `.sh`）：

```bash
# 单元测试
go test ./internal/aot/... -count=1

# 生成一组原生探针
go run ./examples/aot_m10/make_gc_probe.go -mode smoke -o build/m10-smoke
codesign --force --sign - build/m10-smoke
codesign --verify --verbose=2 build/m10-smoke
./build/m10-smoke
```

预期显示 `M10 smoke PASS`。全部 14 组测试可使用附带的一键复制终端指令运行。

## 当前未实现的内容

- 普通 `mio build → mochi-aot` 并不会将数组、Map、对象等字节码转换成 M10 GC 对象；M10 自检是另一条独立路径。
- 尚无自动生成的对象指针 Stack Map、寄存器活跃引用溢出、全局字段布局/类型注册或跨模块根追踪。
- 不能允许跨 Safepoint 的裸对象指针不出现在 Root Map；否则 GC 可能回收仍会被程序使用的对象。
- 非移动 GC 未来仍要处理大对象、碎片、跨页面分配、并发暂停，以及错误的/悬空的引用。
- 目前不做后台 GC、Generational GC、对象 Finalizer 或 Weak Reference。

**下一阶段应该先把自动生成的 Stack Map 与 AOT 类型系统接通，然后才能让普通 Mio 程序安全地使用 GC；切勿直接打开自动回收。**
