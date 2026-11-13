package version

import "strings"

// Identity 描述接入应用的身份与部署默认值。
//
// AppForge 是公共库，不内置任何具体应用的名字、端口、数据库文件名与发版仓库：
// 应用在自己的 main 里、调用 config.Parse 之前 Configure 一次即可。
// 也可以用 ldflags 覆盖 -X .../version.AppName，两种方式等价，Configure 优先。
type Identity struct {
	// Name 是包名 / 进程名，决定数据库文件名与网关前缀默认值。
	// 飞牛生态惯例为 techfunway-<仓库名>。
	Name string
	// DisplayName 出现在帮助信息与启动横幅里；留空回退到 Name。
	DisplayName string
	// DefaultPort 是 -port 与 PORT 都未提供时的监听端口；0 表示 8080。
	DefaultPort int
	// DefaultGatewayPrefix 是飞牛统一网关前缀；留空按 "/app/" + Name 推导。
	DefaultGatewayPrefix string
	// UpdateRepo 是发版仓库 owner/repo；留空按 TechFunWay/<短名> 推导。
	UpdateRepo string
}

// Ident 是所有默认值的单一来源。这里的兜底值只服务库自带的示例，
// 任何真实应用都应当用 Configure 覆盖。
var Ident = Identity{
	Name:        "appforge-app",
	DisplayName: "AppForge 示例应用",
	DefaultPort: 8080,
}

// Configure 注入应用身份，只覆盖非零字段，可在 init() 或 main 开头调用。
func Configure(i Identity) {
	if i.Name != "" {
		Ident.Name = i.Name
		AppName = i.Name
	}
	if i.DisplayName != "" {
		Ident.DisplayName = i.DisplayName
	} else if i.Name != "" {
		Ident.DisplayName = i.Name
	}
	if i.DefaultPort != 0 {
		Ident.DefaultPort = i.DefaultPort
	}
	if i.DefaultGatewayPrefix != "" {
		Ident.DefaultGatewayPrefix = i.DefaultGatewayPrefix
	}
	if i.UpdateRepo != "" {
		Ident.UpdateRepo = i.UpdateRepo
	}
}

// Port 返回默认监听端口，未设置时回退 8080。
func (i Identity) Port() int {
	if i.DefaultPort == 0 {
		return 8080
	}
	return i.DefaultPort
}

// GatewayPrefix 返回飞牛统一网关路径前缀的默认值。
func (i Identity) GatewayPrefix() string {
	if i.DefaultGatewayPrefix != "" {
		return i.DefaultGatewayPrefix
	}
	return "/app/" + i.Name
}

// DBFileName 返回数据库文件名：<Name>.db，点号形式包名换成连字符。
func (i Identity) DBFileName() string {
	return strings.NewReplacer(".", "-", " ", "").Replace(i.Name) + ".db"
}

// ShortName 去掉 techfunway- / techfunway. 包名前缀。
func (i Identity) ShortName() string {
	n := strings.TrimPrefix(i.Name, "techfunway-")
	return strings.TrimPrefix(n, "techfunway.")
}

// ReleaseRepo 返回发版仓库 owner/repo。无法确定（仍是库默认身份）时返回空串，
// 调用方必须按"渠道不可达"降级，不许把未知当成"已是最新"。
func (i Identity) ReleaseRepo() string {
	if i.UpdateRepo != "" {
		return i.UpdateRepo
	}
	if i.Name == "" || i.Name == "appforge-app" {
		return ""
	}
	return "TechFunWay/" + i.ShortName()
}
