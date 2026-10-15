# Mio Module API v0.2

Mio Module API（MMA）为 Go 宿主提供显式的原生模块注册、静态检查、编译和执行边界。MMA v0.2 增加范围字节访问和通用不透明资源；仍不包含动态插件发现、DLL/SO 加载、异步 FFI 或权限沙箱。

## 接入示例

```go
registry := mma.NewModuleRegistry()
err := registry.RegisterModule(mma.ModuleDefinition{
    Path: "test/native",
    Functions: []mma.FunctionDefinition{{
        Name:   "add",
        Public: true,
        Signature: mma.FunctionSignature{
            Parameters: []mma.Type{mma.TypeInt64, mma.TypeInt64},
            Return:     mma.TypeInt64,
        },
        Function: func(call *mma.CallContext) error {
            leftValue, err := call.Argument(0)
            if err != nil {
                return err
            }
            rightValue, err := call.Argument(1)
            if err != nil {
                return err
            }
            left, err := mma.Values.AsInt64(leftValue)
            if err != nil {
                return err
            }
            right, err := mma.Values.AsInt64(rightValue)
            if err != nil {
                return err
            }
            return call.Return(mma.Values.Int64(left + right))
        },
    }},
})
if err != nil {
    return err
}

environment := mio.NewEnvironment(registry)
program, err := environment.CompileFile("main.mio", "bunko")
if err != nil {
    return err
}
return environment.Run(program, os.Stdout)
```

对应的 Mio 程序：

```csharp
yobu "test/native";
chant(native.add(10, 20));
```

注册表属于 `Environment`，没有进程级可变注册表。不同编译或执行环境可使用不同模块集合。

## 编译与 Mochi 绑定

导入器只为已导入原生模块生成签名声明。名称解析与静态类型检查继续使用普通函数流程，编译器继续生成 `OpCall`，不增加 Opcode。

原生函数在函数表中使用模块路径、公开符号名和签名 SHA-256 指纹组成的稳定标识。Mochi 不保存 Go 函数指针、闭包地址或 VM 指针。VM 在执行前解析一次标识并建立按 FunctionID 索引的绑定表；调用时不再做字符串查找。缺少模块、缺少公开符号或签名变化都会产生明确错误。

该方案不改变 Mochi 二进制布局，因此不会仅因资源签名升级 Mochi，并继续读取 v1 至 v14。未被程序引用的原生函数由现有可达性裁剪移除；只有导入但未调用的模块不会改变生成的 Mochi。

## 值与生命周期

MMA v0.2 支持 `bool`、固定宽度整数、`int64`、`float32`、`float64`、`string`、`[]uint8`、`resource` 和 `unit`。参数在同步调用期间有效，返回值通过 `CallContext.Return` 提交。边界值不暴露 VM Stack、Frame、Cell 或集合后备存储。

`[]uint8` 参数以调用期借用方式提供。`ReadBytes` 复制指定范围，`WriteBytes` 只写回已有范围；调用结束后保存的借用值失效。原生返回的字节数组拥有独立副本。其他数组、结构体、枚举、Map、函数值和闭包仍不进入 MMA 原生边界。

## Resource Handle

`resource` 是不能从 Mio 源码构造、转换或按整数访问的不透明类型。原生函数通过 `CallContext.CreateResource` 创建句柄，通过 `UseResource` 指定资源类型后使用，通过 `CloseResource` 显式关闭。

每个 Runtime 使用独立 `ResourceManager`：

- 句柄保存所属管理器、槽位和代次，不公开 Go 指针或整数 ID。
- 跨 Runtime 使用、资源类型不匹配、旧代次和无效句柄都会返回结构化 MMA 错误。
- Mio 值复制只复制句柄；所有别名共享同一槽位和关闭状态。
- 关闭先使槽位失效，再调用宿主清理函数；重复关闭返回 `closed_resource`。
- 槽位复用会递增代次，旧句柄不能指向新资源。
- VM 执行结束调用 `Cleanup` 关闭遗留资源；`Environment.Close` 调用 `Shutdown` 并拒绝后续创建。
- `ResourceManager` 的 Go finalizer 只作为遗漏清理的兜底，不代替显式 `close`。

资源载荷及关闭函数只存在于宿主资源表中。Mio 侧的 `resource` 不提供反射、字段访问、相等比较或序列化为 Mochi 常量的能力。该机制可供文件、Socket 或数据库连接复用，但本版本只随附文件系统适配。

## 错误与线程

原生函数返回的错误会进入 Mio 的结构化运行时错误和调用栈。`mma.Invoke` 捕获原生函数 panic，避免 panic 越过 VM 错误边界。

原生调用是同步契约。`ModuleRegistry` 可并发读取和注册不同环境的模块，`ResourceManager` 对创建、检查、关闭和清理加锁；同一 VM 不支持并发执行，也不允许原生函数从其他 goroutine 操作该 VM。MMA 不向模块公开任何 VM 可变状态。

## 稳定性

v0.2 稳定可用的接口：

- `ModuleRegistry`、`ModuleDefinition`、`FunctionDefinition`
- `NativeFunction`、`FunctionSignature`、基础 `Type`
- `CallContext`、`ValueAPI`、`ErrorAPI`
- `ResourceManager`、`Resource` 和 `CallContext` 的资源操作
- `mio.Environment` 的源码编译、Mochi 解码和同步执行
- 模块路径、符号名和签名指纹组成的 Mochi 重绑定语义

实验性并留待后续版本的设计：

- CapabilityAPI
- 异步调用、线程调度与取消传播
- 动态库 ABI、插件发现和热加载

`mio/fs/runtime` 等随附原生模块路径是实现细节，不属于冻结的宿主 ABI。v1 到 v2 的兼容目标是 Mio 用户源码及稳定 Bunko 公共 API，而不是当前 Go 适配函数保持不变。
