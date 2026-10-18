# Mio Native AOT M16b：多代原生自举闭环

M16b 在不修改 Mochi v14 格式、Opcode 编号和现有 GC 对象布局的前提下，完成
Native Compiler B → C → D。两个原生阶段生成的 Mochi 均与 VM 固定点逐字节一致。

## Runtime 与 AOT 增量

- NativeFS 增加 Darwin ARM64 `stat64(338)` 与 `mkdir(136)` syscall 路径。
- `stat` 保持 `os.Stat` 的符号链接跟随语义，写入 `status[0]=0/1/2` 和
  `status[1]=FsError`；`mkdir` 保持单级目录与 `0777 & ~umask` 语义。
- 路径仍通过私有 `mmap` 转成 NUL 结尾字节串；嵌入 NUL 返回 `InvalidPath`。
- 补齐 Compiler B 中出现的一元浮点、浮点谓词、符号与负号溢出 Math primitive。
- 对 self-host 生成的精确 `kiso/array`/`kiso/math` 转发函数按静态调用点专门化，
  避免 Mochi v14 不携带泛型实例参数时把不同 `[]T` 错误统一。
- 从 main 对 `OpCall`、`OpFunction`、`OpClosure` 建立保守可达闭包；无引用库函数
  生成不可达占位体，动态可达的函数值和闭包仍进入 ABI、类型与精确 Root Map 分析。

## Compiler B 静态审计

输入 `build/compiler-B-native.mochi`：

- 大小：120,707 字节
- SHA256：`b44a55bf3f9020e6b0ff83ad137b25774814a32dfe8acaf070abcd167bfd1465`
- Mochi：v14
- 函数 / 常量：477 / 497
- 字节码指令：36,662
- Lowered / Limited / Missing：6,816 / 29,846 / 0
- Structural Issues：0
- Native/MMA candidates：0

`Limited` 是静态类型与范围能力分类，不代表执行失败；真实 AOT 编译另外发现并修复了
泛型数组转发、Math primitive 和无引用抽象回调函数体三个静态审计无法证明的问题。

## 原生固定点结果

| 阶段 | 产物 | 字节 | SHA256 | VM `cmp` | 执行时间 | 最大 RSS | GC 次数 | GC 总 ticks |
| --- | --- | ---: | --- | --- | ---: | ---: | ---: | ---: |
| Native B → C | Compiler C | 120,707 | `b44a55bf3f9020e6b0ff83ad137b25774814a32dfe8acaf070abcd167bfd1465` | 一致 | 50.85 s | 1,076,543,488 B | 34 | 600,888,787 |
| Native C → D | Compiler D | 120,707 | `b44a55bf3f9020e6b0ff83ad137b25774814a32dfe8acaf070abcd167bfd1465` | 一致 | 52.32 s | 1,076,314,112 B | 34 | 622,137,878 |

计时频率为 24,000,000 Hz，因此 GC 总耗时约为 25.04 s 与 25.92 s。两个产物均通过
Mochi v14 解码和结构验证。原生编译器使用 1 GiB managed heap；B、C 的 Mach-O 均完成
ad-hoc codesign 与严格验证。

## 末尾 `hai` 的语义

`selfhost/bootstrap` 的成功路径返回 `hai`，顶层源码以表达式语句调用 `compile`，对应
main 中的 `OpCall 476; OpPop`。VM 自举测试直接调用 `RunWithOptions`，它保留
`LastPoppedValue` 但只把 `chant` 写到 stdout；Native AOT 可执行文件沿用 `mio exec`
语义，在进程结束时显示顶层最后一次 `OpPop`。因此原生 stdout 多一行 `hai`。

这不是布尔 ABI、弹栈或 NativeFS 错误，也不是随机日志。删除该输出会破坏既有 M1/M6b
顶层表达式结果语义；若未来需要静默嵌入入口，应以显式执行模式区分，而不是修改
`OpPop` 或针对自举程序隐藏输出。
