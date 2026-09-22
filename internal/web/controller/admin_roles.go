package controller

import (
	"encoding/json"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"

	"github.com/gin-gonic/gin"
)

// RBAC (NOVA X PANEL) — admin role management API.

type AdminRoleController struct {
	roleService panel.AdminRoleService
}

func NewAdminRoleController(g *gin.RouterGroup) *AdminRoleController {
	a := &AdminRoleController{}
	a.initRouter(g)
	return a
}

func (a *AdminRoleController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", requireAdminPermission("roles", "view"), a.list)
	g.GET("/get/:id", requireAdminPermission("roles", "view"), a.get)
	g.POST("/add", requireAdminPermission("roles", "create"), a.add)
	g.POST("/update/:id", requireAdminPermission("roles", "update"), a.update)
	g.POST("/duplicate/:id", requireAdminPermission("roles", "create"), a.duplicate)
	g.POST("/del/:id", requireAdminPermission("roles", "delete"), a.del)
}

// actorIsOwner reports whether the calling account holds the owner role.
func actorIsOwner(role *model.AdminRole) bool {
	return role != nil && role.OwnerRole
}

// permissionsOfView recovers the permission map from a role view, whose
// Permissions field is decoded JSON rather than a typed map.
func permissionsOfView(view *panel.AdminRoleView) map[string]any {
	if view == nil {
		return nil
	}
	switch typed := view.Permissions.(type) {
	case map[string]any:
		return typed
	case string:
		var out map[string]any
		if err := json.Unmarshal([]byte(typed), &out); err == nil {
			return out
		}
	}
	return nil
}

func (a *AdminRoleController) list(c *gin.Context) {
	rows, err := a.roleService.List()
	jsonObj(c, rows, err)
}

func (a *AdminRoleController) get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	row, err := a.roleService.Get(id)
	jsonObj(c, row, err)
}

func (a *AdminRoleController) add(c *gin.Context) {
	var payload panel.AdminRolePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		jsonMsg(c, "create role", err)
		return
	}
	// Anti-escalation: a role may not grant more than its author already holds.
	if actor := actingRole(c); panel.PermissionsWidenActor(actor, payload.Permissions) {
		jsonMsg(c, "create role", errEscalation())
		return
	}
	row, err := a.roleService.Create(payload)
	jsonMsgObj(c, "create role", row, err)
}

func (a *AdminRoleController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	var payload panel.AdminRolePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		jsonMsg(c, "update role", err)
		return
	}
	actor := actingRole(c)
	if !actorIsOwner(actor) {
		existing, err := a.roleService.Get(id)
		if err != nil {
			jsonMsg(c, "update role", err)
			return
		}
		// The target role must already sit inside the actor's own grants...
		if panel.PermissionsWidenActor(actor, permissionsOfView(existing)) {
			jsonMsg(c, "update role", errEscalation())
			return
		}
		// ...and the edit may not add anything beyond them either, or an admin
		// could edit its own role into the administrator preset.
		if panel.PermissionsWidenActor(actor, payload.Permissions) {
			jsonMsg(c, "update role", errEscalation())
			return
		}
	}
	row, err := a.roleService.Update(id, payload)
	jsonMsgObj(c, "update role", row, err)
}

func (a *AdminRoleController) duplicate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	row, err := a.roleService.Duplicate(id)
	jsonMsgObj(c, "duplicate role", row, err)
}

func (a *AdminRoleController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonMsg(c, "delete role", a.roleService.Delete(id))
}
