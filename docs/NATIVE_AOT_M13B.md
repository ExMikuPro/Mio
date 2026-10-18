# Mio Native AOT M13b：原生 Enum

M13b 在不修改 Mochi v14、Opcode 编号和 M8 对象头的前提下，为 macOS
ARM64 AOT 增加 `OpEnum`、`OpEnumTag` 与 `OpGetEnumValue`。

## VM 语义审计

- `OpEnum(tag, count)` 的两个操作数均为 16 位无符号数。它取操作数栈顶
  `count` 个值，保持原顺序浅复制为关联值，并压回一个 Enum。
- tag 是枚举声明中变体的零基序号。Mochi 文件不保存枚举定义表，因此 VM
  对手工字节码中的任意 16 位 tag 都按普通 tag 保存；源码中的未知变体由
  TypeChecker 拒绝。
- 变体可无载荷、单载荷或多载荷。源码参数数量和每个参数类型由
  TypeChecker 校验；VM 负责栈下溢检查。
- `OpEnumTag` 原位把 Enum 替换为 `int64` tag；非 Enum 和栈下溢均报错。
- `OpGetEnumValue(index)` 原位把 Enum 替换为第 `index` 个关联值；索引按
  构造参数顺序，非 Enum、负索引（编码上不可产生）、越界和栈下溢均拒绝。
- `awase` 先把枚举保存到临时 Local/Global，再以
  `OpEnumTag`、`OpConstant`、`OpEqual`、`OpJumpIfFalse` 选择分支；命中后逐个
  `OpGetEnumValue` 写入绑定槽。TypeChecker 保证穷尽、变体不重复及绑定数量。
- VM Enum 是不可变对象，赋值、传参、返回和容器写入共享同一个值；构造时
  仅浅复制载荷。VM 的通用比较明确不支持 Enum 整体比较。
- `nullable<Enum>` 使用 nil 或 Enum 引用；Enum 内的 nullable 标量继续使用
  M13a box，nullable 引用继续使用零指针。因此 nil 与合法标量零值不混淆。

## 原生布局

Enum 使用 `TypeID=107`，不占用 Struct/Array/Storage 的 100–102、
Closure/Environment/Cell 的 103–105 或 Nullable box 的 106。对象地址由现有
非移动 Mark-Sweep heap 保持稳定。

16 字节 M8 对象头之后的 payload 为：

| 偏移 | 大小 | 内容 |
| ---: | ---: | --- |
| 0 | 8 | kind = 8 |
| 8 | 8 | tag |
| 16 | 8 | payload count |
| 24 | 8 | bitmap word count |
| 32 | 8×N | 精确引用位图，低位对应较小 payload index |
| 32+8×N | 8×count | 单槽 payload，保持源码顺序 |

`N = ceil(count/64)`，请求大小为
`32 + 8*N + 8*count`，交给 M8/M10 分配器做 16 字节物理对齐。AOT 在生成前
检查 tag、栈深度、位图长度和大小溢出；分配器在失败时先做精确 GC 并重试，
仍不足则走明确 OOM。GC Stress 在分配前触发，此时所有已计算 payload 都已由
当前 Shadow Frame 的精确 root vector 发布。分配返回的 payload 已清零，且从
对象初始化开始到完成之间没有 safepoint，扫描器不会看到半初始化对象。

## 精确 GC

M11 scanner 识别 TypeID 107，验证 kind、payload 大小、count、位图字数和所需
字节数，只对位图标记的 payload 槽调用精确 mark。因而标量位模式不会被当作
引用，同时支持 Enum 到 Struct、Array、Closure、Nullable box 和 Enum 的边，
以及通过 Array/Cell 形成的环。Sweep preflight 把 TypeID 107 作为合法的动态
变长对象；不可达对象和环均由现有 Mark-Sweep 回收。

## 静态类型恢复

Mochi 字节码没有 Enum 类型表。AOT 从所有 `OpEnum` 构造点收集按 tag 分离的
payload 类型与引用信息，并识别 Compiler 生成的标准 `awase` tag 分支，为
`OpGetEnumValue` 做 tag narrowing。函数参数、返回值、Local、Global、容器和
Closure Cell 仍通过现有联合约束与活跃性分析传播。无法从字节码恢复出唯一
payload 类型的非标准手工字节码会被明确拒绝，而不是保守扫描。

## Compiler A 边界

固定 `build/compiler-A.mochi` 为 Mochi v14、152805 字节、377 个函数、33477
条指令，SHA256
`b5904e8fd51e96be16183826177504ed395be67143e0c78d7a4b5d304cfd00a1`。
M13b 后 `<main>` byte offset 19 的 `OpEnum` 不再是缺失实现；当前首次编译阻塞
推进为函数 `kiso/fs::error_from` byte offset 6 的 `OpIntegerConvert`。本阶段不
实现 M13c、DataConvert 或 MMA Runtime。
