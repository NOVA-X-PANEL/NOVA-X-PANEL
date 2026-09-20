package model

import "encoding/json"

// RBAC (NOVA X PANEL) — admin roles and permission presets.
//
// This adds a role-based access-control layer on top of the panel's single
// account model: every panel account (model.User) is bound to one AdminRole,
// and each role carries four independent JSON documents:
//
//	permissions — resource → action → true|false|{"scope":0|1|2}
//	limits      — bounds applied to clients the admin may create
//	features    — behavioural flags (block/disconnect limited admins, ...)
//	access      — which client groups / inbounds the role may touch
//
// The JSON is stored as text so new permissions can be added without a
// schema migration, matching the panel's setting storage convention.

const (
	AdminStatusActive   = "active"
	AdminStatusDisabled = "disabled"

	AdminRoleSlugOwner         = "owner"
	AdminRoleSlugAdministrator = "administrator"
	AdminRoleSlugOperator      = "operator"
)

// AdminRole is a named permission preset assignable to panel accounts.
type AdminRole struct {
	Id              int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name            string `json:"name" gorm:"uniqueIndex;not null"`
	Slug            string `json:"slug" gorm:"uniqueIndex;not null"`
	BuiltIn         bool   `json:"builtIn" gorm:"column:built_in;default:false"`
	OwnerRole       bool   `json:"ownerRole" gorm:"column:owner_role;default:false"`
	PermissionsJSON string `json:"permissions" gorm:"column:permissions;type:text"`
	LimitsJSON      string `json:"limits" gorm:"column:limits;type:text"`
	FeaturesJSON    string `json:"features" gorm:"column:features;type:text"`
	AccessJSON      string `json:"access" gorm:"column:access;type:text"`
	CreatedAt       int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt       int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}

func (AdminRole) TableName() string { return "admin_roles" }

// AdminRoleScope is the numeric permission scope used inside permission maps.
type AdminRoleScope int

const (
	// ScopeNone denies access entirely.
	ScopeNone AdminRoleScope = 0
	// ScopeOwn limits the action to records owned by the acting admin.
	ScopeOwn AdminRoleScope = 1
	// ScopeAll grants the action across all records (owner-only in practice).
	ScopeAll AdminRoleScope = 2
)

func mustRoleJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// DefaultAdminRoles returns the three built-in roles seeded on first run.
func DefaultAdminRoles() []AdminRole {
	return []AdminRole{
		{
			Name:            "owner",
			Slug:            AdminRoleSlugOwner,
			BuiltIn:         true,
			OwnerRole:       true,
			PermissionsJSON: mustRoleJSON(ownerPermissions()),
			LimitsJSON:      mustRoleJSON(defaultRoleLimits()),
			FeaturesJSON:    mustRoleJSON(defaultRoleFeatures()),
			AccessJSON:      mustRoleJSON(allowAllGroupsAccess()),
		},
		{
			Name:            "Administrator",
			Slug:            AdminRoleSlugAdministrator,
			BuiltIn:         true,
			OwnerRole:       false,
			PermissionsJSON: mustRoleJSON(administratorPermissions()),
			LimitsJSON:      mustRoleJSON(defaultRoleLimits()),
			FeaturesJSON:    mustRoleJSON(administratorFeatures()),
			AccessJSON:      mustRoleJSON(allowAllGroupsAccess()),
		},
		{
			Name:            "Operator",
			Slug:            AdminRoleSlugOperator,
			BuiltIn:         true,
			OwnerRole:       false,
			PermissionsJSON: mustRoleJSON(operatorPermissions()),
			LimitsJSON:      mustRoleJSON(defaultRoleLimits()),
			FeaturesJSON:    mustRoleJSON(operatorFeatures()),
			AccessJSON:      mustRoleJSON(allowAllGroupsAccess()),
		},
	}
}

// allRolePermissions grants every action on every resource.
func allRolePermissions() map[string]any {
	return map[string]any{
		"inbounds": map[string]any{
			"read": true, "read_simple": true, "create": true,
			"update": true, "delete": true, "reset_usage": true,
		},
		"users": map[string]any{
			"read":               map[string]any{"scope": ScopeOwn},
			"read_simple":        map[string]any{"scope": ScopeOwn},
			"create":             true,
			"update":             map[string]any{"scope": ScopeOwn},
			"delete":             map[string]any{"scope": ScopeOwn},
			"reset_usage":        map[string]any{"scope": ScopeOwn},
			"revoke_sub":         map[string]any{"scope": ScopeOwn},
			"set_owner":          map[string]any{"scope": ScopeOwn},
			"activate_next_plan": map[string]any{"scope": ScopeOwn},
			"admin_filter":       true,
		},
		"groups": map[string]any{
			"read": true, "read_simple": true, "create": true,
			"update": true, "delete": true,
		},
		"nodes": map[string]any{
			"read": true, "read_simple": true, "create": true, "update": true,
			"delete": true, "reconnect": true, "update_core": true,
			"stats": true, "logs": true,
		},
		"admins": map[string]any{
			"read": true, "read_simple": true, "create": true,
			"update": true, "delete": true, "reset_usage": true,
		},
		"admin_roles": map[string]any{
			"read": true, "read_simple": true, "create": true,
			"update": true, "delete": true,
		},
		"outbounds": map[string]any{
			"read": true, "create": true, "update": true, "delete": true,
		},
		"routing": map[string]any{
			"read": true, "create": true, "update": true, "delete": true,
		},
		"settings": map[string]any{
			"read": true, "read_general": true, "update": true,
		},
		"cores": map[string]any{
			"read": true, "read_simple": true, "create": true,
			"update": true, "delete": true,
		},
		"hosts": map[string]any{
			"read": true, "create": true, "update": true,
		},
		"system": map[string]any{
			"read": true,
		},
	}
}

