package manifest

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// RenderVersionFile 生成 VERSION 文件内容：带 v 前缀的单行版本号。
func (m *Manifest) RenderVersionFile() []byte {
	return []byte(m.Tag() + "\n")
}

// appJSONDoc / dockerDoc / fnosDoc 决定 app.json 的字段顺序，
// 与家族存量应用（medcare 等）保持逐字节一致的结构。
type appJSONDoc struct {
	AppName     string    `json:"appname"`
	DisplayName string    `json:"display_name"`
	Version     string    `json:"version"`
	Desc        string    `json:"desc"`
	Author      string    `json:"author"`
	Website     string    `json:"website"`
	Port        int       `json:"port"`
	Docker      dockerDoc `json:"docker"`
	FnOS        fnosDoc   `json:"fnos"`
}

type dockerDoc struct {
	Image     string `json:"image"`
	Container string `json:"container"`
	TagPrefix string `json:"tag_prefix"`
}

type fnosDoc struct {
	OSMinVersion          string `json:"os_min_version"`
	ServicePort           int    `json:"service_port"`
	CtlStop               bool   `json:"ctl_stop"`
	CheckPort             bool   `json:"checkport"`
	DisableAuthorizationP bool   `json:"disable_authorization_path"`
}

// RenderAppJSON 生成 app.json：两空格缩进、结尾换行。
func (m *Manifest) RenderAppJSON() ([]byte, error) {
	doc := appJSONDoc{
		AppName:     m.Pkg(),
		DisplayName: m.DisplayName,
		Version:     m.Version,
		Desc:        m.Desc,
		Author:      m.Author_(),
		Website:     m.Website_(),
		Port:        m.Port,
		Docker: dockerDoc{
			Image:     m.Image(),
			Container: m.Pkg(),
			TagPrefix: m.Pkg(),
		},
		FnOS: fnosDoc{
			OSMinVersion:          m.OSMin_(),
			ServicePort:           m.Port,
			CtlStop:               m.CtlStop_(),
			CheckPort:             m.CheckPort_(),
			DisableAuthorizationP: m.DisableAuthz_(),
		},
	}
	buf, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("生成 app.json 失败: %w", err)
	}
	return append(buf, '\n'), nil
}

// fnpack/manifest 的 key 列宽（含尾随空格到 27 列，再跟 "= 值"），
// 与飞牛现有包保持对齐风格。
const fnpackKeyWidth = 27

func fnpackLine(key, value string) string {
	return fmt.Sprintf("%-*s= %s", fnpackKeyWidth, key, value)
}

// RenderFnpackManifest 生成 fnpack/manifest：固定字段顺序，
// changelog 块用三引号多行字符串；changelog 为空时省略该块。
func (m *Manifest) RenderFnpackManifest(changelog string) []byte {
	// fnpack 的 manifest 格式不支持注释行，「派生自哪里」写进应用 README。
	var out strings.Builder
	write := func(k, v string) { out.WriteString(fnpackLine(k, v) + "\n") }
	write("appname", m.Pkg())
	write("version", m.Version)
	write("display_name", m.DisplayName)
	write("desc", m.Desc)
	write("source", "thirdparty")
	write("platform", m.Platform_())
	write("maintainer", m.Maintainer_())
	write("maintainer_url", "https://github.com/"+m.ReleaseRepo())
	write("distributor", m.Distributor_())
	write("distributor_url", m.Website_())
	write("os_min_version", m.OSMin_())
	write("ctl_stop", fmt.Sprintf("%t", m.CtlStop_()))
	write("desktop_uidir", "ui")
	write("desktop_applaunchname", m.DesktopLaunchName())
	write("service_port", fmt.Sprintf("%d", m.Port))
	write("checkport", fmt.Sprintf("%t", m.CheckPort_()))
	write("disable_authorization_path", fmt.Sprintf("%t", m.DisableAuthz_()))
	if cl := strings.TrimRight(changelog, "\n"); cl != "" {
		out.WriteString(fnpackLine("changelog", `"""`))
		out.WriteString("\n")
		out.WriteString(cl)
		out.WriteString("\n")
		out.WriteString(`"""`)
		out.WriteString("\n")
	}
	return []byte(out.String())
}

// changelog 段落标题：## v0.1.37 / ## 0.1.37（可带（未发布）等后缀，只取版本号本体）。
var (
	changelogHeadRE = regexp.MustCompile(`^##+ +(v?([0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)).*$`)
	changelogItemRE = regexp.MustCompile(`^(?:- |[0-9]+\. )`)
)

// ExtractChangelog 从 CHANGELOG.md 提取飞牛包更新日志：
// 每个 "## v" 小节输出「vX.Y.Z 更新：」加其下的条目行（"- " 或 "1. " 开头），
// 跳过 ### 子标题（其下条目照样收集）。不传版本号上限时全部收录。
// 找不到任何小节时返回空串——调用方据此决定省略还是报错。
func ExtractChangelog(markdown string) string {
	var sections []string
	var cur []string
	curTitle := ""
	flush := func() {
		if curTitle != "" && len(cur) > 0 {
			sections = append(sections, curTitle+" 更新：\n"+strings.Join(cur, "\n"))
		}
		cur, curTitle = nil, ""
	}
	for _, line := range strings.Split(markdown, "\n") {
		t := strings.TrimSpace(line)
		if m := changelogHeadRE.FindStringSubmatch(t); m != nil {
			flush()
			ver := m[2]
			if !strings.HasPrefix(ver, "v") {
				ver = "v" + ver
			}
			curTitle = ver
			continue
		}
		if curTitle == "" {
			continue
		}
		if changelogItemRE.MatchString(t) {
			cur = append(cur, t)
		}
	}
	flush()
	return strings.Join(sections, "\n\n")
}
