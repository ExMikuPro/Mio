# Mio M17c/M17d：纯 Mio ARM64 Encoder 与 Mach-O Writer

M17cd 已完成一个不依赖 Go Writer 的纯 Mio Darwin ARM64 可执行文件生成路径。Go 只用于引导编译、
黄金差分、落盘、签名和测试调度；最终未签名 Mach-O 的指令、Load Commands、Section 内容与地址回填
均由 Mio 源码生成。

## 实现路径

- `bunko/kiso/aot/arm64/arm64.mio`：有界 ARM64 指令编码、Label、Fixup 与汇编。
- `bunko/kiso/aot/macho/macho.mio`：Section/Symbol/Relocation 数据结构、布局规划、地址 Patch 与 Mach-O 写入。
- `examples/m17_native_builder/build.mio`：真实多页纯 Mio Builder。
- `internal/aot/m17cd_pure_mio_test.go`：Go/Mio 黄金差分、VM/Native 差分、codesign、真实执行和 Mochi v14 集成。

二进制写入直接复用 `kiso/encoding/binary.BinaryWriter`、`U64`、reserve 与 patch 能力，没有引入第二套
缓冲格式。输出上限为 64 MiB；全局槽位上限与 Go 基线相同，为 2,048 个 8 字节槽位。

## ARM64 覆盖

当前纯 Mio Encoder 提供：

- `MOVZ/MOVK/MOVN`（X）；
- `ADD/SUB/ADDS/SUBS/CMP`（X 寄存器，及 ADD/SUB/CMP 12 位立即数）；
- `AND/ORR/EOR`（X 寄存器）；
- `LSL/LSR/ASR`（X 立即数别名）；
- `MUL/SDIV/UDIV`（X）；
- `LDR/STR`（X 无符号缩放偏移）、`LDRB/STRB`；
- `LDP/STP`（X signed offset），以及函数帧使用的 pre-index STP / post-index LDP；
- `B/BL/BR/BLR/RET`、`B.cond`、`CBZ/CBNZ`；
- `ADR/ADRP`，其中 ADRP 严格使用 4 KiB 架构页；
- `FADD/FSUB/FMUL/FDIV/FCMP`（D）、`SCVTF D,X`、`FCVTZS X,D`；
- Darwin syscall 使用的 `SVC`。

所有公开编码入口验证寄存器、立即数、位宽、对齐和有符号范围，错误不会被掩码后静默截断。未实现的
扩展包括 W/S 的完整算术矩阵、逻辑立即数、literal load、寄存器变量移位、原子/独占访存及通用 SIMD；
这些不属于本轮纯 Mio Builder 和验收程序使用的形式。

## Label、Fixup 与地址重定位

`Assembler` 使用稳定数组保存 `Label` 与 `Fixup`，支持前向和后向引用。汇编完成时统一回填 B、BL、
B.cond、CBZ 和 CBNZ，并分别执行 26 位或 19 位有符号范围验证。重复 Label、未定义 Label、非法寄存器、
越界分支和非法 Patch 均返回分类错误。

内部指令 Label Fixup 与布局后地址 Patch 是两个阶段。`AddressPatch` 只描述 Text/Const/Data 目标，
`macho.executable` 在最终 16 KiB 段布局确定后再计算 ADRP 与可选 ADD low-12。文件偏移、相对 VM 偏移
和 PC-relative 位移不混用。当前镜像没有动态未决符号，因此 Mach-O 重定位表为空；`Relocation` 与
`Symbol` 数据结构保留给 M17e/后续动态链接扩展，不能把内部已解析 Patch 误报为文件重定位记录。

## Mach-O 布局

Writer 与已验证的 Go AOT 保持相同布局：

- 64 位 ARM64 `MH_EXECUTE` Header；
- `__PAGEZERO`；
- RX `__TEXT`，含 `__text` 和按需出现的 `__const`；
- 按需出现的 RW `__DATA,__data`；
- R `__LINKEDIT`；
- `LC_LOAD_DYLINKER`、`LC_BUILD_VERSION`、空 `LC_SYMTAB`、空 `LC_DYSYMTAB`、`LC_MAIN`；
- 入口文件偏移固定为 `0x300`，段按 16 KiB 扩展，并为 codesign 的 `LC_CODE_SIGNATURE` 预留空间。

小型带常量和数据的纯 Mio 镜像与 `darwinarm64.ExecutableWordsWithGlobalsAndConst` 逐字节相同。

## 验收结果

综合测试程序真实执行以下路径：整数加、减、乘；长距离 taken `B.cond`；短距离 untaken `B.cond`；
循环中 taken/not-taken `CBNZ`；向后跳转；`BL/RET`；16 字节对齐 FP/LR 栈帧；全局 `LDR/STR`；
带 UTF-8、CJK、emoji 和内嵌 NUL 的 `__const`；以及超过 16 KiB 的 `__text`。

本机 macOS ARM64 结果：

| 项目 | 结果 |
| --- | --- |
| 原始文件大小 | 49,152 字节（2 页 `__TEXT` + 1 页 `__DATA`） |
| 原始 SHA-256 | `efac02a5d3664ba6d08fa1ead12062261f08ca0cf2fb32e4caf5a930a5844cd6` |
| VM / Native Builder | `cmp` 完全一致 |
| 重复生成 | 未签名原始字节一致 |
| ad-hoc codesign | `--verify --strict` 通过 |
| 真实执行 | 状态码 0 |
| VM Builder 测量 | 约 0.35 s，最大 RSS 9,977,856 bytes，peak footprint 7,913,832 bytes |
| Native Builder 测量 | 约 0.26 s，最大 RSS 2,441,216 bytes，peak footprint 1,999,088 bytes |

性能数字来自 `/usr/bin/time -l` 的单次本机测量，只作为本轮记录，不是稳定基准。Go 单元测试另行记录
VM 执行时间和包含 Native 编译、签名、启动在内的端到端时间。

Mochi 集成测试由 Go 仅生成一个合法 v14 fixture；纯 Mio Decoder 读取并验证其版本及指令元数据，
然后纯 Mio Encoder/Writer 生成退出状态由该元数据决定的真实 Mach-O。该案例只证明受限集成链路，
不宣称已经完成 Mochi Opcode Lowering。

## M17e 接口

M17e 可直接把 Opcode lowering 目标接到 `arm64.Assembler`，通过 `assembler_emit`、Label/Fixup API 生成
机器码；常量、全局和函数地址使用 `macho.AddressPatch`，最后提交 `macho.ImageSpec`。完整 lowering 需要
继续补齐按 Opcode 实际需求出现的 W/S、内存寻址和运行时调用形式，但不需要修改 BinaryWriter 或现有
Mach-O 布局协议。

建议提交信息：

```text
feat(aot): 完成纯 Mio ARM64 编码器与 Mach-O Writer
```
