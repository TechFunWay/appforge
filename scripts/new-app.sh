#!/usr/bin/env bash
# new-app.sh —— 生成一个依赖 AppForge 底座的新应用骨架。
#
# 与 smallgo/scripts/new-app.sh 的区别：那个把框架复制一份当模板，这个只写业务代码，
# 公共能力通过 go.mod 引用 appforge。
#
# 用法：
#   ./scripts/new-app.sh <仓库名> "<显示名>" [默认端口] [生成目录]
# 例：
#   ./scripts/new-app.sh notes "我的笔记" 8914
#
# 生成后自动跑 go mod tidy + go build 自检，失败就明确报错，不留半成品。
set -euo pipefail

MOD=github.com/TechFunWay/appforge
LIB_VERSION=v0.0.1

if [[ $# -lt 2 ]]; then
  echo "用法: $0 仓库名 显示名 [默认端口] [生成目录]" >&2
  exit 1
fi

NAME=$1
DISPLAY=$2
PORT=${3:-8914}
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT=${4:-"$(dirname "$HERE")/$NAME"}

[[ $NAME =~ ^[a-z][a-z0-9-]{1,30}$ ]] || { echo "仓库名只能是小写字母、数字与连字符且以字母开头: $NAME" >&2; exit 1; }
[[ $PORT =~ ^[0-9]+$ ]] || { echo "端口必须是数字: $PORT" >&2; exit 1; }
[[ -e $OUT ]] && { echo "目标目录已存在，不覆盖: $OUT" >&2; exit 1; }

PKG=${NAME//-/}
PKG=${PKG//./}
mkdir -p "$OUT/server/$PKG" "$OUT/tmp" "$OUT/.gitea/workflows"
: > "$OUT/tmp/.gitkeep"
echo "==> 生成 $OUT（Go 包名 $PKG，默认端口 $PORT）"

# tpl 用占位符做替换：模板一律用带引号的 heredoc 写，Go 结构体标签里的反引号才不会被 shell 吃掉。
tpl() {
  sed -e "s|@NAME@|$NAME|g" -e "s|@PKG@|$PKG|g" -e "s|@DISPLAY@|$DISPLAY|g" \
      -e "s|@PORT@|$PORT|g" -e "s|@MOD@|$MOD|g" -e "s|@LIB@|$LIB_VERSION|g"
}

tpl > "$OUT/server/go.mod" <<"GOSRC"
module @NAME@/server

go 1.26.5

require (
    @MOD@ @LIB@
    github.com/gin-gonic/gin v1.12.0
    gorm.io/gorm v1.31.1
)
GOSRC

tpl > "$OUT/server/main.go" <<"GOSRC"
package main

import (
    // 空白导入即注册：业务包在自己的 init() 里把模型、配置项与路由挂进底座。
    _ "@NAME@/server/@PKG@"

    "@MOD@/kit/config"
    "@MOD@/kit/server"
    "@MOD@/kit/version"
)

func main() {
    // 应用身份只在这里声明一次：端口、网关前缀、数据库文件名与发版仓库都由它推导，
    // 不必再复制一份 config.go / version.go 回来改。
    version.Configure(version.Identity{
        Name:        "techfunway-@NAME@",
        DisplayName: "@DISPLAY@",
        DefaultPort: @PORT@,
    })

    config.LoadDotEnv(".env", "server/.env")
    config.Parse()
    server.Start()
}
GOSRC

tpl > "$OUT/server/@PKG@/app.go" <<"GOSRC"
package @PKG@

import (
    "@MOD@/kit/apps"
    "@MOD@/kit/database"
    "@MOD@/kit/sysconfig"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

// init 是本应用与底座的全部接线：注册模型、注册配置项、注册路由。
func init() {
    database.RegisterModels(&Item{})

    sysconfig.RegisterConfig(sysconfig.ConfigDef{
        Key:         "@PKG@_page_size",
        Scope:       sysconfig.ScopeUser,
        Type:        sysconfig.TypeInt,
        Default:     "20",
        Group:       "@PKG@",
        Label:       "@DISPLAY@ 每页条数",
        Description: "列表分页大小，1 到 100",
    })

    apps.Register(apps.App{
        Name:        "@PKG@",
        DisplayName: "@DISPLAY@",
        RoutePrefix: "/@PKG@",
        NavPosition: 100,
        Setup:       setupRoutes,
    })
}

// setupRoutes 收到的 api 已由底座挂好认证与审计中间件。
func setupRoutes(api *gin.RouterGroup, db *gorm.DB) {
    g := api.Group("/@PKG@")
    g.GET("", handleList(db))
    g.POST("", handleCreate(db))
    g.PUT("/:id", handleUpdate(db))
    g.DELETE("/:id", handleDelete(db))
}
GOSRC

tpl > "$OUT/server/@PKG@/models.go" <<"GOSRC"
package @PKG@

import "time"

// Item 是示例业务表：按用户隔离。
type Item struct {
    ID        uint      `gorm:"primarykey"`
    UserID    uint      `gorm:"index;not null"`
    Title     string    `gorm:"size:200;not null"`
    Note      string    `gorm:"size:2000"`
    CreatedAt time.Time
    UpdatedAt time.Time
}
GOSRC

tpl > "$OUT/server/@PKG@/handler.go" <<"GOSRC"
package @PKG@

import (
    "@MOD@/kit/audit"
    "@MOD@/kit/response"
    "@MOD@/kit/utils"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

func handleList(db *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetUint("userID")
        if userID == 0 {
            response.ErrorUnauthorized(c, "未授权")
            return
        }
        page, pageSize := utils.NormalizePage(utils.Atoi(c.Query("page"), 1), utils.Atoi(c.Query("pageSize"), 20))

        var total int64
        if err := db.Model(&Item{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
            response.ErrorInternal(c, "统计失败")
            return
        }
        var items []Item
        if err := db.Where("user_id = ?", userID).Order("id desc").
            Limit(pageSize).Offset(utils.Offset(page, pageSize)).Find(&items).Error; err != nil {
            response.ErrorInternal(c, "查询失败")
            return
        }
        response.SuccessPage(c, items, total, page, pageSize)
    }
}

type itemPayload struct {
    Title string `json:"title" binding:"required,max=200"`
    Note  string `json:"note" binding:"max=2000"`
}

func handleCreate(db *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetUint("userID")
        if userID == 0 {
            response.ErrorUnauthorized(c, "未授权")
            return
        }
        var p itemPayload
        if err := c.ShouldBindJSON(&p); err != nil {
            response.ErrorBadRequest(c, "标题必填，且不超过 200 字")
            return
        }
        item := Item{UserID: userID, Title: p.Title, Note: p.Note}
        if err := db.Create(&item).Error; err != nil {
            response.ErrorInternal(c, "创建失败")
            return
        }
        audit.Log(db, c, "@PKG@_create", "Item", item.ID, p.Title)
        response.Success(c, item)
    }
}

func handleUpdate(db *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetUint("userID")
        id := utils.Atoi(c.Param("id"), 0)
        var p itemPayload
        if err := c.ShouldBindJSON(&p); err != nil {
            response.ErrorBadRequest(c, "标题必填，且不超过 200 字")
            return
        }
        res := db.Model(&Item{}).Where("id = ? AND user_id = ?", id, userID).
            Updates(map[string]interface{}{"title": p.Title, "note": p.Note})
        if res.Error != nil {
            response.ErrorInternal(c, "更新失败")
            return
        }
        if res.RowsAffected == 0 {
            response.ErrorNotFound(c, "记录不存在或无权修改")
            return
        }
        audit.Log(db, c, "@PKG@_update", "Item", uint(id), p.Title)
        response.Success(c, gin.H{"id": id})
    }
}

func handleDelete(db *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetUint("userID")
        id := utils.Atoi(c.Param("id"), 0)
        res := db.Where("id = ? AND user_id = ?", id, userID).Delete(&Item{})
        if res.Error != nil {
            response.ErrorInternal(c, "删除失败")
            return
        }
        if res.RowsAffected == 0 {
            response.ErrorNotFound(c, "记录不存在或无权删除")
            return
        }
        audit.Log(db, c, "@PKG@_delete", "Item", uint(id), "")
        response.Success(c, gin.H{"id": id})
    }
}
GOSRC

tpl > "$OUT/.gitignore" <<"GOSRC"
tmp/*
!tmp/.gitkeep
data/
db/
logs/
uploads/
dist/
build/
release/
node_modules/
*.fpk
*.db
*.log
.DS_Store
.idea/
.zcode/
.mimocode/
GOSRC

tpl > "$OUT/VERSION" <<"GOSRC"
v0.1.0
GOSRC

tpl > "$OUT/CHANGELOG.md" <<"GOSRC"
# @DISPLAY@ 更新日志

## v0.1.0（未发布）

### 新增

- 基于 AppForge 底座的首个骨架：条目列表、新建、修改、删除，按用户隔离。
GOSRC

tpl > "$OUT/README.md" <<"GOSRC"
# @DISPLAY@（@NAME@）

基于 AppForge 底座的应用：账号与登录、角色权限、系统配置、审计、备份与上传恢复、自更新、
实时通道、匿名统计都来自依赖 @MOD@，本仓库只有业务代码。

```bash
cd server && go mod tidy && go run . -port @PORT@ -data-dir ../data -log-console
```

```bash
make build   # 产物在 tmp/@NAME@，ldflags 会注入版本与应用名
make run
```
GOSRC

tpl > "$OUT/AGENTS.md" <<"GOSRC"
# @DISPLAY@（@NAME@）

- 底座：依赖 `@MOD@`；公共能力一律复用，**不要把框架代码复制回本仓库**。
- 业务代码在 `server/@PKG@/`，通过 `apps.Register` 自注册；当前用户取 `c.GetUint("userID")`。
- 应用身份只在 `server/main.go` 的 `version.Configure` 里声明一次
  （包名 techfunway-@NAME@、默认端口 @PORT@、网关前缀 /app/techfunway-@NAME@）。
- 前端与 fnpack 目前**不在底座里**（尚未抽包），起步请从 smallgo 复制再逐处核对：
  包名 6 个位置见父仓库记忆 `.agents/memory/fnos-packaging.md`，图标见技能 `icon-design`。
- 提交备注：`类型(@DISPLAY@)：备注`，全中文两字类型，见技能 `commit-notes`。
GOSRC

tpl > "$OUT/Makefile" <<"GOSRC"
GO ?= go
APP := @NAME@
LDFLAGS := -s -w -X @MOD@/kit/version.Version=$(shell cat VERSION) -X @MOD@/kit/version.AppName=techfunway-@NAME@

.PHONY: build run test vet tidy
build:
    cd server && CGO_ENABLED=1 $(GO) build -ldflags "$(LDFLAGS)" -o ../tmp/$(APP) .
run: build
    ./tmp/$(APP) -port @PORT@ -data-dir ./data -log-console
test:
    cd server && $(GO) test ./...
vet:
    cd server && $(GO) vet ./...
tidy:
    cd server && $(GO) mod tidy
GOSRC

tpl > "$OUT/.gitea/workflows/ci.yml" <<"GOSRC"
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"
      - name: 指向自建 Gitea 拉公共库
        run: |
          go env -w GOPRIVATE=github.com/TechFunWay/* GOSUMDB=off GOPROXY=direct
          # 公共库已在 GitHub 公开，无需改写；仅离线内网构建时设 APPFORGE_SSH 覆盖
          if [ -n "${APPFORGE_SSH:-}" ]; then git config --global url."$APPFORGE_SSH".insteadOf https://github.com/TechFunWay/appforge; fi
      - run: cd server && go mod tidy
      - run: cd server && go vet ./...
      - run: cd server && go build ./...
      - run: cd server && go test ./...
GOSRC

echo "==> 自检：go mod tidy + go build"
cd "$OUT/server"
if go mod tidy >/dev/null 2>&1 && go build ./... >/dev/null 2>&1; then
  echo "==> 通过"
else
  echo "!! 自检失败。请进 $OUT/server 手动执行 go mod tidy 看原因" >&2
  echo "   多半是本机没配公共库的地址改写，见 $HERE/README.md 的这一段" >&2
  exit 2
fi

cat <<"GOSRC"

生成完成。已就绪：
  server/main.go   应用身份注入
  server/<包名>/   业务包：模型 + 分页列表 + 增改删 + 审计
  Makefile / CI / VERSION / CHANGELOG / README / AGENTS / .gitignore / tmp/

还没做（公共库不提供，必须人工补）：
  1. 前端 web/：目前未抽包，从 smallgo/web 起步，按技能 mobile-ui 与 icon-design 调整
  2. fnpack/ 与 app.json：同上起步，包名 6 处一致性按记忆 fnos-packaging 核对
  3. 自建 Gitea 建仓并推送（见记忆 sub-repositories.md）
  4. 父仓库 .gitignore 第 1 节补一行；按技能 app-analysis 建档案并更新 INDEX 总表与端口家族
  5. 门户 techfunway-web 补介绍页；按技能 local-app-instance 建本机 86xx 实例
GOSRC
