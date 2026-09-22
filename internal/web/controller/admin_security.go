package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

// RBAC (NOVA X PANEL) — permission enforcement middleware.
//
// Every panel account is bound to one model.AdminRole whose PermissionsJSON is
// a resource → action → value map. These helpers resolve the login account's
// role and answer whether it may perform an action, mirroring the role schema
// stored in admin_roles.
//
// API-token callers: the token belongs to the owner, so token-authenticated
// requests bypass the role gate (they are already scoped by enforceTokenScope).
// Browser sessions are always checked unless the role is the owner role.

// loginActiveAdminRole resolves the logged-in account and its role. It writes a
// response and returns ok=false when the request must be aborted.
func loginActiveAdminRole(c *gin.Context) (*model.User, *model.AdminRole, bool) {
	if c.GetBool("api_authed") {
		pureJsonMsg(c, http.StatusForbidden, false, "browser session required")
		c.Abort()
		return nil, nil, false
	}

	if !session.Available(c) {
		// No sessions middleware on this route group. Unit tests mount a single
		// controller on a bare engine, and there full access is the convention
		// (see ClientController.clientScope); anything else is a misconfiguration
		// and is refused rather than crashing the process.
		if gin.Mode() == gin.TestMode {
			return nil, &model.AdminRole{Name: "test", Slug: "test", OwnerRole: true}, true
		}
		pureJsonMsg(c, http.StatusUnauthorized, false, "login required")
		c.Abort()
		return nil, nil, false
	}

	user := session.GetLoginUser(c)
	if user == nil {
		pureJsonMsg(c, http.StatusUnauthorized, false, "login required")
		c.Abort()
		return nil, nil, false
	}

	if user.Status != "" && user.Status != model.AdminStatusActive {
		pureJsonMsg(c, http.StatusForbidden, false, "admin account is disabled")
		c.Abort()
		return nil, nil, false
	}

	db := database.GetDB()
	if db == nil {
		pureJsonMsg(c, http.StatusInternalServerError, false, "database is not initialized")
		c.Abort()
		return nil, nil, false
	}

	var role model.AdminRole
	if err := db.Where("id = ?", user.RoleId).First(&role).Error; err != nil {
		pureJsonMsg(c, http.StatusForbidden, false, "admin role not found")
		c.Abort()
		return nil, nil, false
	}

	return user, &role, true
}

