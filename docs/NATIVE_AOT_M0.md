# Mochi AOT M0 — macOS Apple Silicon 的首个原生进程

本阶段只接受「空的 Mochi 程序」，生成独立的 macOS ARM64 Mach-O 可执行文件。
不执行 Go Runtime / Mochi VM，不使用 LLVM、C 编译器、汇编器或系统链接器。
主机 Go 仅负责构建编译器本身；运行时无 Go 依赖。

## 在 Mio 仓库根目录运行

```sh
# 检查本机有 Go (工程支持 go 1.22.2 及以上)
go version

# 运行 AOT 编码和格式单元测试
go test ./internal/aot/...

# 编译空 Mio 文件为真实 Mochi 文件
mkdir -p build
go run ./cmd/mio build -o build/empty.mochi examples/aot_m0/empty.mio

# AOT: Mochi -> Mach-O arm64
go run ./cmd/mochi-aot -o build/mochi-exit0 build/empty.mochi

# macOS Apple Silicon: 签名、检查并运行
file build/mochi-exit0
codesign --force --sign - build/mochi-exit0
codesign --verify --verbose=2 build/mochi-exit0
otool -hv build/mochi-exit0
otool -l build/mochi-exit0 | grep -A 4 LC_MAIN
./build/mochi-exit0
printf 'exit code: %s\n' "$?"
# 应输出 exit code: 0，且程序不打印任何内容
```

注意：`codesign` 是 Apple Silicon 可执行文件签名步骤，不能省略。它不是 C 链接器或 VM。

## 首个里程碑验收

- 编译器能读取原始 `Mochi` 文件，并经过 `mochi.Decode`、`mochi.Validate` 校验。
- 仅对真正空程序进行编译；含任何指令、常量、函数或全局变量均明确拒绝，避免偷偷降级成 `exit(0)`。
- 输出解析为 `MH_EXECUTE`, `CPU_TYPE_ARM64`，包含 `__TEXT,__text`、`LC_MAIN`、`LC_LOAD_DYLINKER`、`LC_BUILD_VERSION`。
- 正常退出码为 0，没有任何标准输出，没有调用 v1 Mochi VM。

## 已知限制

- 容器中只能测试 Mach-O 格式和字节编码；macOS 运行成功仍需用户在 M4 Pro 上验证。
- M0 仅支持空字节码，不代表算术、输出、函数、GC 已实现。
- Mach-O 基于 Apple Silicon 的 16 KiB 页面以及 `dyld` 动态装载，采用临时 ad-hoc codesign。
- `LC_BUILD_VERSION` 中的最低部署版本暂设为 macOS 11.0。
- 下一步再支持非空 Mochi 指令流、简单整数常量以及原生计算结果。

## 拒绝未实现指令的测试

```sh
printf 'chant(1);\n' > build/not_empty.mio
go run ./cmd/mio build -o build/not_empty.mochi build/not_empty.mio
# 预期：非 0 退出状态，包含 "AOT M0 supports only an empty Mochi program"
go run ./cmd/mochi-aot -o build/not_empty build/not_empty.mochi
```

如果第一轮 macOS 执行报 `Killed: 9` 或 `Bad CPU type`，请收集 `file`、`codesign --verify --verbose=4`、`otool -l` 输出，不要立刻修改编译器语义或关闭系统安全设置。

## M0 修复 01：`__LINKEDIT` 虚拟内存大小

最初版本将 `__LINKEDIT` 的 `vmsize` 写为 0，未为签名所需的链接编辑数据预留内存映射。
修复后 `__LINKEDIT` 初始布局为：`vmaddr=0x100004000, vmsize=0x4000, fileoff=0x4000, filesize=0`。
运行 `codesign` 会增加/调整签名数据及对应装载命令。必须对**重新生成**的可执行文件重新签名，旧可执行文件不会自动修复。

在 macOS Apple Silicon 上重新测试：

```sh
# 根目录下运行；确保已覆盖 internal/aot/darwinarm64 相关源码
go test ./internal/aot/...
go run ./cmd/mochi-aot -o build/mochi-exit0 build/empty.mochi
codesign --force --sign - build/mochi-exit0
codesign --verify --verbose=4 build/mochi-exit0
otool -l build/mochi-exit0 | grep -A 11 -B 2 '__LINKEDIT'
./build/mochi-exit0
printf 'exit code: %s\n' "$?"
```

如仍 SIGSEGV，请先不要改动 Mochi 语义，收集：

