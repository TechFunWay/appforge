package manifest

import (
	"strings"
	"testing"

	"github.com/TechFunWay/appforge/kit/version"
)

func minimalJSON() string {
	return `{"name":"kitdemo","displayName":"底座試作","version":"0.1.0","desc":"测试应用","port":8914}`
}

func TestParseMinimalDefaults(t *testing.T) {
	m, err := Parse([]byte(minimalJSON()))
	if err != nil {
		t.Fatalf("最小清单应通过: %v", err)
	}
	if got := m.Pkg(); got != "techfunway-kitdemo" {
		t.Fatalf("包名推导错误: %q", got)
	}
	if got := m.Gateway(); got != "/app/techfunway-kitdemo" {
		t.Fatalf("网关前缀推导错误: %q", got)
	}
	if got := m.Image(); got != "techfunways/kitdemo" {
		t.Fatalf("镜像名推导错误: %q", got)
	}
	if got := m.ReleaseRepo(); got != "TechFunWay/kitdemo" {
		t.Fatalf("发版仓库推导错误: %q", got)
	}
	if got := m.Tag(); got != "v0.1.0" {
		t.Fatalf("版本号形态错误: %q", got)
	}
	if got := m.DesktopLaunchName(); got != "techfunway-kitdemo.main" {
		t.Fatalf("桌面入口推导错误: %q", got)
	}
	if m.Author_() != "TechFunWay" || m.Website_() != "https://techfunway.wycto.cn" {
		t.Fatalf("作者/站点默认值错误")
	}
	if m.Maintainer_() != "科技智趣坊" || m.Distributor_() != "科技智趣坊" {
		t.Fatalf("署名人默认值错误")
	}
	if m.OSMin_() != "1.0.0" || m.Platform_() != "x86" {
		t.Fatalf("系统默认值错误")
	}
	if !m.CtlStop_() {
		t.Fatal("ctlStop 默认应为 true")
	}
	if m.CheckPort_() || m.DisableAuthz_() {
		t.Fatal("checkPort / disableAuthorizationPath 默认应为 false")
	}
}

func TestParseFullFields(t *testing.T) {
	body := `{"name":"medcare","displayName":"用药管理","version":"0.1.37","desc":"家庭协作用药本","port":8913,
		"pkgName":"techfunway-medcare","gatewayPrefix":"/app/techfunway-medcare","dockerImage":"techfunways/medcare",
		"updateRepo":"TechFunWay/medcare","author":"TechFunWay","website":"https://example.test",
		"maintainer":"某坊","distributor":"另坊","osMinVersion":"1.1.0","platform":"x86,arm64",
		"ctlStop":false,"checkPort":true,"disableAuthorizationPath":true}`
	m, err := Parse([]byte(body))
	if err != nil {
		t.Fatalf("全字段清单应通过: %v", err)
	}
	if m.CtlStop_() {
		t.Fatal("显式 false 的 ctlStop 被吃掉了")
	}
	if !m.CheckPort_() || !m.DisableAuthz_() {
		t.Fatal("显式 true 的布尔项未生效")
	}
	if m.Distributor_() != "另坊" {
		t.Fatalf("distributor 显式值未生效: %q", m.Distributor_())
	}
	if m.Platform_() != "x86,arm64" || m.OSMin_() != "1.1.0" {
		t.Fatal("平台/系统版本显式值未生效")
	}
}

func TestValidateReportsAllErrors(t *testing.T) {
	_, err := Parse([]byte(`{"name":"Bad Name","displayName":"","version":"1.0","desc":"","port":0}`))
	if err == nil {
		t.Fatal("全坏清单应报错")
	}
	for _, want := range []string{"name ", "displayName", "version ", "desc ", "port "} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("报错缺少 %q 项: %v", want, err)
		}
	}
	// 一次报全部，而不是只报第一个：改一处跑一遍的反馈太慢。
	if n := strings.Count(err.Error(), "\n- "); n != 5 {
		t.Fatalf("应一次报 5 条问题，实际 %d: %v", n, err)
	}
}

func TestUnknownFieldRejected(t *testing.T) {
	// 全新字段：必须点名报错。
	_, err := Parse([]byte(`{"name":"kitdemo","displayName":"d","version":"0.1.0","desc":"x","port":8914,"gpu":true}`))
	if err == nil || !strings.Contains(err.Error(), "gpu") {
		t.Fatalf("未知字段应报错并点名: %v", err)
	}
	// displayname 少个大写字母 N——encoding/json 默认大小写不敏感会吞掉这类拼错，
	// 白名单必须把它也拦下来，否则元数据悄悄漂移。
	_, err = Parse([]byte(`{"name":"kitdemo","displayname":"错拼","version":"0.1.0","desc":"x","port":8914}`))
	if err == nil || !strings.Contains(err.Error(), "displayname") {
		t.Fatalf("大小写拼错字段应报错并点名: %v", err)
	}
}

func TestTrailingContentRejected(t *testing.T) {
	_, err := Parse([]byte(minimalJSON() + `{"name":"second"}`))
	// 实现走 json.Unmarshal 的整值校验，报「invalid character」即达标：
	// 关键是双对象输入必须被拒，具体措辞不锁死。
	if err == nil {
		t.Fatal("尾随第二个 JSON 应报错")
	}
}

func TestPkgNameMustKeepFamilyPrefix(t *testing.T) {
	_, err := Parse([]byte(`{"name":"kitdemo","displayName":"d","version":"0.1.0","desc":"x","port":8914,"pkgName":"other-kit"}`))
	if err == nil || !strings.Contains(err.Error(), "techfunway-") {
		t.Fatalf("非家族前缀包名应被拒绝: %v", err)
	}
}

func TestInjectWiresIdentity(t *testing.T) {
	defer func() {
		version.Ident = version.Identity{}
		version.AppName = ""
		version.Version = "dev"
	}()
	m := MustParse([]byte(minimalJSON()))
	m.Inject()
	if version.AppName != "techfunway-kitdemo" {
		t.Fatalf("AppName 未注入: %q", version.AppName)
	}
	if version.Ident.DefaultPort != 8914 {
		t.Fatalf("端口未注入: %d", version.Ident.DefaultPort)
	}
	if got := version.Ident.GatewayPrefix(); got != "/app/techfunway-kitdemo" {
		t.Fatalf("网关前缀未注入: %q", got)
	}
	if got := version.Ident.ReleaseRepo(); got != "TechFunWay/kitdemo" {
		t.Fatalf("发版仓库未注入: %q", got)
	}
	if version.Version != "v0.1.0" {
		t.Fatalf("版本号应从清单取: %q", version.Version)
	}
}

func TestInjectKeepsLdflagsVersion(t *testing.T) {
	defer func() {
		version.Ident = version.Identity{}
		version.AppName = ""
		version.Version = "dev"
	}()
	// 模拟 ldflags 已注入发布版本号：Inject 不得覆盖构建期事实。
	version.Version = "v9.9.9"
	MustParse([]byte(minimalJSON())).Inject()
	if version.Version != "v9.9.9" {
		t.Fatalf("ldflags 版本被清单覆盖: %q", version.Version)
	}
}

func TestMustParsePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("坏清单必须 panic")
		}
	}()
	MustParse([]byte(`{"name":""}`))
}
