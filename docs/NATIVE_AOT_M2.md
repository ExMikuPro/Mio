# Mochi AOT M2 — macOS ARM64 原生全局变量

M0 在 Apple M4 Pro 上执行 `exit(0)`，M1 验证了未经常量折叠的整数运算与 VM 差分。M2 沿用已有 Mach-O 和整数输出例程，给 AOT 增加真正的可写全局变量存储。

## M2 新增能力

| Mochi Opcode | 原生实现 |
| --- | --- |
| `OpSetGlobal` | `ADRP x8` 定位数据页，`STR Xt, [x8, #offset]` 保存整数，弹出操作数栈顶 |
| `OpGetGlobal` | `ADRP x8` 定位数据页，`LDR Xt, [x8, #offset]` 读取整数并入栈 |

`OpConstant`、`OpAdd`、`OpSub`、`OpMul`、`OpNeg`、`OpChant`、`OpPop` 继续沿用 M1 的实现。**没有添加 Mochi v14 Opcode，也没有修改 VM。**

### macOS Mach-O 布局

| 段 | 文件偏移 | 用途 |
| --- | --- | --- |
| `__TEXT` | `0x0000` | Mach-O 头、装载命令和 ARM64 指令（16 KiB） |
| `__DATA` | `0x4000` | 全局 `int64` 槽位，单页读写映射（16 KiB） |
| `__LINKEDIT` | `0x8000` | `codesign` 后续追加签名信息 |

含全局变量时，**签名前文件大小为 32768 字节**。没有全局变量的 M0/M1 路径继续生成 16384 字节，不改变其已有布局。

每个全局槽位占 8 字节，最多 2048 个槽位。编译器使用 AArch64 `ADRP` 的 **4 KiB 指令页单位**计算 PC 相对地址，`LDR/STR` 再使用偏移访问数据；这个单位与 macOS M4 Pro 的 **16 KiB 虚拟内存页**不同。这里不产生绝对地址重定位，也不会因 ASLR 改变目标访问地址。

当前仅支持 `int64` 全局变量；不支持浮点、布尔、字符串、函数、分支、局部变量、GC 或线程。M2 的指令仍按字节码顺序直接执行；对于 `OpGetGlobal` 早于对应 `OpSetGlobal` 的情况，编译期直接报出“uninitialized”，避免把页内的零值误认为有效的 Mochi 值。

### 手动执行示例

```bash
mkdir -p build

go test ./internal/aot/...

go run ./cmd/mio build -o build/m2.mochi examples/aot_m2/globals.mio
go run ./cmd/mochi-aot -o build/mochi-m2 build/m2.mochi
codesign --force --sign - build/mochi-m2
./build/mochi-m2
# 预期：42 和 50，各占一行

go run ./examples/aot_m2/make_globals_mochi.go -o build/m2-globals.mochi
go run ./cmd/mio exec build/m2-globals.mochi > build/m2-vm.txt
go run ./cmd/mochi-aot -o build/m2-globals build/m2-globals.mochi
codesign --force --sign - build/m2-globals
./build/m2-globals > build/m2-native.txt
diff -u build/m2-vm.txt build/m2-native.txt
```

未经折叠的 Mochi 示例预期输出：

```text
42
50
-1
-9223372036854775808
9223372036854775807
99
7
```

该输入覆盖 `OpSetGlobal` 的弹栈、`OpGetGlobal` 入栈、重复更新、负数、`MinInt64` 溢出回绕、跨越 `__DATA` 内不同 4KiB 子页的槽位访问，以及末尾的 `OpPop` 输出。

### 安全边界与下一阶段

- 尚不支持函数、闭包、控制流与任何动态内存管理；它们必须显式报错。
- 单页 `__DATA` 是 M2 的刻意限制；后续支持更多变量时才扩展多页数据布局。
- `go test` 和 LLVM 结构检查在 Linux 上可执行，但**只有用户的 M4 Pro 能验证 macOS dyld 加载、代码签名和最终输出**。
- 原生目标程序不包含 Mochi VM 或 Go Runtime；Go 只用于运行 AOT 编译器。
- 下一阶段可以考虑比较、条件跳转与标签回填；不要在本阶段提前加入 GC。
