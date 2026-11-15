package manifest

import (
	"encoding/json"
	"strings"
	"testing"
)

// medcareParams 渲染参考参数与 medcare 线上真源一致；
// 全量逐字节比对见 scripts/golden-fnpack.sh（拿本仓库模板对照 medcare/fnpack）。
func medcareParams(t *testing.T) *Manifest {
	t.Helper()
	m, err := Parse([]byte(`{"name":"medcare","displayName":"用药管理","version":"0.1.37",
		"desc":"面向用药人与监护人的家庭协作用药本","port":8913,
		"intro":"用药管理是家庭协作用药本。安装完成后，首次访问并注册的用户将成为管理员。运行数据（数据库、上传文件、日志）保存在应用数据目录中。"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return m
}

func TestRenderFnpackFilesCompleteness(t *testing.T) {
	files, err := medcareParams(t).RenderFnpackFiles()
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	want := []string{
		"app/ui/config",
		"cmd/config_callback", "cmd/config_init", "cmd/install_callback", "cmd/install_init",
		"cmd/main", "cmd/port_config", "cmd/uninstall_callback", "cmd/uninstall_init",
		"cmd/upgrade_callback", "cmd/upgrade_init",
		"config/privilege", "config/resource",
		"wizard/config", "wizard/install", "wizard/uninstall", "wizard/upgrade",
	}
	if len(files) != len(want) {
		t.Fatalf("文件数 = %d, want %d", len(files), len(want))
	}
	for _, name := range want {
		f, ok := files[name]
		if !ok {
			t.Fatalf("缺文件 %s", name)
		}
		if len(f.Data) == 0 {
			t.Fatalf("%s 内容为空", name)
		}
		if s := string(f.Data); strings.Contains(s, "{{.") || strings.Contains(s, "}}") {
			t.Fatalf("%s 残留未替换占位符", name)
		}
	}
	for name, f := range files {
		isCmd := strings.HasPrefix(name, "cmd/")
		if f.Exec != isCmd {
			t.Fatalf("%s 可执行位 = %v, want %v", name, f.Exec, isCmd)
		}
	}
}

func TestRenderFnpackFilesParameterizedLines(t *testing.T) {
	files, err := medcareParams(t).RenderFnpackFiles()
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string][]string{
		"cmd/main": {
			"./medcare",
			`-gateway-prefix "/app/techfunway-medcare"`,
			"Starting 用药管理 through the fnOS unified gateway and port ",
			"Stopping 用药管理 ...",
			"TRIM_PKGVAR",
		},
		"cmd/port_config": {
			`printf "%s" "8913"`,
			// bash printf 的转义字面量必须原样保留（不能被渲染成真换行）
			`printf "%s\n"`,
			"TRIM_SERVICE_PORT",
		},
		"config/resource": {
			`"name": "techfunway-medcare"`,
			`"name": "techfunway-medcare/data"`,
		},
		"app/ui/config": {
			`"techfunway-medcare.main": {`,
			`"title": "用药管理",`,
			`"url": "/app/techfunway-medcare/fnos-entry.html"`,
		},
		"wizard/install": {
			`"stepTitle": "安装 用药管理"`,
			`"initValue": "8913"`,
		},
		"wizard/uninstall": {`"stepTitle": "卸载 用药管理"`},
		"wizard/upgrade":   {`"stepTitle": "升级 用药管理"`, `"initValue": "8913"`},
	}
	for name, wants := range checks {
		got := string(files[name].Data)
		for _, w := range wants {
			if !strings.Contains(got, w) {
				t.Fatalf("%s 缺少期望片段 %q", name, w)
			}
		}
	}
}

// JSON 类文件渲染后必须仍是合法 JSON。
func TestRenderFnpackFilesJSONValid(t *testing.T) {
	files, err := medcareParams(t).RenderFnpackFiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config/privilege", "config/resource", "app/ui/config", "wizard/config", "wizard/install", "wizard/uninstall", "wizard/upgrade"} {
		var v any
		if err := json.Unmarshal(files[name].Data, &v); err != nil {
			t.Fatalf("%s 渲染后不是合法 JSON: %v", name, err)
		}
	}
}

// Intro 缺省推导、句号去重与显式覆盖。
func TestIntroFallback(t *testing.T) {
	m, err := Parse([]byte(`{"name":"kitdemo","displayName":"底座試作","version":"0.1.0","desc":"底座验证应用","port":8914}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.IntroText(); !strings.HasPrefix(got, "底座验证应用。安装完成后") {
		t.Fatalf("缺省 intro 应由 desc 拼出: %q", got)
	}
	m.Desc = "带句号的描述。"
	if !strings.HasPrefix(m.IntroText(), "带句号的描述。安装完成后") {
		t.Fatalf("句号去重失败: %q", m.IntroText())
	}
	m.Intro = "完全自定义的介绍。"
	if m.IntroText() != "完全自定义的介绍。" {
		t.Fatal("显式 intro 应原样使用")
	}
}
