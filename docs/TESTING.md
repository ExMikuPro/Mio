# Mio 测试

Mio v1.0 使用与 Go 类似的同目录测试约定。文件名以 `_test.mio` 结尾时，它属于所在目录的同一个包，但只由 `mio test` 加载。普通 `check`、`run`、`build` 和包导入会忽略这些文件；直接对测试文件执行普通命令会提示改用 `mio test`。

## 测试函数

测试函数必须在 `_test.mio` 中声明，名称以 `test_` 开头，无参数，返回 `unit`，并且不能带类型参数。它不需要 `koukai`：测试文件与普通源码共享包作用域，因此可以访问同包私有声明。

```mio
yobu "kiso/testing";

waza test_add() -> unit {
    testing.expect(1 + 1 == 2, "addition should work");
}
```

测试文件按文件名排序，测试按声明顺序执行。测试函数及其调用的函数会作为编译可达性入口保留。

## kiso/testing

- `expect(condition: bool, message: string)`：条件为假时记录一次失败。
- `fail(message: string)`：无条件记录一次失败。

同一测试可以记录多次断言失败。自定义消息保持原文。断言不会通过 Go panic 中止 VM；运行时错误只终止当前测试，后续测试仍会执行。

测试报告接口是 `mio/testing/runtime.report(bool, string)`。它通过现有 MMA 注册，只存在于测试编译和执行环境中。每个测试都有独立记录器与 VM，不继承其他测试的 Globals、Array、Map 或闭包状态。普通程序不会注册该接口，也不会把 `kiso/testing` 加入 Mochi。

## 命令与退出码

```bash
mio test ./bunko/kiso/math
mio test ./...
mio test ./... --run=test_gcd
mio test ./bunko/kiso/bytes --format=json --lang=en-US
```

`./...` 递归发现含测试文件的包，并跳过隐藏目录、`build`、`dist`、`node_modules`、`vendor`、`cache` 和 `tmp`。包和测试均按稳定顺序串行执行。

`--run=<regexp>` 按测试函数名筛选执行。筛选发生在整个测试包完成发现、签名检查和编译之后，因此未选中的测试仍参与编译验证，但不会创建 VM；每个被选中的测试仍使用独立 VM。没有名称匹配时会给出明确提示，并保持退出码 `0`。JSON 的字段、状态和退出码规则不变。

- 所有测试通过或没有发现测试：退出码 `0`。
- 断言失败、运行时错误、资源限制或编译失败：退出码 `1`。
- 命令参数错误：退出码 `2`。

每个测试使用现有 VM 的指令预算、Value Stack 与调用深度限制。测试隔离是执行状态隔离，不是操作系统安全沙箱。

## CI 与 JSON

CI 可使用 `mio test ./... --format=json`。输出顶层包含稳定的 `schema_version`、`packages` 和 `summary`；测试状态使用 `passed`、`failed`，错误代码使用稳定的 `E0601` 至 `E0605`。字段名和状态值不会随输出语言变化。

当前字节码尚未保存完整源码映射，因此断言失败能报告测试文件与测试名，但不能可靠伪造具体调用行号。VM 运行时错误仍包含已有的字节码调用栈。

## Go 测试分级

仓库自身的 Go 测试分为三级：

```bash
# Fast：日常开发
go test -short ./... -count=1

# Full：完整确定性回归，自举单独验收
go test -skip='^TestBootstrapSelfCompilation$' ./... -count=1 -timeout=35m

# Stress：扩大固定种子的随机样本和大输入规模
MIO_STRESS_TEST=1 go test -skip='^TestBootstrapSelfCompilation$' ./... -count=1 -timeout=50m

# 发布候选：A → B → C → D 连续自编译
go test ./selfhost/bootstrap -run '^TestBootstrapSelfCompilation$' -count=1 -v -timeout=80m
```

Fast 保留普通单元测试、浮点与整数边界、最短浮点格式化回归、Unicode 错误、Mochi v1–v14、VM、Compiler、MMA、testing 和 i18n；它只跳过大规模随机浮点验证、超长浮点输入、大型 UTF-8 数据和大型文本。Full 运行除昂贵自举外的全部确定性正确性测试，随机测试使用固定种子。Stress 在 Full 基础上扩大随机浮点位模式、随机十进制输入、UTF-8 数据和文本规模。Bootstrap 单独验证 Compiler A → B → C → D 的编译、解码与执行，并要求 B、C、D 字节完全一致。

Race Detector 可与 Fast 或 Full 组合：

```bash
go test -race -short ./... -count=1
go test -race -skip='^(TestBootstrapSelfCompilation|TestCodegenSelfAndBunkoSources)$' ./... -count=1 -timeout=50m
```

完整 Race 跳过 `TestCodegenSelfAndBunkoSources` 是为了避免在 Race Detector 下重复极耗时的自解释 Codegen 工作；该测试仍由 Full / Stress 执行。A → B → C → D 也始终由独立 Bootstrap 门禁执行，不能用上述跳过项替代。
