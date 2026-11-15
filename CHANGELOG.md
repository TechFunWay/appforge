# AppForge 更新日志

## v0.0.5

### 新增

- `kit/authz`：「模块:动作」逐格权限决策引擎（源自用药管理的落地模式）。
  能力矩阵与角色默认、显式勾选覆盖（空串=跟随默认、[]=显式零权限、脏数据不锁人）、
  超管角色恒全勾且削减无效、旧键展开补基础可见、gin 守卫 Require/RequireAll
  带中文能力标签的 403 文案。成员表与范围解析留在应用侧。

## v0.0.4

### 新增

- fnpack 全目录派生：17 个模板内嵌 `kit/manifest`（以 medcare 线上文件为金样，
  逐字节对照零漂移），`gen-manifest` 一次生成生命周期脚本、wizard、桌面入口与
  数据共享声明；清单新增可选 `intro` 字段。
- `scripts/golden-fnpack`：拿真实应用的 fnpack/ 目录校验模板等价性的金样程序。
- 脚手架完成指引更新：新应用手工区只剩前端与图标。

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
