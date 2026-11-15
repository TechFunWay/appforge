// Command gen-manifest 从 appmanifest.json（加 CHANGELOG.md）派生应用的三份打包元数据：
// VERSION、app.json、fnpack/manifest。真源只有清单文件，三份产物禁止手改。
//
// 用法：
//
//	go run github.com/TechFunWay/appforge/cmd/gen-manifest -dir <应用目录> -write
//	go run github.com/TechFunWay/appforge/cmd/gen-manifest -dir <应用目录> -check   # 不一致 exit 1，CI 门禁
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/TechFunWay/appforge/kit/manifest"
)

func main() {
	dir := flag.String("dir", ".", "应用目录（含 appmanifest.json）")
	write := flag.Bool("write", false, "写入派生文件")
	check := flag.Bool("check", false, "比对派生文件（默认动作）")
	flag.Parse()
	if *write && *check {
		fatal(errors.New("-write 与 -check 只能选一个"))
	}
	code, err := run(*dir, *write)
	if err != nil {
		fatal(err)
	}
	if code != 0 {
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gen-manifest:", err)
	os.Exit(2)
}

// findManifest 定位清单：优先应用根目录，其次 server/（go:embed 不能跨出
// 所在目录，主接线在 server/main.go 时清单必须放那儿）。
func findManifest(dir string) (string, error) {
	for _, cand := range []string{
		filepath.Join(dir, "appmanifest.json"),
		filepath.Join(dir, "server", "appmanifest.json"),
	} {
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}
	return "", errors.New("找不到 appmanifest.json（查过应用根目录与 server/）")
}

// targets 返回 文件名 → 派生文件 的表（fnpack/ 前缀下含生命周期脚本文件组）。
func targets(dir string) (map[string]manifest.RenderedFile, error) {
	manifestPath, err := findManifest(dir)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("读取清单失败: %w", err)
	}
	m, err := manifest.Parse(raw)
	if err != nil {
		return nil, err
	}
	// CHANGELOG.md 可选：没有就生成不带 changelog 块的 fnpack manifest。
	cl := ""
	if md, err := os.ReadFile(filepath.Join(dir, "CHANGELOG.md")); err == nil {
		cl = manifest.ExtractChangelog(string(md))
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("读取 CHANGELOG.md 失败: %w", err)
	}
	appJSON, err := m.RenderAppJSON()
	if err != nil {
		return nil, err
	}
	out := map[string]manifest.RenderedFile{
		"VERSION":         {Rel: "VERSION", Data: m.RenderVersionFile()},
		"app.json":        {Rel: "app.json", Data: appJSON},
		"fnpack/manifest": {Rel: "fnpack/manifest", Data: m.RenderFnpackManifest(cl)},
	}
	files, err := m.RenderFnpackFiles()
	if err != nil {
		return nil, err
	}
	for rel, f := range files {
		f.Rel = "fnpack/" + rel
		out["fnpack/"+rel] = f
	}
	return out, nil
}

// run 执行 write 或 check，返回退出码（0 通过 / 1 有差异）与错误。
func run(dir string, write bool) (int, error) {
	tt, err := targets(dir)
	if err != nil {
		return 0, err
	}
	names := make([]string, 0, len(tt))
	for name := range tt {
		names = append(names, name)
	}
	sort.Strings(names) // 稳定顺序：write/check 输出可比对
	bad := 0
	for _, name := range names {
		want := tt[name].Data
		path := filepath.Join(dir, name)
		cur, err := os.ReadFile(path)
		exists := err == nil
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return 0, fmt.Errorf("读取 %s 失败: %w", name, err)
		}
		if write {
			if exists && string(cur) == string(want) {
				fmt.Printf("一致  %s\n", name)
				continue
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return 0, err
			}
			mode := os.FileMode(0o644)
			if tt[name].Exec {
				mode = 0o755
			}
			if err := os.WriteFile(path, want, mode); err != nil {
				return 0, err
			}
			if err := os.Chmod(path, mode); err != nil {
				return 0, err
			}
			fmt.Printf("已写  %s\n", name)
			continue
		}
		if !exists {
			fmt.Printf("缺失  %s（跑 gen-manifest -write 生成）\n", name)
			bad++
			continue
		}
		if string(cur) != string(want) {
			fmt.Printf("漂移  %s：与 appmanifest.json/CHANGELOG.md 派生结果不一致\n", name)
			for _, d := range diffLines(string(cur), string(want)) {
				fmt.Printf("        %s\n", d)
			}
			bad++
			continue
		}
		fmt.Printf("一致  %s\n", name)
	}
	if bad > 0 {
		return 1, nil
	}
	return 0, nil
}

// diffLines 输出前 8 处首个不同的行号与两边内容，够定位漂移，不做大 diff。
func diffLines(cur, want string) []string {
	var out []string
	cs := strings.Split(strings.TrimRight(cur, "\n"), "\n")
	ws := strings.Split(strings.TrimRight(want, "\n"), "\n")
	n := len(cs)
	if len(ws) < n {
		n = len(ws)
	}
	for i := 0; i < n && len(out) < 8; i++ {
		if cs[i] != ws[i] {
			out = append(out, fmt.Sprintf("L%d 现为 %q，应为 %q", i+1, cs[i], ws[i]))
		}
	}
	if (len(cs) != len(ws)) && len(out) < 8 {
		out = append(out, fmt.Sprintf("行数：现为 %d，应为 %d", len(cs), len(ws)))
	}
	return out
}
