# Mochi Native AOT M6b — 原生浮点最短十进制输出

M6a 已在 Apple M4 Pro 上完成 float32/float64 硬件算术、比较、转换的实机差分验证。
M6b 在此基础上为 `OpChant` 和顶层 `OpPop` 增加浮点输出；不更改 Mochi v14 的 Opcode / 容器格式。

## 本阶段能力

- `chant(float32)` / `chant(float64)`：真正执行 ARM64 例程，从运行时 FP 位模式生成十进制文本。
- `float32` 使用最多 **9** 位有效数字，`float64` 使用最多 **17** 位有效数字，选择能往返恢复原始位模式的最短文本。
- 按 Mio VM 的 `strconv.FormatFloat(value, 'g', -1, 32/64)` 规则选择定点/科学记数法：指数小于 `-4` 或大于等于 `6` 时采用科学计数法。
- IEEE 754 的 `NaN`、`+Inf`、`-Inf`、`+0`、`-0`，以及指数符号和补零规则均与 VM 对齐。
- 主程序、用户函数、递归函数里的 `OpChant` 都使用同一原生例程；函数参数与返回值仍保持 FP 原始位模式。
- 顶层 `OpPop` 保留“最后一次真正执行的表达式”语义，并在结束时按 `int64/bool/float32/float64` 的运行时标签输出。
- 不依赖 libc、Go Runtime、Mochi VM、动态符号和系统浮点格式化函数。ARM64 程序依旧仅通过 Darwin 系统调用输出。

## 算法与内存约束

`docs/NATIVE_FLOAT_RUNTIME_ARM64.S` 是可审计的 AArch64 汇编源码。编译器实际使用与它对应的、已经编码为 `uint32` 的 `internal/aot/darwinarm64/float_runtime_words.go`。
Mio 的 Go-host AOT 在生产编译时**不调用**外部汇编器或链接器，也不把 FP 运算结果在 Go 端预格式化。

这是一个偏重容易验证的**精确边界算法**，不声称是 Ryu / Schubfach 的高性能实现：

1. 从 IEEE 754 位模式提取符号、尾数 `m` 和二进制指数。
2. 在统一的十进制量级中构造 `center = 4m`、`lower = 4m - delta`、`upper = 4m + 2`，其中 `delta` 处理 normal 2 的幂附近的非对称半 ULP。
3. 用固定容量的 **96 个 uint32 小端 limb** 精确乘以 2 或 5，将上下浮点舍入边界归一到同一整数尺度。
4. 以整数除法构造十进制数字，按 `p=1..9/17` 搜索，只有候选文本落在 IEEE 754 的舍入区间内才接受它；边界是否包含由原始尾数奇偶决定。
5. 两个候选都可往返时采用“距离真实值最近、平局选偶数”的十进制舍入方式，最后按 Mio 的 `'g', -1` 风格渲染并由 Darwin `write` 输出。

单次调用使用约 **4.5 KiB** 的栈空间，`SP` 16 字节对齐；保留 `x18`、保存并恢复 `x19..x29` 和 `x30`，不会覆盖调用者寄存器操作数或递归深度状态。
本版本的整数算法时间复杂度较高，适合作为**正确性基线**，未来可用 Ryu/Schubfach 加速，但必须先保持逐字符输出兼容。

## 实机验收（macOS Apple Silicon）

首先在 Mio 项目根目录解压 M6b 增量源码。

```bash
mkdir -p build
go test ./internal/aot/...

go run ./cmd/mio build -o build/m6b-source.mochi examples/aot_m6b/floating_output.mio
go run ./cmd/mio exec build/m6b-source.mochi > build/m6b-source-vm.txt
go run ./cmd/mochi-aot -o build/m6b-source-native build/m6b-source.mochi
codesign --force --sign - build/m6b-source-native
./build/m6b-source-native > build/m6b-source-aot.txt
diff -u build/m6b-source-vm.txt build/m6b-source-aot.txt
cat build/m6b-source-aot.txt

go run ./examples/aot_m6b/make_float_print_mochi.go -o build/m6b-strict.mochi
go run ./cmd/mio exec build/m6b-strict.mochi > build/m6b-strict-vm.txt
go run ./cmd/mochi-aot -o build/m6b-strict-native build/m6b-strict.mochi
codesign --force --sign - build/m6b-strict-native
./build/m6b-strict-native > build/m6b-strict-aot.txt
diff -u build/m6b-strict-vm.txt build/m6b-strict-aot.txt
cat build/m6b-strict-aot.txt
```

两组输出预期：

```text
3.141592653589793
3.75
NaN
+Inf
-Inf
-0
1.2345679
0.1
1e-05
1e+06
5e-324
1.7976931348623157e+308
6
3.141592653589793
```

还可以构建 `examples/aot_m6b/mixed_pop.mio` 验证分支中的 int/bool/float 最后一次 `OpPop`（当前选择浮点分支，预期输出 `1.25`）。

## 本地已验证 / 仍需实机验证

- M0–M6a AOT Go 测试、M6b 格式化指令模拟、Mach-O 静态结构：通过。
- 浮点格式化模拟覆盖 f32/f64 的特殊值、舍入边界、随机位模式；检查 16 字节栈对齐及 x18/x19..x29 保存：通过。
- **macOS M4 Pro 实机测试：等待运行者验证。**

### 仍未实现

- 高性能 Ryu/Schubfach 实现及量化优化。
- 浮点从 stdin 解析、文件 I/O、堆分配、GC、外部符号调用。
- 本例程尚未设计为信号异步安全或跨线程共享缓存；它仅使用当前栈，不持有全局缓存。

这仍是 Go 编写的宿主 AOT 编译器生成的独立 ARM64 程序；生成的原生程序不依赖 Go 或 Mochi VM 运行。
