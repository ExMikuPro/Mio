# M18：纯 Mio Native AOT 自编译与原生机器码固定点

2026-10-20，Apple Silicon / Darwin ARM64。当前：**M18 PASS，全部九项验收条件均已满足**。

## 基线审计：M17e 与 M18 的区别

保留 `8b53a25` 和开始时全部工作树改动；快照见
`build/m18-20261020-audit/task-start.diff`、`task-start.status`。

M17e 的 Compiler A/B/C/D 是 `selfhost/bootstrap/bootstrap.mio` 及其依赖组成的
**Mio 源码→Mochi 编译器**。A 的 Mochi 来自 Go 前端；A 运行后生成 B Mochi，B 生成 C，C 生成 D。
其固定点是 B=C=D Mochi，黄金 SHA256 为
`b44a55bf3f9020e6b0ff83ad137b25774814a32dfe8acaf070abcd167bfd1465`。

两条已有构建路径应区分：

| 入口 | 原生 A/B/C 的生成者 | 实际原生程序输出 |
|---|---|---|
| `scripts/check-native-bootstrap.sh`（现有 CI） | 每一代均调用 Go `cmd/mochi-aot` | 下一代源码编译器 Mochi |
| M17e `TestM17eCompilerAProgressAfterClosures` | Go AOT 构建纯 Mio Builder Seed；Builder 的 Decoder/Lowering/Encoder/Writer 生成 A/B/C 镜像 | 下一代源码编译器 Mochi |

因此 M17e 的 B/C 未签名 Mach-O 一致是真实有效证据，但这些镜像没有在运行中输出下一代
AOT Mach-O。M18 直接复用 M17e 全部 backend/runtime，不重建 Decoder、assembler 或 writer。
M17e 最终结果和日志仍见 `docs/NATIVE_AOT_M17E.md`、
`build/m17e-heap-20261020/fixed-point.json`。

## 本次实现与正式来源链

新增 `selfhost/native-aot/main.mio`，完整导入现有：

- `bunko/kiso/encoding/mochi/`：读取、校验并解码 Mochi v14。
- `bunko/kiso/aot/lower/types/types.mio`：表示、可达性、Root、CFG 分析。
- `bunko/kiso/aot/lower/lower.mio` 和 `lower/nativefs/`：lowering、运行时与文件 syscall bridge。
- `bunko/kiso/aot/arm64/arm64.mio`：ARM64 编码、label/fixup、长短分支。
- `bunko/kiso/aot/macho/macho.mio`：AddressPatch 和最终 Mach-O 写入。

入口在**运行时**读取当前工作目录的 `input.config`、`output.config`（UTF-8 路径，无换行），
从 NativeFS 读取真实输入，调用 `lower.compile_with_heap(file, 1073741824, iie)`，
再调用 `macho.executable` 和 `fs.write_all`。每代输入均是同一份包含该入口及所有依赖的
`native-aot.mochi`；没有将每代路径烘焙进字节码。没有修改现有 backend/runtime 实现。

```text
Go 源码前端 → native-aot.mochi (v14)
Go AOT(native-aot.mochi) → Seed                 # 唯一正式链 Go AOT 调用
Seed(native-aot.mochi) → Native Stage 1         # Seed 执行纯 Mio AOT 实现
Native Stage 1(native-aot.mochi) → Native Stage 2
Native Stage 2(native-aot.mochi) → Native Stage 3
Native Stage 3(native-aot.mochi) → Native Stage 4
Native Stage 4(native-aot.mochi) → Native Stage 5 # 额外连续生成见证
```

Stage 1 起，其正在执行的机器码也由纯 Mio AOT 生成。各代实际读取自身的 Mochi；
这里验收的是用户指定的 Mochi→Native 自编译，不宣称本阶段另外完成了 Mio 源码前端的自举。

正式证据目录：`build/m18-20261020-acceptance/`。
`provenance.json` 保存完整 argv、cwd、输入输出路径、退出码、耗时、SHA256、Mach-O 结构和产物来源；
`source-manifest.json` 保存相关 Mio/Go 源码哈希；`SHA256SUMS` 保存全部代次摘要。
每条命令有单独的 `.stdout`、`.stderr`，每次运行的 config 和沙箱策略也留在目录中。

