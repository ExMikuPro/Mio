# Mochi Native AOT M7 — 静态 UTF-8 字符串与只读常量区

M7 在已通过 M4 Pro 实机验证的 M6b 基线上扩展 **静态字符串**，不修改 Mochi v14 格式和 Opcode，不引入 GC、Go Runtime、libc 或 LLVM。

## 功能

- `OpConstant` 接受 `string`，并生成静态字符串 ID。
- `OpChant` 通过原生 `SYS_write` 输出完整 UTF-8 字节串，再追加换行。
- 支持 ASCII、中文、日文、空字符串和内嵌 `NUL`（`\x00`）。
- 只读字符串可以经 `OpSetGlobal/GetGlobal`、`OpSetLocal/GetLocal/DefineLocal`、`OpCall`、`OpReturnValue` 传递。
- `OpEqual/OpNotEqual` 对静态字符串按字节值比较：编译阶段将相同 UTF-8 内容驻留为同一个 ID。
- 顶层 `OpPop` 记录真实运行的最后一个值，能在 int64、bool、float32、float64、string 之间切换。

## 内存布局

```text
Mach-O __TEXT (R-X，固定 16 KiB)
  ├── __text: ARM64 主程序、函数及原生输出例程
  └── __const: [offset:uint32, len:uint32] * N + UTF-8 原始字节

可选 __DATA (RW，固定 16 KiB)
  └── __data: int64 / string ID / float 原始位模式等 8 字节槽位
```

`offset` 相对于 `__const` 起点；`len` 是 **字节长度**，不是 Unicode 字符数。`NUL` 没有终止字符串的含义；空串描述符长度为零。

原生值采用一个 64 位静态字符串 ID，传参和返回时无需复制数据，也不需要 GC。
打印例程使用 `ADRP + ADD` 计算 `__const` 的 PC 相对地址，地址随 Mach-O 的 ASLR 自动调整，没有动态符号或重定位。

目前 `__TEXT` 总量仍限定一页：代码和静态字符串共同使用 16 KiB 空间；超出容量则编译失败。M7 不支持运行时动态拼接、切片、堆字符串、复杂字符串 API 或通用 GC。非静态字符串排序也暂未支持。

## 在 Apple M4 Pro 实机验证

先将 ZIP 覆盖解压到仓库根目录。在项目根目录执行：

```bash
set -euo pipefail
mkdir -p build

go test ./internal/aot/...

go run ./cmd/mio build -o build/m7-source.mochi examples/aot_m7/strings.mio
go run ./cmd/mio exec build/m7-source.mochi > build/m7-source-vm.txt
go run ./cmd/mochi-aot -o build/m7-source-native build/m7-source.mochi
codesign --force --sign - build/m7-source-native
codesign --verify --verbose=2 build/m7-source-native
./build/m7-source-native > build/m7-source-aot.txt
diff -u build/m7-source-vm.txt build/m7-source-aot.txt
cat build/m7-source-aot.txt

go run ./examples/aot_m7/make_strings_mochi.go -o build/m7-strict.mochi
go run ./cmd/mio exec build/m7-strict.mochi > build/m7-strict-vm.bin
go run ./cmd/mochi-aot -o build/m7-strict-native build/m7-strict.mochi
codesign --force --sign - build/m7-strict-native
codesign --verify --verbose=2 build/m7-strict-native
./build/m7-strict-native > build/m7-strict-aot.bin
cmp build/m7-strict-vm.bin build/m7-strict-aot.bin

go run ./examples/aot_m7/make_strings_mochi.go -int-pop -o build/m7-intpop.mochi
go run ./cmd/mio exec build/m7-intpop.mochi > build/m7-intpop-vm.bin
go run ./cmd/mochi-aot -o build/m7-intpop-native build/m7-intpop.mochi
codesign --force --sign - build/m7-intpop-native
./build/m7-intpop-native > build/m7-intpop-aot.bin
cmp build/m7-intpop-vm.bin build/m7-intpop-aot.bin

echo '===== Mochi Native AOT M7: PASS ====='
```

`cmp` 用于内嵌 NUL 的二进制安全差分，不能用肉眼检查终端显示代替。

## 严格测试期望输出

默认 `make_strings_mochi.go`：

```text
Hello, Mio!
你好，Mio！
こんにちは、Mio！
a<此处有一个 NUL 字节>b

hai
duplicate
branch yes
```

带 `-int-pop` 时，最后一行改为 `123`；其他字节完全一致。

## 回归注意事项

- M0～M6b 不含字符串的测试仍走原来的路径，已有 Mach-O 不受影响。
- `__const` 仅在存在字符串常量时创建，`__DATA` 仍只在有全局槽位时创建。
- 函数调用依然遵循 M5 帧内 8 字节槽 ABI，额外输出例程不碰 x19～x29。
- 超出静态容量、非法 UTF-8、字符串算术/排序和不支持的 Opcode 都必须报错。
- 旧字节码格式与 VM 行为不做任何修改。

在 macOS 实机测试前只能确认 Go 单元测试、Mach-O 校验和 AArch64 模拟；最终加载、签名和真实执行仍以 M4 Pro 为准。
