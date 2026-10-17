# Mochi AOT M1 — ARM64 原生整数计算

M0 的 Mach-O `exit(0)` 已在 Apple M4 Pro 上成功执行。M1 继续使用同一套
`LC_MAIN`、`LC_SYMTAB`、`LC_DYSYMTAB`、dyld 装载布局与 ad-hoc codesign，
不更改 Mochi v14 格式，不增加字节码 Opcode。

## M1 能力

| Mochi 字节码 | 原生实现 |
| --- | --- |
| `OpConstant` | 仅 `int64`；MOVZ + 3×MOVK 组成 64 位位模式 |
| `OpAdd` | AArch64 `ADD`，64 位溢出按位回绕 |
| `OpSub` | AArch64 `SUB`，64 位溢出按位回绕 |
| `OpMul` | AArch64 `MUL`，保留低 64 位 |
| `OpNeg` | AArch64 `NEG`，符合 int64 回绕语义 |
| `OpChant` | 原生十进制输出，调用 Darwin `write` 系统调用 |
| `OpPop` | 保存顶层表达式的最后值，在程序结束时输出 |

实现边界：操作数栈暂时使用 x9..x15（最大深度 7），目前只支持 `int64`，
不支持全局变量、函数、float、数组、字符串、比较、循环、GC 或任意复杂运行时。
所有未支持功能会在编译阶段明确报错，绝不悄悄替换为 exit(0)。

运行时有一个自行编码的整数打印例程，使用 32 字节栈缓冲区，
对负数与 `MinInt64` 使用无符号除法处理绝对值；没有使用 libc、Go Runtime 或 Mochi VM。

## 在 Mio 仓库根目录测试（macOS Apple Silicon）

```bash
# 首先确认已在 M0 修复版本上通过 exit(0) 实机测试
go test ./internal/aot/...
mkdir -p build

# 从真正的 Mio 源码生成 Mochi，再翻译为 ARM64 Mach-O
go run ./cmd/mio build -o build/m1.mochi examples/aot_m1/arithmetic.mio
go run ./cmd/mochi-aot -o build/mochi-m1 build/m1.mochi

# 对本机 ARM64 可执行文件进行临时签名
codesign --force --sign - build/mochi-m1
codesign --verify --verbose=4 build/mochi-m1

# 验证真实打印与正常退出
./build/mochi-m1
echo $?
```

期望标准输出：

```text
42
42
-3
-42
0
```

其中前四行来自 Mio 的 `chant`，最后一行 `0` 来自 shell 的 `echo $?`。

### 未支持功能必须明确报错

```bash
printf 'chant(1.5);\n' > build/unsupported.mio
go run ./cmd/mio build -o build/unsupported.mochi build/unsupported.mio
go run ./cmd/mochi-aot -o build/unsupported build/unsupported.mochi
# 应提示 AOT M1 constant ... unsupported type / only int64
```

## 实现注意事项

- M1 **没有新增 Mochi Opcode**，只是 Mochi AOT 能翻译的指令变多。
- Mio 原生可执行文件正常退出码始终为 0，计算结果通过 stdout 验证。
- `BL` 调用的打印例程在 `SYS_exit` 机器码之后，用 PC 相对偏移回填；不使用静态地址重定位。
- `LC_SYMTAB` 和 `LC_DYSYMTAB` 继续使用空表；它们是已验证的 dyld 装载元数据。
- 只在 macOS ARM64 目标机器上执行真正的程序；Linux CI 只能检查文件格式和机器码布局。
- 此版本是过渡实现，x9..x15 的寄存器栈不是最终的 Mochi ABI。
- 暂时不要在该阶段引入 GC、线程、Mio Runtime 或 LLVM。

## 严格验证：排除 Mio 常量折叠的影响

Mio v1 编译器可能将 `chant(40 + 2)` 提前折叠为 `chant(42)`。
因此上面的 `.mio` 示例主要验证源码到原生程序的端到端链路，**不能单凭这个示例证明 ARM64 ADD 被执行**。

用本目录的测试工具构造包含真实算术 Opcode、但未经过常量折叠的标准 Mochi 文件：

```bash
go run ./examples/aot_m1/make_arithmetic_mochi.go -o build/m1-arithmetic.mochi

# 先用 v1 Mochi VM 验证基准语义
go run ./cmd/mio exec build/m1-arithmetic.mochi

# 然后从同一个 Mochi 文件生成原生程序
go run ./cmd/mochi-aot -o build/m1-arithmetic build/m1-arithmetic.mochi
codesign --force --sign - build/m1-arithmetic
./build/m1-arithmetic
echo $?
```

两种执行方式的预期输出应一致：

```text
42
42
-3
-9223372036854775808
4
-9223372036854775808
99
```

原生程序后续的 `echo $?` 还应输出 `0`。

这个工具没有加入新的 VM 指令，也没有修改自举编译器，只通过现有 Mochi 编码器创建测试输入。
