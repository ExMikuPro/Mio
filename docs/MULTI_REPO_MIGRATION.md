# Mio Kōbō 多仓库迁移

核心基线：29746a65692c73d637510cbc3f4463e68a49d05c（Mio-2）。核心历史不重写，module Mio 不变。

编辑器当前源码移至 mio-kobo/mio-vscode、mio-kobo/mio-jetbrains，官方文档移至 mio-kobo/mio-docs。AOT、VM、Mochi v14、Bunko、LSP 与 selfhost 保留在此仓库；迁移不改变语言语义或 M17e/M18 Runtime。

旧工作树保留；新工作副本保留 M17e 未提交修改，迁移提交仅包含仓库结构、CI 与链接改动。LSP 旧工作树源码已在较新 Mio-2 合入，相同内容不重复应用。详细逐文件比对和未提交内容快照保存在本地迁移备份中。

核心 GitHub Transfer 尚待用户明确确认，完成后才更新旧、新工作副本 Remote；原 Issues、PR、Stars、Releases、Tags 通过原仓库 Transfer 保留。没有创建替代核心仓库、版本标签或正式 Release。
