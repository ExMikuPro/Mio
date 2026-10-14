# Mochi v1/v2/v3/v4/v5/v6/v7/v8 字节码格式

Mochi 是 Mio 的二进制字节码容器。所有多字节整数使用大端字节序，文件不包含填充、校验和或尾随数据。

编码器对旧指令与常量写入最低可用版本：基础字节码使用 Mochi v2，浮点常量使用 Mochi v3，显式数值转换指令使用 Mochi v4，数组构造与读取使用 Mochi v5，数组长度与元素赋值使用 Mochi v6。可空值检查，以及结构体、枚举、Map 或函数值指令使用 Mochi v8。解码器和 VM 可以读取 Mochi v1 至 v8，包括既有 Mochi v7 文件。

## 公共头部和主指令流

| 字段 | 大小 | 说明 |
| --- | ---: | --- |
| 签名 | 5 字节 | ASCII `MOCHI` |
| 版本 | `uint16` | `1`、`2`、`3`、`4`、`5`、`6`、`7` 或 `8` |
| 常量数量 | `uint32` | 后续常量条目数 |
| 常量表 | 可变 | 见下节 |
| 全局槽位数量 | `uint32` | VM 启动时分配的槽位数 |
| 主指令长度 | `uint32` | 指令字节数 |
| 主指令 | 可变 | 编码后的 Opcode 与操作数 |

Mochi v1 在主指令后结束。Mochi v2 至 v8 继续写入相同的函数表。

## 常量表

每个常量以一个 `uint8` 类型标签开始：

| 标签 | 数据 |
| ---: | --- |
| `1` | 一个按位保存的 `int64`，占 8 字节 |
| `2` | `uint32` UTF-8 字节长度，随后是字符串数据 |
| `3` | 一个 IEEE 754 binary64 位模式，占 8 字节，Mochi v3 及后续版本允许 |

布尔值和 `mu` 不进入常量表，分别由 `OpTrue`、`OpFalse` 和 `OpNil` 产生。

## Mochi v2/v3/v4/v5/v6/v7/v8 函数表

主指令后先写入 `uint32` 函数数量，每个函数依次包含：

| 字段 | 大小 | 说明 |
| --- | ---: | --- |
| FunctionID | `uint16` | 从 0 开始，必须与函数表索引一致 |
| 名称长度 | `uint16` | UTF-8 名称的字节数 |
| 名称 | 可变 | 非空 UTF-8 文本 |
| 参数数量 | `uint32` | 函数参数槽位数 |
| 局部槽位数量 | `uint32` | 包含参数槽位，不能少于参数数量 |
| 标志 | `uint8` | bit 0 表示函数返回值，其余位必须为 0 |
| 指令长度 | `uint32` | 函数指令字节数 |
| 指令 | 可变 | 独立函数指令流 |

## Opcode

Opcode 占 1 字节。表中的 `u16` 操作数使用大端字节序。

