package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	must := func(name, content string) {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("appmanifest.json", `{"name":"kitdemo","displayName":"底座試作","version":"0.2.0","desc":"底座验证应用","port":8914}`)
	must("CHANGELOG.md", "# 更新日志\n\n## v0.2.0\n\n- 第二版。\n\n## v0.1.0\n\n- 首版。\n")
	return dir
}

func TestWriteThenCheckRoundTrip(t *testing.T) {
	code, err := run(fixture(t), true)
	if err != nil || code != 0 {
		t.Fatalf("write 应成功: code=%d err=%v", code, err)
	}
}

func TestCheckDetectsDrift(t *testing.T) {
	// 完整流程：write → check 通过 → 篡改 VERSION（模拟手改产物）→ check 必须失败
	// 并指出行，且能靠 write 修复回一致。
	var dir string
	// fixture 每次建新目录，这里手动编排。
	dir = fixture(t)
	if code, err := run(dir, true); err != nil || code != 0 {
		t.Fatalf("write: %d %v", code, err)
	}
	if code, err := run(dir, false); err != nil || code != 0 {
		t.Fatalf("write 之后 check 应通过: %d %v", code, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("v9.9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, err := run(dir, false); err != nil || code != 1 {
		t.Fatalf("篡改后 check 应返回 1: %d %v", code, err)
	}
	if code, err := run(dir, true); err != nil || code != 0 {
		t.Fatalf("修复 write 应成功: %d %v", code, err)
	}
	if code, err := run(dir, false); err != nil || code != 0 {
		t.Fatalf("修复后 check 应通过: %d %v", code, err)
	}
}

func TestCheckMissingFile(t *testing.T) {
	// 只有清单、没有产物：check 应报缺失并返回 1。
	if code, err := run(fixture(t), false); err != nil || code != 1 {
		t.Fatalf("缺产物时 check 应返回 1: %d %v", code, err)
	}
}

func TestChangelogFlowsIntoFnpack(t *testing.T) {
	dir := fixture(t)
	if code, err := run(dir, true); err != nil || code != 0 {
		t.Fatalf("write: %d %v", code, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "fnpack", "manifest"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "v0.2.0 更新：\n- 第二版。") || !strings.Contains(s, "v0.1.0 更新：\n- 首版。") {
		t.Fatalf("fnpack changelog 未从 CHANGELOG.md 流入:\n%s", s)
	}
}

func TestManifestUnderServerDir(t *testing.T) {
	// 脚手架把清单放 server/（go:embed 约束），工具必须两处都能找。
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "server"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server", "appmanifest.json"),
		[]byte(`{"name":"kitdemo","displayName":"d","version":"0.1.0","desc":"x","port":8914}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, err := run(dir, true); err != nil || code != 0 {
		t.Fatalf("server/ 下清单应可用: %d %v", code, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "VERSION")); err != nil {
		t.Fatalf("VERSION 应生成到应用根目录: %v", err)
	}
}

func TestBadManifestFailsHard(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "appmanifest.json"), []byte(`{"name":"ok","version":"1.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := run(dir, true)
	if err == nil {
		t.Fatal("坏清单必须报错而不是写产物")
	}
}
