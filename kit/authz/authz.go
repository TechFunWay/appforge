// Package authz 是「模块:动作」逐格权限的通用决策引擎。
//
// 设计源自用药管理（medcare）2026-11-14 的落地模式：家庭成员分工差异大
// （老人只看不改、有人只管买药入库、有人不该看到处方单），角色三档不够用，
// 于是把权限拆成能力格，管理员逐格勾选；角色只提供默认值，显式勾选覆盖默认。
// 本包只负责纯内存判定与序列化约定，不碰数据库、不认识任何业务表：
// 「当前用户在当前范围的角色与勾选」由应用侧中间件解析后交给守卫。
//
// 存储约定（应用侧成员表的 permissions 列照此实现即可）：
//
//   - 空串        跟随角色默认（没有单独勾选）
//   - "[]"        显式零权限（连看都看不到）
//   - JSON 数组    管理员显式勾选的能力键
//   - 脏数据      按「跟随角色默认」处理——绝不让一条坏数据把人锁在门外
package authz

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Action 是矩阵里的一格。Key 形如 "medicine:delete"。
type Action struct {
	Key   string `json:"key"`
	Act   string `json:"action"`
	Label string `json:"label"`
}

// Module 是矩阵里的一行。前端按 Modules 渲染勾选表格：
// 新增模块或动作只改这张表，判定与文案都不动别处。
type Module struct {
	Module  string   `json:"module"`
	Label   string   `json:"label"`
	Hint    string   `json:"hint"`
	Actions []Action `json:"actions"`
}

// Options 是矩阵的构造输入。
type Options struct {
	// Modules 权威能力清单，顺序即展示与序列化顺序。必填。
	Modules []Module
	// RoleDefaults 角色 → 默认勾选的能力键；未列出的角色默认空集。
	RoleDefaults map[string][]string
	// SuperRoles 恒为全勾、不受 override 削减的角色（如 owner/admin）。
	SuperRoles []string
	// Legacy 旧粗粒度键 → 展开后的能力键（升级时不清零既有勾选）。
	Legacy map[string][]string
	// LegacyBase 旧键展开后自动补上的基础可见键（旧模型里「能看」不言自明）。
	LegacyBase []string
}

// Matrix 是编译后的只读决策器。构造时全量校验，带病配置起不了服务。
type Matrix struct {
	modules      []Module
	order        map[string]int
	labels       map[string]string
	roleDefaults map[string]map[string]bool
	superRoles   map[string]bool
	legacy       map[string][]string
	legacyBase   []string
	keys         []string
}

// New 校验配置并构建决策器。
func New(opts Options) (*Matrix, error) {
	m := &Matrix{
		modules:      opts.Modules,
		order:        map[string]int{},
		labels:       map[string]string{},
		roleDefaults: map[string]map[string]bool{},
		superRoles:   map[string]bool{},
		legacy:       map[string][]string{},
		legacyBase:   append([]string(nil), opts.LegacyBase...),
	}
	if len(opts.Modules) == 0 {
		return nil, errors.New("authz: 能力清单不能为空")
	}
	seen := map[string]bool{}
	for _, mod := range opts.Modules {
		if strings.TrimSpace(mod.Module) == "" || len(mod.Actions) == 0 {
			return nil, fmt.Errorf("authz: 模块 %q 缺少名字或动作", mod.Module)
		}
		for _, a := range mod.Actions {
			if !strings.HasPrefix(a.Key, mod.Module+":") {
				return nil, fmt.Errorf("authz: 能力键 %s 与模块 %s 不匹配", a.Key, mod.Module)
			}
			if seen[a.Key] {
				return nil, fmt.Errorf("authz: 能力键重复 %s", a.Key)
			}
			seen[a.Key] = true
			m.order[a.Key] = len(m.keys)
			m.labels[a.Key] = mod.Label + "·" + a.Label
			m.keys = append(m.keys, a.Key)
		}
	}
	for role, list := range opts.RoleDefaults {
		set := map[string]bool{}
		for _, k := range list {
			if _, ok := m.order[k]; !ok {
				return nil, fmt.Errorf("authz: 角色 %s 的默认能力 %q 不在清单里", role, k)
			}
			set[k] = true
		}
		m.roleDefaults[role] = set
	}
	for _, r := range opts.SuperRoles {
		m.superRoles[r] = true
		set := map[string]bool{}
		for k := range m.order {
			set[k] = true
		}
		m.roleDefaults[r] = set
	}
	for old, list := range opts.Legacy {
		if _, ok := m.order[old]; ok {
			return nil, fmt.Errorf("authz: 旧键 %q 与新能力键冲突", old)
		}
		for _, k := range list {
			if _, ok := m.order[k]; !ok {
				return nil, fmt.Errorf("authz: 旧键 %q 展开出的 %q 不在清单里", old, k)
			}
		}
		m.legacy[old] = append([]string(nil), list...)
	}
	for _, k := range m.legacyBase {
		if _, ok := m.order[k]; !ok {
			return nil, fmt.Errorf("authz: LegacyBase 中的 %q 不在清单里", k)
		}
	}
	return m, nil
}

