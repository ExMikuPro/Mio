# 编辑器插件独立发布

- [mio-vscode](https://github.com/mio-kobo/mio-vscode)：`exmikupro.mio-language-support`，标签 `vscode-V<version>`。
- [mio-jetbrains](https://github.com/mio-kobo/mio-jetbrains)：`com.exmikupro.mio`，标签 `jetbrains-V<version>`。

每个插件仓库拥有自己的 CI、打包、发布历史守卫与 Marketplace Environment；真实发布默认禁用。请在相应仓库阅读 MIGRATION.md，完成 Dry Run、凭据与 Environment 保护审核后再启用。核心仓库不再发布编辑器插件。
