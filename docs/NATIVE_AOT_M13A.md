# Mio Native AOT M13a：Nullable 原生语义

## VM 语义与表示

VM 以独立的 `value.KindNil` 表示 `mu`；静态类型允许任意 `T?`，并不只限于
引用类型。因此 Native 不能把原始 64 位零值解释为 nil。

M13a 保持 M15a 的单个 64 位值槽 ABI：

- `mu` 使用空指针 `0`。
- `array`、`struct`、`function` 等可空堆引用使用空指针或真实 M8 payload
  指针，不额外装箱。
- int、uint8、bool、float32、float64 和静态 string ID 等可空标量使用
  M8 TypeID 106 的 16 字节 payload：`kind=7`、`rawBits`。因此整数零、
  `false`、正负零浮点位模式和空字符串 ID 都与 nil 明确区分。

`OpIsNil` 只检查静态分析确认可空的表示；对非可空原始零值固定返回 false。
在已经由语言 Flow Narrowing 保证非空的标量使用点，Native 解箱；畸形字节码
若让 nil 到达该路径，会以状态码 1 和 `M13 nullable value is nil` 诊断退出。

## ABI 与精确 GC

Nullable 参数、返回值、Local、Global 和 Closure Cell 都继续占一个 64 位槽。
类型约束分析把 nil、赋值、调用点、返回值、Struct 字段和 Array 元素统一求解，
并把可空标量槽作为精确引用加入 M11 Shadow Frame。

M11 标记器和 M10 Sweep 预检识别 TypeID 106。标量 box 不含子引用；空指针由
精确扫描器跳过。可空 Struct 字段和 Array 元素进入原有引用位图；Cell 的
reference 位也覆盖可空标量 box 与可空对象引用。把字段、元素、Global 或 Cell
改写为 nil 会清除唯一指针，旧对象随后可正常回收。

`OpSetNullableIndex` 使用 VM 的既有 `[]T?` 语义：nullable uint8 不再使用紧凑
单字节 storage，而使用 8 字节的 nil/box 槽；可空引用数组则保存 nil/对象指针。

## Compiler A 边界

固定 `build/compiler-A.mochi`（Mochi v14，152805 字节，377 个函数，33477 条
指令，SHA256
`b5904e8fd51e96be16183826177504ed395be67143e0c78d7a4b5d304cfd00a1`）
已越过主程序 byte offset 15 的 `OpIsNil`。当前首个真实编译阻塞为主程序
byte offset 19 的 `OpEnum`；Enum 不属于 M13a。
