# AppForge（appforge）

- 定位：TechFunWay 家族的 **Go 应用底座库**，不是可部署应用；不监听端口、不打飞牛包、不发 Docker 镜像。
- 模块：`github.com/TechFunWay/appforge`，Go 1.26，公共 API 全在 `kit/` 下。
- 来源：2026-11-13 从 `smallgo/server` 基线分叉（21 个包、7,020 非测试行），除下表外代码原样保留；
  **`smallgo` 仓库本身不做任何改动**，9 个存量应用也不迁移。

## 相对 smallgo 基线的改动

| 位置 | 改动 | 原因 |
|---|---|---|
| `kit/version/identity.go` | 新增 `Identity` 与 `Configure` | 库不内置应用身份：端口、网关前缀、数据库文件名、发版仓库改由应用注入 |
| `kit/version/version.go` | `AppName` 默认值由 `smallgo` 改为 `appforge-app` | 同上；仍可用 ldflags 覆盖 |
| `kit/config/config.go` | 三处默认值改读 `version.Ident`；帮助文案参数化 | 去掉 8900、`/app/techfunway-smallgo`、`smallgo.db` 硬编码 |
| `kit/backup/backup.go` | 回退库文件名改用 `version.Ident.DBFileName()` | 同上 |
| `kit/update/update.go` | `CheckURL` 由身份推导，推不出时返回空串 | 库不该假装自己有发版仓库 |
| `kit/sysconfig` | `ConfigDef.DefaultFunc` + `DefaultValue()`，四处读取点改用 | **包 init() 早于应用 Configure**，`site_title` 会把占位身份写进数据库（已复现并修复） |
| `kit/manifest` + `cmd/gen-manifest` | 新增（2026-11-15） | 应用清单 `appmanifest.json` 做身份与打包元数据唯一真源；VERSION / app.json / fnpack/manifest 派生 + CI 一致性门禁 |
| `kit/qrcode` | 逻辑未改 | 保留为「怎么写业务模块」的范本 |
| `examples/minimal` | 新增 | 演示注入身份 + 空白导入注册模块 |
| 全仓 | `gofmt -w` | 基线有 3 个文件未过 gofmt，本仓库要求为零 |

## 常用命令

```bash
export GOCACHE=/tmp/gocache GOMODCACHE=/Users/<用户名>/go/pkg/mod   # 本机 HOME 被重定向，指到真实家目录的缓存
make vet && make build && make test
gofmt -l kit examples        # 必须为空
make run                     # 起 examples/minimal，默认 8080
```

## 约定

- 提交备注：全中文 `类型(AppForge)：备注`，两字类型，按父仓库 `commit-notes` 技能。
- 新增公共能力先判「业务无关吗」，无关才进 `kit/`，同时更新 README 包清单。
- 任何默认值涉及应用身份（名字/端口/前缀/仓库）一律走 `version.Identity`，不许写死。
- **应用元数据唯一真源是 `appmanifest.json`**（`kit/manifest`）：运行时 `MustParse().Inject()`，
  构建期 `cmd/gen-manifest` 派生 VERSION / app.json / fnpack/manifest。
  新字段进清单结构体与渲染器，禁止再开第四个身份声明入口。字段名大小写敏感（白名单反射生成），
  `encoding/json` 的大小写不敏感匹配已被 Parse 显式拦截。
- 默认值可能在 `Configure` 之前被读取的，一律用 `DefaultFunc` 延迟解析。
- `kit/*` 导出符号遵守 semver；破坏性变更必须升主版本并在 CHANGELOG 写明迁移方法。
- 本仓库不产飞牛包；CI 只做 gofmt / vet / build / test。
