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
echo "==> 生成 ${OUT}（Go 包名 ${PKG}，默认端口 ${PORT}）"

# 模板一律用带引号的 heredoc 写，这样 Go 结构体标签里的反引号不会被 shell 吃掉；
# 变量与制表符用占位符替换（BSD sed 不认 \t，所以先用 printf 造一个真制表符）。
TAB=$(printf "\t")
tpl() {
  sed -e "s|@NAME@|$NAME|g" -e "s|@PKG@|$PKG|g" -e "s|@DISPLAY@|$DISPLAY|g" \
      -e "s|@PORT@|$PORT|g" -e "s|@MOD@|$MOD|g" -e "s|@LIB@|$LIB_VERSION|g" \
      -e "s|@TAB@|$TAB|g"
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
@TAB@// 空白导入即注册：业务包在自己的 init() 里把模型、配置项与路由挂进底座。
@TAB@_ "@NAME@/server/@PKG@"

@TAB@"@MOD@/kit/config"
@TAB@"@MOD@/kit/server"
@TAB@"@MOD@/kit/version"
)

func main() {
@TAB@// 应用身份只在这里声明一次：端口、网关前缀、数据库文件名与发版仓库都由它推导，
@TAB@// 不必再复制一份 config.go / version.go 回来改。
@TAB@version.Configure(version.Identity{
@TAB@@TAB@Name:        "techfunway-@NAME@",
@TAB@@TAB@DisplayName: "@DISPLAY@",
@TAB@@TAB@DefaultPort: @PORT@,
@TAB@})

@TAB@config.LoadDotEnv(".env", "server/.env")
@TAB@config.Parse()
@TAB@server.Start()
}
GOSRC

tpl > "$OUT/server/$PKG/app.go" <<"GOSRC"
package @PKG@

import (
@TAB@"@MOD@/kit/apps"
@TAB@"@MOD@/kit/database"
@TAB@"@MOD@/kit/sysconfig"

@TAB@"github.com/gin-gonic/gin"
@TAB@"gorm.io/gorm"
)

// init 是本应用与底座的全部接线：注册模型、注册配置项、注册路由。
func init() {
@TAB@database.RegisterModels(&Item{})

@TAB@sysconfig.RegisterConfig(sysconfig.ConfigDef{
@TAB@@TAB@Key:         "@PKG@_page_size",
@TAB@@TAB@Scope:       sysconfig.ScopeUser,
@TAB@@TAB@Type:        sysconfig.TypeInt,
@TAB@@TAB@Default:     "20",
@TAB@@TAB@Group:       "@PKG@",
@TAB@@TAB@Label:       "@DISPLAY@ 每页条数",
@TAB@@TAB@Description: "列表分页大小，1 到 100",
@TAB@})

@TAB@apps.Register(apps.App{
@TAB@@TAB@Name:        "@PKG@",
@TAB@@TAB@DisplayName: "@DISPLAY@",
@TAB@@TAB@RoutePrefix: "/@PKG@",
@TAB@@TAB@NavPosition: 100,
@TAB@@TAB@// SetupAuth 挂到认证组：未登录请求由底座中间件直接 401。
@TAB@@TAB@// 公开接口用 Setup，仅管理员接口用 SetupAdmin。
@TAB@@TAB@SetupAuth:     setupRoutes,
@TAB@})
}

// setupRoutes 收到的 api 已由底座挂好认证与审计中间件。
func setupRoutes(api *gin.RouterGroup, db *gorm.DB) {
@TAB@g := api.Group("/@PKG@")
@TAB@g.GET("", handleList(db))
@TAB@g.POST("", handleCreate(db))
@TAB@g.PUT("/:id", handleUpdate(db))
@TAB@g.DELETE("/:id", handleDelete(db))
}
GOSRC

tpl > "$OUT/server/$PKG/models.go" <<"GOSRC"
package @PKG@

import "time"

// Item 是示例业务表：按用户隔离。
type Item struct {
@TAB@ID        uint      `gorm:"primarykey"`
@TAB@UserID    uint      `gorm:"index;not null"`
@TAB@Title     string    `gorm:"size:200;not null"`
@TAB@Note      string    `gorm:"size:2000"`
@TAB@CreatedAt time.Time
@TAB@UpdatedAt time.Time
}
GOSRC

tpl > "$OUT/server/$PKG/handler.go" <<"GOSRC"
package @PKG@

import (
@TAB@"@MOD@/kit/audit"
@TAB@"@MOD@/kit/response"
@TAB@"@MOD@/kit/utils"

@TAB@"github.com/gin-gonic/gin"
@TAB@"gorm.io/gorm"
)

