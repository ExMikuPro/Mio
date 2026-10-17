# Mio v1.0.0 正式发布候选检查清单

状态基线：`v1.0.0-beta.2` / `c0e6a6b`。本清单记录 2026-10-17 已取得的证据；`[x]` 只表示对应范围已经实际验证，不表示 `v1.0.0` 已发布。最终候选提交若包含代码、字节码、Bunko、MMA、SDK 或插件变更，必须重新执行受影响的门禁。

## 冻结与兼容性

- [x] Mio v1 已进入 Feature Freeze；本轮不增加关键字、语法、核心类型、Opcode、字节码版本、GC、Native AOT、异步或协程。
- [x] 当前最高 Mochi 写入格式为 v14，自动测试覆盖 v1–v14 解码、结构验证、非法/截断输入拒绝与反汇编。
- [x] 现有测试覆盖 MinInt64、固定宽度整数回绕、`float32` 舍入、Unicode / 非法 UTF-8、泛型约束、闭包捕获、多包编译与失败诊断。
- [x] Bunko 公开导入路径保持 `kiso/*`，当前发布候选没有破坏性 API 变更。
- [x] Core、Bunko、Mochi 与 MMA 的版本边界已在 README、CHANGELOG 与各自文档中说明。

## 测试门禁

- [x] Fast：`go test -short ./... -count=1` 在本地通过。
- [x] Fast Race：`go test -race -short ./... -count=1` 在本地通过。
- [x] Vet：`go vet ./...` 在本地通过。
- [x] Full：beta.2 Tag 的 GitHub Actions `Full Regression` 通过。
- [x] Stress：beta.2 Tag 的 GitHub Actions `Stress tests` 通过。
- [x] Full Race：beta.2 Tag 的 GitHub Actions 通过；耗时 Codegen 自检由 Full / Stress 覆盖，Bootstrap 独立执行。
- [x] A → B → C → D Bootstrap：beta.2 Tag 的独立 Job 通过，B、C、D 字节完全一致并通过 Mochi 解码、验证与执行。
- [x] Bunko：beta.2 macOS ARM64 SDK 在干净解压目录运行 `mio test ./...`，10 个测试通过。
- [x] MMA：本地 Fast 与 Fast Race 覆盖根包 MMA 回归并通过。
- [x] `git diff --check` 在本轮最终变更上通过。

## SDK 与编辑器

- [x] Release 工作流交叉编译 Linux x64/ARM64、macOS Intel/Apple Silicon 与 Windows x64 SDK。
- [x] SDK 打包逻辑包含 CLI、Bunko、README 与 MIT LICENSE，并在 Linux x64 构建上核对 `mio version`。
- [x] beta.2 macOS ARM64 SDK 已独立下载、校验 SHA-256、解压并运行版本与 Bunko 测试。
- [ ] Linux x64/ARM64、macOS Intel 与 Windows x64 的 beta.2 SDK 尚未在对应平台完成本轮实机安装测试。
- [x] JetBrains 插件 0.6.1 的 beta.2 CI 已通过测试、`buildPlugin`、`verifyPlugin` 与 ZIP 完整性检查。
- [x] beta.2 Release 中的插件 ZIP 已独立校验 SHA-256 与 ZIP 结构。
- [ ] 插件 ZIP 尚未在 IntelliJ IDEA 2025.3.2 与 GoLand 2025.3.2 中完成人工安装和基础功能冒烟。

## 文档、许可与发布资产

- [x] README 的源码安装、快速开始、CLI 命令、Bunko 路径和 Go VM / 自举关系已核对。
- [x] CHANGELOG 已补充 beta.2 相对 beta.1 的实际变化与 v1.0.0 发布说明草稿。
- [x] `docs/LANGUAGE.md`、`MOCHI.md`、`BUNKO.md`、`MMA.md`、`TESTING.md` 与 `I18N.md` 已核对版本范围和现有行为。
- [x] 根目录 MIT LICENSE 存在；README 明确代码许可证不自动覆盖角色美术。
- [ ] Mio / Mochi 角色图片、插件宣传图与插件图标的权利人、发布授权和再分发条款仍待人工确认并留档。
- [x] beta.2 Release 包含五个平台 SDK、JetBrains 插件 ZIP 与 `SHA256SUMS.txt`；资产命名符合工作流约定。
- [x] beta.2 的已下载 SDK 与插件资产通过 `SHA256SUMS.txt` 校验。
- [ ] 正式版 Release Notes 尚待最终人工审核。

## 已知限制

- [x] 已明确 v1 自举编译器和用户 Mochi 程序仍由 Go 实现的 VM 执行；Native AOT、独立 Runtime 与 GC 不属于 v1。
- [x] 已明确 Mochi 文件本身不含校验和或数字签名，CLI 也未开放指令预算参数。
- [x] 已明确 LSP、VS Code 扩展、Debugger、JIT、异步与协程不属于 v1。

## 正式发布授权

- [ ] 对最终候选提交重新执行所有必需门禁，并确认结果对应同一 Commit / Tag。
- [ ] 完成跨平台实机与 JetBrains 人工抽样，或由发布负责人书面接受剩余平台风险。
- [ ] 完成角色美术与插件图像授权确认。
- [ ] 发布负责人完成最终变更、版本号、Release Notes 与资产清单审核。
- [ ] 经明确授权后创建签名或受保护的 `v1.0.0` Tag。
- [ ] 经明确授权后发布 GitHub Stable Release；不得在授权前自动执行。
