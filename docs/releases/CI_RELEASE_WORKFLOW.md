# Tag-driven GitHub Release

唯一发布入口为 `.github/workflows/release.yml`。推送合法的 `v2.x.y` 创建正式 Release，`v2.x.y-rc.N`（N 为非零、无前导零的十进制整数）创建 Pre-release。其他版本格式被拒绝；非 `v2.*` Tag 不触发该流程。分支 push 和 PR 不获得发布权限。Core CI 的 push 限定为分支，避免同一 Tag 再启动另一套重复自举；PR 和手动 Core CI 保留。以后 v2.0.1、v2.1.0、v2.2.0 共用此流程。

## 版本与来源

`VERSION` 和 `cmd/mio/main.go` 的默认版本必须等于 Tag 的基础版本，例如 RC 和正式版均使用 `2.0.0`。构建时通过 Go `-X main.version` 写入实际 Tag 版本，RC CLI/LSP 显示 `2.0.0-rc.5`。发布说明从 `docs/releases/v<基础版本>.md` 获取，RC 和正式版共用该文件；新版本须先提交相应说明。运行从事件的完整 commit SHA 检出，tag push 时同时验证本地 Tag、GitHub 远端 Tag（含 annotated tag）和 workflow SHA；发布前再次读取远端 Tag，移动或删除均失败。

## 六个阻断 Job

1. **preflight**：检查干净 checkout、版本元数据、说明路径、Tag 与 commit、已存在 Release、工作流结构和发布策略测试。查询远端失败也阻断，不把 403/网络错误当成“没有发行”。
2. **test**：复用 `docs/TESTING.md` / M18 中实际命令：先运行 `check-native-bootstrap.sh` 从零生成 A/B/C/D 并强制验证 Decoder golden 不得跳过，再运行全仓 Go、AOT 全量、AOT race、vet、纯 Mio 全包与 AOT 子包、diff check。这样干净 checkout 不会因本地 golden 文件缺失而跳过必要差分。数组、Nullable、Closure、ABI、精确 GC、Shadow Frame、arena/大对象由这些既有 AOT 全量测试覆盖。全仓 30m 命令中耗时 VM `TestBootstrapSelfCompilation` 按既有期限条件跳过；不宣称执行该独立 VM 链，正式原生验收由下一 Job 负责。
3. **m18**：调用 `check-native-aot-fixed-point.py --gc-witness`，拒绝旧目录，从 Seed 执行 Stage 1–4、生成 Stage 5，验证 Mach-O、签名、纯 Mio 生成来源、固定点和 256 MiB 真 GC。正式堆保持 1 GiB，不加入历史 128 MiB OOM。额外核对已审核的 Seed/Stage/Mochi golden SHA、GC=1、Shadow Frame=0。普通及 race M18 三端差分分别使用独立目录，避免测试产生的文件冲突。
4. **package**：全新检出同一 commit，只下载本次 run/attempt 的 M18 artifact。重新核验来源清单、golden 和所有 signed/unsigned 摘要，然后用现有 `build-release.sh` 构建和签名，保留 RC4 目录布局。重新安装验证后生成 provenance。
5. **verify**：下载实际候选资产，重新校验摘要和 run/commit 身份；检查解压清单、逐文件摘要、ARM64 Mach-O、strict codesign、ad-hoc 类型和系统动态依赖。空 PATH、隔离 HOME、sandbox 禁读 checkout 且禁止 fork/其他 exec；执行 CLI/help/LSP、Hello、数组/Nullable/Closure、VM 和两条 AOT 的编译与真实执行。只将这些原始归档字节提升为 verified artifact。
6. **publish**：显式依赖以上所有 Job；只有 Tag push 可运行。重新验证 SHA、版本、commit、run、attempt 和远端 Tag。先检查同名 Release 不存在，再建草稿并上传全部资产，成功后公开；RC `prerelease=true`，正式版为 false。失败或取消不会自动删除草稿、覆盖资产或重建 Tag。重试应选择 Re-run all jobs；只重跑失败 Job 时，新的 attempt 不会复用上一 attempt 的上游资产，而会安全停止。若上传中断，管理员需先审查/处理残留草稿，再决定重跑；本流程不自动替换公开发行。

