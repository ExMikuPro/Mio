# Mio 插件发布手册

两个插件独立发布，可信源码分支为 `Mio-2`。只有推送对应标签才会启动上传流程；普通分支 Push、PR、手动运行均不能触发这两个发布 Workflow。原有 `ci.yml`、`ci-jetbrains.yml`、`ci-vscode.yml` 保持独立。

| 插件 | 标签示例（大写 V） | Workflow | GitHub Environment | Marketplace 身份 |
| --- | --- | --- | --- | --- |
| JetBrains | `jetbrains-V0.6.2` | `.github/workflows/publish-jetbrains.yml` | `marketplace-jetbrains` | `com.exmikupro.mio`（数字 ID `35197`） |
| VS Code | `vscode-V0.3.1` | `.github/workflows/publish-vscode.yml` | `marketplace-vscode` | `exmikupro.mio-language-support` |

标签 Glob 仅负责路由，Job 另用完整 SemVer 2.0 校验：必须有 major.minor.patch，不允许数字前导零、换行、额外路径或小写 `v`。校验器支持合法预发布和 build metadata；本套流程面向 Stable，常规发布请使用表中的三段数字版本。不要把带预发布含义的标签当成测试渠道；VS Code 的版本还必须满足 Marketplace 自身规则。

工作流不会升级版本、提交、创建标签或 GitHub Release。构建前及审批后均从 origin 获取 `Mio-2`，要求标签指向当前 checkout 的提交，且该提交是 `origin/Mio-2` 的祖先（可以是经过审阅的历史提交，不强制最新 HEAD）。请同时保护 `Mio-2`，否则“属于该分支”不能代替代码审查。

## 一次性配置 GitHub

