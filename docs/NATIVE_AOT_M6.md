# Mochi Native AOT M6a — ARM64 原生浮点运算

M6a 基于已经在 Apple M4 Pro 上通过实机差分的 M5 版本，新增 `float32` 与
`float64` 的 FP/SIMD 运算。**这是 M6 的第一阶段，不包含浮点最短十进制输出。**

## 新增能力

| Mochi Opcode | AArch64 指令 | 语义 |
| --- | --- | --- |
| `OpConstant` | MOVZ / MOVK | 保留 IEEE 754 原始位模式，包括 `-0`、NaN、Inf |
| `OpAdd` | FADD S/D | 对应精度相加 |
| `OpSub` | FSUB S/D | 对应精度相减 |
| `OpMul` | FMUL S/D | 对应精度相乘 |
| `OpDiv` | FDIV S/D | IEEE 754 除法，零除产生 Inf/NaN |
| `OpNeg` | FNEG S/D | 符号位取反，保留负零 |
| `OpEqual`、`OpNotEqual` | FCMP + CSET | NaN 的相等为 false，不等为 true |
| `OpLess`、`OpLessEqual` | FCMP + CSET MI/LS | NaN 不会被误判为小于 |
| `OpGreater`、`OpGreaterEqual` | FCMP + CSET GT/GE | NaN 不会被误判为大于 |
| `OpIntToFloat`、`OpIntToFloat32` | SCVTF | int64 → FP64 / FP32，按目标精度舍入 |
| `OpFloatToInt`、`OpFloat32ToInt` | FCVTZS | FP → int64，先检查 NaN/Inf/越界 |
| `OpFloat32ToFloat64`、`OpFloat64ToFloat32` | FCVT | 精度转换 |

M6a 使用 M5 的 `StackEmitter`。主程序、函数的 `Global/Local/Operand` 槽位仍是
连续的 8 字节位模式存储，没有引入堆对象或 GC。

### 类型与内部调用约定

- 先验证各基本块的栈深度、跳转边界和确定初始化，再为全局槽位、局部槽位、
  函数参数、函数返回值与操作数建立统一的类型约束。
- `float32`、`float64`、`int64`、`bool` **禁止隐式混用**；精度转换必须通过原有转换 Opcode。
- 内部函数传参沿用 M5 的 X0..X6 存储 8 字节**原始位模式**，并不是 C ABI 的
  浮点参数约定；浮点计算在 V0/V1 上执行，跨调用保存的操作数位于当前栈帧。
- 浅整数程序仍使用 M0～M4 原实现，已有 M5 深整数程序仍使用 M5 实现。
- 尚不支持函数内部读写全局变量、闭包、动态调用、MMA、浮点数学原语 `OpMath`。
- `OpPop` 在普通函数内部只丢弃浮点值；在主程序里，尚不允许将浮点值作为
  最后的表达式结果输出。

### 浮点输出的边界

现有 Mochi VM 使用 Go 的 `strconv.FormatFloat(..., 'g', -1, bits)` 输出浮点数。
正确兼容这一行为需要实现最短往返十进制格式化（包括 `-0`、NaN、Inf、科学计数法
及舍入）。**M6a 不会用固定小数位数伪装为相同格式。**

因此，下面的代码在 M6a AOT 编译阶段明确报错：

```mio
chant(1.5);
```

M6a 测试使用 `chant(floatExpression == expected)` 或 `chant(int64(...))`，
检验的计算、比较与转换仍由原生 FP/SIMD 指令在运行时完成。M6b 再实现完整
浮点十进制打印。

### 非法浮点转换

现有 Mochi VM 对 NaN、Inf、`x < -2^63`、`x >= 2^63` 转 int64 返回运行时错误。
M6a 在执行 `FCVTZS` 前插入 `FCMP`、`B.VS`、`B.GE`、`B.LT`，并让非法输入
以**退出码 1** 退出，不静默饱和或回绕。原生错误信息暂时不与 VM 的文本相同，
未来需要统一 Native Runtime 的错误报告。

## 在 Apple Silicon Mac 上验证

在已集成 M5 的 Mio 仓库中覆盖 M6a ZIP，随后在仓库根目录运行：

```bash
go test ./internal/aot/...
mkdir -p build

go run ./cmd/mio build -o build/m6-source.mochi examples/aot_m6/floating.mio
go run ./cmd/mio exec build/m6-source.mochi > build/m6-source-vm.txt
go run ./cmd/mochi-aot -o build/m6-source-native build/m6-source.mochi
codesign --force --sign - build/m6-source-native
codesign --verify --verbose=2 build/m6-source-native
./build/m6-source-native > build/m6-source-aot.txt
diff -u build/m6-source-vm.txt build/m6-source-aot.txt
cat build/m6-source-aot.txt

go run ./examples/aot_m6/make_float_mochi.go -o build/m6-strict.mochi
go run ./cmd/mio exec build/m6-strict.mochi > build/m6-strict-vm.txt
go run ./cmd/mochi-aot -o build/m6-strict-native build/m6-strict.mochi
codesign --force --sign - build/m6-strict-native
codesign --verify --verbose=2 build/m6-strict-native
./build/m6-strict-native > build/m6-strict-aot.txt
diff -u build/m6-strict-vm.txt build/m6-strict-aot.txt
cat build/m6-strict-aot.txt
```

两组程序的输出应该相同：

```text
hai
hai
hai
hai
hai
3
hai
hai
hai
hai
hai
1
iie
hai
iie
iie
iie
iie
hai
hai
hai
hai
hai
```

### 验证层级

当前离线环境完成：Go 单元测试、Race/Vet、Mach-O 解析、LLVM 反汇编；
利用独立的 ARM64 指令模拟验证了未经常量折叠的完整测试程序，以及
NaN、±Inf、`±2^63` 等转换边界。**尚未在 Apple M4 Pro 上执行 M6a 原生程序。**

M6a 不修改 Mochi Opcode 编号、编码格式、Go VM 或 Mio 语言层面的类型语义。
不引入 LLVM、动态链接依赖、Go Runtime 或 GC。
