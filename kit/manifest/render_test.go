package manifest

import (
	"encoding/json"
	"strings"
	"testing"
)

const tripleQuote = "\"\"\"" // 三引号字面量，fnpack changelog 块的定界符

func testManifest(t *testing.T) *Manifest {
	t.Helper()
	m, err := Parse([]byte(minimalJSON()))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return m
}

func TestRenderVersionFile(t *testing.T) {
	if got := string(testManifest(t).RenderVersionFile()); got != "v0.1.0\n" {
		t.Fatalf("VERSION 内容错误: %q", got)
	}
}

func TestRenderAppJSONByteExact(t *testing.T) {
	got, err := testManifest(t).RenderAppJSON()
	if err != nil {
		t.Fatal(err)
	}
	want := "{" + "\n" +
		`  "appname": "techfunway-kitdemo",` + "\n" +
		`  "display_name": "底座試作",` + "\n" +
		`  "version": "0.1.0",` + "\n" +
		`  "desc": "测试应用",` + "\n" +
		`  "author": "TechFunWay",` + "\n" +
		`  "website": "https://techfunway.wycto.cn",` + "\n" +
		`  "port": 8914,` + "\n" +
		`  "docker": {` + "\n" +
		`    "image": "techfunways/kitdemo",` + "\n" +
		`    "container": "techfunway-kitdemo",` + "\n" +
		`    "tag_prefix": "techfunway-kitdemo"` + "\n" +
		`  },` + "\n" +
		`  "fnos": {` + "\n" +
		`    "os_min_version": "1.0.0",` + "\n" +
		`    "service_port": 8914,` + "\n" +
		`    "ctl_stop": true,` + "\n" +
		`    "checkport": false,` + "\n" +
		`    "disable_authorization_path": false` + "\n" +
		`  }` + "\n" +
		`}` + "\n"
	if string(got) != want {
		t.Fatalf("app.json 逐字节不一致:\n实际:\n%s\n期望:\n%s", got, want)
	}
}

func TestRenderAppJSONIsValid(t *testing.T) {
	got, err := testManifest(t).RenderAppJSON()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("生成的 app.json 不合法: %v", err)
	}
	if doc["appname"] != "techfunway-kitdemo" {
		t.Fatalf("appname 字段错误: %v", doc["appname"])
	}
}

func TestRenderFnpackManifestAlignment(t *testing.T) {
	got := string(testManifest(t).RenderFnpackManifest(""))
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	wantKeys := []string{"appname", "version", "display_name", "desc", "source", "platform",
		"maintainer", "maintainer_url", "distributor", "distributor_url", "os_min_version",
		"ctl_stop", "desktop_uidir", "desktop_applaunchname", "service_port", "checkport",
		"disable_authorization_path"}
	if len(lines) != len(wantKeys) {
		t.Fatalf("行数 %d ≠ 期望 %d:\n%s", len(lines), len(wantKeys), got)
	}
	for i, ln := range lines {
		key := strings.Fields(ln)[0]
		if key != wantKeys[i] {
			t.Fatalf("第 %d 行字段应为 %s，实际 %s: %q", i+1, wantKeys[i], key, ln)
		}
		if eq := strings.Index(ln, "="); eq != fnpackKeyWidth {
			t.Fatalf("第 %d 行等号未对齐到 %d 列: %q", i+1, fnpackKeyWidth, ln)
		}
	}
	if strings.Contains(got, "changelog") {
		t.Fatal("空 changelog 不应出现 changelog 字段")
	}
}

func TestRenderFnpackManifestValues(t *testing.T) {
	got := string(testManifest(t).RenderFnpackManifest(""))
	wants := []string{
		"appname                    = techfunway-kitdemo",
		"service_port               = 8914",
		"ctl_stop                   = true",
		"checkport                  = false",
		"desktop_applaunchname      = techfunway-kitdemo.main",
		"maintainer_url             = https://github.com/TechFunWay/kitdemo",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Fatalf("缺少行 %q，实际:\n%s", w, got)
		}
	}
}

func TestRenderFnpackManifestWithChangelog(t *testing.T) {
	cl := "v0.2.0 更新：\n- 加了个功能\n\nv0.1.0 更新：\n- 首发"
	got := string(testManifest(t).RenderFnpackManifest(cl))
	if !strings.HasSuffix(got, tripleQuote+"\n") {
		t.Fatalf("changelog 块应以三引号行结尾: %q", got[len(got)-12:])
	}
	head := "changelog                  = " + tripleQuote + "\n" + "v0.2.0 更新："
	if !strings.Contains(got, head) {
		t.Fatalf("changelog 块头不对:\n%s", got)
	}
	if strings.Count(got, tripleQuote) != 2 {
		t.Fatal("三引号应恰好成对")
	}
}

func TestExtractChangelogSections(t *testing.T) {
	md := strings.Join([]string{
		"# 更新日志",
		"",
		"介绍段落的 - 不该被收",
		"",
		"## v0.1.37",
		"",
		"- 顶部新增菜单。",
		"- 菜单可收起。",
		"",
		"### 修复",
		"",
		"- 修了个 bug。",
		"",
		"## v0.1.36",
		"",
		"1. 数字条目。",
		"",
		"## v0.1.0（未发布）",
		"",
		"- 首个骨架。",
	}, "\n")
	got := ExtractChangelog(md)
	if !strings.HasPrefix(got, "v0.1.37 更新：\n") {
		t.Fatalf("开头错误: %q", got)
	}
	if strings.Contains(got, "介绍段落") {
		t.Fatal("首个小节之前的内容不应被收集")
	}
	if !strings.Contains(got, "- 修了个 bug。") {
		t.Fatal("### 子标题下的条目应被收集")
	}
	if strings.Contains(got, "### 修复") {
		t.Fatal("子标题本身不应出现")
	}
	if !strings.Contains(got, "v0.1.0 更新：\n- 首个骨架。") {
		t.Fatalf("（未发布）后缀应被剥掉: %q", got)
	}
	if !strings.Contains(got, "1. 数字条目。") {
		t.Fatal("数字列表条目应被收集")
	}
	if !strings.Contains(got, "菜单可收起。\n- 修了个 bug。\n\nv0.1.36") {
		t.Fatalf("节内/节间排版错误:\n%s", got)
	}
}

func TestExtractChangelogEmpty(t *testing.T) {
	if got := ExtractChangelog("# 只有标题\n没有小节\n"); got != "" {
		t.Fatalf("无小节应返回空串: %q", got)
	}
	if got := ExtractChangelog(""); got != "" {
		t.Fatalf("空输入应返回空串: %q", got)
	}
}

func TestExtractChangelogDoesNotEditContent(t *testing.T) {
	// 提取器只搬运不审判：「不进对外日志」是 CHANGELOG.md 的维护规范（父仓库记忆），
	// 固化当前行为，防止将来有人给提取器加白名单式删行。
	md := "## v1.0.0\n\n- 新增了赞赏入口。\n"
	if got := ExtractChangelog(md); !strings.Contains(got, "赞赏") {
		t.Fatal("提取器不应擅自删改条目内容")
	}
}