在 [ExMikuPro/Mio Settings → Environments](https://github.com/ExMikuPro/Mio/settings/environments) 分别创建上述两个 Environment，名称必须完全一致。必须在首次推送发布标签之前完成；仅在 YAML 中写 `environment` 不会自动建立审批规则，GitHub 自动创建的同名 Environment 可能没有保护。

分别配置：

1. Required reviewers：选择发布负责人或团队，建议启用 Prevent self-review，并禁止管理员绕过保护。是否支持审批取决于仓库可见性和 GitHub 套餐；若界面不可用，应先解决账户/仓库设置，不能把没有审批的 Environment 当成审批保护。
2. Deployment branches and tags：选择 **Selected branches and tags**。JetBrains 添加类型为 **Tag**、模式 `jetbrains-V*` 的规则；VS Code 添加类型为 **Tag**、模式 `vscode-V*` 的规则。不要添加任意分支或通配所有标签。这里仍是 Glob，严格 SemVer 由 Job 验证。
3. JetBrains Environment 的 Secrets 添加 `JETBRAINS_PUBLISH_TOKEN`；VS Code Environment 不需要 PAT。
4. 为仓库配置 Rulesets：保护 `Mio-2` 的合并和 `.github/` 修改；限制上述两类发布标签的创建者，并阻止标签更新/删除。Environment 不能阻止有权限的人修改 Workflow，应由仓库规则和审核共同保护。

每个发布 Job 使用独立的固定 concurrency group、`cancel-in-progress: false`，同一平台的上传不会因后续运行自动取消，两个平台彼此独立。GitHub concurrency 通常只保留一个等待运行，不是无限 FIFO 队列；应逐个发布并确认完成，再推送同一平台下一版本。不要在上传期间手动 Cancel。

## JetBrains Token 与签名

1. 以拥有现有 Mio 插件发布权限的账户登录 [JetBrains Marketplace](https://plugins.jetbrains.com/)。在个人账户的 **My Tokens** 页面创建用于插件上传的 Token，采用最短可用有效期及所需权限，保存到 `marketplace-jetbrains` 的 `JETBRAINS_PUBLISH_TOKEN`。
2. Workflow 仅在实际上传步骤将其映射为 `PUBLISH_TOKEN`，Gradle 的 `intellijPlatform.publishing.token` 从该环境变量读取；默认 `default` 为 Stable Channel。
3. 保留插件 XML ID `com.exmikupro.mio`。重复检查会确认 Marketplace 数字 ID `35197` 的 XML ID，分页检查历史版本，不能仅检查最新版。
4. 当前仓库没有自定义签名证书/私钥配置，本次没有生成、替换或上传任何签名身份。JetBrains Marketplace 自身签名与作者自有签名是不同环节。如果未来增加作者签名，必须由负责人继续使用原有身份，在构建阶段签名并验证，再对最终签名 ZIP 生成校验和。不要只在发布阶段替换 ZIP。当前发布命令排除 `buildPlugin` / `signPlugin`，直接上传已经检查的 Artifact；新增签名配置时必须同步调整构建产物选择，不能静默跳过签名。

参考：[官方发布和 Token 文档](https://plugins.jetbrains.com/docs/intellij/publishing-plugin.html)、[Gradle 发布配置](https://plugins.jetbrains.com/docs/intellij/tools-intellij-platform-gradle-plugin-extension.html#publishing)。

## VS Code Trusted Publishing Policy（用户手动完成）

使用已有 Publisher `exmikupro`，不要另建 Publisher 或修改插件名称。

1. 以 Publisher 管理者身份进入 [Visual Studio Marketplace 管理页](https://marketplace.visualstudio.com/manage/publishers/exmikupro)，找到 Trusted Publishing / Trusted Publishers 策略管理入口，为现有插件配置 GitHub Actions 信任策略。
2. 将策略绑定到 **Owner `ExMikuPro`、Repository `Mio`、Workflow 文件 `publish-vscode.yml`**（完整路径 `.github/workflows/publish-vscode.yml`，按界面字段要求填写）。不要填写 Workflow 的展示名称，也不要绑定到 `ci-vscode.yml` 或 JetBrains Workflow。
3. 如果策略支持 Environment 字段，填入 **`marketplace-vscode`**。该 Job 使用 Environment，因此 GitHub 默认 OIDC subject 为 `repo:ExMikuPro/Mio:environment:marketplace-vscode`，不是分支 subject；若策略要求 ref/workflow ref，则按策略说明限定对应 `refs/tags/vscode-V…` 工作流身份。Audience 为 `marketplace.visualstudio.com`。
4. 保存策略，核对它授权的是 `exmikupro.mio-language-support` 的更新。在尚未开放此功能或找不到策略入口的账户上，应联系 Marketplace 管理员/支持确认可用性；不能凭空创建 PAT 回退路径。
5. Workflow 发布 Job 已授予 `contents: read`、`id-token: write`；构建 Job 没有 OIDC 写权限。依赖锁定的 `@vscode/vsce` 4.x 支持 `publish --oidc`，发布命令只使用下载的已验证 VSIX。OIDC 获取或交换失败会使 Job 失败，不会重试成 PAT，也不需要 `vsce login`。

官方依据：[vsce Trusted publishing](https://github.com/microsoft/vscode-vsce#trusted-publishing)。Marketplace 界面可能随账户或功能开放情况变化，以实际策略字段为准。不要改成 Azure/PAT 流程来绕过失败。

## 升级版本（用户在正式发布前执行）

当前配置时源码版本仍是 JetBrains `0.6.1`、VS Code `0.3.0`。下面是下一版示例，**本轮未执行这些升级/提交/标签操作**。

JetBrains：手动将 `editors/jetbrains/build.gradle.kts` 的 `version` 改为 `0.6.2`，维护 changeNotes 和相关变更记录。校验通过 Gradle `printReleaseVersion` 获取实际求值后的 `project.version`，不会用文本正则猜版本或通过 `-Pversion` 覆盖版本。

VS Code：在 `editors/vscode` 运行：

```bash
npm version 0.3.1 --no-git-tag-version
```

同时维护 CHANGELOG；确认 `package.json.version`、`package-lock.json.version` 和 `package-lock.json.packages[""].version` 都为 `0.3.1`。该命令用于人工准备源码，不在 Workflow 中执行。

通过正常代码审核将版本修改和发布配置纳入 `Mio-2`，确认全部需要发布的源码（包括 MioCore LSP）已经入库。不能指望 Actions 访问本地尚未提交的 LSP 文件。

## 本地 Dry Run（不上传）

仓库根目录：

```bash
actionlint .github/workflows/publish-jetbrains.yml .github/workflows/publish-vscode.yml
bash -n .github/scripts/check-release-history.sh
python3 -B .github/scripts/test_plugin_release.py -v
```

Java 21 下运行 JetBrains（Linux 无显示环境用 `xvfb-run -a` 包裹 Gradle；macOS 直接运行）：

```bash
cd editors/jetbrains
bash ./gradlew --no-daemon -q printReleaseVersion
bash ./gradlew --no-daemon test buildPlugin verifyPluginStructure verifyPluginProjectConfiguration verifyPlugin
```

`verifyPlugin` 必须成功检查 IntelliJ IDEA **2025.3.2** 和 GoLand **2025.3.2**；第一次需要下载两个 IDE 和验证器，可能耗时较长。失败不得在正式发布 Workflow 中改成可选或 `continue-on-error`。

Node.js 24 和项目 Go 工具链下运行 VS Code：

```bash
cd editors/vscode
npm ci
npm run compile
npm run lint
MIO_RELEASE_TESTS=1 xvfb-run -a npm test
npm run package
npm run check
```

macOS 上去掉 `xvfb-run -a`。`pretest` 会从本仓库源码构建真实 Mio CLI，集成测试执行真实 MioCore LSP 诊断、诊断清除、语义导航及 CLI 回退。发布模式 `MIO_RELEASE_TESTS=1` 会禁止 pending/skip；不能用假 CLI、删除测试、跳过集成测试来修复失败。

回到仓库根目录后，使用与当前源码一致的标签字符串校验产物（仅字符串，不创建标签）：

```bash
python3 .github/scripts/plugin-release.py jetbrains jetbrains-V0.6.1 \
  --gradle-version 0.6.1 --archive editors/jetbrains/build/distributions/mio-language-support-0.6.1.zip --checksum
python3 .github/scripts/plugin-release.py vscode vscode-V0.3.0 \
  --archive editors/vscode/mio-language-support-0.3.0.vsix --checksum
```

升级后同步修改命令版本。JetBrains `--gradle-version` 应使用 `printReleaseVersion` 的实际输出。`--checksum` 在产物旁写入 SHA256 文件；Linux 使用 `sha256sum --check <文件>.sha256`，macOS 使用 `shasum -a 256 -c <文件>.sha256`，执行时先进入产物目录。

添加 `--unpublished` 只读取 Marketplace 元数据，不上传；已经发布的版本应失败。网络或响应结构错误也会失败。公开接口可能无法显示审核中、隐藏或已删除版本，最终 Marketplace 上传端仍会拒绝冲突；重跑前必须人工检查管理后台中的占用状态。不要使用 `--skip-duplicate` 掩盖重复。

本地 Dry Run 不会产生 GitHub Artifact 记录，也不会验证 Environment 审批或真实 OIDC 交换；这三项需要首次正式标签运行时由平台验证。GitHub Artifact 中包含 `plugin.zip`/`plugin.vsix` 和相应 `.sha256`，保留 30 天。

## 下一次正式发布操作

以下命令由用户确认平台设置、审核并推送 `Mio-2` 源码之后执行。先检查工作区并核对远端提交，避免把尚未提交的修改遗漏进发布：

```bash
git switch Mio-2
git fetch origin Mio-2
git status --short
git rev-parse HEAD origin/Mio-2
```

两个 SHA 应与计划发布提交一致；所有所需源文件应已提交。版本分别升级为 `0.6.2`、`0.3.1` 并完成检查后，只发布需要的平台：

```bash
# JetBrains 下一版；勿重复已有标签
git tag -a jetbrains-V0.6.2 origin/Mio-2 -m 'JetBrains 0.6.2'
git push origin refs/tags/jetbrains-V0.6.2

# VS Code 下一版；可单独执行
git tag -a vscode-V0.3.1 origin/Mio-2 -m 'VS Code 0.3.1'
git push origin refs/tags/vscode-V0.3.1
```

不要 `git push --tags`。标签指向的提交必须已包含对应 Workflow 及版本修改。JetBrains 标签不会启动 VS Code 发布，反之亦然。

## Actions 审批与结果确认

1. 在 [Actions](https://github.com/ExMikuPro/Mio/actions) 打开对应发布运行。build Job 会完整 checkout 标签源码并重新构建和测试，不依赖过去某个提交的绿色 CI。
2. build 全部成功后，下载 Artifact 核验 SHA256、插件 ID 和版本；检查两 IDE 验证报告或 VS Code LSP 测试输出。
3. publish 显示 Waiting 后，由 Environment reviewer 在 Review deployments 中审核对应标签、提交和产物，再批准。拒绝或未批准都不会上传。没有 Required reviewers 的 Environment 不会自动等待人工确认。
4. publish 会再次检查分支历史、源码/产物版本、SHA256 和重复版本，然后执行唯一上传步骤。JetBrains 使用 Java 21、Gradle Wrapper 和项目现有 IntelliJ Platform Gradle Plugin；VS Code 使用构建产物 `--packagePath`，不会重新打包。
5. Job 成功仅表示 Marketplace 接受上传，并不代表已公开可安装。继续在两个 Marketplace 的管理后台检查扫描、签名、人工审核、可见性和发布状态，再从公开详情页/IDE 检查版本真正可用。不要因搜索索引或 CDN 延迟立即重复上传。

## 发布失败排查

| 失败位置 | 检查与处理 |
| --- | --- |
| Workflow 没有触发 | 大写 `V`、插件前缀、标签是否推送到正确仓库，以及标签提交是否包含发布 Workflow。普通 Push/PR 不上传是预期行为。 |
| SemVer / version mismatch | 检查三段数字、前导零和所有源码/产物版本。修改源码并走新提交；不要在 Job 内改版本。 |
| history gate | 标签提交是否已合入 `Mio-2`；是否误打在其他分支。不要绕过 ancestry 检查或强推旧发布标签。 |
| JetBrains 测试或验证失败 | 查看错误和 `jetbrains-release-diagnostics`，区分源码/兼容性错误与 IDE 下载、代理、磁盘空间、JDK 问题；必须让两个 2025.3.2 目标均通过。 |
| VS Code 测试失败或出现 pending | 检查 Go 构建、已提交的 MioCore LSP 源码、VS Code 下载/显示依赖、真实服务器日志。保留 `MIO_RELEASE_TESTS=1`。 |
| Artifact / SHA256 | 产物必须唯一且来自本次 build；下载正确 Artifact，不要改文件或覆盖为本地包。过期则重新执行完整构建。 |
| Environment 阻止部署 | 检查 Tag 类型规则和审批人；不要放开到任意分支。 |
| JetBrains 401/403 | Token 有效期、插件发布权限、Environment 名称和 Secret 名称；只在 Environment 更换 Token，不打印 Secret。 |
| OIDC 获取/交换失败 | `id-token: write`、策略 Owner/Repo/Workflow/Environment、Audience、平台是否支持策略、Publisher 权限。修复策略后重试；禁止回退 PAT。 |
| Already published / duplicate | 先查后台是否已有版本或仍在审核。不要强制覆盖；确认失败发生在上传前才可重跑同一运行。 |
| 网络、TLS 或 Marketplace API 错误 | 保持失败关闭，排查网络、CA 与平台状态；不要关闭 TLS 验证，也不要把查询失败当成“版本不存在”。 |

## 下架、重发与凭据轮换

- 本流程不自动下架、删除版本或执行 `vsce unpublish`。下架影响现有用户更新和新用户安装，应由插件负责人在平台确认影响后手动操作。
- 已上传版本（包括待审核）不要再上传同版本。修复源码并增加版本，使用新标签；不要删掉标签再指向新提交，也不要假定删除 Marketplace 版本会释放版本号。需要恢复旧版本时，优先将修复内容作为新版本发布，必要时联系 Marketplace 支持。
- 工作流在上传前失败且后台确认没有该版本时，可重跑失败 Job；如果上传已成功但后续状态异常，先查后台，不能直接重跑。重复防护会拒绝已经可见的同版本。
- JetBrains Token 定期轮换：创建新 Token → 更新 Environment Secret → 验证配置 → 撤销旧 Token；泄漏时立即撤销并审查操作记录。作者签名证书/私钥独立管理，不随上传 Token 随意更换。
- VS Code 无长期 PAT，轮换重点是 Publisher 成员、GitHub 权限和 Trusted Publishing Policy；仓库转移、重命名、Workflow 文件改名或 Environment 变更后必须更新策略并重新审核。取消授权应删除/禁用对应策略，不要把 PAT 加回来。
