# Mochi Native AOT：首次自举兼容性扫描

当前基线：**M11a macOS ARM64**。这个工具是只读审计器，不是 M11b 的 Stack Map，也不是新的字节码格式。

## 目标

根据真实的 `compiler-A.mochi` 识别 Native AOT 自举还缺哪些能力，而不是凭里程碑数量猜测完成进度。

分析全部主程序和函数指令块（包括不可达代码及从未调用的函数），统计：

- Mochi 版本、文件 SHA-256、函数数、静态指令数、全局/局部槽位数量。
- 每个 Opcode 的次数、涉及函数数量、首次出现的函数与字节偏移。
- 常量类型数量、已被 `OpConstant` 引用的条目数量。
- 当前 M11a 已有机器码降级路径、受限路径及完全缺少的路径。
- 带 Captures 的函数、缺少函数体的疑似外部/MMA 函数。
- 原生 ABI 与存储容量：7 个函数参数、256 个局部槽位、2,048 个全局槽位上限，超过时列为结构性缺口。
- 缺口最多的函数与机器可读 JSON 报告，方便后续按真实使用量规划开发。

## 在 Mio 仓库根目录使用

```bash
mkdir -p build

go test ./internal/aot/bootstrapaudit ./cmd/mochi-aot-audit -count=1

# 由现有 Go 编译器编译纯 Mio 的自举源代码；无需执行耗时的 B -> C -> D 自编译。
go run ./cmd/mio build -o build/compiler-A.mochi selfhost/bootstrap

test -s build/compiler-A.mochi

go run ./cmd/mochi-aot-audit \
  -in build/compiler-A.mochi \
  -json build/aot-bootstrap-audit.json \
  -top 20
```

如果已经保存了先前验证过的 `compiler-A.mochi` 或 `compiler-B.mochi`，也可以直接使用 `-in` 指向该文件，无需重新编译。扫描器不会执行它，因而不会修改 `selfhost/bootstrap/bootstrap.config`，也不会读写其他自举产物。

可选 `-strict`：只要出现当前缺失 Opcode、潜在外部函数或结构性 ABI 缺口，返回状态码 `3`；默认只是生成报告，不把正常发现缺口当作错误。

`-json ""` 可以关闭写文件，`-top` 控制终端打印的缺口数量。默认 JSON 文件路径是 `build/aot-bootstrap-audit.json`，所在目录必须存在。

## 报告的三个状态

| 状态 | 说明 |
| --- | --- |
| `lowered` | M11a 有对应的简单机器码处理路径，仍需要完整程序的 ABI/栈验证 |
| `limited` | M11a 只覆盖该 Opcode 的特定类型、位置或调用方式；不能认为已完全兼容 |
| `missing` | 在当前 M11a 中没有实现，或某个实际使用位置明确越过支持边界 |

例如 `OpCall` 本身有原生实现，但如果它指向没有函数体的 MMA 绑定，则这个调用点被标为 `missing`。`OpGetGlobal` 在主程序内可用，在当前 M11a 的函数体中仍被拒绝，因此也按每个出现位置单独分类。

静态字符串 `OpConstant` 与动态字符串操作不同；前者有限实现，后者通常涉及数组/数据转换/Heap 与 GC。

**重要：这些不是“自举已经完成多少百分比”。** 即使九成指令落在已有路径，少量未实现的对象构造、动态数组或文件 I/O 仍足以让整套编译器无法启动。报告也不能自动证明类型约束、资源限制、GC Root Map 和运行时接口是否完备。

## 依据真实结果排期

优先关注 JSON 的 `opcodes[].missing`、`opcodes[].category`、`example_sites`、`structural_issues` 与 `functions[].missing`：

1. 若 `array`、`struct`、`enum`、`nullable` 的缺口高，先设计统一的对象布局与动态数据类型，并衔接 M11b 精确 GC Root。
2. 若 `data`、`narrow-int` 高，补齐 `[]uint8`、定长整数转换与字节处理。
3. 若 `closure`、`function` 高，优先补函数值、闭包捕获和可变调用约定。
4. 若 `NativeCandidates` 主要来自 `mio/fs/runtime`，必须建立文件/目录的原生 Runtime 绑定；现有 `mmap` GC 分配器并不会自动提供文件 I/O。
5. 各类型补齐后才开始真正的原生自举，先比对 compiler-A 运行结果，再逐步完成 B、C、D 一致性验证。

## 本轮安全边界

- 不修改 `internal/code`、`internal/mochi`、Compiler 或现有 AOT 编译路径。
- 读取字节码并调用原有 `mochi.Decode` / `mochi.Validate`，不执行 VM、不执行生成的 Mach-O。
- 报告只写用户指定的 JSON 路径，默认不会 Commit/Push/Tag。
- 原生 GC 的自动 Safepoint 与完整动态对象支持仍未完成。
- 这批源文件附带了 mock 环境的隔离测试；**尚未在用户的完整 Mio 仓库和 `compiler-A.mochi` 上实际执行**，需要以本机扫描结果为准。
