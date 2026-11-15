// Package manifest 提供应用清单（appmanifest.json）的解析、校验与派生，
// 是应用身份与打包元数据的唯一真源。
//
// 一个应用只在 appmanifest.json 里声明一次名字、显示名、版本、描述与端口；
// 运行时经 Inject 注入 kit/version 的身份体系（网关前缀、数据库文件名、
// 发版仓库都由它推导），构建期由 cmd/gen-manifest 派生 VERSION、app.json 与
// fnpack/manifest 三份打包文件。CHANGELOG.md 是更新日志的真源，fnpack 的
// changelog 块从它提取。
//
// 应用侧接线（脚手架已生成）：
//
//	//go:embed appmanifest.json
//	var manifestData []byte
//
//	func main() {
//		manifest.MustParse(manifestData).Inject()
//		...
//	}
package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/TechFunWay/appforge/kit/version"
)

// Manifest 是应用清单。必填：Name、DisplayName、Version、Desc、Port。
// 其余留空按家族惯例推导（见各 * 派生方法）。
type Manifest struct {
	// Name 是应用短名，等于仓库名（如 medcare、kitdemo）。
	Name string `json:"name"`
	// DisplayName 是中文显示名（如「用药管理」）。
	DisplayName string `json:"displayName"`
	// Version 是不带 v 前缀的语义化版本号（如 0.1.37）。
	Version string `json:"version"`
	// Desc 是一句话应用描述，进 app.json 与 fnpack。
	Desc string `json:"desc"`
	// Port 是默认监听端口。
	Port int `json:"port"`

	// PkgName 是飞牛包名 / 进程名；留空推导为 "techfunway-" + Name。
	PkgName string `json:"pkgName,omitempty"`
	// GatewayPrefix 是飞牛统一网关前缀；留空推导为 "/app/" + 包名。
	GatewayPrefix string `json:"gatewayPrefix,omitempty"`
	// DockerImage 是镜像名；留空推导为 "techfunways/" + Name。
	DockerImage string `json:"dockerImage,omitempty"`
	// UpdateRepo 是发版仓库 owner/repo；留空推导为 "TechFunWay/" + Name。
	UpdateRepo string `json:"updateRepo,omitempty"`

	Author        string `json:"author,omitempty"`
	Website       string `json:"website,omitempty"`
	Maintainer    string `json:"maintainer,omitempty"`
	Distributor   string `json:"distributor,omitempty"`
	OSMinVersion  string `json:"osMinVersion,omitempty"`
	Platform      string `json:"platform,omitempty"`
	CtlStop       *bool  `json:"ctlStop,omitempty"`
	CheckPort     *bool  `json:"checkPort,omitempty"`
	DisableAuthzP *bool  `json:"disableAuthorizationPath,omitempty"`
}

// 家族惯例默认值。
const (
	pkgPrefix       = "techfunway-"
	dockerPrefix    = "techfunways/"
	releaseOwner    = "TechFunWay"
	defaultAuthor   = "TechFunWay"
	defaultWebsite  = "https://techfunway.wycto.cn"
	defaultHouse    = "科技智趣坊"
	defaultOSMin    = "1.0.0"
	defaultPlatform = "x86"
)

var (
	nameRE    = regexp.MustCompile(`^[a-z][a-z0-9-]{1,38}$`)
	versionRE = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)
)

