// Command minimal 演示接入 AppForge 的最小后端：注入应用身份 + 注册业务模块，
// 登录、权限、系统配置、审计、备份、自更新、实时通道、匿名统计全部由底座提供。
//
// 运行：
//
//	cd examples/minimal && go run . -port 8080
package main

import (
	"github.com/TechFunWay/appforge/kit/config"
	"github.com/TechFunWay/appforge/kit/server"
	"github.com/TechFunWay/appforge/kit/version"

	// 空白导入即注册：apps.Register 会把模块路由挂到 /api 下。
	// 底座自带的 qrcode 模块可以当范本抄。
	_ "github.com/TechFunWay/appforge/kit/qrcode"
)

func main() {
	// 1. 注入应用身份：端口、网关前缀、数据库文件名与发版仓库都由这里决定，
	//    不必再复制一份 config.go / version.go 回应用里改。
	version.Configure(version.Identity{
		Name:        "techfunway-minimal",
		DisplayName: "AppForge 示例应用",
		DefaultPort: 8080,
		// 网关前缀留空即 /app/techfunway-minimal；发版仓库留空即 TechFunWay/minimal。
	})

	// 2. 本地 .env 只是开发便利：Docker、飞牛与进程环境的同名变量优先级更高。
	config.LoadDotEnv(".env", "server/.env")
	config.Parse()

	// 3. 起服务：自动迁移、路由装配、日志、调度器、优雅退出都在底座里。
	server.Start()
}