M18 golden 固定在 `scripts/release_automation.py`，来自已审核的 M18 记录，不是 RC4 归档摘要。编译器输入改变导致 golden 变化时必须审核新自举证据并显式更新 golden；不能自动接受新哈希。每次归档 SHA256 均重新计算。

## Runner、资源和权限

截至 2026-10-21，[GitHub 标准 Runner 文档](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)列出 `macos-15` 为 Apple Silicon ARM64（3 CPU / 7 GB RAM / 14 GB 存储）。所有构建/执行 Job 显式要求 Darwin、`uname -m = arm64`、至少 6 GiB RAM 和 6 GiB 可用磁盘，并检查 Xcode/LLDB；不符合即失败，不回退 Intel。M18 timeout 120 分钟；LLDB、sandbox、黄金字节或资源不足均阻断发布，不降低验收标准。

本地只读 GitHub API 审计确认 `mio-kobo/mio` 是 public、Actions enabled、allowed_actions=all；当前账号有管理权限。这不证明一次 hosted 作业能成功调度，也不证明未来策略、额度和 GITHUB_TOKEN 写权限不变。第一次 dry-run 需要核对实际 Runner Image、资源日志、LLDB 见证及仓库 Actions 策略；首次 Tag 发布还需验证 Publish Job 的实际权限。

