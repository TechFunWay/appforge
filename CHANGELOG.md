# AppForge 更新日志

## v0.0.3

### 新增

- `kit/manifest`：应用清单（`appmanifest.json`）成为身份与打包元数据的唯一真源。
  解析、大小写敏感字段白名单、一次性全量校验、`Inject()` 注入 kit/version 身份体系；
  派生器输出与家族存量应用逐字节兼容的 VERSION / app.json / fnpack/manifest。
- `cmd/gen-manifest`：构建期派生与一致性门禁（`-write` 生成、`-check` 供 CI 断言）；
  更新日志从 CHANGELOG.md 的 `## vX.Y.Z` 小节提取进 fnpack changelog 块。
- 脚手架生成的新应用直接带 `server/appmanifest.json`，`make build` 与 CI 默认挂门禁。

## v0.0.2

- 脚手架业务路由改挂认证组。

## v0.0.1

- 首个可用底座：从 smallgo 基线抽离 21 个 kit 包，应用身份改由 `version.Identity` 注入。