| 产物 | 生成者 | 未签名 bytes / machine words | 未签名 SHA256 | 执行 |
|---|---|---|---|---|
| Seed | Go AOT，仅此一次 | 2211840 / 505603 | `b92e4887ace8be54b76d62f29a708037eb8633edbb04876d0aade65e5ae0794d` | exit 0 → Stage 1 |
| Stage 1 | Seed 中的纯 Mio AOT | 2080768 / 516681 | `1fe684077b0e0aba6a9e8f73cb062b5f0b1e94e35f36da683cf0fc94d9fedb2d` | exit 0 → Stage 2 |
| Stage 2 | Native Stage 1 | 同 Stage 1 | 同 Stage 1 | exit 0 → Stage 3 |
| Stage 3 | Native Stage 2 | 同 Stage 1 | 同 Stage 1 | exit 0 → Stage 4 |
| Stage 4 | Native Stage 3 | 同 Stage 1 | 同 Stage 1 | exit 0 → Stage 5 |
| Stage 5 | Native Stage 4 | 同 Stage 1 | 同 Stage 1 | 已解析、签名及 strict verify；不计作已执行代次 |

共同输入 186933 bytes，Mochi v14，SHA256
`a07ee129a44938ab276806be30bc06a85a477ad74c0b4064489ed592b1b0da13`。
Stage 1=2=3=4=5 **全部原始字节一致**，比较由 Python 读取完整文件进行，另核对 SHA256。
不比较签名后哈希来判定固定点。Seed 与纯 Mio 产物不同是预期的后端实现差异。

## 防止隐藏后端、缓存和旧产物复用

`scripts/check-native-aot-fixed-point.py` 拒绝已有证据目录，生成前拒绝已有输出；
签名只修改原始镜像的独立副本，`.unsigned` 原样保留。每次调用前后检查输入哈希。
正式链只在 `build-seed` 步骤调用 `go-aot-seed-only`，之后所有生成都执行具体代次原生程序。

每个原生编译进程都在 `sandbox-exec` 下运行：禁止 `process-fork`，禁止 `process-exec`，
只放行本次启动的确切镜像路径，PATH 为空。允许签名和验证发生在编译进程之外。
策略实际经 `/bin/sh` 尝试 `exec /usr/bin/true` 的反向检查，必须非零退出；
日志 `sandbox-denial-probe.stderr` 记录拒绝。故 Go AOT、Clang、LLVM 和其他程序不能在
该原生生成进程中被启动。Python 不含代码生成，仅管理文件、解析结构、比较字节及调用签名工具。

Stage 4 另读取截断 v14，必须输出 `native-aot: decode failed`，且无目标镜像。
它还编译四类其他真实 Mochi，生成不同镜像并实际执行，排除固定返回自身缓存的伪实现。
入口目前用诊断和成功标记区分编译失败，失败分支可能仍以 OS exit 0 结束；
验收脚本同时检查退出码、**精确成功输出**及新产物，绝不把单独 exit 0 当成编译成功。

试跑记录保留在 `build/m18-20261020-audit/`、`build/m18-20261020-final/`。
第一次入口把 bool 顶层结果输出为 `hai` / `1`；改为 unit `run()` 入口后正式重跑。
这些试验也构建过 Seed，不属于上表正式生成链；从未在 Native Stage 1→后续代次调用 Go 后端。

## 确定性、布局与运行时验证

共同镜像 `__text` file offset=768、size=2066724（516681 words）；
`__const` offset=2067496、size=7330；`__TEXT` file size=2080768，16 KiB 对齐。
入口无全局变量，故本镜像不需要 `__DATA`；有 globals 的路径由既有回归覆盖。
全部 section VA 与所属 segment 的 file offset 映射相符；入口位于 text 起点；
所有 relocation 数目为零（assembler fixup 与 AddressPatch 在输出前完成），原始镜像没有代码签名。
每代（包括 Seed 和额外 Stage 5）均经独立结构检查、ad-hoc 签名和 `codesign --verify --strict`。

