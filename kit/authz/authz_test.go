package authz

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// demoMatrix 覆盖全部特性面：两模块六格、一个超管角色、editor/viewer 两档默认、一个旧键展开。
func demoMatrix(t *testing.T) *Matrix {
	t.Helper()
	m, err := New(Options{
		Modules: []Module{
			{Module: "item", Label: "条目", Actions: []Action{
				{Key: "item:view", Act: "view", Label: "查看"},
				{Key: "item:create", Act: "create", Label: "新增"},
				{Key: "item:edit", Act: "edit", Label: "修改"},
				{Key: "item:delete", Act: "delete", Label: "删除"},
			}},
			{Module: "member", Label: "成员", Actions: []Action{
				{Key: "member:view", Act: "view", Label: "查看"},
				{Key: "member:manage", Act: "manage", Label: "管理"},
			}},
		},
		RoleDefaults: map[string][]string{
			"editor": {"item:view", "item:create", "item:edit", "member:view"},
			"viewer": {"item:view", "member:view"},
		},
		SuperRoles: []string{"owner"},
		Legacy:     map[string][]string{"item": {"item:view", "item:create", "item:edit", "item:delete"}},
		LegacyBase: []string{"member:view"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

func TestNewRejectsBadConfigs(t *testing.T) {
	cases := []struct {
		name string
		opts Options
	}{
		{"空清单", Options{}},
		{"键与模块不匹配", Options{Modules: []Module{{Module: "a", Actions: []Action{{Key: "b:x"}}}}}},
		{"重复键", Options{Modules: []Module{{Module: "a", Actions: []Action{{Key: "a:x"}, {Key: "a:x"}}}}}},
		{"角色默认引用未知键", Options{Modules: []Module{{Module: "a", Actions: []Action{{Key: "a:x"}}}}, RoleDefaults: map[string][]string{"r": {"a:missing"}}}},
		{"旧键与新键冲突", Options{Modules: []Module{{Module: "a", Actions: []Action{{Key: "a:x"}}}}, Legacy: map[string][]string{"a:x": {"a:x"}}}},
		{"LegacyBase 未知键", Options{Modules: []Module{{Module: "a", Actions: []Action{{Key: "a:x"}}}}, LegacyBase: []string{"a:nope"}}},
	}
	for _, tc := range cases {
		if _, err := New(tc.opts); err == nil {
			t.Fatalf("%s：坏配置应报错", tc.name)
		}
	}
}

func TestRoleDefaultsAndSuper(t *testing.T) {
	m := demoMatrix(t)
	if !m.Allowed("editor", nil, false, "item:create") {
		t.Fatal("editor 应有新增")
	}
	if m.Allowed("editor", nil, false, "item:delete") {
		t.Fatal("editor 不应有删除")
	}
	if m.Allowed("unknown-role", nil, false, "item:view") {
		t.Fatal("未配置角色默认空集")
	}
	if !m.Allowed("owner", []string{}, true, "member:manage") {
		t.Fatal("owner 恒应全勾，显式削减无效")
	}
	if len(m.Keys()) != 6 {
		t.Fatalf("全键数 %d", len(m.Keys()))
	}
}

func TestExplicitOverrideAndEmptyArray(t *testing.T) {
	m := demoMatrix(t)
	keys, explicit := m.ParseOverride(`["item:view","item:delete"]`)
	if !explicit || !reflect.DeepEqual(keys, []string{"item:view", "item:delete"}) {
		t.Fatalf("显式勾选解析错: %v %v", keys, explicit)
	}
	if !m.Allowed("editor", keys, explicit, "item:delete") {
		t.Fatal("显式删除应放行")
	}
	if m.Allowed("editor", keys, explicit, "item:create") {
		t.Fatal("显式勾选后角色默认不再叠加")
	}
	emptyKeys, emptyExplicit := m.ParseOverride("[]")
	if !emptyExplicit || len(emptyKeys) != 0 {
		t.Fatal("[] 应解析为显式空集（连看都没有）")
	}
	if m.Allowed("viewer", emptyKeys, emptyExplicit, "item:view") {
		t.Fatal("显式零权限不应可见")
	}
}

func TestDirtyDataFollowsRoleDefault(t *testing.T) {
	m := demoMatrix(t)
	if _, explicit := m.ParseOverride("坏数据{{{"); explicit {
		t.Fatal("脏数据应视为无显式勾选，跟随角色默认")
	}
	keys, explicit := m.ParseOverride(`["item:view","ghost:key"]`)
	if !explicit || !reflect.DeepEqual(keys, []string{"item:view"}) {
		t.Fatalf("坏键应被丢弃但保留好键: %v", keys)
	}
}

func TestLegacyExpansion(t *testing.T) {
	m := demoMatrix(t)
	got, err := m.Normalize([]string{"item"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"item:view", "item:create", "item:edit", "item:delete", "member:view"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("旧键展开错误:\n got %v\nwant %v", got, want)
	}
	got, err = m.Normalize([]string{"item:view"})
	if err != nil || !reflect.DeepEqual(got, []string{"item:view"}) {
		t.Fatalf("无旧键路径不应补 LegacyBase: %v %v", got, err)
	}
}

func TestNormalizeRejectsUnknown(t *testing.T) {
	m := demoMatrix(t)
	if _, err := m.Normalize([]string{"item:view", "ghost:act"}); err == nil || !strings.Contains(err.Error(), "ghost:act") {
		t.Fatalf("未知键应报错并点名: %v", err)
	}
}

func TestEncodeRoundTripAndOrdering(t *testing.T) {
	m := demoMatrix(t)
	raw, err := m.Encode([]string{"member:view", "item:view", "member:view"})
	if err != nil {
		t.Fatal(err)
	}
	if raw != `["item:view","member:view"]` {
		t.Fatalf("应按权威顺序去重序列化: %s", raw)
	}
	empty, err := m.Encode(nil)
	if err != nil || empty != "[]" {
		t.Fatalf("空集合应编码为 []：%q %v", empty, err)
	}
	back, explicit := m.ParseOverride(raw)
	if !explicit || !reflect.DeepEqual(back, []string{"item:view", "member:view"}) {
		t.Fatal("编解码往返不一致")
	}
}

func TestDefaultForAndLabels(t *testing.T) {
	m := demoMatrix(t)
	if got := m.DefaultFor("viewer"); !reflect.DeepEqual(got, []string{"item:view", "member:view"}) {
		t.Fatalf("viewer 默认错: %v", got)
	}
	if len(m.DefaultFor("owner")) != 6 {
		t.Fatal("owner 默认应全六格")
	}
	if m.Label("item:delete") != "条目·删除" {
		t.Fatalf("文案拼接错: %s", m.Label("item:delete"))
	}
	if m.Label("nope") != "nope" {
		t.Fatal("未知键文案应回落原键")
	}
}

// ===== 守卫 =====

func callGuard(t *testing.T, m *Matrix, key, role, perms string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x",
		func(c *gin.Context) { Put(c, role, perms) },
		m.Require(key),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	return w.Code
}

func TestGuardMatrix(t *testing.T) {
	m := demoMatrix(t)
	cases := []struct {
		key, role, perms string
		want             int
	}{
		{"item:create", "editor", "", http.StatusOK},
		{"item:delete", "editor", "", http.StatusForbidden},
		{"item:delete", "editor", `["item:delete"]`, http.StatusOK},
		{"item:view", "viewer", "[]", http.StatusForbidden},
		{"member:manage", "owner", "[]", http.StatusOK},
		{"item:view", "", "", http.StatusForbidden},
		{"item:view", "ghost", "", http.StatusForbidden},
	}
	for i, tc := range cases {
		if got := callGuard(t, m, tc.key, tc.role, tc.perms); got != tc.want {
			t.Fatalf("用例%d %s/%s/%s: %d want %d", i, tc.key, tc.role, tc.perms, got, tc.want)
		}
	}
}

func TestGuardForbiddenMessageCarriesLabel(t *testing.T) {
	m := demoMatrix(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", func(c *gin.Context) { Put(c, "viewer", "") }, m.Require("item:delete"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.Message, "条目·删除") {
		t.Fatalf("403 文案应带能力中文名: %q", body.Message)
	}
}

func TestRequireAllCrossAction(t *testing.T) {
	m := demoMatrix(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/a",
		func(c *gin.Context) { Put(c, "editor", "") },
		m.RequireAll("item:create", "member:manage"),
		func(c *gin.Context) { c.JSON(http.StatusOK, nil) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/a", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("缺 member:manage 应 403: %d", w.Code)
	}
	r2 := gin.New()
	r2.GET("/a",
		func(c *gin.Context) { Put(c, "owner", "") },
		m.RequireAll("item:create", "member:manage"),
		func(c *gin.Context) { c.JSON(http.StatusOK, nil) })
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, httptest.NewRequest("GET", "/a", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("owner 两格齐应放行: %d", w2.Code)
	}
}
