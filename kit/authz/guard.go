package authz

import (
	"github.com/TechFunWay/appforge/kit/response"
	"github.com/gin-gonic/gin"
)

// 应用侧中间件解析出「当前用户在本范围的身份」后，按这两个键写进 gin.Context，
// 守卫只认这两样：角色名与成员表 permissions 列的原始文本。
const (
	// CtxRole 成员在本范围的角色名（字符串）。
	CtxRole = "authzRole"
	// CtxPerms 显式勾选的 JSON 原文（字符串；空串表示跟随角色默认）。
	CtxPerms = "authzPerms"
)

// Put 供应用侧中间件写入上下文（集中键名防拼错）。
func Put(c *gin.Context, role, permsRaw string) {
	c.Set(CtxRole, role)
	c.Set(CtxPerms, permsRaw)
}

// AllowedFor 按 gin 上下文判定单个能力格。上下文缺 role 时按空角色处理
// （非超管即拒绝）——路由忘了挂解析中间件必须失败关闭，不能放行。
func (m *Matrix) AllowedFor(c *gin.Context, key string) bool {
	role := c.GetString(CtxRole)
	if m.superRoles[role] {
		return true
	}
	keys, explicit := m.ParseOverride(c.GetString(CtxPerms))
	if !explicit {
		return m.roleDefaults[role][key]
	}
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}

// Require 生成路由守卫：无格即 403（文案带能力中文名）。
// 成员身份与 404 边界（非成员不暴露资源存在性）由应用侧先行处理。
func (m *Matrix) Require(key string) gin.HandlerFunc {
	label := m.Label(key)
	return func(c *gin.Context) {
		if m.AllowedFor(c, key) {
			c.Next()
			return
		}
		response.ErrorForbidden(c, "管理员没有把「"+label+"」的权限给你，需要的话请联系管理员")
		c.Abort()
	}
}

// RequireAll 生成要求多格全中的守卫（交叉动作：如处方确认另需药品新增）。
func (m *Matrix) RequireAll(keys ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, key := range keys {
			if !m.AllowedFor(c, key) {
				response.ErrorForbidden(c, "管理员没有把「"+m.Label(key)+"」的权限给你，需要的话请联系管理员")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// Effective 返回上下文的最终勾选集（权威顺序），供「我的权限」接口与界面回显。
func (m *Matrix) Effective(c *gin.Context) []string {
	role := c.GetString(CtxRole)
	if m.superRoles[role] {
		return m.Keys()
	}
	if keys, explicit := m.ParseOverride(c.GetString(CtxPerms)); explicit {
		return keys
	}
	return m.DefaultFor(role)
}
