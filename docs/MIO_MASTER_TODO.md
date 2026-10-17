# Mio · 全量 TODO / Master Roadmap

> **整理日期：2026-10-17**  
> **项目：** [ExMikuPro/Mio](https://github.com/ExMikuPro/Mio)  
> **定位：** 静态强类型、最小 Core、模块化 Bunko，兼顾**数学计算、GUI 与 Web**，最终实现独立 Native AOT + Mio Runtime + GC。  
> **资料范围：** 依据此前的设计讨论、阶段验收报告，以及本次核对的 GitHub `Mio-1.0.0`、`Mio-2` 和 `v1.0.0-beta.2` 状态整理。**这是愿望清单 + 实施路线图，不代表每一项都已成为最终语言规范。**

## 如何使用这份清单

- `[x]`：已经在对话中的验收报告确认，或在当前 GitHub 记录中确认的里程碑。
- `[ ]`：未完成、未验证或仍属于候选设计。
- **P0**：下一阶段阻塞项；**P1**：重要基础；**P2**：增强能力；**P3**：长期探索。
- **【已定方向】**：你明确选择过的架构或兼容要求；**【候选】**：我们讨论过、但尚未冻结为 Mio 规范的方案。
- 本文件不把 Go 实现的参考编译器、Mochi VM 的旧机制或过时的 v0.x 提案误当成 Mio v2 的最终设计。

## 0. 一页总览

| 主线 | 当前状态（2026-10-17） | 下一关 |
| --- | --- | --- |
| Mio v1 纯 Mio 编译器自举 | **完成**：A → B → C → D；B、C、D 字节一致 | v1 稳定版验收、文档收尾 |
| Mio v1 发布 | **GitHub Beta 2 已发布**：`v1.0.0-beta.2` | RC / 稳定版发布条件 |
| Mochi Native AOT | **M0 完成；M1 完成并已提交 `Mio-2`** | M2 原生 Globals/变量存储 |
| 原生 GC | **架构方向已确定，未实现** | Native TypeLayout、Root ABI、最小 Allocator |
| Native AOT 自举 | **未开始** | 逐步实现全部语言语义、Bunko 与 Native Runtime |
| C# 风格异步 | **愿望 / 设计待定** | `Task/Future`、调度器、`await` 语义设计 |
| Scala 风格隐式上下文 | **愿望 / 设计待定** | `given/using` 类上下文传递的解析规则 |
| IDE / LSP | JetBrains 插件已有；Marketplace 首次上传成功，公开状态待确认 | 纯 Mio LSP、VS Code 扩展 |
| 数学 / GUI / Web | **三类目标场景已确定** | 以真实负载制定标准库和 GC 性能要求 |

### 不可随意改变的设计原则

- [x] **【已定方向】Mio v1 → v2 的兼容承诺是：用户拥有 `.mio` 源码、仅依赖已承诺的 Bunko 公开 API，就应尽可能无痛重新编译。**
- [x] **【已定方向】用户代码与标准库公开契约优先；不承诺旧 Mochi 字节码、VM 内部表示、Go 宿主实现的二进制兼容。**
- [x] **【已定方向】不为了数学的极致性能强迫所有用户学习 Rust 式借用/所有权；Mio 保留自动 GC。**
- [x] **【已定方向】数学、GUI、Web 都是一等应用目标，不将 Mio 设计成只能做科学计算的语言。**
- [x] **【已定方向】保持 Core 精简，算法与高级类型优先通过 Bunko 提供（Minimal Core / Modular Bunko / Pay for What You Use）。**
- [x] **【已定方向】不同数值类型不做隐式转换，所有数值规则必须跨后端一致。**
- [x] **【已定方向】版本分支独立推进；每个版本达到验收条件才考虑变更 GitHub 默认分支，用户不希望依靠传统合并流程维护版本历史。**
- [x] **【已定方向】Tag 由你亲自创建、决定何时公开发布。** 自动 Release 工作流可以在 Tag 推送后执行。
- [x] **【已定方向】保留 Mio 与 Mochi 的角色形象：v2 移除 Mochi VM 执行依赖，不代表从项目中删除 Mochi 小团子。**
- [ ] **P0** 在 `docs/` 中正式冻结这些兼容承诺、语义边界及版本政策。

---

## 1. Mio v1 · 已完成的语言与自举基础

以下项目用于保留历史里程碑，不是要求重新开发。

### 1.1 编译器及语义

- [x] 词法分析 Lexer；UTF-8、字符串、数值、关键字、错误定位。
- [x] Pratt Parser、扁平 AST Arena、结构化 SourceSpan。
- [x] Resolver：多文件包、作用域、预声明、跨文件符号、捕获、可见性及循环依赖检查。
- [x] TypeChecker：Core 数值类型、可空、结构体、枚举、Map、泛型、闭包、Flow Narrowing。
- [x] Constant Evaluator：前向常量引用、依赖检测、常量运算、精确 float32 和 MinInt64 边界。
- [x] Codegen：闭包共享 Cell、泛型函数共享字节码、复合类型、MMA 调用。
- [x] Mochi v14 编码器、常量池与 Fixup；不随意改已有 Opcode 编号。
- [x] 自举真实编译入口：读取文件、稳定依赖排序、完整编译、Mochi 文件输出。
- [x] Go/Mio 差分、实际 Bunko 与 selfhost 自解析测试。
- [x] 自举 A → B → C → D：三代自编译产物 B/C/D 完全一致。

**自举历史基线：** B/C/D 均为 `120,707 B`，SHA-256：`b44a55bf3f9020e6b0ff83ad137b25774814a32dfe8acaf070abcd167bfd1465`；四代引导总耗时约 `20m44s`，单轮编译约 `183 亿` 条 Mochi VM 指令。该数据仅是当时特定机器与源码的基线。

### 1.2 语言当前的重要语义

- [x] `kaku`、`waza`、`moshi/sore`、`aida`、`kaesu`、`yobu`、`koukai`、`teisu` 等现有语法。
- [x] `int8/16/32/64`、`uint8/16/32`、`float32/64`、`bool`、`string`、`rune`、`unit`、`resource`。
- [x] `[]T` 共享引用语义；`Map<K,V>` 共享可变语义；结构体不可变值语义。
- [x] `T?`、枚举关联数据与穷尽匹配、函数值与共享闭包捕获。
- [x] 泛型与 `Number`、`Integer`、`Signed`、`Float` 等约束。
- [x] 显式数值转换与严格范围 / 溢出 / IEEE 754 测试。

### 1.3 Bunko / MMA / 测试工具

- [x] `kiso/math`、`array`、`bytes`、`utf8`、`string`、`convert`。
- [x] `kiso/io`（Reader、Writer、`read_full`、`write_all`、`copy_stream`）。
- [x] `kiso/fs`、`kiso/path`、`kiso/encoding/binary`。
- [x] `kiso/testing`：`expect`、`fail`、`mio test`、`--run` 过滤与隔离 VM。
- [x] MMA：`[]uint8` 受控借用/读写，不透明 `resource`、资源表、代次、关闭语义。
- [x] CLI human/short/json 诊断与 `zh-CN`、`ja-JP`、`en-US` i18n。
- [x] Fast / Full / Stress 与 Race 分层、Go 回归、Mochi v1–v14 兼容检查。
- [x] MIT 代码许可证已放入仓库；GitHub Beta 2 已发布。

---

## 2. Mio v1 · 发布与维护收尾

> 目标：让 v1 成为可长期获取、可验证、能为 v2 提供语义参考的稳定基线；不把 v2 功能反向塞进 v1。

- [x] GitHub `v1.0.0-beta.2` 预发布版已经出现；包含跨平台 CLI、Bunko、JetBrains 插件 ZIP 与 SHA256SUMS。
- [ ] **P0** 确认 v1 最终提交快照能够独立通过 Fast / Full / 必要 Race / Bootstrap。
- [ ] **P0** 发布稳定版前检查 Mochi v1–v14 解码、验证、反汇编与运行兼容性。
- [ ] **P0** 检查所有需要保护的未提交用户工作区修改；保持提交范围清晰、严禁无差别 `git add -A`。
- [ ] **P1** 更新仍停留在早期 v0.4 的语言文档，补齐实际的泛型、闭包、模块、常量与自举现状。
- [ ] **P1** README、CHANGELOG、版本字符串与 SDK 安装说明保持一致。
- [ ] **P1** 确认根 `LICENSE` 的代码授权与 `assets/mascot/` 角色插画的独立授权边界。
- [ ] **P1** 独立整理稳定版发布验收表：源码、安装包、CLI 版本号、Bunko 路径、哈希、三语言测试。
- [ ] **P1** 为 CI 的昂贵 Race / 自举测试设独立任务与合理超时，不因耗时自检拖死发布流程。
- [ ] **P1** 保留完整的 A→B→C→D 日志和可重复构建说明，不把哈希相同当作唯一验收标准。
- [ ] **P2** 整理 v1 编译器性能数据作为 v2 对比基线：冷编译、热编译、VM 指令、内存峰值、产物体积。
- [ ] **P2** 仅在收益明确时改进 v1 日常构建缓存；**不用大规模 VM 优化阻塞 v2**。
- [ ] **P1** 稳定候选版（RC）门槛通过之后，再由用户自行创建 `v1.0.0` Tag。

---

## 3. Mochi Native AOT · v2 主线（当前正在做）

**当前仓库事实：** 已有独立 `Mio-2` 分支，且 AOT M0、M1 在该分支存在提交。M0/M1 实机验证平台为 macOS Apple Silicon（M4 Pro）。**编译器目前先由 Go 编写，目标程序的执行不依赖 Go Runtime 或 Mochi VM。**

### 3.1 已完成

- [x] **M0：** 手工生成 ARM64 三条退出指令，封装 64 位 Mach-O，支持 `codesign`，实机 `exit(0)`。
- [x] **M0 装载问题：** 修正 `__LINKEDIT` 布局，补齐空的 `LC_SYMTAB` / `LC_DYSYMTAB`，解决 dyld 的 `EXC_BAD_ACCESS +0x48`。
- [x] **M1：** 翻译部分 Mochi `int64` 指令为 ARM64 原生机器码：`OpConstant`、`OpAdd`、`OpSub`、`OpMul`、`OpNeg`、`OpChant`、`OpPop`。
- [x] **M1 差分：** 未经过常量折叠的算术字节码，在 Mochi VM 与原生进程中输出一致（含负数、`MinInt64` 和回绕）；程序退出码为 `0`。
- [x] M0/M1 代码中文注释风格：中文说明意图，保留 ARM64、Mach-O、Opcode、ABI 等技术名词。

### 3.2 接下来最直接的 M2–M5

- [ ] **P0 / M2** `OpSetGlobal`、`OpGetGlobal`：真正的全局槽位与初始化顺序，拒绝越界和未初始化读取。
- [ ] **P0 / M2** 为 Globals 选择可靠表示：静态数据区 `.data/.bss` 或明确管理的内存块；设计重定位与 Mach-O Segment 扩展。
- [ ] **P0 / M2** 验证 `kaku x = 40; x = x + 10; chant(x);` 的真实存取，不允许把赋值优化成常量输出。
- [ ] **P1 / M2** 负数、重复赋值、遮蔽、最大槽位与跨表达式存活测试。
- [ ] **P0 / M3** `OpSetLocal` / `OpGetLocal`、栈帧布局与操作数栈超过 7 个值时的 spilling。
- [ ] **P0 / M3** 条件、比较、短路、`OpJump` / `OpJumpIfFalse` 和 Label/Fixup。
- [ ] **P0 / M3** 整数除法/取余：零除、MinInt64/−1、符号规则与 VM 差分。
- [ ] **P0 / M4** 函数 `OpCall`、参数传递、返回值、递归、互递归、调用深度与栈对齐。
- [ ] **P0 / M4** 依据 AArch64 ABI 冻结调用约定；正确保存调用者/被调用者寄存器。
- [ ] **P1 / M4** 处理闭包、捕获环境、共享 Cell 的 Native ABI 设计（实现可晚于基础函数）。
- [ ] **P1 / M5** Typed IR / Native IR：控制流图、类型、值生命周期、符号、重定位和目标指令选择。
- [ ] **P1 / M5** 将 Mach-O Writer、AArch64 Encoder、Label/Fixup、Stack Frame 与链接流程拆成清晰的独立模块。
- [ ] **P1 / M5** 完整边界验证：AArch64 immediate/branch 范围、页对齐、Mach-O 长度、签名、段权限与错误诊断。

### 3.3 AOT 中期与完整语义

- [ ] **P1** `float32/float64` 原生运算：准确舍入、NaN、Inf、±0、转换、比较、异常边界。
- [ ] **P1** 各整数宽度和有符号/无符号行为，保持 v1 已承诺的溢出与转换语义。
- [ ] **P1** `bool`、字符串、`rune`、可空类型和相应布局。
- [ ] **P1** 原生数组 `[]T`、Map、结构体、枚举、匹配及其值/引用语义。
- [ ] **P1** 泛型生成与共享/特化策略；在不改变用户语义前提下按数据测试选择。
- [ ] **P1** 原生函数值、闭包逃逸、共享 Cell、跨包调用与模块链接。
- [ ] **P1** Native Runtime 与受信平台接口：文件、stdio、时间、系统调用和资源 Handle。
- [ ] **P1** 接入原生 Allocator / GC、栈与寄存器的精确根追踪。
- [ ] **P1** Bunko 公共 API 在原生后端下按相同 Mio 源码重新编译并通过行为差分。
- [ ] **P1** Native Bootstrap：原生编译器 A → B → C，再对比确定性与语义；运行时不得依赖 Go/Mochi VM。
- [ ] **P2** 从 Go 参考实现逐步迁移 AOT 代码到纯 Mio；使用 Go/Mio 双实现做差分，不强求一次性重写。
- [ ] **P2** 目标扩展：Linux x86-64 ELF、Windows x64 PE/COFF、Linux ARM64 ELF；macOS x86-64 按实际需求考虑。
- [ ] **P2** 跨平台的构建、链接、调试信息与 SDK 打包。

### 3.4 原生优化（正确性完成之后）

- [ ] **P2** 寄存器分配、活跃性分析、合理 spilling、死代码消除、常量传播、内联与循环优化。
- [ ] **P2** CPU 自动向量化和手工 SIMD intrinsic（特别是 Apple Silicon NEON）。
- [ ] **P2** FMA、目标 CPU 特性探测与可重复浮点语义边界。
- [ ] **P2** 可选 PGO / 增量构建 / 链接优化，以真实基准决定引入顺序。
- [ ] **P3** 更先进的原生 IR、优化管线及交叉编译缓存。

---

## 4. Mio Native Runtime + GC（你和我亲手开发的主线）

> **【已定方向】第一代：单线程、精确、非移动、Stop-the-World Mark-Sweep。** 不强求 Rust 式所有权，也不在第一版追求 Go/Java 等成熟 GC 的复杂优化。

### 4.1 GC 与 AOT 共同契约（最优先）

- [ ] **P0** 设计 `NativeTypeLayout`：类型大小、对齐、字段偏移、引用字段与目标平台差异。
- [ ] **P0** 设计 Managed Object 表示：Header 或侧表、类型描述、Mark 位、对象大小、分配状态。
- [ ] **P0** 区分原生值类型、托管引用、内联包含引用的结构体与非托管资源。
- [ ] **P0** 设计 `RootManager`、Shadow Stack / 栈根槽、全局 Root、原生寄存器安全点契约。
- [ ] **P0** 固定 `Safepoint` 与可能触发分配/回收的函数调用规则。
- [ ] **P0** 定义 GC Runtime ABI：分配、类型描述、根登记、收集入口、错误和 OOM 行为。
- [ ] **P0** 确保 GC 自身在标记/清扫阶段不会意外递归触发 GC。

### 4.2 第一代 GC 正确性

- [ ] **P0** 实现最小 Allocator：系统内存页申请、对象对齐、空闲块、回收复用。
- [ ] **P0** `Mark`：从 Root 出发，使用显式工作队列遍历对象引用。
- [ ] **P0** `Sweep`：只回收不可达对象，重置标记，安全复用内存。
- [ ] **P0** 支持循环引用回收，正确保留从 Root 可达的对象。
- [ ] **P0** 完整扫描含引用的数组、结构体字段、枚举活动分支、Map、闭包环境及 Cell。
- [ ] **P0** 非托管 `resource` 保持显式 `close`；GC 不承担及时关闭 FD/Socket/GPU Buffer 的职责。
- [ ] **P0** `GC Stress Mode`：在可收集安全点频繁 GC，验证每一个活跃临时引用都能找到。
- [ ] **P0** 压测共享引用、嵌套容器、深对象图、循环、闭包逃逸、递归、错误退出、OOM 和地址复用。
- [ ] **P1** 自举源码、Bunko、编译器 AST 与大型整数数组作为 GC 集成测试负载。

### 4.3 三种应用负载：数学 / GUI / Web

- [ ] **P1** 默认 `Balanced`：作为一般程序的稳定基线，暂不暴露过多用户配置。
- [ ] **P1** `Compute`：连续非装箱数值数组、大缓冲区、减少扫描和总 GC 工作量。
- [ ] **P2** `Interactive`：为 GUI 帧响应预留短暂停顿机制，统计最大暂停与尾延迟。
- [ ] **P2** `Server`：Web 请求负载下考量吞吐、尾延迟、并发分配与内存限制。
- [ ] **P2** 统一 GC 核心 + 策略调参，而不是复制三套完全不同的垃圾回收器。
- [ ] **P2** 大对象区与小对象分配器分离；大数组仍计入 GC 内存预算。
- [ ] **P2** 记录对象大小分布、存活年龄、分配率、堆增长、吞吐量和暂停分布。
- [ ] **P3【候选】** 根据真实基准检验**弱分代假设**；有显著收益才加入 Young/Old、晋升与 Remembered Set。
- [ ] **P3【候选】** 并发标记/并行清扫、写屏障、增量回收、进一步降低 GUI/Web 尾延迟。
- [ ] **P3【候选】** 研究压缩/移动式 GC，仅在收益超过 ABI 与指针更新复杂度时引入。

### 4.4 GC 与多核

- [ ] **P1** 线程局部分配、线程登记和全线程 Safepoint 协作。
- [ ] **P1** 真正的多核并行执行，不使用全局解释器锁作为最终并行方案。
- [ ] **P2** 原子操作与内存序、同步原语、跨线程写入下的 GC 正确性。
- [ ] **P2** 安全管理外部原生调用对 Roots / GC Safepoint 的影响。

---

## 5. 新语言特性：你明确想加入的两大方向

### 5.1 C# 风格异步 `async/await`【明确愿望】

- [ ] **P1 / 设计** 定义 Mio 的异步语义，而不是只照搬 `async`、`await` 字面写法。
- [ ] **P1** 确定 `Task<T>` / `Future<T>`、异步 `unit`、完成/失败/取消状态。
- [ ] **P1** `await` 的类型检查、挂起点、局部变量生命周期、错误传播和结构化诊断。
- [ ] **P1** Compiler / IR 状态机转换；确保不破坏同步代码的现有调用规则。
- [ ] **P1** 调度器与事件循环：定时器、非阻塞 IO、唤醒、排队、公平性。
- [ ] **P1** `CancellationToken` 类取消传播、任务生命周期、资源清理。
- [ ] **P1** 异步文件/网络接口与已有 `kiso/io` Reader/Writer 的兼容桥接。
- [ ] **P2** 结构化并发、子任务组、任务失败聚合、超时与取消组合。
- [ ] **P2** 异步栈追踪、调试体验、LSP 语义支持及性能测试。
- [ ] **P2** 后续接入真正多核执行；不要把 `async` 和 OS 多线程误当成同一功能。

### 5.2 Scala 风格隐式上下文【明确愿望】

- [ ] **P1 / 设计** 定义类似 Scala `given/using` 的**显式可声明、调用处隐式传递**的上下文机制。
- [ ] **P1** 决定 Mio 风格命名和语法（目前尚未冻结关键字）。
- [ ] **P1** 词法作用域优先、候选解析优先级、跨包可见性与歧义报错。
- [ ] **P1** 编译期上下文查找与泛型约束、接口/能力参数的协作方式。
- [ ] **P1** 必须保持严格数值类型规则：**隐式上下文不等于自动进行数值隐式转换**。
- [ ] **P1** 不能将运行时动态全局状态伪装成静态上下文；要保证可预测、可调试。
- [ ] **P2** 针对日志、配置、Allocator、请求上下文、异步取消上下文提供示例。
- [ ] **P2** JetBrains / VS Code LSP 展示隐式传入参数及来源，避免“魔法”难以理解。
- [ ] **P2** 与 `async/await` 组合时的上下文传播、跨任务隔离和取消语义。

### 5.3 其他语义与类型设施

- [ ] **P1** 冻结 v2 基础 TypeLayout，不因为后端变化而改变数组共享、Map 可变或结构体值语义。
- [ ] **P1** 统一跨后端浮点、整数溢出、转换、`MinInt64`、NaN/±0 等边界验收。
- [ ] **P2【候选】** 评估公开 `uint64`、`usize`、原生指针宽度类型的必要性和安全边界。
- [ ] **P2【候选】** 更丰富的错误模型 / `Result<T,E>`、匹配与错误传播便利语法，优先保持兼容。
- [ ] **P2【候选】** 根据 Bunko 实际开发痛点改善接口、迭代、集合与泛型表达能力。
- [ ] **P3【候选】** 高级数值类型尽量先走 Bunko，不急于把任意精度、复数、矩阵、Decimal 扩成 Core 原语。

---

## 6. Bunko 标准库 · 后续扩充

### 6.1 现有库完善与跨后端兼容

- [ ] **P1** 让现有 11 个 `kiso/*` 包能以不改变用户调用方式的形式运行在 Native AOT / Runtime 上。
- [ ] **P1** 对 `kiso/io`、`fs`、`path`、`encoding/binary` 增加原生后端/平台边界回归。
- [ ] **P1** 文件系统 Handle、关闭检查、权限、路径、错误文本与资源泄漏检查跨平台一致。
- [ ] **P1** 公开 API 稳定性测试：比较 v1 VM 与 v2 原生后端的 Mio 用户源码行为。

### 6.2 开发工具及应用基础

- [ ] **P1** `kiso/json`：严格 JSON Parser / Encoder，供 LSP、Web、配置使用。
- [ ] **P1** 标准输入输出、进程参数、退出码、环境变量与受控系统 IO。
- [ ] **P1** `kiso/time`：时钟、时间戳、定时器、超时与跨平台抽象。
- [ ] **P1** `kiso/net`：Socket、TCP、UDP、DNS 等基础网络能力。
- [ ] **P1** 与原生 `resource` 协同的安全 Handle 生命周期和跨线程使用规则。
- [ ] **P2** `kiso/http`：HTTP 客户端 / 服务端、请求流、响应、超时和取消。
- [ ] **P2** 并发标准库：锁、原子、线程/任务、同步队列、取消与通道（接口形式待定）。
- [ ] **P2** 配置、日志、错误包装、序列化、集合/迭代器等通用工具模块。
- [ ] **P2** TLS / HTTPS 与证书验证，避免为了发布 Web 服务自行编写不安全的加密协议。

### 6.3 数学导向 Bunko（候选模块）

- [ ] **P1** 强化基础数学的准确性、可测试性、溢出/舍入边界与跨平台结果。
- [ ] **P1** 连续非装箱向量、矩阵与数值缓冲区，避免逐元素堆对象。
- [ ] **P2【候选】** 复数、矩阵、线性代数、统计、数值积分和随机数工具。
- [ ] **P2【候选】** FFT、卷积、DSP 与基础科学计算算法。
- [ ] **P2【候选】** SIMD / NEON 加速与纯 Mio fallback 做行为比较。
- [ ] **P3【候选】** BigInt、BigFloat、Rational、Decimal 与高精度数值格式。
- [ ] **P3【候选】** 向量化、并行数值计算、自动微分与更高级数学模块（以明确需求为前提）。

---

## 7. GUI / Web / GPU 三大应用生态

### 7.1 GUI（Mio 原生桌面程序）

- [ ] **P1** Native AOT 程序可正常使用事件循环、平台窗口、鼠标键盘 IO。
- [ ] **P1** GUI 生命周期和 GC 安全点协作；不允许 GUI 主线程被不可控长暂停频繁卡住。
- [ ] **P1** 窗口、图片、字体、纹理等外部资源使用显式关闭/释放。
- [ ] **P2** 设计稳定的 `kiso/gui` 或独立 GUI 包（具体名称待定）。
- [ ] **P2** 布局、控件、绘制、事件处理、资源打包、跨平台适配。
- [ ] **P2** 压测交互响应、渲染帧耗时与 GC P95/P99 暂停。

### 7.2 Web（Mio 服务端）

- [ ] **P1** 原生 Socket/HTTP、异步 IO、超时、取消、可靠的错误处理。
- [ ] **P1** Web 请求生命周期、连接池、资源关闭与内存限制。
- [ ] **P2** `Server` GC 策略：吞吐量与 P99 延迟平衡；公平处理并发请求。
- [ ] **P2** 路由、请求体流、JSON、WebSocket、TLS 与基础服务监控。
- [ ] **P2** HTTP 基准、长连接、并发压力、网络故障恢复与内存泄漏测试。

### 7.3 GPU / 并行数学（长期方向）

- [ ] **P3【候选】** 研究 GPU API 的可行性与工具链成本，暂不在 AOT M0–M4 阶段引入。
- [ ] **P3【候选】** M4 Pro 首个目标可考虑 Metal Compute；跨平台路线再评估 Vulkan Compute / 其他后端。
- [ ] **P3【候选】** Buffer、CPU/GPU 传输、显式资源生命周期与安全同步模型。
- [ ] **P3【候选】** CPU fallback、结果一致性、精度差异、基准及 GPU 错误隔离。
- [ ] **P3【候选】** 与数学库的向量、矩阵、卷积、FFT 接口协同。

---

## 8. IDE、LSP 与开发者体验

### 8.1 JetBrains · Mio Language Support

- [x] 语法高亮、PSI、补全、查找引用、跳转、Quick Documentation、诊断和 Quick Fix 的基础能力。
- [x] SDK/Bunko 配置、GitHub SDK 安装、Run Configuration 和新项目入口。
- [x] 插件 v0.6.1 的源码/发布流程、GitHub 插件 ZIP；JetBrains Marketplace 首次**上传成功**。
- [ ] **P1** 确认 Marketplace 官方审核公开状态，并在 GoLand / IntelliJ Marketplace 实机搜索安装。
- [ ] **P1** 插件 Logo 使用规范的 SVG `pluginIcon.svg`（深色版本按需要），宣传图与角色立绘单独展示。
- [ ] **P1** 完善插件文档、截图、更新日志和插件媒体页；保持代码 MIT 与素材授权边界清楚。
- [ ] **P1** 首次上架后配置 `publishPlugin` + GitHub Secrets 的自动更新工作流。
- [ ] **P2** 补全更复杂的泛型、闭包、跨文件定位和导入错误来源。
- [ ] **P2** 安全重命名、持久化索引、增量语义模型及性能优化。
- [ ] **P2** 原生调试器与 AOT 源码级断点（需要原生调试信息）。
- [ ] **P2** 用开发 GC/AOT 的真实过程评测插件：补全、跳转、诊断、构建/调试体验。

### 8.2 使用 Mio 自己实现的 LSP

- [ ] **P1** 为 CLI 规划 `mio lsp` 启动方式，服务端主要协议与语言分析逻辑使用 Mio 编写。
- [ ] **P1** 补齐标准输入/输出二进制流、长期运行服务、`kiso/json`。
- [ ] **P1** JSON-RPC 2.0 与 `Content-Length` framing、UTF-16 Position 变换。
- [ ] **P1** `initialize`、`shutdown`、`exit` 和打开/修改/关闭文档同步。
- [ ] **P1** 语法诊断、保存时检查、跨文件符号、Hover、Definition、Completion。
- [ ] **P1** 复用自举 Lexer/Parser/Resolver/TypeChecker；避免独立维护第二套语义规则。
- [ ] **P2** 增量分析、诊断防抖、文档版本缓存、取消旧请求；避免每次按键重编整个项目。
- [ ] **P2** References、Rename、Code Action、Semantic Tokens、Workspace Symbols。
- [ ] **P2** LSP 标准输出只允许协议数据，日志必须走 stderr/独立日志通道。
- [ ] **P2** JetBrains 与 VS Code 共享同一个 Mio LSP，编辑器端各保留自己的必要 UI 逻辑。

### 8.3 VS Code 扩展

- [ ] **P1** 建立 `editors/vscode/`，独立 `package.json` 与 VSIX 打包。
- [ ] **P1** TextMate Grammar、文件关联、括号、注释、主题、Mio PNG 图标。
- [ ] **P1** Mio CLI 的运行、构建、诊断、SDK 配置和 Bunko 路径。
- [ ] **P2** 接入 Mio LSP 的补全、跳转、Hover、诊断与引用。
- [ ] **P2** Visual Studio Marketplace Publisher、首次发布与后续自动发布。
- [ ] **P2** 独立 `vscode-v*` Tag；不与 `jetbrains-v*` 或编译器 `v*` 混用。

---

## 9. 编译速度、增量编译与工具链

- [ ] **P1** 按阶段记录 Native AOT 编译耗时、产物大小、CPU 指令/运行性能与内存占用。
- [ ] **P1** 对热点 `find_current_symbol`、`binding_of`、`node_type` 做基准与复杂度审计，优先解决明显的重复线性查找。
- [ ] **P1** 添加可靠的整构建缓存：包含源码、传递依赖、编译器版本、选项、平台、Bunko/MMA 签名。
- [ ] **P1** A→B→C 自举验收**必须可以完全禁用缓存**，避免缓存代替真正编译。
- [ ] **P2** 包级增量编译：依赖图、公开 API 指纹、失效传播、稳定产物链接。
- [ ] **P2** 细粒度函数级增量编译在 IR 与链接边界稳定后再讨论。
- [ ] **P2** `mio build` 的原生目标三元组、输出类型、调试符号、发行选项、错误定位。
- [ ] **P2** 编译期可重复构建、跨机器构建、缓存损坏检测和可观测性能日志。
- [ ] **P3** PGO、LTO 等高级优化，必须有实测收益且不破坏调试体验。

---

## 10. 真正多核、数据竞争检测与运行时安全

- [ ] **P1** 固定 Mio 并发内存模型：允许什么共享、原子/锁是什么语义、何为 data race。
- [ ] **P1** OS 线程与调度、同步原语、Safepoint、线程局部分配，与 GC 协作。
- [ ] **P2** `mio build -race`、`mio run -race`、`mio test -race`（未来 CLI 方案）。
- [ ] **P2** Native AOT 内存访问插桩；Shadow Memory、Happens-Before、线程同步事件。
- [ ] **P2** 检测器报告 Mio 源码行列、冲突读写、线程栈；支持最小复现。
- [ ] **P2** 数据竞争测试与 GC Stress 分开运行；正常发布构建不强制开启高成本插桩。
- [ ] **P2** 保持边界检查及不安全操作的受控入口，错误输入不能触发内存破坏。
- [ ] **P2** ASan/UBSan 一类原生问题检测的可行性研究（不等于 Go 的 `-race`）。
- [ ] **P3** 死锁检测、并发任务跟踪、可视化事件与性能分析工具。

---

## 11. 测试、CI、发布与项目治理

### 11.1 统一测试原则

- [ ] **P0** AOT 每支持一个新 Opcode，加入单元编码测试、Mach-O/ELF 结构检查、VM/AOT 行为差分及实机测试。
- [ ] **P0** 严格检查“不支持的 Opcode 必须报错”，不能偷偷降级为退出程序或调用 Go VM。
- [ ] **P0** 关键边界：除零、溢出、MinInt64、NaN、UTF-8、空指针、栈溢出、越界与 ABI。
- [ ] **P1** 保留 `go test -short`、普通 Full、Race Short、Stress / 大型差分和 Bootstrap 分级。
- [ ] **P1** 原生基准区分冷编译、热编译、常量折叠与未经折叠的真实 CPU 指令执行。
- [ ] **P1** Codegen / AOT 确定性：固定随机种子、两次编译哈希、语义差分与错误类别差分。
- [ ] **P1** 在 M4 Pro 上保存系统版本、CPU、执行结果、Mach-O 头/签名和可复现日志。
- [ ] **P1** 保持发布门槛：编译器、Bunko、MMA、IDE 插件及自举成果互不掩盖失败。

### 11.2 CI 与版本管理

- [x] `Mio-1.0.0` 默认分支与独立 `Mio-2` 开发分支已存在。
- [x] `v1.0.0-beta.2` 已发布到 GitHub（预发布）。
- [ ] **P1** CI 耗时长项独立：昂贵 Codegen Full、Race、Bootstrap 不要重复在普通 Push 中运行。
- [ ] **P1** 发布前 Race 覆盖范围说明清楚：大型超时用例转独立任务，不能误写为“所有测试均经过 Race”。
- [ ] **P1** 合理拆分不同平台 Runner；不要把 Linux `-race` 当作原生 Mio 并发验证。
- [ ] **P1** 版本分支达到标准再由你决定默认分支切换；留存历史，不强制合并。
- [ ] **P1** 代码、文档、插件、图片分清 Commit 范围；不使用破坏性 Git 操作误伤用户修改。
- [ ] **P1** 更新 GitHub Release / Marketplace 的分工：`v*` 编译器、`jetbrains-v*` JetBrains、`vscode-v*` VS Code。
- [ ] **P2** 正式稳定版的升级文档、版本迁移示例、SDK 包、Release Notes 与校验和。

---

## 12. 候选特性池（尚未承诺，避免冲垮 Core）

以下是根据你“所有想往 Mio 里塞的东西”及当前方向整理的**可能研究课题**。它们是探索项，不应直接当作确定需求。

- [ ] **P3【候选】** 性能敏感场景的非 GC Arena/Region 或内存池 API；保持普通代码默认自动 GC。
- [ ] **P3【候选】** `Result` 风格错误传播、更加方便的枚举匹配、迭代器 / 生成器。
- [ ] **P3【候选】** 特征开关、编译期能力、元编程边界（先确保可诊断、可增量分析）。
- [ ] **P3【候选】** 高精度浮点、复数/矩阵相关 Core 与 Bunko 的合理边界。
- [ ] **P3【候选】** 调度策略、工作窃取、无 GIL 并行任务执行与低延迟服务运行时。
- [ ] **P3【候选】** GPU 数值计算、科学计算生态、可选 FFI 与跨语言调用。
- [ ] **P3【候选】** 裸机 / `freestanding` 编译目标、操作系统开发能力（不是当前 v2 必达目标）。
- [ ] **P3【候选】** Native Debugger、Profiler、Trace、可视化 GC 与编译器性能分析器。

**筛选规则：** 先证明需求、语义、ABI 和可测试性，再决定是否进入 Core；能在 Bunko 或开发工具中实现，就尽量不要增加语言关键字或 Opcode。

---

## 13. 建议依赖顺序（不是强制版本承诺）

```text
v1 已自举 / Beta 发布
        │
        ├─ v1 稳定验收、文档、MIT/角色素材边界
        │
        └─ Mio-2 / Native AOT
            ├─ M0: exit(0) ✓
            ├─ M1: int64 算术 / 输出 ✓
            ├─ M2: Globals + 存储
            ├─ M3: Locals + 分支 + 控制流
            ├─ M4: Function Call + Stack Frame + ABI
            ├─ NativeTypeLayout / Safepoint / GC Root 契约
            ├─ Allocator + 非移动 Mark-Sweep GC
            ├─ 完整 Core / Bunko 原生化
            ├─ 从 Go 参考 AOT 逐步迁移到纯 Mio
            └─ Native Bootstrap A → B → C
                    │
                    ├─ Async/Await + 真正多核
                    ├─ Scala 风格隐式上下文
                    ├─ 统一 Mio LSP + JetBrains / VS Code
                    ├─ Compute / Interactive / Server GC 调优
                    └─ SIMD / GPU / GUI / Web 生态扩展
```

**注意：** `NativeTypeLayout/Root ABI` 可以与 M2–M4 并行设计，但完整 GC 的原生集成需要先有可用的调用约定、原生内存和 Safepoint。

## 14. 当前建议优先处理的 10 项

1. [ ] **P0** 从 `Mio-2` 的 M1 基线开始实现 **M2 `OpSetGlobal` / `OpGetGlobal`**。
2. [ ] **P0** 设计 Globals 的 Mach-O 数据布局与读写范围检查。
3. [ ] **P0** 在 M4 Pro 上做 VM/AOT 差分，证明变量**真正存储、读取和修改**。
4. [ ] **P0** 继续实现条件分支、跳转和局部栈帧；维护 Opcode 不变。
5. [ ] **P0** 同步起草 `NativeTypeLayout` / GC Root / Safepoint 的最小 ABI 文档。
6. [ ] **P1** 建立每个 AOT 里程碑的源码测试、指令反汇编与签名运行脚本。
7. [ ] **P1** 记录 M0/M1 的可重复构建记录，防止早期 Mach-O 崩溃再次出现。
8. [ ] **P1** 检查 JetBrains Marketplace 审核结果，并配置后续自动更新。
9. [ ] **P1** 完善 Mio v1 稳定发布所需文档、CI 和源码兼容契约。
10. [ ] **P1** 开始 `kiso/json` / stdio / Mio LSP 的接口设计，但不抢 AOT/GC 的核心开发优先级。

## 15. 完成的定义（Definition of Done）

一次里程碑只有同时符合下面条件，才打勾：

- 新实现的功能在**真实的对应平台**运行过，而不只是编译或静态验证。
- 与 v1 既有语义一致；若主动改变语义，有清晰的兼容决策与文档。
- 存在正常例子、非法输入、数值边界、资源限制和回归测试。
- 不支持的能力明确报错，不静默忽略、不偷偷调用旧 VM 替代实现。
- 修改不破坏其他版本分支，不误提交素材、缓存、凭据或无关文件。
- Commit / Tag / Release 的状态真实可追溯，**Tag 由你亲自操作**。

---

### 历史与后续维护

- **GitHub 仓库：** https://github.com/ExMikuPro/Mio
- **当前 v1 预发布：** https://github.com/ExMikuPro/Mio/releases/tag/v1.0.0-beta.2
- **v2 开发分支：** https://github.com/ExMikuPro/Mio/tree/Mio-2
- **M0 / M1 文档：** `docs/NATIVE_AOT_M0.md`、`docs/NATIVE_AOT_M1.md`
- **自举源码：** `selfhost/lexer/`、`parser/`、`resolver/`、`typecheck/`、`codegen/`、`bootstrap/`

> 这份清单适合保存在仓库根目录 `MIO_MASTER_TODO.md`（或移动到 `docs/ROADMAP.md`）。每完成一个里程碑更新日期、验收证据与勾选状态；任何新增设想先放入“候选特性池”，不要直接当成承诺。