// requireOwnerAdminMiddleware allows only the owner role through.
func requireOwnerAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetBool("api_authed") {
			c.Next()
			return
		}
		_, role, ok := loginActiveAdminRole(c)
		if !ok {
			return
		}
		if !role.OwnerRole {
			pureJsonMsg(c, http.StatusForbidden, false, "owner permission required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// requireAdminPermission enforces a single resource.action permission.
func requireAdminPermission(section string, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetBool("api_authed") {
			c.Next()
			return
		}
		_, role, ok := loginActiveAdminRole(c)
		if !ok {
			return
		}
		if role.OwnerRole {
			c.Next()
			return
		}
		if !roleAllowsPermission(role, section, permission) {
			pureJsonMsg(c, http.StatusForbidden, false, section+"."+permission+" permission required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// requirePanelAccount allows any active panel account through, with no resource
// permission attached. It is for the handful of structural reads that the panel
// shell and every page need to render at all (the browser-safe settings view and
// the default templates); without them a role granted only, say, clients would
// see a page that can never finish loading.
func requirePanelAccount() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetBool("api_authed") {
			c.Next()
			return
		}
		if _, _, ok := loginActiveAdminRole(c); !ok {
			return
		}
		c.Next()
	}
}

// requirePanelPermission enforces a panel-resource permission for browser
// sessions, while letting API-token callers through (their authority is already
// narrowed by enforceTokenScope). This is the middleware applied across the
// inbounds / clients / groups / hosts / nodes / settings / xray routes, so a
// non-owner role reaches exactly the resources its role document grants.
func requirePanelPermission(section string, permission string) gin.HandlerFunc {
	return requireAdminPermission(section, permission)
}

type panelPermissionRequirement struct {
	Section    string
	Permission string
}

// requireAnyPanelPermission allows the request when at least one requirement is met.
func requireAnyPanelPermission(requirements ...panelPermissionRequirement) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetBool("api_authed") {
			c.Next()
			return
		}
		_, role, ok := loginActiveAdminRole(c)
		if !ok {
			return
		}
		if role.OwnerRole {
			c.Next()
			return
		}
		for _, req := range requirements {
			if roleAllowsPermission(role, req.Section, req.Permission) {
				c.Next()
				return
			}
		}
		pureJsonMsg(c, http.StatusForbidden, false, "permission required")
		c.Abort()
	}
}

// roleAllowsPermission reports whether a role grants section.permission.
func roleAllowsPermission(role *model.AdminRole, section string, permission string) bool {
	if role == nil {
		return false
	}
	if role.OwnerRole {
		return true
	}

	root := map[string]any{}
	if err := json.Unmarshal([]byte(role.PermissionsJSON), &root); err != nil {
		return false
	}

	for _, sectionKey := range permissionSectionKeys(section) {
		sectionValue, ok := root[sectionKey]
		if !ok {
			continue
		}
		sectionMap, ok := sectionValue.(map[string]any)
		if !ok {
			continue
		}
		for _, permissionKey := range permissionKeys(permission) {
			if permissionValueAllowed(sectionMap[permissionKey]) {
				return true
			}
		}
	}
	return false
}

// permissionSectionKeys maps the resource name used by routes to the key(s) the
// role document may use, absorbing the legacy roles ↔ admin_roles spelling.
func permissionSectionKeys(section string) []string {
	switch section {
	case "roles":
		return []string{"roles", "admin_roles"}
	case "admin_roles":
		return []string{"admin_roles", "roles"}
	case "clients":
		return []string{"clients", "users"}
	case "users":
		return []string{"users", "clients"}
	default:
		return []string{section}
	}
}

// permissionKeys maps camelCase route permissions to the snake_case keys roles
// store (and vice versa), so both spellings resolve to the same grant.
func permissionKeys(permission string) []string {
	switch permission {
	case "view":
		return []string{"view", "read"}
	case "read":
		return []string{"read", "view"}
	case "viewSimple":
		return []string{"viewSimple", "read_simple"}
	case "read_simple":
		return []string{"read_simple", "viewSimple"}
	case "viewGeneral":
		return []string{"viewGeneral", "read_general"}
	case "read_general":
		return []string{"read_general", "viewGeneral"}
	case "resetUsage":
		return []string{"resetUsage", "reset_usage"}
	case "reset_usage":
		return []string{"reset_usage", "resetUsage"}
	case "revokeSubscription":
		return []string{"revokeSubscription", "revoke_sub"}
	case "revoke_sub":
		return []string{"revoke_sub", "revokeSubscription"}
	case "activateNextPlan":
		return []string{"activateNextPlan", "activate_next_plan"}
	case "activate_next_plan":
		return []string{"activate_next_plan", "activateNextPlan"}
	case "setOwner":
		return []string{"setOwner", "set_owner"}
	case "set_owner":
		return []string{"set_owner", "setOwner"}
	case "updateCore":
		return []string{"updateCore", "update_core"}
	case "update_core":
		return []string{"update_core", "updateCore"}
	case "viewStatistics":
		return []string{"viewStatistics", "stats"}
	case "stats":
		return []string{"stats", "viewStatistics"}
	case "viewLogs":
		return []string{"viewLogs", "logs"}
	case "logs":
		return []string{"logs", "viewLogs"}
	case "adminFilter":
		return []string{"adminFilter", "admin_filter"}
	case "admin_filter":
		return []string{"admin_filter", "adminFilter"}
	default:
		return []string{permission}
	}
}

// permissionValueAllowed interprets a stored permission value. true / non-zero
// numbers / "own" / "all" all count as granted (scope-none is checked
// separately when ownership matters).
func permissionValueAllowed(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "yes", "1", "own", "all":
			return true
		default:
			return false
		}
	case map[string]any:
		return permissionValueAllowed(v["scope"])
	case float64:
		return v != 0
	case int:
		return v != 0
	case json.Number:
		n, err := v.Int64()
		return err == nil && n != 0
	default:
		return false
	}
}
