# Mochi Native AOT M11a · GC Runtime 可控集成

M10 的 Collector 已在 M4 Pro 通过独立探针验证。M11a 在普通 Mochi AOT 的 Mach-O
末尾链接 M10 的原生 heap.init / gc.alloc / gc.collect / heap.destroy 代码，
并把单个类型描述符与显式 Root Map 放入只读常量区。

**重要边界**：这不是 Mio 语言级堆分配功能，也不是自动 GC。M11a 使用显式
`-m11-gc` 编译开关，在 Mio 主程序完成原有输出后，运行固定的 GC 自检：
初始化 Heap、分配两个对象、精确保留一个 Root、收集另一个、复用内存、
撤掉 Root 后再次收集，再释放整个 Heap。测试失败时原生程序返回非零退出状态；
成功则退出 0。自检不输出额外的文本，因此可以将 VM 标准输出与 Native AOT
标准输出逐字节对照。默认不添加 GC Runtime，原有 M0～M10 路径不变。

M11a 使用的 Type Layout 是 TypeID=1、Payload=8、没有引用字段。
根是独立自检帧的第 0 个槽；第 1 槽故意不登记，从而验证伪根不会阻止回收。
它**尚未**使用 Mio 函数真实局部变量和寄存器上的 Root Map，也不能在用户代码
执行期间安全触发自动 GC；这些留给 M11b/M11c。

## macOS M4 Pro 手动测试

在 Mio 仓库根目录：

```bash
# 编译并对照普通字节码解释执行
mkdir -p build
go test ./internal/aot/... -count=1
go run ./cmd/mio build -o build/m11a.mochi examples/aot_m11a/runtime_integration.mio
go run ./cmd/mio exec build/m11a.mochi > build/m11a-vm.txt

# 编译含 GC Runtime 的 ARM64 Mach-O
go run ./cmd/mochi-aot -m11-gc -o build/m11a-native build/m11a.mochi
codesign --force --sign - build/m11a-native
codesign --verify --verbose=2 build/m11a-native
./build/m11a-native > build/m11a-native.txt
diff -u build/m11a-vm.txt build/m11a-native.txt
cat build/m11a-native.txt
```

期望：

```text
M11a: native GC runtime
42
```

`echo $?` 应为 0。M11a 自检不打印 PASS 字样，以免污染 Mio 程序原有输出。

## 约束

- M11a 所有注释采用简洁中文，保留 Mach-O、ABI、Safepoint 等专有名词。
- 只读区使用相对布局，无绝对重定位、无 Go Runtime、无 LLVM、无 libc。
- 不更改 Mochi v14 指令编号或运行时 Value 语义。
- 未支持的堆对象 Opcode 应明确拒绝，绝不降级为 VM 执行。
- 这只验证受控收尾阶段的 Root / Mark / Sweep / Reuse；不能据此声称完成自动 GC。
