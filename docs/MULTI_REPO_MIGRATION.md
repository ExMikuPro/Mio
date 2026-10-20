# Mio Kōbō 多仓库迁移

核心基线：29746a65692c73d637510cbc3f4463e68a49d05c（Mio-2）。核心历史不重写，module Mio 不变。

编辑器当前源码移至 mio-kobo/mio-vscode、mio-kobo/mio-jetbrains，官方文档移至 mio-kobo/mio-docs。AOT、VM、Mochi v14、Bunko、LSP 与 selfhost 保留在此仓库；迁移不改变语言语义或 M17e/M18 Runtime。

旧工作树保留；新工作副本保留 M17e 未提交修改，迁移提交仅包含仓库结构、CI 与链接改动。LSP 旧工作树源码已在较新 Mio-2 合入，相同内容不重复应用。详细逐文件比对和未提交内容快照保存在本地迁移备份中。

核心 GitHub Transfer 已于 2026-10-20 经用户确认完成：ExMikuPro/Mio → mio-kobo/mio。原仓库 ID 1412349226 保持不变，全部 refs、标签、Releases 与附件标识核对一致；公开可见性和 Mio-1.0.0 默认分支不变。旧、新核心副本 Remote 已更新，旧 LSP 工作树继续保留。迁移提交不包含用户尚未提交的 M17e/M18 开发内容。没有创建替代核心仓库、版本标签或正式 Release。
