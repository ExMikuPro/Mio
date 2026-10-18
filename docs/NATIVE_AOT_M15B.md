# Mio Native AOT M15b：闭包与捕获环境

## VM 语义基线

M15b 保持 Mochi VM 的共享 `Cell` 语义。`OpClosure` 捕获父 Local 时惰性创建
Cell；同一绑定后续产生的闭包复用该 Cell。捕获父 Upvalue 时直接转发父
Environment 中的 Cell。`OpSetLocal`、`OpSetUpvalue` 写入共享 Cell，
`OpDefineLocal` 切断当前 Local 与旧 Cell 的关联，从而保证循环内重复声明产生
独立绑定。闭包跨越创建帧生命周期时，值由 GC 对象图继续持有。

## 原生对象布局

所有指针均指向沿用 M8 对象头之后的 payload。

- Closure，TypeID 103，48 字节：`kind`、FunctionID、参数数、返回标志、
  PC-relative 链接得到的原生入口、Environment 指针。
- Environment，TypeID 104，`16 + 8*n` 字节：`kind`、Cell 数、按
  `CompiledFunction.Captures` 顺序排列的 Cell 指针。
- Cell，TypeID 105，16 字节：`kind | reference<<8`、共享的 64 位值槽。

M11 标记器验证并扫描 Closure→Environment、Environment→Cell；Cell 仅在
精确 reference 位为 1 时扫描其值。M10 Sweep 预检同步识别三种布局。活动帧
为传入 Environment 与已创建的 Local Cell 预留隐藏精确 Root 槽；闭包创建中
新分配的 Cell 和 Environment 会在下一次分配前发布到 Shadow Frame。

## Closure ABI

M15a 的普通值参数 ABI 保持不变：x0…x7 是前八个 64 位槽，其余参数位于调用
方栈区，SP 始终 16 字节对齐。x22、x23、x24 继续分别保存调用深度、Heap 和
Shadow Frame。

动态 `OpCallValue` 使用 x25 传递 Environment。调用方在独立的 16 字节对齐
槽中保存并恢复原 x25；该槽位位于所有栈参数之后，因此不会占用或移动普通参数。
静态 `OpCall` 仍使用 M15a 直接 `BL` 快速路径。Closure 内保存的是数值
FunctionID 和链接器生成的原生入口，不进行字符串函数名查找。

## Compiler A 边界

固定 `build/compiler-A.mochi` 的 SHA256 为
`b5904e8fd51e96be16183826177504ed395be67143e0c78d7a4b5d304cfd00a1`。
M15b 移除了 `<closure:376>` 的 Captures 拒绝路径。当前重新编译首先停在主程序
byte offset 15 的 `OpIsNil`；Nullable 属于后续里程碑，本轮不扩展。
