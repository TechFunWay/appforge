# AppForge

> 一个 Go import 就带上：账号与 JWT 登录、角色权限、系统配置、审计日志、数据库备份与上传恢复、
> 版本检查与应用自更新、实时通道（SSE）、定时任务、匿名使用统计、CSV/JSON 导出、二维码、上传、统一响应体与日志。
> 技术栈 Gin + GORM + SQLite + JWT，面向飞牛 fnOS 统一网关与 Docker 两种部署形态。

## 为什么要有它

TechFunWay 家族有 18 个自建应用，其中 9 个是 smallgo 框架实例。它们的 `server/go.mod` 模块名至今仍是
`smallgo/server`，框架层靠**复制**分发：2026-11-13 实测，22 个框架文件（3,407 行）在 9 个仓库里各存一份，
与基线同源的代码合计约 4.9 万行，占这 9 个应用后端代码的 **47%**。改一处公共逻辑要改 9 遍，
漂移靠 `check-sync.sh` 人工比对。

AppForge 把同一批能力改成**依赖**：应用只写业务包，公共部分 `go get` 进来。它从 smallgo 基线分叉而来，
**smallgo 与 9 个存量应用一律不迁移**，新应用从这里起步。

## 快速开始

```bash
mkdir -p notes/server && cd notes/server
go mod init notes/server
go get github.com/TechFunWay/appforge@latest
```

写完 `main.go` 之后**必须先 `go mod tidy`**：库的传递依赖（Gin、GORM 等）要由它写进本应用的 `go.mod`
与 `go.sum`，否则 `go build` 会报一串 `missing go.sum entry for module providing package`。

```go
package main

import (
	_ "embed"

	"github.com/TechFunWay/appforge/kit/config"
	"github.com/TechFunWay/appforge/kit/manifest"
	"github.com/TechFunWay/appforge/kit/server"

	_ "github.com/TechFunWay/appforge/kit/qrcode" // 空白导入即注册业务模块
)

//go:embed appmanifest.json
var manifestData []byte

func main() {
	// 1. 注入应用身份：appmanifest.json 是身份与打包元数据的唯一真源，
	//    端口、网关前缀、数据库文件名、发版仓库都由它推导。
	manifest.MustParse(manifestData).Inject()
	config.LoadDotEnv(".env")
	config.Parse()
	server.Start() // 迁移、路由、日志、调度器、优雅退出都在底座里
}
```

清单旁边放一份 `CHANGELOG.md`，构建期跑一次派生器，`VERSION`、`app.json`、
`fnpack/manifest` 三份打包元数据就永远和清单一致（脚手架已在 `make build`
与 CI 里挂了 `gen-manifest -check` 门禁）：

```bash
go run github.com/TechFunWay/appforge/cmd/gen-manifest -dir . -write
```

完整可跑示例见 `examples/minimal`：

```bash
go build -o app ./examples/minimal && ./app -port 8080 -data-dir ./data
curl -s localhost:8080/api/version
```

## 包清单