`internal/aot/m18_selfhost_test.go` 新增两项 opt-in 验收：

1. 对同一份 self Mochi 运行 VM 中的纯 Mio AOT，生成镜像与 Stage 2 完整字节比较；
   普通模式通过，17.6 秒左右；最终 race 模式 129.25 秒，输出 `native-aot: ok / 516681 / 2080768`。
2. 用真实 Stage 4 编译 Closure/共享 Cell、12 参数 ABI、Enum payload、Some(0)/None、
   紧凑 uint8 数组扩容/克隆/append、保留对象、2 MiB 文件读写及 missing 文件；
   输出镜像严格签名后执行，与 VM 和独立 Go Native（GCStress）逐字节比较 stdout。
   文件复制结果额外比较全部 2097152 bytes。四项全部通过。

独立 Go Native 语义参考和 M17e 回归会调用 Go AOT；这些调用属于**测试对照**，没有参与上表
任何 Native Stage 生成。VM 也是额外对照，不为正式链提供镜像。

M17e 原有精确 GC、Shadow Frame、nullable、Closure/Cell/Enum bitmap、typed Storage、
大对象/跨页、空闲合并/拆分/尾部回收、OOM 边界、imm19 远条件分支、imm26/短分支、
label/fixup/AddressPatch 和 NativeFS 错误路径全部保留并随全包/race 重跑。
没有放宽 Root 类型，没有关闭 GC，没有修改黄金哈希，没有调大测试超时解决问题。

## 本轮正式验证

| 检查 | 状态 / 证据 |
|---|---|
| Seed→Stage 1→2→3→4→5，字节固定点 | 通过；`m18-20261020-acceptance/provenance.json` |
| 各代 Mach-O 解析、strict codesign | 通过；同目录各 `*-verify.*`（正式链 6 个、GC 见证 3 个） |
| VM 自编译镜像确定性 + Stage 4 三端差分 | 常规通过（`m18-20261020-verified/m18-tests.log`）；最终额外 race 也通过，154.014s，见 `m18-20261020-acceptance/m18-tests-race.log` |
| 全仓 Go 测试（包含 AOT 全量） | 通过；`m18-20261020-audit/full.log`，AOT 203.350s、selfhost/codegen 286.226s |
| AOT 全量 race | 通过，主包 825.067s；`m18-20261020-audit/race.log`，无竞态报告 |
| go vet（包含新 M18 测试） | 通过；`m18-20261020-audit/vet-with-m18.log` |
| 纯 Mio 全包 / AOT 子包 | 74 / 18 项通过；`m18-20261020-audit/mio.log`、`mio-aot.log` |
| M17 纯 Mio Decoder 4 个黄金输入 VM/Native | 通过；`m18-20261020-audit/decoder-golden.log`，无 skip |
| whitespace | `git diff --check` 通过 |

全仓沿用既定测试规则跳过昂贵 VM 连续源码自举，没有声称另跑 60/80 分钟 VM 自举。
本轮不重跑 M17e 已验收的源码→Mochi A→B→C→D 链，也不覆盖它的黄金产物。

## 复现

在 macOS ARM64 仓库根目录执行；证据目录必须全新，不会自动清理旧目录：

```sh
python3 scripts/check-native-aot-fixed-point.py --evidence build/m18-new-run --gc-witness
MIO_M18_EVIDENCE_DIR="$PWD/build/m18-new-run" \
  go test ./internal/aot -run '^TestM18' -count=1 -v -timeout 30m
go test ./... -count=1 -timeout 30m
go test -race ./internal/aot/... -count=1 -timeout 30m
go vet ./...
go run ./cmd/mio test ./...
go run ./cmd/mio test ./bunko/kiso/aot/...
go test ./internal/aot -run '^TestM17PureMioDecoderGoldenVMAndNative$' -count=1 -v -timeout 30m
git diff --check
```

