# Mochi Native AOT M5 — ARM64 扩展操作数栈

M0–M4 已在 Apple M4 Pro 上完成实机执行验证。
M5 的目标是移除 AOT 原有的 **最多 7 个表达式操作数**限制，
同时继续使用 Mio 的现有 Mochi 字节码和已验证的 Mach-O 文件布局。

## M5 的内存布局

浅表达式（最大操作数深度不超过 7）仍使用 M4 的 x9..x15 快速路径。
只有某个指令块的最大深度超过 7 时，才将主程序及所有函数统一切换到帧内槽位模式，
避免同一程序的函数调用同时存在两种操作数布局。

深层表达式不在运行时动态改变 SP，而是先经过控制流固定点分析，
为每个主程序或函数预留所需的操作数槽位：

```text
高地址

  x29 + 8   保存的 x30
  x29 + 0   保存的 x29   <- FP
  x29 - 8   local 0
  ...       local N-1
  ...       operand 0
  ...       operand M-1
  SP        16 字节对齐（帧底）

低地址
```

函数参数 x0..x6 在序言中写入 local 0..P-1；返回值仍使用 x0。
调用者的全部临时值都已经在当前栈帧，函数调用不再保存和恢复 x9..x15。
每个函数在返回前恢复 SP、x29 和 x30。x22 保留原来的调用深度保护逻辑。

### 仍然存在的限制

- 当前每个指令块最多 **512 个静态操作数槽**，不是无限增长的动态栈。
- 单个函数仍然最多 7 个 int64 参数和 256 个局部槽位。
- 全局变量仍使用 M2 的单页 __DATA，最多 2048 个 int64 槽位。
- __TEXT 仍为 16 KiB；特别长的程序可能先达到机器码文件大小上限。
- 依然只支持 int64 算术、布尔表达式、分支、循环和静态函数调用。
- 暂不支持浮点、字符串、数组、闭包、GC、系统级 ABI 互操作或动态链接。
- 不更改 Mochi v14 Opcode，不依赖 LLVM，也不依赖 Go Runtime 或 Mochi VM 执行目标程序。

## 单元测试

从 Mio 项目根目录执行：

```bash
go test ./internal/aot/...
go test -race ./internal/aot/...
go vet ./internal/aot/...
```

涵盖 8、32、128 层操作数、递归保留调用者临时值、分支汇合、
条件 OpPop、512 槽限制，以及带有 256 个局部槽和 300 个操作数的大帧访问。

## Apple Silicon 实机差分

以下步骤都在 Mio 仓库根目录执行，不需要任何 `.sh` 文件。

```bash
mkdir -p build

# 真实 Mio 源码：16 层嵌套表达式 + 递归 + 循环 + 布尔逻辑
go run ./cmd/mio build -o build/m5-source.mochi examples/aot_m5/deep_stack.mio
go run ./cmd/mio exec build/m5-source.mochi > build/m5-source-vm.txt
go run ./cmd/mochi-aot -o build/m5-source-native build/m5-source.mochi
codesign --force --sign - build/m5-source-native
codesign --verify --verbose=2 build/m5-source-native
./build/m5-source-native > build/m5-source-aot.txt
diff -u build/m5-source-vm.txt build/m5-source-aot.txt

# 不经过 Mio 常量折叠的真实 Mochi 字节码
go run ./examples/aot_m5/make_stack_mochi.go -o build/m5-strict.mochi
go run ./cmd/mio exec build/m5-strict.mochi > build/m5-strict-vm.txt
go run ./cmd/mochi-aot -o build/m5-strict-native build/m5-strict.mochi
codesign --force --sign - build/m5-strict-native
codesign --verify --verbose=2 build/m5-strict-native
./build/m5-strict-native > build/m5-strict-aot.txt
diff -u build/m5-strict-vm.txt build/m5-strict-aot.txt
```

两组预期输出均为：

```text
16
113
7
3
hai
iie
99
```

其中 113 = 1 + 8×(2+3+4+5)，由真正的原生递归得到。
严格输入测试中使用 16 个活跃的 int64 操作数，原生执行器不能只读预先算好的常量。

## 验证状态

在 Linux 隔离环境中，M5 已通过 Go 单元测试、Race、Vet、Mach-O 结构检查、
ARM64 LLVM 反汇编和指令执行模拟（包括正常退出、递归深栈、大帧偏移和调用深度保护）。
**完整 Mio 仓库 + Apple Silicon 实机测试仍需实际执行后才能确认通过。**

## 后续方向

M5 只解决 *int64/bool 的操作数存储和函数调用栈帧*，不负责 GC 根扫描。
未来的精确 GC 需要保存对象类型布局和根位置图，不能只凭这里的槽位地址猜测指针。