// Keys 返回全部能力键（权威顺序）。
func (m *Matrix) Keys() []string { return append([]string(nil), m.keys...) }

// Modules 返回权威清单副本（前端渲染用）。
func (m *Matrix) Modules() []Module { return m.modules }

// Label 返回能力格的中文文案（报错提示用）。
func (m *Matrix) Label(key string) string {
	if l, ok := m.labels[key]; ok {
		return l
	}
	return key
}

// IsKnown 判断能力键是否在权威清单里。
func (m *Matrix) IsKnown(key string) bool { _, ok := m.order[key]; return ok }

// Allowed 判定：超管角色恒真；有显式勾选按勾选；否则按角色默认。
// override 传 ParseOverride 的结果，explicit=false 时忽略 keys。
func (m *Matrix) Allowed(role string, keys []string, explicit bool, key string) bool {
	if m.superRoles[role] {
		return true
	}
	if explicit {
		for _, k := range keys {
			if k == key {
				return true
			}
		}
		return false
	}
	return m.roleDefaults[role][key]
}

// ExpandLegacy 把旧键换成新键；没有旧键时原样返回。
// 发生过展开时补上 LegacyBase（旧模型里「能看」不言自明）。
func (m *Matrix) ExpandLegacy(in []string) []string {
	hasLegacy := false
	for _, k := range in {
		if _, hit := m.legacy[strings.TrimSpace(k)]; hit {
			hasLegacy = true
			break
		}
	}
	if !hasLegacy {
		return in
	}
	out := make([]string, 0, len(in)+8)
	for _, raw := range in {
		key := strings.TrimSpace(raw)
		if mapped, hit := m.legacy[key]; hit {
			out = append(out, mapped...)
			continue
		}
		out = append(out, key)
	}
	return append(out, m.legacyBase...)
}

// Normalize 校验并整理提交的勾选：旧键展开、去空去重、按权威顺序排列；
// 出现不认识的能力键直接报错——这是人提交的输入，宁可退回重填。
func (m *Matrix) Normalize(in []string) ([]string, error) {
	expanded := m.ExpandLegacy(in)
	seen := map[string]bool{}
	out := make([]string, 0, len(expanded))
	for _, raw := range expanded {
		key := strings.TrimSpace(raw)
		if key == "" || seen[key] {
			continue
		}
		if !m.IsKnown(key) {
			return nil, fmt.Errorf("包含不认识的能力项：%s", key)
		}
		seen[key] = true
		out = append(out, key)
	}
	sort.SliceStable(out, func(i, j int) bool { return m.order[out[i]] < m.order[out[j]] })
	return out, nil
}

// ParseOverride 解析成员上存的勾选列。空串/脏数据 → explicit=false（跟随角色默认）。
func (m *Matrix) ParseOverride(raw string) (keys []string, explicit bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, false
	}
	var parsed []string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false
	}
	normalized, err := m.Normalize(parsed)
	if err != nil {
		// 库里存了不认识的历史键：丢掉坏键、保留可识别部分，不整体作废。
		keep := make([]string, 0, len(parsed))
		for _, p := range parsed {
			if k := strings.TrimSpace(p); m.IsKnown(k) {
				keep = append(keep, k)
			}
		}
		normalized, _ = m.Normalize(keep)
	}
	return normalized, true
}

// Encode 把勾选集合序列化成入库文本；nil 与空集合都写成 "[]"（显式零权限），
// 空集合要落库为 [] 而不是空串——两者语义不同。
func (m *Matrix) Encode(keys []string) (string, error) {
	normalized, err := m.Normalize(keys)
	if err != nil {
		return "", err
	}
	if normalized == nil {
		normalized = []string{}
	}
	b, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DefaultFor 返回角色的默认勾选（权威顺序），供界面展示「跟随角色」时的实际效果。
func (m *Matrix) DefaultFor(role string) []string {
	out := make([]string, 0, len(m.keys))
	for _, k := range m.keys {
		if m.roleDefaults[role][k] {
			out = append(out, k)
		}
	}
	return out
}