| 包 | 提供什么 |
|---|---|
| `kit/response` | 统一响应体 `{code, message, data}` |
| `kit/config` | 启动参数与环境变量（`-port`、`-data-dir`、`-fnos-app`、`-gateway-prefix` 等） |
| `kit/version` | 构建信息与应用身份（`Identity` / `Configure`；一般经 `kit/manifest` 注入） |
| `kit/manifest` | **应用清单唯一真源**：`appmanifest.json` 的解析、大小写敏感校验、身份注入与打包元数据派生 |
| `cmd/gen-manifest` | 构建期工具：从清单 + CHANGELOG.md 派生 `VERSION` / `app.json` 与**整个 fnpack 目录**（manifest、生命周期脚本、wizard、桌面入口、数据共享声明），`-check` 做一致性门禁 |
| `kit/logger` | 文件日志、保留策略、可选同时输出终端 |
| `kit/database` | GORM + SQLite（WAL、外键、单连接）、模型注册、版本化升降级、用户删除守卫与清理钩子 |
| `kit/auth` | JWT 签发与校验，TTL 跟随 `login_expiry_days` |
| `kit/user` | 账号、注册登录、角色、飞牛身份绑定、安全问题 |
| `kit/middleware` | 认证（Bearer / X-API-Key / 网关 Cookie）、限流、CORS |
| `kit/security` | 登录保护与安全问题 |
| `kit/sysconfig` | 运行期配置注册表（system/user 两级作用域、四种类型、internal 键不外泄） |
| `kit/audit` | 操作审计与导出 |
| `kit/backup` | `VACUUM INTO` 快照、自动备份、下载删除与**本地上传恢复** |
| `kit/update` | 版本检查（Gitee → GitHub 多渠道）与 fpk **自更新**（原子换装、回滚、re-exec） |
| `kit/scheduler` | 定时任务（panic 恢复、启停顺序） |
| `kit/realtime` | 每用户 SSE 推送 |
| `kit/stats` | 匿名使用统计与赞赏计数（可关） |
| `kit/upload` | 上传目录与静态服务 |
| `kit/utils` | 分页、CSV/JSON 导出（Excel 安全的 BOM 与时间戳文件名） |
| `kit/qrcode` | 二维码模块，同时是「怎么写一个业务模块」的范本 |
| `kit/apps` | 模块注册表：`Setup` / `Migrate` / `Shutdown` 生命周期 |

## 写业务模块

```go
func init() {
	apps.Register(apps.App{
		Name: "note", DisplayName: "笔记", RoutePrefix: "/note",
		// 三个钩子按需选：Setup 公开组、SetupAuth 认证组、SetupAdmin 仅管理员。
		// 认证与写操作审计由中间件负责，业务侧只管自己的数据。
		SetupAuth: func(api *gin.RouterGroup, db *gorm.DB) {
			g := api.Group("/notes")
			g.GET("", listNotes) // 当前用户取 c.GetUint("userID")
		},
	})
	database.RegisterModels(&Note{})
	scheduler.Register(scheduler.Job{Name: "note_clean", Interval: time.Hour, Run: clean})
}
```

飞牛统一网关注入、HttpOnly 会话 Cookie、账号密码登录绝不自动绑定飞牛身份等家族约束，底座已实现，应用侧无需重复处理。

## 从内网 Git 服务取代码（可选）

本仓库在 GitHub 公开，`go get github.com/TechFunWay/appforge` 开箱即用，**正常情况什么都不用配**。

只有两种情况需要一条地址改写：库还没公开的过渡期，或要在离线的内网 Git 服务上取代码。
把 `<gitea-host>` 换成你自己的内网地址（库自身与各应用的 import、`go.mod` 都不用改）：

```bash
go env -w GOPRIVATE=github.com/TechFunWay/* GOSUMDB=off GOPROXY=direct
git config --global url."ssh://git@<gitea-host>/org/appforge".insteadOf \
  "https://github.com/TechFunWay/appforge"
```

> 为什么不能把自建 Git 的带端口地址直接写成模块路径：Go 的模块路径**不允许带端口**，
> 形如 `192.0.2.10:9080/org/appforge` 会直接报 `malformed module path: invalid char ':'`。
所以路径固定用 GitHub 形式，物理来源靠 `insteadOf` 切换——开源前后应用侧代码零改动。


## 版本与兼容

- 语义化版本，tag 即版本；`v0.x` 期间允许破坏性变更，会在更新日志逐条写明。
- 应用侧最低 Go 1.26；SQLite 驱动 `mattn/go-sqlite3` 需要 `CGO_ENABLED=1`。
- 依赖面刻意保持小：Gin、GORM、golang-jwt/v5、uuid、x/crypto、go-qrcode。
- 任何带应用身份色彩的默认值都不许进 `kit/`，一律走 `version.Identity`。

## 与 smallgo 的关系

| | smallgo | AppForge |
|---|---|---|
| 定位 | 家族**模板**：复制出新应用 | 家族**依赖**：`go get` 进应用 |
| 存量 9 个应用 | 继续按它的基线维护 | 不要求迁移 |
| 新应用 | 不再从这里复制 | 从这里起步 |
| 公共能力演进 | `check-sync.sh` 人工比对漂移 | 编译期与 `go test` 直接暴露 |

## 许可

MIT，见 [LICENSE](LICENSE)。
