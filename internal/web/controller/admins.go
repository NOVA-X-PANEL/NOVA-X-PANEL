package controller

import (
	"errors"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

// errSelfAction rejects an admin acting on their own account in a way that
// would lock them out.
func errSelfAction() error {
	return errors.New("cannot perform this action on your own account")
}

// RBAC (NOVA X PANEL) — panel account management API.

type AdminController struct {
	adminService panel.AdminService
}

func NewAdminController(g *gin.RouterGroup) *AdminController {
	a := &AdminController{}
	a.initRouter(g)
	return a
}

func (a *AdminController) initRouter(g *gin.RouterGroup) {
	g.GET("/current", a.current)
	g.GET("/list", requireAdminPermission("admins", "view"), a.list)
	g.GET("/stats", requireAdminPermission("admins", "view"), a.stats)
	g.GET("/get/:id", requireAdminPermission("admins", "view"), a.get)
	g.POST("/add", requireAdminPermission("admins", "create"), a.add)
	g.POST("/update/:id", requireAdminPermission("admins", "update"), a.update)
	g.POST("/del/:id", requireAdminPermission("admins", "delete"), a.del)
	g.POST("/enable/:id", requireAdminPermission("admins", "update"), a.enable)
	g.POST("/disable/:id", requireAdminPermission("admins", "update"), a.disable)
	g.POST("/resetUsage/:id", requireAdminPermission("admins", "reset_usage"), a.resetUsage)
	g.POST("/users/disableActive/:id", requireAdminPermission("users", "update"), a.disableActiveUsers)
	g.POST("/users/activateDisabled/:id", requireAdminPermission("users", "update"), a.activateDisabledUsers)
	g.POST("/users/removeAll/:id", requireAdminPermission("users", "delete"), a.removeAllUsers)
}

// current returns the logged-in account with its role document.
func (a *AdminController) current(c *gin.Context) {
	user, role, ok := loginActiveAdminRole(c)
	if !ok {
		return
	}
	jsonObj(c, gin.H{
		"id":          user.Id,
		"username":    user.Username,
		"status":      user.Status,
		"roleId":      user.RoleId,
		"role_id":     user.RoleId,
		"permissions": role.PermissionsJSON,
		"limits":      role.LimitsJSON,
		"features":    role.FeaturesJSON,
		"access":      role.AccessJSON,
		"isOwner":     role.OwnerRole,
		"role": gin.H{
			"id":          role.Id,
			"name":        role.Name,
			"slug":        role.Slug,
			"is_owner":    role.OwnerRole,
			"ownerRole":   role.OwnerRole,
			"owner_role":  role.OwnerRole,
			"permissions": role.PermissionsJSON,
			"limits":      role.LimitsJSON,
			"features":    role.FeaturesJSON,
			"access":      role.AccessJSON,
		},
	}, nil)
}

func (a *AdminController) list(c *gin.Context) {
	self := 0
	if user := session.GetLoginUser(c); user != nil {
		self = user.Id
	}
	rows, err := a.adminService.ListFor(self)
	jsonObj(c, rows, err)
}

func (a *AdminController) stats(c *gin.Context) {
	stats, err := a.adminService.Stats()
	jsonObj(c, stats, err)
}

func (a *AdminController) get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	row, err := a.adminService.Get(id)
	jsonObj(c, row, err)
}

func (a *AdminController) add(c *gin.Context) {
	var payload panel.AdminPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		jsonMsg(c, "create admin", err)
		return
	}
	row, err := a.adminService.Create(payload)
	jsonMsgObj(c, "create admin", row, err)
}

func (a *AdminController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	var payload panel.AdminPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		jsonMsg(c, "update admin", err)
		return
	}
	row, err := a.adminService.Update(id, payload)
	jsonMsgObj(c, "update admin", row, err)
}

func (a *AdminController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	if self := session.GetLoginUser(c); self != nil && self.Id == id {
		jsonMsg(c, "delete admin", errSelfAction())
		return
	}
	jsonMsg(c, "delete admin", a.adminService.Delete(id))
}

func (a *AdminController) enable(c *gin.Context) {
	a.setStatus(c, "active")
}

func (a *AdminController) disable(c *gin.Context) {
	a.setStatus(c, "disabled")
}

// resetUsage zeroes the traffic of every client owned by the admin.
func (a *AdminController) resetUsage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonMsg(c, "reset admin usage", a.adminService.ResetUsage(id))
}

// disableActiveUsers disables every enabled client owned by the admin.
func (a *AdminController) disableActiveUsers(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	count, err := a.adminService.DisableAllActiveUsers(id)
	jsonMsgObj(c, "disable admin active users", gin.H{"changed": count}, err)
}

// activateDisabledUsers enables every disabled client owned by the admin.
func (a *AdminController) activateDisabledUsers(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	count, err := a.adminService.ActivateAllDisabledUsers(id)
	jsonMsgObj(c, "activate admin disabled users", gin.H{"changed": count}, err)
}

// removeAllUsers deletes every client owned by the admin.
func (a *AdminController) removeAllUsers(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	count, err := a.adminService.RemoveAllUsers(id)
	jsonMsgObj(c, "remove admin users", gin.H{"deleted": count}, err)
}

func (a *AdminController) setStatus(c *gin.Context, status string) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	if status == "disabled" {
		if self := session.GetLoginUser(c); self != nil && self.Id == id {
			jsonMsg(c, "set admin status", errSelfAction())
			return
		}
	}
	row, err := a.adminService.SetStatus(id, status)
	jsonMsgObj(c, "set admin status", row, err)
}