```sh
otool -hv build/mochi-exit0
otool -l build/mochi-exit0
codesign -dvv build/mochi-exit0 2>&1
lldb --batch -o run -o bt -o 'register read pc x0 x16' -- build/mochi-exit0
```

说明：本修复在非 macOS 环境验证了 Mach-O 字段、指令编码及测试；本地 macOS dyld 实际装载仍需在目标机器验证。


## M0 修复 02（历史实验）：显式 LC_LOAD_DYLIB（已在 Fix03 撤销）

当前 Apple Silicon 上 vmsize 修复后仍出现 `SIGSEGV (139)`。这是针对装载路径的下一轮兼容性修复，而非已证实的根因。

- 新增 `LC_LOAD_DYLIB /usr/lib/libSystem.B.dylib`（使用 macOS 系统库，不引入 Go Runtime、LLVM 或 C 后端）。
- 保留 `LC_LOAD_DYLINKER /usr/lib/dyld`、`LC_MAIN` 和三条退出指令。
- 测试必须验证导入 dylib、正确 ncmds/sizeofcmds、入口与指令流。

macOS 验证命令（用新源码**重新生成**并**重新签名**）：

```sh
go test ./internal/aot/...
go run ./cmd/mochi-aot -o build/mochi-exit0 build/empty.mochi
codesign --force --sign - build/mochi-exit0
codesign --verify --verbose=4 build/mochi-exit0
otool -l build/mochi-exit0 | grep -A8 'LC_LOAD_DYLIB'
./build/mochi-exit0
printf 'exit code: %s\n' "$?"
```

如果仍是 `139`，不要继续猜测：先用 LLDB 定位崩溃 PC（是否位于 `__TEXT,__text`）以及 dyld 的 backtrace：

```sh
lldb --batch -o run -o 'register read pc x0 x16 lr' -o bt -- build/mochi-exit0
```

此 ZIP 在 Linux 上可验证 Mach-O 结构，但无法在 macOS 上执行；修复有效性必须以用户 M4 Pro 实机结果为准。


## M0 修复 03：dyld 的空符号表 / 重定位元数据（需 M4 Pro 实机验证）

**已由用户 LLDB 日志确认，崩溃发生在 dyld 的**
`MachOAnalyzer::forEachRebase_Relocations`，`EXC_BAD_ACCESS address=0x48`，
而非 M0 的 ARM64 `__text` 指令。

已检查用户上传的已签名 Mach-O：原来的 7 个 Load Command 中没有
`LC_SYMTAB` / `LC_DYSYMTAB`。Apple dyld 的旧式 relocation 解析路径
会读取动态符号表元数据。缺失 `LC_DYSYMTAB` 时可能出现空指针访问。
公开的 Apple Silicon Mach-O 最小程序实验遇到**相同 PC 函数及 address=0x48**，
增加空的 `LC_SYMTAB`、`LC_DYSYMTAB` 后成功退出。
参考：https://owenbrooks.org/blog/constructing-a-valid-macho/

Fix03 采用已经得到公开实机案例验证的**最小路径**：保留 Fix01 对 `__LINKEDIT` 的修复，
撤销尚未证实必要的 Fix02 `LC_LOAD_DYLIB` 添加，同时新增下列装载元数据：

- `LC_SYMTAB`，24 字节（零个符号、空字符串表）。
- `LC_DYSYMTAB`，80 字节（全部 symbol ranges / relocation counts 为零）。
- 装载命令计数 `ncmds = 8`、`sizeofcmds = 480`（签名前）。
- 回归测试检查命令大小、空表字段、无重叠以及 `codesign` 插入空间。
- 无额外 `libSystem` 动态库依赖；保留 dyld 和原三条原生退出指令。

这是一份 **完整替换文件包**，无论此前应用了 Fix01 还是 Fix02，覆盖本包的两个 `.go` 文件即可，不应在之前补丁基础上手工叠加。

在 M4 Pro 上：

```sh
# 仓库根目录
rm -f build/mochi-exit0

go test ./internal/aot/...
go run ./cmd/mochi-aot -o build/mochi-exit0 build/empty.mochi
codesign --force --sign - build/mochi-exit0
codesign --verify --verbose=4 build/mochi-exit0
otool -l build/mochi-exit0 | grep -A 7 -E 'LC_(SYM|DYSYM)TAB'
./build/mochi-exit0
printf 'exit code: %s\n' "$?"
```

预期 `exit code: 0`。如果仍然失败，请保存 `lldb --batch -o run -o bt -- build/mochi-exit0` 输出。
注意：结构测试在 Linux 通过不代表代码已在 macOS 运行成功；目标机验收才算 M0 完成。