[GitHub Release API](https://docs.github.com/en/rest/releases/releases#create-a-release) 当前限制：目标 commit 相对默认分支新增或修改 workflow 时，contents-only GITHUB_TOKEN 不能获得所需 Workflows 写权限。因此 Tag preflight 和 Publish 会比较目标 commit 与默认分支的 workflow 文件 blob SHA；不同即阻断，维护者须先把审核后的同版工作流合入默认分支。完整 dry-run 可以在待审核 ref 上执行，不会因此获得发布权限。

默认 `contents: read`，仅 Publish 为 `contents: write`，checkout 不保留凭据。发布工作流仅使用固定完整 SHA 的 GitHub 官方 Actions（2026-10-21 核对 checkout v7.0.1、setup-go v7.0.0、setup-python v7.0.0、upload-artifact v7.0.2、download-artifact v8.0.2 的官方 Tag 和 action.yml，均为 Node 24；download 的 artifact digest 不符默认失败），构建不使用跨 run cache，artifact 名绑定 run ID 和 attempt，下载默认限制在本次运行。Tag 和输入只通过环境变量/参数传递，严格解析，不拼接执行 shell。Token 只在查询和发布步骤注入，不输出。不使用 PR 特权事件、生产凭据或 Apple 证书。

目前提供带摘要的 JSON provenance，未启用 GitHub Artifact Attestation，因此不宣称已提供 OIDC 签名证明。以后如启用，须只给专用 Job `id-token: write` 和 `attestations: write`，并在 verify/publish 核验实际证明。

## 资产和签名

每版发布三个文件：

- `mio-<实际版本>-darwin-arm64.tar.gz`
- `SHA256SUMS`（归档的 SHA256）
- `release-provenance.json`（版本、Tag、完整 commit、run/attempt/workflow、构建时间和入口、OS/架构、Go/Python/Clang/LLDB、实际归档摘要、M18 golden/来源摘要/GC 见证、安装 smoke、签名类型及资产清单）

M18 全证据和二次安装报告保留为 Actions artifacts，默认 14 天，维护者应及时下载审查并按需要归档。三个发布资产保留在 GitHub Release。

当前签名为 **ad-hoc**，不是 Developer ID；strict codesign 不是苹果公证，包没有 notarization，Gatekeeper 可能限制安装或启动。没有伪造认证信息，也没有强制需要真实证书的步骤。未来身份签名应在归档前接入，再重新计算清单、摘要并重复安装验证。密钥、Apple ID、证书不得提交。

## 安全预演和维护者步骤

`release.yml` 的 `workflow_dispatch` 是完整非发布 dry-run：选择已推送的源码 ref，输入目标版本（如 `v2.0.0-rc.5`），无需提前创建 Tag。它运行同一套六阶段中的前五阶段，Publish 显式跳过，不请求写权限，不创建 Tag、Release 或远端发行资产；会生成 Actions artifacts。旧 `release-candidate.yml` 保留为静态策略检查入口，避免保留第二套构建/发布实现。dry-run 不替代真实 Tag 事件完整重验。

以下命令仅供维护者执行，本次实施没有执行任何 commit/push/tag/远端 workflow dispatch/release：

```sh
# 1. 审核全部原有与新增修改，显式选择文件；特别包括未跟踪 M18 源文件。
git status --short
git diff --check
git add -- <逐项审核的路径>
git diff --cached
git commit -m "Prepare Mio v2.0.0 tag-driven release"
git push origin <发布分支>
# 2. 该 workflow 必须先存在于默认分支，才能显示手动 dispatch 入口。
gh workflow run release.yml --ref <发布分支> -f tag=v2.0.0-rc.5
# 3. dry-run 全通过并审查 artifact / M18 / smoke 后，创建并推送 RC Tag。
git tag -a v2.0.0-rc.5 <审核后的完整commit> -m "Mio v2.0.0-rc.5"
git push origin refs/tags/v2.0.0-rc.5
# 4. 观察 Actions，核对自动 Pre-release 和全部三个资产。
gh run list --workflow release.yml
gh release download v2.0.0-rc.5 --dir <新的下载目录>
# 在下载目录执行：shasum -a 256 -c SHA256SUMS，并核对 provenance commit/run。
# 在新的 macOS ARM64 环境按 Release Notes 安装并复验 Gatekeeper/VM/双 AOT。
# 5. 审核最终 Release Notes、源码与安装体验；如有改动先重新提交/预演。
git tag -a v2.0.0 <最终审核commit> -m "Mio v2.0.0"
git push origin refs/tags/v2.0.0
# CI 从正式 Tag 全量重新构建验收并自动公开正式 Release；无需手动上传本地包。
```

版本 Tag 应指向已审核、已推送且完整包含全部 M18 输入的 commit。RC 和正式版可以指向不同 commit，但每次都重新测试和构建，不能直接重命名 RC4 本地产物为正式资产。

## 推荐 Tag Ruleset（仅管理员手动配置）

在 Settings → Rules → Rulesets 创建 **Tag ruleset**，匹配 `v2.*`，设为 Active。启用 Restrict creations、Restrict updates、Restrict deletions，只允许指定发布维护者/发布团队 bypass 创建；不要让普通写权限成员或任意自动化修改/删除既有 Tag。限制绕过成员到最小范围，并保留审计记录。可分开 RC `v2.*-rc.*` 与正式版规则设置不同创建者，但必须确认匹配重叠时的实际效果。流程本身从不移动或删除 Tag；Release 写权限不能替代 Tag 保护。另保护默认分支、要求源码/工作流审查和 CI 状态通过。管理员需按仓库计划确认规则可用性，本次未修改任何远端设置。

## 发布阻断条件

首次远端 dry-run 和首次真实 Tag CI 尚未执行，不能标为 PASS。源代码审查、完整提交、Hosted M18/LLDB 验收、实际安装/Gatekeeper 接受度及首次自动 Release 权限验证仍由维护者完成。所有必要步骤失败即阻断；不允许人工忽略失败后上传本地 RC4 代替。

## 本地维护验证

使用 Python 3.12+ 和 PyYAML 6.0.3：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts -p 'test_release_automation.py' -v
PYTHONDONTWRITEBYTECODE=1 python3 scripts/release_automation.py static
actionlint .github/workflows/*.yml   # 已安装 actionlint 时运行
# macOS ARM64，从新目录重建正式链与候选，绝不发布：
PYTHONDONTWRITEBYTECODE=1 bash scripts/build-release.sh --version 2.0.0-rc.5 --output build/<新的目录>
git diff --check
```

本轮本地证据在 `build/release-automation-audit/`；它来自保留用户改动的工作树，仅用于验证自动化逻辑，不是已推送 Tag 的正式来源声明，不得上传为正式资产。历史 RC4 及 M18 验收目录完整保留。构建过程中曾发现本机仅有 Command Line Tools：provenance 最初要求 `xcodebuild -version` 而失败，现改为记录实际 Clang/LLDB 版本后重新验证通过；没有伪造 Xcode、Developer ID 或公证结果。