// ownerPermissions is the legacy (camelCase) shape kept for the owner role so
// older front-end guards that read `view`/`create` keep working alongside the
// canonical snake_case keys.
func ownerPermissions() map[string]any {
	return map[string]any{
		"users": map[string]any{
			"view": "all", "viewSimpleList": "all", "create": true,
			"update": "all", "delete": "all", "resetUsage": "all",
			"revokeSubscription": "all", "setOwner": "all", "activateNextPlan": "all",
			"read": map[string]any{"scope": ScopeAll}, "read_simple": map[string]any{"scope": ScopeAll},
			"reset_usage": map[string]any{"scope": ScopeAll}, "revoke_sub": map[string]any{"scope": ScopeAll},
			"set_owner": map[string]any{"scope": ScopeAll}, "activate_next_plan": map[string]any{"scope": ScopeAll},
			"admin_filter": true,
		},
		"inbounds": map[string]any{
			"read": true, "read_simple": true, "create": true,
			"update": true, "delete": true, "reset_usage": true,
		},
		"admins": map[string]any{
			"view": true, "viewSimple": true, "create": true,
			"update": true, "delete": true, "resetUsage": true,
			"read": true, "read_simple": true, "reset_usage": true,
		},
		"roles": map[string]any{
			"view": true, "viewSimple": true, "create": true,
			"update": true, "delete": true,
		},
		"admin_roles": map[string]any{
			"read": true, "read_simple": true, "create": true,
			"update": true, "delete": true,
		},
		"nodes": map[string]any{
			"read": true, "read_simple": true, "create": true, "update": true,
			"delete": true, "reconnect": true, "update_core": true, "stats": true, "logs": true,
		},
		"cores": map[string]any{
			"read": true, "read_simple": true, "create": true, "update": true, "delete": true,
		},
		"hosts": map[string]any{
			"read": true, "create": true, "update": true,
		},
		"groups": map[string]any{
			"read": true, "read_simple": true, "create": true, "update": true, "delete": true,
		},
		"settings": map[string]any{
			"read": true, "read_general": true, "update": true,
		},
		"outbounds": map[string]any{
			"read": true, "create": true, "update": true, "delete": true,
		},
		"routing": map[string]any{
			"read": true, "create": true, "update": true, "delete": true,
		},
		"system": map[string]any{
			"read": true,
		},
	}
}

func administratorPermissions() map[string]any {
	return allRolePermissions()
}

func operatorPermissions() map[string]any {
	return map[string]any{
		"inbounds": map[string]any{
			"read_simple": true,
		},
		"users": map[string]any{
			"read":               map[string]any{"scope": ScopeOwn},
			"read_simple":        map[string]any{"scope": ScopeOwn},
			"create":             true,
			"update":             map[string]any{"scope": ScopeOwn},
			"delete":             map[string]any{"scope": ScopeOwn},
			"reset_usage":        map[string]any{"scope": ScopeOwn},
			"revoke_sub":         map[string]any{"scope": ScopeOwn},
			"set_owner":          map[string]any{"scope": ScopeOwn},
			"activate_next_plan": map[string]any{"scope": ScopeOwn},
		},
		"groups": map[string]any{
			"read_simple": true,
		},
		"settings": map[string]any{
			"read_general": true,
		},
		"system": map[string]any{
			"read": true,
		},
	}
}

func defaultRoleLimits() map[string]any {
	return map[string]any{
		"max_users":             nil,
		"data_limit_min":        nil,
		"data_limit_max":        nil,
		"expire_days_min":       nil,
		"expire_days_max":       nil,
		"download_mbps_min":     nil,
		"download_mbps_max":     nil,
		"upload_mbps_min":       nil,
		"upload_mbps_max":       nil,
		"minOnHoldTimeoutDays":  nil,
		"maxOnHoldTimeoutDays":  nil,
	}
}

func defaultRoleFeatures() map[string]any {
	return map[string]any{
		"blockLimitedAdmins":          false,
		"disconnectUsersWhenLimited":  true,
		"disconnectUsersWhenDisabled": true,
		"can_use_reset_strategy":      true,
		"can_use_next_plan":           true,
	}
}

func administratorFeatures() map[string]any {
	return map[string]any{
		"blockLimitedAdmins":          true,
		"disconnectUsersWhenLimited":  true,
		"disconnectUsersWhenDisabled": true,
		"can_use_reset_strategy":      true,
		"can_use_next_plan":           true,
	}
}

func operatorFeatures() map[string]any {
	return map[string]any{
		"blockLimitedAdmins":          true,
		"disconnectUsersWhenLimited":  true,
		"disconnectUsersWhenDisabled": true,
		"can_use_reset_strategy":      false,
		"can_use_next_plan":           true,
	}
}

func allowAllGroupsAccess() map[string]any {
	return map[string]any{
		"allowAllGroups":   true,
		"allowAllInbounds": true,
		"allowedGroups":    []string{},
	}
}
