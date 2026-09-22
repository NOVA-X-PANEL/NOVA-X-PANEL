package controller

import (
	"errors"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

// errSelfAction rejects an admin acting on their own account in a way that
// would lock them out.
func errSelfAction() error {
	return errors.New("cannot perform this action on your own account")
}

// errEscalation rejects an attempt to grant more authority than the caller
// holds. Without it any account with admins.create or admin_roles.update could
// hand itself the administrator preset and take the panel over.
func errEscalation() error {
	return errors.New("cannot grant permissions beyond your own role")
}

// RBAC (NOVA X PANEL) — panel account management API.

// maxOwnApiTokens caps how many tokens one account may hold, so a scripted loop
// cannot fill the table.
const maxOwnApiTokens = 10

type AdminController struct {
	adminService    panel.AdminService
	apiTokenService panel.ApiTokenService
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

	// Self-service API tokens. Any account the owner has granted apiAccess may
	// mint a token that acts as itself: the token carries this account's role, so
	// it can never exceed what the account may already do in the panel.
	g.GET("/apiTokens", requirePanelAccount(), a.listOwnApiTokens)
	g.POST("/apiTokens/create", requirePanelAccount(), a.createOwnApiToken)
	g.POST("/apiTokens/delete/:id", requirePanelAccount(), a.deleteOwnApiToken)
	g.POST("/apiTokens/setEnabled/:id", requirePanelAccount(), a.setOwnApiTokenEnabled)
}

// errNoApiAccess is returned when an account without the permission tries to use
// the self-service token endpoints.
func errNoApiAccess() error {
	return errors.New("API access is not enabled for this account")
}

// apiAccessActor returns the acting account when it may manage its own tokens.
func (a *AdminController) apiAccessActor(c *gin.Context) (*model.User, *model.AdminRole, bool) {
	if admin := apiTokenAdmin(c); admin != nil {
		// A bound token may list and revoke its own tokens, which is how a
		// compromised token is rotated.
		if role, err := adminRoleOf(admin); err == nil {
			return admin, role, true
		}
	}
	if !session.Available(c) {
		return nil, nil, false
	}
	user, role, ok := loginActiveAdminRole(c)
	if !ok {
		return nil, nil, false
	}
	if role.OwnerRole || user.ApiAccess {
		return user, role, true
	}
	jsonMsg(c, "api access", errNoApiAccess())
	c.Abort()
	return nil, nil, false
}

// listOwnApiTokens returns only the caller's tokens.
func (a *AdminController) listOwnApiTokens(c *gin.Context) {
	user, _, ok := a.apiAccessActor(c)
	if !ok {
		return
	}
	rows, err := a.apiTokenService.ListForAdmin(user.Id)
	jsonObj(c, rows, err)
}

type ownApiTokenPayload struct {
	Name      string `json:"name"`
	ExpiresAt int64  `json:"expiresAt"`
}

// createOwnApiToken mints a token bound to the caller. The plaintext is in the
// response exactly once; only its hash is stored.
func (a *AdminController) createOwnApiToken(c *gin.Context) {
	user, _, ok := a.apiAccessActor(c)
	if !ok {
		return
	}
	var payload ownApiTokenPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		jsonMsg(c, "create api token", err)
		return
	}
	if count, err := a.apiTokenService.CountForAdmin(user.Id); err == nil && count >= maxOwnApiTokens {
		jsonMsg(c, "create api token", errors.New("token limit reached; delete one first"))
		return
	}
	row, err := a.apiTokenService.CreateForAdmin(payload.Name, user.Id, payload.ExpiresAt)
	if err != nil {
		jsonMsg(c, "create api token", err)
		return
	}
	logger.Infof("%s created an API token for its own account", user.Username)
	jsonMsgObj(c, "create api token", row, nil)
}

// deleteOwnApiToken removes one of the caller's tokens.
func (a *AdminController) deleteOwnApiToken(c *gin.Context) {
	user, _, ok := a.apiAccessActor(c)
	if !ok {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "delete api token", err)
		return
	}
	jsonMsg(c, "delete api token", a.apiTokenService.DeleteForAdmin(user.Id, id))
}

// setOwnApiTokenEnabled flips one of the caller's tokens on or off.
func (a *AdminController) setOwnApiTokenEnabled(c *gin.Context) {
	user, _, ok := a.apiAccessActor(c)
	if !ok {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "api token", err)
		return
	}
	body := struct {
		Enabled bool `json:"enabled"`
	}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, "api token", err)
		return
	}
	row, err := a.apiTokenService.SetEnabledForAdmin(user.Id, id, body.Enabled)
	jsonMsgObj(c, "api token", row, err)
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
		// Lets the shell decide whether to offer the account its own API page; the
		// owner always may.
		"apiAccess":  user.ApiAccess || role.OwnerRole,
		"api_access": user.ApiAccess || role.OwnerRole,
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

// actingRole resolves the role of the account making the call, going through the
// same paths the permission gate uses (session or admin-bound API token).
func actingRole(c *gin.Context) *model.AdminRole {
	if u := apiTokenAdmin(c); u != nil {
		if role, err := adminRoleOf(u); err == nil {
			return role
		}
		return nil
	}
	if !session.Available(c) {
		// Unit mounts and panel-wide API tokens have no account: treat as owner,
		// matching the convention the permission gate already uses.
		if gin.Mode() == gin.TestMode || c.GetBool("api_authed") {
			return &model.AdminRole{Name: "owner", Slug: model.AdminRoleSlugOwner, OwnerRole: true}
		}
		return nil
	}
	_, role, ok := loginActiveAdminRole(c)
	if !ok {
		return nil
	}
	return role
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
	// Anti-escalation: an admin may not mint an account more privileged than its
	// own role, or every account holding admins.create could become the owner.
	if actor := actingRole(c); actor != nil {
		target, err := a.adminService.RoleByID(payload.RoleId)
		if err != nil {
			jsonMsg(c, "create admin", err)
			return
		}
		if panel.RedirectsOrWidens(actor, target) {
			jsonMsg(c, "create admin", errEscalation())
			return
		}
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
	if self := session.GetLoginUser(c); self != nil && self.Id == id {
		// Self-service edits are scoped to password/username; a role change is how
		// an admin would promote itself, and enabling its own API access cannot be
		// allowed without a second pair of eyes either.
		if payload.RoleId > 0 && payload.RoleId != self.RoleId {
			jsonMsg(c, "update admin", errSelfAction())
			return
		}
		if payload.ApiAccess != nil {
			payload.ApiAccess = nil
		}
	}
	target, err := a.adminService.Get(id)
	if err != nil {
		jsonMsg(c, "update admin", err)
		return
	}
	if actor := actingRole(c); actor != nil && !actor.OwnerRole {
		if target.OwnerRole {
			jsonMsg(c, "update admin", errEscalation())
			return
		}
		if payload.RoleId > 0 && payload.RoleId != target.RoleId {
			role, rErr := a.adminService.RoleByID(payload.RoleId)
			if rErr != nil {
				jsonMsg(c, "update admin", rErr)
				return
			}
			if panel.RedirectsOrWidens(actor, role) {
				jsonMsg(c, "update admin", errEscalation())
				return
			}
		}
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
	// Disabling yourself is a lockout, not a permission decision.
	if self := session.GetLoginUser(c); self != nil {
		if id, err := strconv.Atoi(c.Param("id")); err == nil && id == self.Id {
			jsonMsg(c, "disable admin", errSelfAction())
			return
		}
	}
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
