package service

import (
	"encoding/json"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

// RBAC (NOVA X PANEL) — client access scoping.
//
// A role's `permissions.users.*` values carry a scope (0/1/2 = none/own/all)
// and its `access` document lists the allowed client groups and inbound ids.
// These helpers turn that into a ClientAccessScope which is applied to every
// client query so a non-owner admin can only ever see and touch their own
// clients, inside the groups/inbounds the role allows.

// ClientAccessMode is the ownership dimension of a scope.
type ClientAccessMode string

const (
	ClientAccessAll  ClientAccessMode = "all"
	ClientAccessOwn  ClientAccessMode = "own"
	ClientAccessNone ClientAccessMode = "none"
)

// ClientAccessScope is the resolved permission scope for one admin + action.
type ClientAccessScope struct {
	AdminID           int
	Mode              ClientAccessMode
	RestrictGroups    bool
	AllowAllGroups    bool
	AllowedGroups     []string
	RestrictInbounds  bool
	AllowAllInbounds  bool
	AllowedInboundIDs []int
}

func normalizeAllowedClientGroups(groups []string) []string {
	if len(groups) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(groups))
	out := make([]string, 0, len(groups))
	for _, group := range groups {
		key := strings.ToLower(strings.TrimSpace(group))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func normalizeAllowedInboundIDs(ids []int) []int {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(ids))
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func normalizeClientAccessScope(scope ClientAccessScope) ClientAccessScope {
	if scope.Mode == "" {
		scope.Mode = ClientAccessAll
	}
	if scope.Mode == ClientAccessOwn && scope.AdminID <= 0 {
		scope.Mode = ClientAccessNone
	}
	if scope.RestrictGroups {
		scope.AllowedGroups = normalizeAllowedClientGroups(scope.AllowedGroups)
		if scope.AllowAllGroups {
			scope.AllowedGroups = nil
		}
	}
	if scope.RestrictInbounds {
		scope.AllowedInboundIDs = normalizeAllowedInboundIDs(scope.AllowedInboundIDs)
		if scope.AllowAllInbounds {
			scope.AllowedInboundIDs = nil
		}
	}
	return scope
}

// clientGroupAllowed reports whether the scope permits a client in `group`.
func clientGroupAllowed(scope ClientAccessScope, group string) bool {
	scope = normalizeClientAccessScope(scope)
	if !scope.RestrictGroups || scope.AllowAllGroups {
		return true
	}
	if len(scope.AllowedGroups) == 0 {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(group))
	for _, allowed := range scope.AllowedGroups {
		if key == allowed {
			return true
		}
	}
	return false
}

// applyClientGroupAccessScope narrows a query to the scope's allowed groups.
func applyClientGroupAccessScope(db *gorm.DB, scope ClientAccessScope) *gorm.DB {
	scope = normalizeClientAccessScope(scope)
	if !scope.RestrictGroups || scope.AllowAllGroups {
		return db
	}
	if len(scope.AllowedGroups) == 0 {
		return db.Where("1 = 0")
	}
	return db.Where("LOWER(group_name) IN ?", scope.AllowedGroups)
}

// applyClientAccessScope narrows a query to the scope's ownership mode.
func applyClientAccessScope(db *gorm.DB, scope ClientAccessScope) *gorm.DB {
	scope = normalizeClientAccessScope(scope)
	db = applyClientGroupAccessScope(db, scope)
	switch scope.Mode {
	case ClientAccessAll:
		return db
	case ClientAccessOwn:
		return db.Where("owner_admin_id = ?", scope.AdminID)
	default:
		return db.Where("1 = 0")
	}
}

// ClientRecordAllowed reports whether a single client is inside the scope.
func ClientRecordAllowed(scope ClientAccessScope, rec *model.ClientRecord) bool {
	scope = normalizeClientAccessScope(scope)
	if rec == nil {
		return false
	}
	if !clientGroupAllowed(scope, rec.Group) {
		return false
	}
	switch scope.Mode {
	case ClientAccessAll:
		return true
	case ClientAccessOwn:
		return rec.OwnerAdminId > 0 && rec.OwnerAdminId == int64(scope.AdminID)
	default:
		return false
	}
}

// ClientInboundsAllowedForScope reports whether a scoped client operation may
// use every one of the given inbound ids.
func ClientInboundsAllowedForScope(scope ClientAccessScope, inboundIds []int) bool {
	scope = normalizeClientAccessScope(scope)
	if scope.Mode == ClientAccessNone {
		return false
	}
	if !scope.RestrictInbounds || scope.AllowAllInbounds {
		return true
	}
	if len(scope.AllowedInboundIDs) == 0 || len(inboundIds) == 0 {
		return false
	}
	allowed := make(map[int]struct{}, len(scope.AllowedInboundIDs))
	for _, id := range scope.AllowedInboundIDs {
		allowed[id] = struct{}{}
	}
	for _, id := range inboundIds {
		if id <= 0 {
			return false
		}
		if _, ok := allowed[id]; !ok {
			return false
		}
	}
	return true
}

// ClientGroupAllowedForScope reports whether a scoped operation may touch a
// client group.
func ClientGroupAllowedForScope(scope ClientAccessScope, group string) bool {
	scope = normalizeClientAccessScope(scope)
	if scope.Mode == ClientAccessNone {
		return false
	}
	return clientGroupAllowed(scope, group)
}

// ---- role document readers -------------------------------------------------

func rolePermissionValue(role *model.AdminRole, section string, key string) any {
	if role == nil {
		return nil
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(role.PermissionsJSON), &root); err != nil {
		return nil
	}
	sectionValue, ok := root[section]
	if !ok {
		return nil
	}
	sectionMap, ok := sectionValue.(map[string]any)
	if !ok {
		return nil
	}
	for _, candidate := range clientPermissionKeys(key) {
		if value, ok := sectionMap[candidate]; ok {
			return value
		}
	}
	return nil
}

func clientPermissionKeys(key string) []string {
	switch key {
	case "view":
		return []string{"view", "read"}
	case "read":
		return []string{"read", "view"}
	case "view_simple":
		return []string{"view_simple", "read_simple", "viewSimpleList", "viewSimple"}
	case "reset_usage":
		return []string{"reset_usage", "resetUsage"}
	case "revoke_sub":
		return []string{"revoke_sub", "revokeSubscription"}
	case "activate_next_plan":
		return []string{"activate_next_plan", "activateNextPlan"}
	case "admin_filter":
		return []string{"admin_filter", "adminFilter"}
	case "set_owner":
		return []string{"set_owner", "setOwner"}
	default:
		return []string{key}
	}
}

// ClientAccessModeFromPermission converts a stored permission value into an
// ownership mode: "all"/2 → all, "own"/1/true → own, false/null/0 → none.
func ClientAccessModeFromPermission(v any) ClientAccessMode {
	switch t := v.(type) {
	case map[string]any:
		return ClientAccessModeFromPermission(t["scope"])
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "all", "2":
			return ClientAccessAll
		case "own", "1":
			return ClientAccessOwn
		default:
			return ClientAccessNone
		}
	case bool:
		if t {
			return ClientAccessAll
		}
		return ClientAccessNone
	case float64:
		switch int(t) {
		case 2:
			return ClientAccessAll
		case 1:
			return ClientAccessOwn
		}
	case int:
		switch t {
		case 2:
			return ClientAccessAll
		case 1:
			return ClientAccessOwn
		}
	}
	return ClientAccessNone
}

func roleAccessValue(root map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if v, ok := root[key]; ok {
			return v, true
		}
	}
	return nil, false
}

