// Command minimal 演示接入 AppForge 的最小后端：一份 appmanifest.json 是身份与
// 打包元数据的唯一真源，登录、权限、系统配置、审计、备份、自更新、实时通道、
// 匿名统计全部由底座提供。
//
// 运行：
//
//	cd examples/minimal && go run . -port 8080
package main

import (
	_ "embed"

	"github.com/TechFunWay/appforge/kit/config"
	"github.com/TechFunWay/appforge/kit/manifest"
	"github.com/TechFunWay/appforge/kit/server"

	// 空白导入即注册：apps.Register 会把模块路由挂到 /api 下。
	// 底座自带的 qrcode 模块可以当范本抄。
	_ "github.com/TechFunWay/appforge/kit/qrcode"
)

// 清单与 main.go 同目录：go:embed 不能跨目录，这也是清单放在 server/ 下的原因。
//
//go:embed appmanifest.json
var manifestData []byte

func main() {
	// 1. 注入应用身份：端口、网关前缀、数据库文件名与发版仓库都由清单推导，
	//    不必再复制一份 config.go / version.go 回应用里改。
	manifest.MustParse(manifestData).Inject()

	// 2. 本地 .env 只是开发便利：Docker、飞牛与进程环境的同名变量优先级更高。
	config.LoadDotEnv(".env", "server/.env")
	config.Parse()

	// 3. 起服务：自动迁移、路由装配、日志、调度器、优雅退出都在底座里。
	server.Start()
}