// Parse 解析并校验清单 JSON。未知字段直接报错——防止字段名拼错后
// 静默走默认值，产出一份悄悄不同的打包元数据。
//
// 注意 encoding/json 的字段匹配对大小写不敏感（displayname 会命中 displayName），
// 光靠 DisallowUnknownFields 拦不住大小写拼错，所以先按原始 key 做
// 大小写敏感的白名单校验（白名单由反射从结构体 tag 生成，加字段不会漏同步）。
func Parse(data []byte) (*Manifest, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("解析 appmanifest.json 失败: %w", err)
	}
	known := knownFields()
	var unknown []string
	for k := range raw {
		if !known[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("appmanifest.json 含未知字段：%s（字段名区分大小写）", strings.Join(unknown, "、"))
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("解析 appmanifest.json 失败: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("appmanifest.json 的 JSON 对象之后还有多余内容")
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// knownFields 反射收集结构体 json tag，作为大小写敏感的字段白名单。
func knownFields() map[string]bool {
	out := map[string]bool{}
	t := reflect.TypeOf(Manifest{})
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		out[strings.Split(tag, ",")[0]] = true
	}
	return out
}

// MustParse 解析清单，失败即 panic——用于 //go:embed 的启动期接线：
// 清单坏了就让应用起不来，绝不带着错误身份继续跑。
func MustParse(data []byte) *Manifest {
	m, err := Parse(data)
	if err != nil {
		panic(err)
	}
	return m
}

// Validate 校验必填项与取值范围，一次性报出全部问题。
func (m *Manifest) Validate() error {
	var errs []string
	if !nameRE.MatchString(m.Name) {
		errs = append(errs, "name 必填，且只能是小写字母、数字与连字符（2-39 位，字母开头）")
	}
	if strings.TrimSpace(m.DisplayName) == "" {
		errs = append(errs, "displayName 必填")
	}
	if !versionRE.MatchString(m.Version) {
		errs = append(errs, "version 必须是不带 v 前缀的语义化版本号（如 0.1.0）")
	}
	if strings.TrimSpace(m.Desc) == "" {
		errs = append(errs, "desc 必填")
	}
	if m.Port < 1 || m.Port > 65535 {
		errs = append(errs, "port 必须在 1-65535")
	}
	if m.PkgName != "" && !strings.HasPrefix(m.PkgName, pkgPrefix) {
		errs = append(errs, "pkgName 若显式指定必须以 techfunway- 开头（家族包名规范）")
	}
	if len(errs) > 0 {
		return errors.New("应用清单校验失败:\n- " + strings.Join(errs, "\n- "))
	}
	return nil
}

// Pkg 返回包名 / 进程名（进 app.json.appname、fnpack.appname、容器名）。
func (m *Manifest) Pkg() string {
	if m.PkgName != "" {
		return m.PkgName
	}
	return pkgPrefix + m.Name
}

// Gateway 返回飞牛统一网关前缀。
func (m *Manifest) Gateway() string {
	if m.GatewayPrefix != "" {
		return m.GatewayPrefix
	}
	return "/app/" + m.Pkg()
}

// Image 返回 Docker 镜像名。
func (m *Manifest) Image() string {
	if m.DockerImage != "" {
		return m.DockerImage
	}
	return dockerPrefix + m.Name
}

// ReleaseRepo 返回发版仓库 owner/repo。
func (m *Manifest) ReleaseRepo() string {
	if m.UpdateRepo != "" {
		return m.UpdateRepo
	}
	return releaseOwner + "/" + m.Name
}

// Tag 返回带 v 前缀的版本号（VERSION 文件与 ldflags 注入的形态）。
func (m *Manifest) Tag() string { return "v" + m.Version }

// DesktopLaunchName 返回飞牛桌面入口标识。
func (m *Manifest) DesktopLaunchName() string { return m.Pkg() + ".main" }

// 打包元数据的固定项。
func (m *Manifest) Author_() string      { return def(m.Author, defaultAuthor) }
func (m *Manifest) Website_() string     { return def(m.Website, defaultWebsite) }
func (m *Manifest) Maintainer_() string  { return def(m.Maintainer, defaultHouse) }
func (m *Manifest) Distributor_() string { return def(m.Distributor, m.Maintainer_()) }
func (m *Manifest) OSMin_() string       { return def(m.OSMinVersion, defaultOSMin) }
func (m *Manifest) Platform_() string    { return def(m.Platform, defaultPlatform) }
func (m *Manifest) CtlStop_() bool       { return m.CtlStop == nil || *m.CtlStop }
func (m *Manifest) CheckPort_() bool     { return m.CheckPort != nil && *m.CheckPort }
func (m *Manifest) DisableAuthz_() bool  { return m.DisableAuthzP != nil && *m.DisableAuthzP }

func def(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// Identity 转成 kit/version 的应用身份。数据库文件名、短名等
// 由 version.Identity 既有规则继续推导，这里不重复实现。
func (m *Manifest) Identity() version.Identity {
	return version.Identity{
		Name:                 m.Pkg(),
		DisplayName:          m.DisplayName,
		DefaultPort:          m.Port,
		DefaultGatewayPrefix: m.Gateway(),
		UpdateRepo:           m.ReleaseRepo(),
	}
}

// Inject 把清单注入底座：身份走 version.Configure，版本号在 ldflags
// 未注入（仍是 dev）时以清单为准——VERSION 文件本身就派生自清单，
// 两条路径殊途同归。必须在 config.Parse 之前调用。
func (m *Manifest) Inject() {
	version.Configure(m.Identity())
	if version.Version == "" || version.Version == "dev" {
		version.Version = m.Tag()
	}
}
