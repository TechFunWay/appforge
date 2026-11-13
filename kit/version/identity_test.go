package version

import "testing"

func TestConfigureOnlyOverridesNonZero(t *testing.T) {
	defer func(name, display string, port int) {
		AppName = name
		Ident = Identity{Name: name, DisplayName: display, DefaultPort: port}
	}(Ident.Name, Ident.DisplayName, Ident.DefaultPort)

	Configure(Identity{Name: "techfunway-notes", DefaultPort: 8914})
	if AppName != "techfunway-notes" {
		t.Fatalf("AppName 应被注入覆盖，实际 %q", AppName)
	}
	if Ident.DisplayName != "techfunway-notes" {
		t.Fatalf("DisplayName 留空应回退到 Name，实际 %q", Ident.DisplayName)
	}
	if got := Ident.GatewayPrefix(); got != "/app/techfunway-notes" {
		t.Fatalf("网关前缀推导错误：%q", got)
	}
	if got := Ident.DBFileName(); got != "techfunway-notes.db" {
		t.Fatalf("数据库文件名推导错误：%q", got)
	}
	if got := Ident.ReleaseRepo(); got != "TechFunWay/notes" {
		t.Fatalf("发版仓库推导错误：%q", got)
	}
}

func TestDotPackageNamesNormalize(t *testing.T) {
	Configure(Identity{Name: "techfunway.bookmarks"})
	if got := Ident.DBFileName(); got != "techfunway-bookmarks.db" {
		t.Fatalf("点号包名未归一化：%q", got)
	}
	if got := Ident.ShortName(); got != "bookmarks" {
		t.Fatalf("点号前缀未剥离：%q", got)
	}
}

func TestUnconfiguredIdentityHasNoReleaseRepo(t *testing.T) {
	base := Ident
	defer func() { Ident = base }()
	Ident = Identity{Name: "appforge-app", DefaultPort: 8080}
	if got := Ident.ReleaseRepo(); got != "" {
		t.Fatalf("库默认身份不应推出发版仓库，实际 %q", got)
	}
	if got := Ident.Port(); got != 8080 {
		t.Fatalf("零值端口应回退 8080，实际 %d", got)
	}
}
