package controller

import (
	"strconv"

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