func handleList(db *gorm.DB) gin.HandlerFunc {
@TAB@return func(c *gin.Context) {
@TAB@@TAB@userID := c.GetUint("userID")
@TAB@@TAB@if userID == 0 {
@TAB@@TAB@@TAB@response.ErrorUnauthorized(c, "未授权")
@TAB@@TAB@@TAB@return
@TAB@@TAB@}
@TAB@@TAB@page, pageSize := utils.NormalizePage(utils.Atoi(c.Query("page"), 1), utils.Atoi(c.Query("pageSize"), 20))

@TAB@@TAB@var total int64
@TAB@@TAB@if err := db.Model(&Item{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
@TAB@@TAB@@TAB@response.ErrorInternal(c, "统计失败")
@TAB@@TAB@@TAB@return
@TAB@@TAB@}
@TAB@@TAB@var items []Item
@TAB@@TAB@if err := db.Where("user_id = ?", userID).Order("id desc").
@TAB@@TAB@@TAB@Limit(pageSize).Offset(utils.Offset(page, pageSize)).Find(&items).Error; err != nil {
@TAB@@TAB@@TAB@response.ErrorInternal(c, "查询失败")
@TAB@@TAB@@TAB@return
@TAB@@TAB@}
@TAB@@TAB@response.SuccessPage(c, items, total, page, pageSize)
@TAB@}
}

type itemPayload struct {
@TAB@Title string `json:"title" binding:"required,max=200"`
@TAB@Note  string `json:"note" binding:"max=2000"`
}

func handleCreate(db *gorm.DB) gin.HandlerFunc {
@TAB@return func(c *gin.Context) {
@TAB@@TAB@userID := c.GetUint("userID")
@TAB@@TAB@if userID == 0 {
@TAB@@TAB@@TAB@response.ErrorUnauthorized(c, "未授权")
@TAB@@TAB@@TAB@return
@TAB@@TAB@}
@TAB@@TAB@var p itemPayload
@TAB@@TAB@if err := c.ShouldBindJSON(&p); err != nil {
@TAB@@TAB@@TAB@response.ErrorBadRequest(c, "标题必填，且不超过 200 字")
@TAB@@TAB@@TAB@return
@TAB@@TAB@}
@TAB@@TAB@item := Item{UserID: userID, Title: p.Title, Note: p.Note}
@TAB@@TAB@if err := db.Create(&item).Error; err != nil {
@TAB@@TAB@@TAB@response.ErrorInternal(c, "创建失败")
@TAB@@TAB@@TAB@return
@TAB@@TAB@}
@TAB@@TAB@audit.Log(db, c, "@PKG@_create", "Item", item.ID, p.Title)
@TAB@@TAB@response.Success(c, item)
@TAB@}
}

func handleUpdate(db *gorm.DB) gin.HandlerFunc {
@TAB@return func(c *gin.Context) {
@TAB@@TAB@userID := c.GetUint("userID")
@TAB@@TAB@id := utils.Atoi(c.Param("id"), 0)
@TAB@@TAB@var p itemPayload
@TAB@@TAB@if err := c.ShouldBindJSON(&p); err != nil {
@TAB@@TAB@@TAB@response.ErrorBadRequest(c, "标题必填，且不超过 200 字")
@TAB@@TAB@@TAB@return
@TAB@@TAB@}
@TAB@@TAB@res := db.Model(&Item{}).Where("id = ? AND user_id = ?", id, userID).
@TAB@@TAB@@TAB@Updates(map[string]interface{}{"title": p.Title, "note": p.Note})
@TAB@@TAB@if res.Error != nil {
@TAB@@TAB@@TAB@response.ErrorInternal(c, "更新失败")
@TAB@@TAB@@TAB@return
@TAB@@TAB@}
@TAB@@TAB@if res.RowsAffected == 0 {
@TAB@@TAB@@TAB@response.ErrorNotFound(c, "记录不存在或无权修改")
@TAB@@TAB@@TAB@return
@TAB@@TAB@}
@TAB@@TAB@audit.Log(db, c, "@PKG@_update", "Item", uint(id), p.Title)
@TAB@@TAB@response.Success(c, gin.H{"id": id})
@TAB@}
}

func handleDelete(db *gorm.DB) gin.HandlerFunc {
@TAB@return func(c *gin.Context) {
@TAB@@TAB@userID := c.GetUint("userID")
@TAB@@TAB@id := utils.Atoi(c.Param("id"), 0)
@TAB@@TAB@res := db.Where("id = ? AND user_id = ?", id, userID).Delete(&Item{})
@TAB@@TAB@if res.Error != nil {
@TAB@@TAB@@TAB@response.ErrorInternal(c, "删除失败")
@TAB@@TAB@@TAB@return
@TAB@@TAB@}
@TAB@@TAB@if res.RowsAffected == 0 {
@TAB@@TAB@@TAB@response.ErrorNotFound(c, "记录不存在或无权删除")
@TAB@@TAB@@TAB@return
@TAB@@TAB@}
@TAB@@TAB@audit.Log(db, c, "@PKG@_delete", "Item", uint(id), "")
@TAB@@TAB@response.Success(c, gin.H{"id": id})
@TAB@}
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
make build && make run   # ldflags 注入版本与应用名
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
@TAB@cd server && CGO_ENABLED=1 $(GO) build -ldflags "$(LDFLAGS)" -o ../tmp/$(APP) .
run: build
@TAB@./tmp/$(APP) -port @PORT@ -data-dir ./data -log-console
test:
@TAB@cd server && $(GO) test ./...
vet:
@TAB@cd server && $(GO) vet ./...
tidy:
@TAB@cd server && $(GO) mod tidy
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
      - name: 离线内网构建时才需要改写公共库地址
        run: |
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
  exit 2
fi

cat <<"GOSRC"

生成完成。已就绪：
  server/main.go   应用身份注入
  server/<包名>/   业务包：模型 + 分页列表 + 增改删 + 审计
  Makefile / CI / VERSION / CHANGELOG / README / AGENTS / .gitignore / tmp/

还没做（公共库不提供，必须人工补）：
  1. 前端 web/：目前未抽包，从 smallgo/web 起步，按技能 mobile-ui 与 icon-design 调整
  2. fnpack/ 与 app.json：同上起步，包名 6 个位置按记忆 fnos-packaging 核对
  3. 自建 Gitea 建仓并推送（见记忆 sub-repositories.md）
  4. 父仓库 .gitignore 第 1 节补一行；按技能 app-analysis 建档案、更新 INDEX 总表与端口家族
  5. 门户 techfunway-web 补介绍页；按技能 local-app-instance 建本机 86xx 实例
GOSRC
