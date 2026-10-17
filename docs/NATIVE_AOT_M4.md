# Mochi Native AOT M4：ARM64 原生函数调用

M0–M3 已在 Apple M4 Pro 上通过实机测试。M4 以已验证的 M3 为基线，
开始将 Mio 用户函数编译为真正的 AArch64 子程序。

## 新增能力

| Mochi 指令 | Native AOT |
| --- | --- |
| `OpCall` | 静态 FunctionID → ARM64 `BL`；递归、相互递归均可使用 |
| `OpGetLocal` | 使用 x29 相对地址，从独立 Stack Frame 加载 int64 |
| `OpSetLocal`、`OpDefineLocal` | 写入当前 Stack Frame 的 int64 槽位 |
| `OpReturnValue` | x0 返回 int64，恢复上一级函数的栈帧 |
| `OpReturn` | 不返回值；仅恢复栈帧 |
| 既有 `OpPop` | 函数内只丢弃值，不覆盖主程序末尾的表达式输出 |

M4 保留 M3 的整数、布尔比较、分支、循环、`chant` 和主程序全局变量支持。
不修改 Mochi v14 格式，不新增 Opcode，不需要 Go Runtime、Mochi VM、LLVM 或 libc 来执行目标程序。

## 函数调用协议（M4 临时内部 ABI）

- x0..x6：最多 7 个 int64 参数；有值返回时通过 x0 传递 int64。
- x9..x15：操作数寄存器栈；每次调用由调用者在原生栈上完整保存、返回后恢复。
- x29：当前函数栈帧基址；x30：链接寄存器。函数入口保存二者，返回时恢复。
- x8：局部槽位及全局槽位访问时的临时地址寄存器。
- x19..x21：主程序最后一次 `OpPop` 的输出状态；函数不覆盖它们。
- x22：M4 运行时调用深度计数；调用层数超过 1024 时程序返回退出码 1。
- 栈始终保持 16 字节对齐。每层函数为所有局部变量分配独立空间。
- 局部槽位由 `x29 - 8 * (localIndex + 1)` 定位。

这是一套过渡协议，不是最终的跨模块 Mio ABI，也不承诺与 C 函数互调。

## 已知限制

- 函数参数、局部变量、有值返回都只支持 `int64`；`unit` 函数支持无值返回。
- 布尔值可用于函数体中的条件判断和 `chant`，但不能作为参数、局部槽位或返回值。
- 每次函数调用最多 7 个参数；每个函数最多 256 个局部槽位。
- **M4 暂时不支持函数内部读取或写入全局变量。** 主程序的全局变量仍正常工作。
- 函数值、闭包捕获、`OpCallValue`、动态调用、浮点、字符串与 GC 仍不支持。
- 生成的函数和主程序必须容纳在一页 16 KiB 的 `__TEXT` 中，超出时明确报错。
- 分支汇合必须保持操作数栈的深度与类型一致；读取局部槽位前必须在所有可能路径上初始化。
- 与 M3 一样，AOT 不允许将暂不支持的指令静默替换为 `exit(0)`。

## 在 M4 Pro 上测试

进入 Mio 仓库根目录，解压 M4 压缩包覆盖到对应路径后执行：

```bash
go test ./internal/aot/...
mkdir -p build

# 第一组：真正的 Mio 源码
go run ./cmd/mio build -o build/m4-source.mochi examples/aot_m4/functions.mio
go run ./cmd/mio exec build/m4-source.mochi > build/m4-source-vm.txt
go run ./cmd/mochi-aot -o build/m4-source-native build/m4-source.mochi
codesign --force --sign - build/m4-source-native
./build/m4-source-native > build/m4-source-aot.txt
diff -u build/m4-source-vm.txt build/m4-source-aot.txt
cat build/m4-source-aot.txt

# 第二组：使用真实 Mochi 编码器直接构造字节码，排除常量折叠影响
go run ./examples/aot_m4/make_functions_mochi.go -o build/m4-functions.mochi
go run ./cmd/mio exec build/m4-functions.mochi > build/m4-functions-vm.txt
go run ./cmd/mochi-aot -o build/m4-functions-native build/m4-functions.mochi
codesign --force --sign - build/m4-functions-native
./build/m4-functions-native > build/m4-functions-aot.txt
diff -u build/m4-functions-vm.txt build/m4-functions-aot.txt
cat build/m4-functions-aot.txt
```

两组期望输出均为：

```text
120
42
7
3
1
0
99
```

其中 `120` 是原生递归阶乘的结果，`1` 和 `0` 是相互递归函数的结果。
同时检查 `echo $?` 应得到 `0`。

## 验证状态

本地在 Linux 上验证了：Go 单元测试、Mach-O 解析、ARM64 反汇编、
递归/相互递归/局部槽位及循环的隔离 AArch64 指令执行模拟。
真正的 macOS M4 Pro 实机签名与执行仍需要在目标机器上验证；不可提前标记为 PASS。
