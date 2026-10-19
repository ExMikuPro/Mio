# Mio M17a/M17b：纯 Mio 二进制基础设施与 Mochi v14 Decoder

M17a/M17b 把 Native AOT 后续所需的二进制基础设施和 Mochi v14 读取/验证路径迁移为
真实 Mio 源码。Go 仍用于引导编译和差分测试，但 Decoder 的运行路径不依赖 Go Runtime。

## 语言能力审计

- `[]uint8` 是紧凑动态数组，支持创建、索引、修改、`with_capacity`、`reserve`、`push`、
  `truncate`、`append_all` 和 `clone`；VM 与 Native AOT 均有对应实现和边界检查。
- Core 整数为 `int8/16/32/64` 与 `uint8/16/32`，没有公开 `uint64`、位运算或移位。
  因此 64 位无符号数据使用两个 `uint32` 保存，掩码/移位改用受界算术拆分。
- `string.to_bytes` 与 `string.from_utf8` 是复制转换；`kiso/utf8.valid` 提供严格 UTF-8
  验证。动态字符串和字节数组互不别名。
- Struct、Enum、Nullable、泛型数组、函数值和闭包均可用；Reader 的可变游标使用私有
  单元素数组承载引用语义，不需要新增指针语义。
- `kiso/fs` 通过 `mio/fs/runtime` 提供 NativeFS。`decode_file` 以 32 KiB 分块读取，并在
  追加前执行 64 MiB 上限。
- Bunko 使用 `yobu "kiso/..."` 显式导入。Native AOT 已支持本实现所用数组、UTF-8、
  Nullable、Enum、Struct 和 NativeFS 语义。

审计没有发现必须扩展语言或 VM 的阻塞项，因此 M17 没有修改语言语义、Mochi v14 布局、
Opcode 编号、GC 或 Mach-O 格式。

## 实现布局

- `bunko/kiso/encoding/binary/stream.mio`：BinaryReader、BinaryWriter、`U64` 位模式。
- `bunko/kiso/encoding/binary/stream_test.mio`：游标、大小端、最大值、回填、复制和上限测试。
- `bunko/kiso/encoding/mochi/decode.mio`：v1–v14 解码、错误分类和结构验证。
- `bunko/kiso/encoding/mochi/encode.mio`：完整结构规范化重编码与摘要。
- `bunko/kiso/encoding/mochi/file.mio`：有文件大小上限的 NativeFS 读取入口。
- `bunko/kiso/encoding/mochi/decode_test.mio`：合法布局及畸形输入拒绝测试。
- `examples/m17_decoder/inspect.mio`：读取真实 Compiler B 并输出规范化摘要。
- `internal/aot/m17_mochi_decoder_test.go`：Go/Mio 完整规范化比较和 VM/Native 差分。

## 黄金结果

本地验证使用 M16b 已确认的 A/B/C/D 产物。测试先用 Go Decoder 解码并重编码，再让纯
Mio Decoder 对每个字段解码并从结构重编码；两侧规范化字节都必须与输入逐字节一致，
因此比较覆盖常量值、函数元数据、捕获和全部指令，而不只是摘要或 SHA-256。

| 产物 | 字节 | SHA-256 | 常量 | 函数 | 全局 | 总指令字节 | 规范化 |
| --- | ---: | --- | ---: | ---: | ---: | ---: | --- |
| Compiler A | 152,805 | `b5904e8fd51e96be16183826177504ed395be67143e0c78d7a4b5d304cfd00a1` | 4,597 | 377 | 2 | 85,661 | 与原文件一致 |
| Compiler B | 120,707 | `b44a55bf3f9020e6b0ff83ad137b25774814a32dfe8acaf070abcd167bfd1465` | 497 | 477 | 2 | 94,004 | 与原文件一致 |
| Compiler C | 120,707 | 同 B | 497 | 477 | 2 | 94,004 | 与原文件一致 |
| Compiler D | 120,707 | 同 B | 497 | 477 | 2 | 94,004 | 与原文件一致 |

同一纯 Mio 程序通过有上限的文件入口一次解码四个文件时，本机 Mochi VM 用时约 3.19 秒；Darwin ARM64
Native AOT 用时约 0.40 秒（包含测试辅助执行计时）。单独 Native Compiler B Inspector
约 0.29 秒，最大 RSS 约 6.2 MiB，峰值 footprint 约 5.8 MiB。这些数字是后续 Encoder
迁移的基线，不是稳定性能承诺。

## 下一阶段接口

ARM64 Encoder 可以直接复用 `BinaryWriter` 的固定宽度写入、`reserve`、指定偏移回填和
`writer_bytes`；Mach-O 中的 64 位无符号字段使用 `U64`。建议下一阶段先定义 Section、
Symbol、Relocation 和 Patch 结构，再在最终布局确定后统一回填，继续保留显式最大输出
长度。M17 不包含 Opcode Lowering 或 Mach-O Writer。
