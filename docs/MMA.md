# Mio Module API v0.1

Mio Module API（MMA）为 Go 宿主提供显式的原生模块注册、静态检查、编译和执行边界。MMA v0.1 不包含动态插件发现、DLL/SO 加载、异步 FFI 或权限沙箱。

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

该方案不改变 Mochi 二进制布局，因此格式版本保持 v11，并继续读取现有旧版本。未被程序引用的原生函数由现有可达性裁剪移除；只有导入但未调用的模块不会改变生成的 Mochi。

## 值与生命周期

MMA v0.1 支持 `bool`、固定宽度整数、`int64`、`float32`、`float64`、`string` 和 `unit`。参数在同步调用期间有效，返回值通过 `CallContext.Return` 提交。边界值不暴露 VM Stack、Frame、Cell 或集合后备存储。

数组、结构体、枚举、Map、函数值和闭包暂不进入 MMA v0.1 原生边界。它们在普通 Mio 代码中的表示和生命周期不变，并继续由 Go GC 管理。因此 v0.1 不需要 Handle，也不建立第二套 GC。后续若允许原生模块跨调用保存引用值，应先增加受管理 Handle，而不是公开裸指针。

## 错误与线程

原生函数返回的错误会进入 Mio 的结构化运行时错误和调用栈。`mma.Invoke` 捕获原生函数 panic，避免 panic 越过 VM 错误边界。

原生调用是同步契约。`ModuleRegistry` 可并发读取和注册不同环境的模块，但同一 VM 不支持并发执行，也不允许原生函数从其他 goroutine 操作该 VM。MMA v0.1 不向模块公开任何 VM 可变状态。

## 稳定性

v0.1 稳定可用的接口：

- `ModuleRegistry`、`ModuleDefinition`、`FunctionDefinition`
- `NativeFunction`、`FunctionSignature`、基础 `Type`
- `CallContext`、`ValueAPI`、`ErrorAPI`
- `mio.Environment` 的源码编译、Mochi 解码和同步执行
- 模块路径、符号名和签名指纹组成的 Mochi 重绑定语义

实验性并留待后续版本的设计：

- 引用类型与受管理 Handle
- CapabilityAPI
- 异步调用、线程调度与取消传播
- 动态库 ABI、插件发现和热加载