func roleAccessBoolValue(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "yes", "1", "all":
			return true
		}
	case float64:
		return t != 0
	case int:
		return t != 0
	}
	return false
}

func roleAccessStringSlice(v any) []string {
	items, ok := v.([]any)
	if !ok {
		if list, ok := v.([]string); ok {
			return list
		}
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func roleAccessIntSlice(v any) []int {
	items, ok := v.([]any)
	if !ok {
		if list, ok := v.([]int); ok {
			return list
		}
		return nil
	}
	out := make([]int, 0, len(items))
	for _, item := range items {
		switch n := item.(type) {
		case float64:
			out = append(out, int(n))
		case int:
			out = append(out, n)
		}
	}
	return out
}

// roleGroupAccessScope resolves the group restriction of a role's access doc.
func roleGroupAccessScope(role *model.AdminRole) (restrict bool, allowAll bool, groups []string) {
	if role == nil {
		return true, false, nil
	}
	if role.OwnerRole {
		return true, true, nil
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(role.AccessJSON), &root); err != nil {
		return true, true, nil
	}

	if value, ok := roleAccessValue(root, "allowAllGroups", "allow_all_groups"); ok && roleAccessBoolValue(value) {
		return true, true, nil
	}
	if value, ok := roleAccessValue(root, "allowedGroups", "allowed_groups"); ok {
		names := normalizeAllowedClientGroups(roleAccessStringSlice(value))
		if len(names) == 0 {
			return true, false, nil
		}
		return true, false, names
	}
	return true, true, nil
}

// roleInboundAccessScope resolves the inbound restriction of a role's access doc.
func roleInboundAccessScope(role *model.AdminRole) (restrict bool, allowAll bool, ids []int) {
	if role == nil {
		return true, false, nil
	}
	if role.OwnerRole {
		return true, true, nil
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(role.AccessJSON), &root); err != nil {
		return true, true, nil
	}

	if value, ok := roleAccessValue(root, "allowAllInbounds", "allow_all_inbounds"); ok && roleAccessBoolValue(value) {
		return true, true, nil
	}
	if value, ok := roleAccessValue(root, "allowed_inbound_ids", "allowedInboundIds"); ok {
		list := normalizeAllowedInboundIDs(roleAccessIntSlice(value))
		if len(list) == 0 {
			return true, true, nil
		}
		return true, false, list
	}
	return true, true, nil
}

func adminRoleForUser(user *model.User) (*model.AdminRole, error) {
	if user == nil {
		return nil, gorm.ErrRecordNotFound
	}
	db := database.GetDB()
	if db == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var role model.AdminRole
	if err := db.Where("id = ?", user.RoleId).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

// ClientAccessScopeForAdmin resolves the client scope for one admin + action.
// Only the owner role may reach across admins; every non-owner role is capped
// at "own" even when its permission says "all".
func ClientAccessScopeForAdmin(user *model.User, permission string) ClientAccessScope {
	role, err := adminRoleForUser(user)
	if err != nil {
		return ClientAccessScope{Mode: ClientAccessNone, RestrictGroups: true}
	}

	restrictGroups, allowAllGroups, allowedGroups := roleGroupAccessScope(role)
	restrictInbounds, allowAllInbounds, allowedInboundIDs := roleInboundAccessScope(role)

	if role.OwnerRole {
		return normalizeClientAccessScope(ClientAccessScope{
			AdminID:           user.Id,
			Mode:              ClientAccessAll,
			RestrictGroups:    restrictGroups,
			AllowAllGroups:    allowAllGroups,
			AllowedGroups:     allowedGroups,
			RestrictInbounds:  restrictInbounds,
			AllowAllInbounds:  allowAllInbounds,
			AllowedInboundIDs: allowedInboundIDs,
		})
	}

	mode := ClientAccessModeFromPermission(rolePermissionValue(role, "users", permission))
	if mode == ClientAccessAll {
		mode = ClientAccessOwn
	}
	return normalizeClientAccessScope(ClientAccessScope{
		AdminID:           user.Id,
		Mode:              mode,
		RestrictGroups:    restrictGroups,
		AllowAllGroups:    allowAllGroups,
		AllowedGroups:     allowedGroups,
		RestrictInbounds:  restrictInbounds,
		AllowAllInbounds:  allowAllInbounds,
		AllowedInboundIDs: allowedInboundIDs,
	})
}

// FilterClientEmailsForScope keeps only the emails the scope may see.
func FilterClientEmailsForScope(scope ClientAccessScope, emails []string) []string {
	if len(emails) == 0 {
		return emails
	}
	scope = normalizeClientAccessScope(scope)
	if scope.Mode == ClientAccessNone {
		return []string{}
	}

	db := database.GetDB()
	if db == nil {
		return []string{}
	}

	var rows []model.ClientRecord
	if err := applyClientAccessScope(db.Model(&model.ClientRecord{}), scope).
		Where("email IN ?", emails).
		Find(&rows).Error; err != nil {
		return []string{}
	}

	allowed := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		allowed[r.Email] = struct{}{}
	}
	out := make([]string, 0, len(allowed))
	for _, email := range emails {
		if _, ok := allowed[email]; ok {
			out = append(out, email)
		}
	}
	return out
}
