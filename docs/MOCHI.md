# Mochi v1 至 v11 字节码格式

Mochi 是 Mio 的二进制字节码容器。所有多字节整数使用大端字节序，文件不包含填充、校验和或尾随数据。

Mochi v11 为固定宽度整数增加紧凑常量标签与显式转换指令。解码器和 VM 可以读取 Mochi v1 至 v11。

## 公共头部和主指令流

| 字段 | 大小 | 说明 |
| --- | ---: | --- |
| 签名 | 5 字节 | ASCII `MOCHI` |
| 版本 | `uint16` | `1` 至 `11` |
| 常量数量 | `uint32` | 后续常量条目数 |
| 常量表 | 可变 | 见下节 |
| 全局槽位数量 | `uint32` | VM 启动时分配的槽位数 |
| 主指令长度 | `uint32` | 指令字节数 |
| 主指令 | 可变 | 编码后的 Opcode 与操作数 |

Mochi v1 在主指令后结束。Mochi v2 至 v11 继续写入函数表。

## 常量表

每个常量以一个 `uint8` 类型标签开始：

| 标签 | 数据 |
| ---: | --- |
| `1` | 一个按位保存的 `int64`，占 8 字节 |
| `2` | `uint32` UTF-8 字节长度，随后是字符串数据 |
| `3` | 一个 IEEE 754 binary64 位模式，占 8 字节，Mochi v3 及后续版本允许 |
| `4` | 一个 IEEE 754 binary32 位模式，占 4 字节，Mochi v10 起允许 |
| `5` / `8` | `int8` / `uint8` 位模式，占 1 字节，Mochi v11 起允许 |
| `6` / `9` | `int16` / `uint16` 位模式，占 2 字节，Mochi v11 起允许 |
| `7` / `10` | `int32` / `uint32` 位模式，占 4 字节，Mochi v11 起允许 |

布尔值和 `mu` 不进入常量表，分别由 `OpTrue`、`OpFalse` 和 `OpNil` 产生。

## Mochi v2 至 v11 函数表

主指令后先写入 `uint32` 函数数量，每个函数依次包含：

| 字段 | 大小 | 说明 |
| --- | ---: | --- |
| FunctionID | `uint16` | 从 0 开始，必须与函数表索引一致 |
| 名称长度 | `uint16` | UTF-8 名称的字节数 |
| 名称 | 可变 | 非空 UTF-8 文本 |
| 参数数量 | `uint32` | 函数参数槽位数 |
| 局部槽位数量 | `uint32` | 包含参数槽位，不能少于参数数量 |
| 标志 | `uint8` | bit 0 表示函数返回值，其余位必须为 0 |
| 捕获数量及捕获项 | 可变 | v9 起：`uint32` 数量，每项为 `uint8` 来源和 `uint32` 索引 |
| 指令长度 | `uint32` | 函数指令字节数 |
| 指令 | 可变 | 独立函数指令流 |

## Opcode

Opcode 占 1 字节。表中的 `u16` 操作数使用大端字节序，`u8` 操作数直接占一个字节。

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
| 46 | `OpClosure` | FunctionID `u16`，Mochi v9 起允许 |
| 47 | `OpGetUpvalue` | Upvalue 索引 `u16`，Mochi v9 起允许 |
| 48 | `OpSetUpvalue` | Upvalue 索引 `u16`，Mochi v9 起允许 |
| 49 | `OpDefineLocal` | 局部槽位 `u16`，Mochi v9 起允许 |
| 50 | `OpIntToFloat32` | 无，Mochi v10 起允许 |
| 51 | `OpFloat32ToInt` | 无，Mochi v10 起允许 |
| 52 | `OpFloat32ToFloat64` | 无，Mochi v10 起允许 |
| 53 | `OpFloat64ToFloat32` | 无，Mochi v10 起允许 |
| 54 | `OpMath` | 原语编号 `u8`、实参数量 `u8`，Mochi v10 起允许 |
| 55 | `OpIntegerConvert` | 目标整数类型编号 `u8`，Mochi v11 起允许 |

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
- v9 在函数项中加入闭包捕获元数据，并追加 Opcode 46 至 49。
- v10 新增 binary32 常量标签 `4`，并追加 Opcode 50 至 54。旧版本严格拒绝这些标签和指令。
- 不使用新能力的既有程序保持原二进制语义；解码器继续接受合法 v1 至 v9。
- 格式没有校验和或签名。加载不可信文件时必须经过 `mochi.Decode`，不能绕过验证直接执行原始字节。

后续格式若需要不兼容变更，应使用新的版本号，不能改变已有字段或 Opcode 编号。