| 编号 | 名称 | 操作数 |
| ---: | --- | --- |
| 1 | `OpConstant` | 常量索引 `u16` |
| 2 | `OpAdd` | 无 |
| 3 | `OpPop` | 无 |
| 4 | `OpSub` | 无 |
| 5 | `OpMul` | 无 |
| 6 | `OpDiv` | 无 |
| 7 | `OpNeg` | 无 |
| 8 | `OpTrue` | 无 |
| 9 | `OpFalse` | 无 |
| 10 | `OpNot` | 无 |
| 11 | `OpEqual` | 无 |
| 12 | `OpNotEqual` | 无 |
| 13 | `OpLess` | 无 |
| 14 | `OpLessEqual` | 无 |
| 15 | `OpGreater` | 无 |
| 16 | `OpGreaterEqual` | 无 |
| 17 | `OpJump` | 目标字节偏移 `u16` |
| 18 | `OpJumpIfFalse` | 目标字节偏移 `u16` |
| 19 | `OpSetGlobal` | 全局槽位 `u16` |
| 20 | `OpGetGlobal` | 全局槽位 `u16` |
| 21 | `OpNil` | 无 |
| 22 | `OpChant` | 无 |
| 23 | `OpSetLocal` | 局部槽位 `u16` |
| 24 | `OpGetLocal` | 局部槽位 `u16` |
| 25 | `OpCall` | FunctionID `u16` |
| 26 | `OpReturn` | 无 |
| 27 | `OpReturnValue` | 无 |
| 28 | `OpIntToFloat` | 无，Mochi v4 起允许 |
| 29 | `OpFloatToInt` | 无，Mochi v4 起允许 |
| 30 | `OpArray` | 元素数量 `u16`，Mochi v5 起允许 |
| 31 | `OpIndex` | 无，Mochi v5 起允许 |
| 32 | `OpLen` | 无，Mochi v6 起允许 |
| 33 | `OpSetIndex` | 无，Mochi v6 起允许 |
| 34 | `OpIsNil` | 无，Mochi v7 起允许 |
| 35 | `OpSetNullableIndex` | 无，Mochi v7 起允许 |
| 36 | `OpStruct` | 字段数量 `u16`，Mochi v8 起允许 |
| 37 | `OpGetField` | 字段索引 `u16`，Mochi v8 起允许 |
| 38 | `OpEnum` | 变体编号 `u16`、关联值数量 `u16`，Mochi v8 起允许 |
| 39 | `OpEnumTag` | 无，Mochi v8 起允许 |
| 40 | `OpGetEnumValue` | 关联值索引 `u16`，Mochi v8 起允许 |
| 41 | `OpMap` | 无，Mochi v8 起允许 |
| 42 | `OpMapGet` | 无，Mochi v8 起允许 |
| 43 | `OpMapSet` | 无，Mochi v8 起允许 |
| 44 | `OpFunction` | FunctionID `u16`，Mochi v8 起允许 |
| 45 | `OpCallValue` | 实参数量 `u16`，Mochi v8 起允许 |

Opcode 编号是 v1/v2 兼容面的一部分。未知 Opcode、截断操作数和跳入操作数字节的目标都会被拒绝。

## 验证限制

| 项目 | 上限 |
| --- | ---: |
| 文件 | 64 MiB |
| 常量 | 65,536 |
| 全局槽位 | 65,536 |
| 单个字符串 | 16 MiB |
| 主指令或单个函数指令 | 16 MiB |
| 函数 | 65,536 |
| 函数名称 | 65,535 字节 |
| 单个函数的参数或局部槽位 | 65,536 |

解码器还会检查 UTF-8、索引范围、函数元数据、返回指令种类和跳转边界。文件末尾不允许额外数据。VM 会继续检查运行时类型、Value Stack、局部槽位初始化、调用深度和指令预算。

## 兼容性

- v1 文件没有函数表，只能表达主指令流。
- v2 保留完整的 v1 前缀，并在末尾增加函数表。
- v3 只新增常量标签 `3`，函数表与 v2 一致。v1/v2 解码严格拒绝该标签。
- v4 只新增 Opcode 28 和 29，二进制布局与 v3 一致。v1/v2/v3 解码严格拒绝这两个 Opcode。
- v5 只新增 Opcode 30 和 31，二进制布局与 v4 一致。v1 至 v4 解码严格拒绝这两个 Opcode。
- v6 只新增 Opcode 32 和 33，二进制布局与 v5 一致。v1 至 v5 解码严格拒绝这两个 Opcode。
- v7 只新增 Opcode 34 和 35，二进制布局与 v6 一致。v1 至 v6 解码严格拒绝这两个 Opcode。
- v8 只追加 Opcode 36 至 45，二进制布局与 v7 一致。v1 至 v7 解码严格拒绝这些 Opcode。
- 不含浮点常量、转换、数组或新类型指令的程序生成 v2；含浮点常量的程序生成 v3；含转换指令的程序生成 v4；只构造或读取数组的程序生成 v5；使用非空数组长度或元素赋值的程序生成 v6；使用可空值或 v8 核心类型指令的程序生成 v8。解码器继续接受合法 v1 至 v7。
- 格式没有校验和或签名。加载不可信文件时必须经过 `mochi.Decode`，不能绕过验证直接执行原始字节。

后续格式若需要不兼容变更，应使用新的版本号，不能改变已有字段或 Opcode 编号。
