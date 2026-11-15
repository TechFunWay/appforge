package manifest

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"text/template"
)

//go:embed tmpl/fnpack
var fnpackTmplFS embed.FS

// RenderedFile 是一个派生文件：目标相对路径、内容与是否可执行位。
type RenderedFile struct {
	Rel  string
	Data []byte
	Exec bool
}

// tmplData 是 fnpack 模板的渲染上下文，全部字段来自清单推导，无第二处真源。
type tmplData struct {
	Name        string
	Pkg         string
	DisplayName string
	Port        string
	Gateway     string
	Desc        string
	Intro       string
}

// IntroText 返回安装向导介绍段：清单显式给了就用，否则用一句话描述加家族通用尾巴。
func (m *Manifest) IntroText() string {
	if strings.TrimSpace(m.Intro) != "" {
		return m.Intro
	}
	desc := strings.TrimRight(strings.TrimSpace(m.Desc), "。")
	return desc + "。安装完成后，首次访问并注册的用户将成为管理员。运行数据（数据库、上传文件、日志）保存在应用数据目录中。"
}

// RenderFnpackFiles 渲染 fnpack 打包目录的全部文本文件（manifest 与图标除外：
// manifest 由 RenderFnpackManifest 生成，图标无法从清单推导、必须人工按
// icon-design 技能放置）。cmd/ 下的生命周期脚本带可执行位。
func (m *Manifest) RenderFnpackFiles() (map[string]RenderedFile, error) {
	data := tmplData{
		Name:        m.Name,
		Pkg:         m.Pkg(),
		DisplayName: m.DisplayName,
		Port:        fmt.Sprintf("%d", m.Port),
		Gateway:     m.Gateway(),
		Desc:        m.Desc,
		Intro:       m.IntroText(),
	}
	out := map[string]RenderedFile{}
	err := fs.WalkDir(fnpackTmplFS, "tmpl/fnpack", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := fnpackTmplFS.ReadFile(p)
		if err != nil {
			return err
		}
		tpl, err := template.New(path.Base(p)).Option("missingkey=error").Parse(string(raw))
		if err != nil {
			return fmt.Errorf("模板 %s 解析失败: %w", p, err)
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("模板 %s 渲染失败: %w", p, err)
		}
		rel := strings.TrimPrefix(p, "tmpl/fnpack/")
		// 模板即输出：渲染结果与模板逐字节一致（含尾换行有无），
		// 金样对照测试要求零加工。
		out[rel] = RenderedFile{
			Rel:  rel,
			Data: buf.Bytes(),
			Exec: strings.HasPrefix(rel, "cmd/"),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