M18 opt-in 测试也拒绝重用其工作目录；如需重跑，先用新的证据目录重新执行脚本。
CI 保留已有 `bootstrap` job，新增 `native-fixed-point` matrix suite，单独跑上面的原生链与 M18
VM/三端差分并上传完整证据。没有代替原有 AOT、race、full 或源码 bootstrap 验证。

## 性能观察和边界

正式记录单次 wall：Seed→1 1.045s，1→2 0.884s，2→3 0.930s，3→4 0.828s，4→5 0.829s。
包含启动/沙箱和真实文件 I/O，不含签名；与 VM 同镜像比较约 18s。
这些是验收观察，不是 M19 性能基准。本阶段未做性能优化。

托管 arena 沿用 M17e 1 GiB 有界协议，每进程独立 mmap，NativeFS 句柄和映射随进程终止释放。

另由 `--gc-witness` 实际构建并运行 256 MiB 的 self compiler。来源为：
Stage 4 编译只将目标容量设为 256 MiB 的辅助 Builder Mochi → 辅助 Builder 原生运行，
读取原始 self Mochi → 输出具有 256 MiB runtime 的原始 self compiler → 该编译器真实执行
原始 self Mochi → 重建正式 1 GiB 固定点镜像。三个辅助镜像也都解析、签名和 strict verify。
辅助 Mochi 由 Go **源码前端**编码，之后三个原生生成步骤全部仍在禁止执行其他程序的沙箱下完成，
没有增加 Go AOT 调用；它们未替代正式 Stage 1–4 链。

LLDB 在实际加载后解析 __TEXT 地址，断在正常退出 syscall 前，读取 runtime metadata：
`gc_count=1`、`shadow_frame=0`、`exit_argument=0`，logical arena 恰为 268435456 bytes。
继续真实 exit 0；沙箱下和调试器下生成的镜像都与正式 Stage 2 完整字节一致。
证据为 `gc-observation.json`、`gc-debug.stdout`、`gc-compiler-to-fixed-point.*`。
1 GiB 正常运行另观察到 GC=0；没有把这个结果写成发生过 GC。

首次尝试 128 MiB 见证返回真实 OOM exit 2，未产生目标，记录保留在
`build/m18-20261020-verified/bounded-gc/`。正式协议原本就是 1 GiB；
额外压力验证选择低于正式容量的 256 MiB，得到一次真实 collect 后的完整输出。
没有改变正式容量或延长超时掩盖失败。

精确 GC 的压力/容量/Root 正确性以既有实机与 race 回归为证，不以多代 hash 一致替代。
一般 CLI argv、任意多态函数值、Map 等未声明语言面仍不自动纳入支持承诺。

M18 全部验收通过，无新的正式自编译阻塞。最终汇总：

1. Stage 1 实际生成 Stage 2；Stage 2 实际生成 Stage 3；Stage 3 实际生成 Stage 4。
2. Stage 2/3/4 原始 Mach-O 完整字节及 SHA256 一致；Stage 1、额外 Stage 5 也相同。
3. Seed、Stage 1–4 均真实执行 exit 0；Stage 4 额外生成 Stage 5，证明可继续生成。
4. 正式链全部镜像、GC 见证及 Stage 4 四个语义差分镜像 strict codesign 通过。
5. 核心生成全由纯 Mio Decoder/Lowering/ARM64 Encoder/Mach-O Writer 完成；正式 Go AOT 只生成 Seed。
6. GC/ABI/NativeFS/类型语义及三端差分通过；256 MiB 编译器自编译实测 GC=1，仍生成相同镜像。
7. 全仓测试、AOT 全量 race、M18 额外 race、go vet、纯 Mio 74+18 和 Decoder 黄金全部通过。
8. 脚本、CI、日志、源码清单、哈希与完整产物 provenance 已保存；汇总 `acceptance.json`。
9. 保留全部原有改动、CI 提交 `8b53a25`、黄金文件及本轮所有中间失败证据。

未 Commit、Push、Tag、Release、hard reset、git clean 或清理证据；未启动 M19。
