# Bunko 标准库

`bunko` 是 Mio 标准库的物理根目录，不属于公开导入路径。基础标准库使用 `kiso` 命名空间，因此基础数学包的公开路径固定为：

```mio
yobu "kiso/math";
```

`yobu "bunko/kiso/math";` 永远不是合法的标准库导入方式。

## 目录与包

- 一个目录对应一个包。
- 同一目录中的 `.mio` 文件共同组成包，加载顺序按文件名稳定排序，但声明不能依赖文件顺序。
- 子目录是独立包，不会被父包隐式包含。
- 包只在出现显式 `yobu` 时读取；普通程序不预载任何 Bunko 源码。
- `kiso/math` 不依赖 `kiso/mathx`。`mathx` 目录在有实际源码前不创建。
- 包加载器拒绝绝对路径、路径回退、物理 `bunko` 前缀和越过 Bunko 根目录的符号链接。

加载器已接入 `check`、`run` 和 `build`。入口既可以是单个 `.mio` 文件，也可以是目录；目录中的源码按文件名稳定读取，并共享包级声明。依赖按拓扑顺序合并，同一路径只编译一次，循环导入会在静态检查前被拒绝。加载器不包含 `kiso/math` 等包名特例。

## 声明可见性

标准库包遵循统一包规则：

- `koukai` 显式导出函数、类型和常量。
- 未使用 `koukai` 的声明仅在当前包内可见。
- `teisu` 声明编译期常量，不能赋值，也不产生运行时初始化代码。
- 标准库包不得通过顶层可执行语句产生隐式初始化副作用。

包成员使用目录名作为默认限定名，例如 `math.sqrt(...)`。跨包函数、泛型、结构体、枚举、闭包和 `teisu` 使用稳定的路径限定内部身份。只有 `koukai` 声明可以从包外访问。

## 当前前置依赖

| 能力 | 当前状态 | 阻塞内容 |
| --- | --- | --- |
| `yobu` | 已实现 | 显式加载、依赖图、循环检测和 CLI 接入 |
| 目录包 | 已实现 | 多文件声明共享和稳定合并 |
| `koukai` | 已实现 | 函数、类型和常量的包外可见性 |
| `teisu` | 已实现 | 纯常量求值、循环检测、折叠和零运行时初始化 |
| `float32` | 已实现 | binary32 类型、上下文字面量、转换、VM 与 Mochi v10 |
| 数值泛型 | 已实现 | `Number`、`Integer`、`Signed` 与 `Float` 约束；实例共享现有泛型函数字节码 |
| 数学错误 | 已实现 | 非 checked API 报运行时错误，checked API 返回 `mu` |
| 浮点底层原语 | 已实现 | 通过单一 `OpMath` 提供精度区分的底层入口 |

底层数学入口只在 Bunko 源码装载期间映射为内部符号，普通程序不能直接调用，也不会因未使用 `yobu` 而引入标准库代码。

`kiso/math` 已提供比较、绝对值、整数数学、浮点数学与数学常量 API，具体语义见 [`bunko/kiso/math/README.md`](../bunko/kiso/math/README.md)。

`kiso/bytes` 已提供 `[]uint8` 的比较、查找、前后缀与原地复制、填充 API，具体语义和动态数组前置限制见 [`bunko/kiso/bytes/README.md`](../bunko/kiso/bytes/README.md)。

`kiso/string` 已提供 UTF-8 字节转换、码点计数、查找、拼接与 `StringBuilder`，具体语义见 [`bunko/kiso/string/README.md`](../bunko/kiso/string/README.md)。

`kiso/convert` 已提供严格 ASCII 十进制整数、IEEE 754 浮点数转换和 Mio 布尔文本转换，具体语义见 [`bunko/kiso/convert/README.md`](../bunko/kiso/convert/README.md)。

`kiso/io` 已提供同步 `Reader`、`Writer`、部分读写结果和流式复制，具体语义见 [`bunko/kiso/io/README.md`](../bunko/kiso/io/README.md)。

`kiso/fs` 组合 MMA Resource Handle、宿主系统调用与 `kiso/io`，提供同步文件和目录基础能力，具体语义见 [`bunko/kiso/fs/README.md`](../bunko/kiso/fs/README.md)。只有显式导入文件系统的程序才会保留原生绑定；未使用模块不会初始化资源表槽位。

`kiso/path` 使用纯 Mio 实现 `/` 分隔的逻辑路径处理，具体平台边界见 [`bunko/kiso/path/README.md`](../bunko/kiso/path/README.md)。

`kiso/encoding/binary` 使用纯 Mio 实现固定宽度大小端整数和严格 LEB128，具体编码规范见 [`bunko/kiso/encoding/binary/README.md`](../bunko/kiso/encoding/binary/README.md)。

随附原生模块只负责宿主能力边界，不是可由用户直接依赖的稳定 Bunko API。v1 到 v2 保证公开 Mio 源码 API 的兼容，不保证 Go 原生适配实现保持不变。
