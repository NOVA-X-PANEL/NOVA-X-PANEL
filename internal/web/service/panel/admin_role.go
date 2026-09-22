package panel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"

	"gorm.io/gorm"
)

// RBAC (NOVA X PANEL) — admin role management service.

type AdminRoleService struct{}

// AdminRolePayload is the create/update body for a role.
type AdminRolePayload struct {
	Name        string         `json:"name" form:"name"`
	Permissions map[string]any `json:"permissions" form:"permissions"`
	Limits      map[string]any `json:"limits" form:"limits"`
	Features    map[string]any `json:"features" form:"features"`
	Access      map[string]any `json:"access" form:"access"`
}

// AdminRoleView is the API representation of a role.
type AdminRoleView struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	BuiltIn     bool   `json:"builtIn"`
	OwnerRole   bool   `json:"ownerRole"`
	Permissions any    `json:"permissions"`
	Limits      any    `json:"limits"`
	Features    any    `json:"features"`
	Access      any    `json:"access"`
	AdminCount  int64  `json:"adminCount"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

func normalizeRoleSlug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		ok := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
		if !ok {
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
			continue
		}
		if r == '-' {
			if lastDash {
				continue
			}
			lastDash = true
		} else {
			lastDash = false
		}
		b.WriteRune(r)
	}
	return strings.Trim(b.String(), "-")
}

func marshalRoleMap(v map[string]any) string {
	if v == nil {
		return "{}"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func decodeRoleJSON(raw string) any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var out any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func roleToView(row *model.AdminRole, adminCount int64) *AdminRoleView {
	if row == nil {
		return nil
	}
	return &AdminRoleView{
		Id:          row.Id,
		Name:        row.Name,
		Slug:        row.Slug,
		BuiltIn:     row.BuiltIn,
		OwnerRole:   row.OwnerRole,
		Permissions: decodeRoleJSON(row.PermissionsJSON),
		Limits:      decodeRoleJSON(row.LimitsJSON),
		Features:    decodeRoleJSON(row.FeaturesJSON),
		Access:      decodeRoleJSON(row.AccessJSON),
		AdminCount:  adminCount,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func (s *AdminRoleService) roleAdminCounts(db *gorm.DB, roleIDs []int) (map[int]int64, error) {
	counts := map[int]int64{}
	if len(roleIDs) == 0 {
		return counts, nil
	}
	type countRow struct {
		RoleId int
		Count  int64
	}
	var grouped []countRow
	if err := db.Model(&model.User{}).
		Select("role_id, COUNT(*) AS count").
		Where("role_id IN ?", roleIDs).
		Group("role_id").
		Scan(&grouped).Error; err != nil {
		return nil, err
	}
	for _, row := range grouped {
		counts[row.RoleId] = row.Count
	}
	return counts, nil
}

// List returns every role with its assigned-admin count.
func (s *AdminRoleService) List() ([]*AdminRoleView, error) {
	db := database.GetDB()
	var rows []*model.AdminRole
	if err := db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.Id)
	}
	counts, err := s.roleAdminCounts(db, ids)
	if err != nil {
		return nil, err
	}

	out := make([]*AdminRoleView, 0, len(rows))
	for _, row := range rows {
		out = append(out, roleToView(row, counts[row.Id]))
	}
	return out, nil
}

// Get returns one role by id.
func (s *AdminRoleService) Get(id int) (*AdminRoleView, error) {
	if id <= 0 {
		return nil, errors.New("invalid role id")
	}
	db := database.GetDB()
	var row model.AdminRole
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	var count int64
	if err := db.Model(&model.User{}).Where("role_id = ?", id).Count(&count).Error; err != nil {
		return nil, err
	}
	return roleToView(&row, count), nil
}

func defaultOperatorRoleMaps() (map[string]any, map[string]any, map[string]any, map[string]any) {
	defaults := model.DefaultAdminRoles()[2]
	return decodeRoleMap(defaults.PermissionsJSON),
		decodeRoleMap(defaults.LimitsJSON),
		decodeRoleMap(defaults.FeaturesJSON),
		decodeRoleMap(defaults.AccessJSON)
}

func decodeRoleMap(raw string) map[string]any {
	out := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func nonEmptyOrDefault(v map[string]any, fallback map[string]any) map[string]any {
	if len(v) > 0 {
		return v
	}
	return fallback
}

// Create adds a new custom role, defaulting to the Operator preset.
func (s *AdminRoleService) Create(payload AdminRolePayload) (*AdminRoleView, error) {
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return nil, errors.New("role name is required")
	}
	if len(name) > 64 {
		return nil, errors.New("role name must be 64 characters or fewer")
	}
	slug := normalizeRoleSlug(name)
	if slug == "" {
		return nil, errors.New("role slug is invalid")
	}

	db := database.GetDB()
	var count int64
	if err := db.Model(&model.AdminRole{}).
		Where("name = ? OR slug = ?", name, slug).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("role already exists")
	}

	defaultPermissions, defaultLimits, defaultFeatures, defaultAccess := defaultOperatorRoleMaps()

	row := &model.AdminRole{
		Name:            name,
		Slug:            slug,
		BuiltIn:         false,
		OwnerRole:       false,
		PermissionsJSON: marshalRoleMap(nonEmptyOrDefault(payload.Permissions, defaultPermissions)),
		LimitsJSON:      marshalRoleMap(nonEmptyOrDefault(payload.Limits, defaultLimits)),
		FeaturesJSON:    marshalRoleMap(nonEmptyOrDefault(payload.Features, defaultFeatures)),
		AccessJSON:      marshalRoleMap(nonEmptyOrDefault(payload.Access, defaultAccess)),
	}
	if err := db.Create(row).Error; err != nil {
		return nil, err
	}
	return roleToView(row, 0), nil
}

// Update edits a non-owner role; built-in roles keep their identity.
// PermissionGrants is one resource → action grant.
type PermissionGrants map[string]map[string]bool

// RolePermissionGrants flattens a role document into resource → action → allowed.
// Only truthy grants are listed: an action absent or set to false is not a grant.
func RolePermissionGrants(role *model.AdminRole) PermissionGrants {
	out := PermissionGrants{}
	if role == nil {
		return out
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(role.PermissionsJSON), &root); err != nil {
		return out
	}
	for section, value := range root {
		sectionMap, ok := value.(map[string]any)
		if !ok {
			continue
		}
		grants := map[string]bool{}
		for action, raw := range sectionMap {
			if permissionValueTruthy(raw) {
				grants[action] = true
			}
		}
		if len(grants) > 0 {
			out[section] = grants
		}
	}
	return out
}

// permissionValueTruthy mirrors the panel's grant test for one permission value.
func permissionValueTruthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return int(t) != 0
	case int:
		return t != 0
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "", "false", "no", "0", "none", "deny", "null":
			return false
		default:
			return true
		}
	case map[string]any:
		return permissionValueTruthy(t["scope"])
	}
	return false
}

// RedirectsOrWidens reports whether `target` grants anything `actor` does not.
//
// It is the anti-escalation test: an admin may only hand out, or edit a role
// into, a permission set inside its own. Without it any account holding
// admins.create or admin_roles.update could grant itself (or a puppet) the
// administrator preset and take the panel over.
//
// The owner role is exempt — it is allowed to do anything by definition.
func RedirectsOrWidens(actor *model.AdminRole, target *model.AdminRole) bool {
	if actor == nil {
		return true
	}
	if actor.OwnerRole {
		return false
	}
	have := RolePermissionGrants(actor)
	want := RolePermissionGrants(target)
	for section, actions := range want {
		for action := range actions {
			if !have[section][action] {
				return true
			}
		}
	}
	return false
}

// PermissionsWidenActor reports whether a raw permission map grants anything the
// actor's role does not, for validating a role before it is stored.
func PermissionsWidenActor(actor *model.AdminRole, permissions map[string]any) bool {
	if actor == nil {
		return true
	}
	if actor.OwnerRole {
		return false
	}
	have := RolePermissionGrants(actor)
	for section, value := range permissions {
		sectionMap, ok := value.(map[string]any)
		if !ok {
			continue
		}
		for action, raw := range sectionMap {
			if permissionValueTruthy(raw) && !have[section][action] {
				return true
			}
		}
	}
	return false
}

func (s *AdminRoleService) Update(id int, payload AdminRolePayload) (*AdminRoleView, error) {
	if id <= 0 {
		return nil, errors.New("invalid role id")
	}
	db := database.GetDB()
	var row model.AdminRole
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	if row.OwnerRole {
		return nil, errors.New("owner role is read-only")
	}

	updates := map[string]any{
		"permissions": marshalRoleMap(payload.Permissions),
		"limits":      marshalRoleMap(payload.Limits),
		"features":    marshalRoleMap(payload.Features),
		"access":      marshalRoleMap(payload.Access),
	}

	if !row.BuiltIn {
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			return nil, errors.New("role name is required")
		}
		if len(name) > 64 {
			return nil, errors.New("role name must be 64 characters or fewer")
		}
		slug := normalizeRoleSlug(name)
		if slug == "" {
			return nil, errors.New("role slug is invalid")
		}
		var count int64
		if err := db.Model(&model.AdminRole{}).
			Where("(name = ? OR slug = ?) AND id <> ?", name, slug, id).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("role already exists")
		}
		updates["name"] = name
		updates["slug"] = slug
	}

	if err := db.Model(&model.AdminRole{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.Get(id)
}

// Duplicate copies a role into a new custom role.
func (s *AdminRoleService) Duplicate(id int) (*AdminRoleView, error) {
	if id <= 0 {
		return nil, errors.New("invalid role id")
	}
	db := database.GetDB()
	var src model.AdminRole
	if err := db.Where("id = ?", id).First(&src).Error; err != nil {
		return nil, err
	}

	baseName := strings.TrimSpace(src.Name) + " (copy)"
	name := baseName
	for i := 2; ; i++ {
		slug := normalizeRoleSlug(name)
		var count int64
		if err := db.Model(&model.AdminRole{}).
			Where("name = ? OR slug = ?", name, slug).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count == 0 {
			row := &model.AdminRole{
				Name:            name,
				Slug:            slug,
				BuiltIn:         false,
				OwnerRole:       false,
				PermissionsJSON: src.PermissionsJSON,
				LimitsJSON:      src.LimitsJSON,
				FeaturesJSON:    src.FeaturesJSON,
				AccessJSON:      src.AccessJSON,
			}
			if err := db.Create(row).Error; err != nil {
				return nil, err
			}
			return roleToView(row, 0), nil
		}
		name = fmt.Sprintf("%s %d", baseName, i)
	}
}

// Delete removes a custom role that is not assigned to any admin.
func (s *AdminRoleService) Delete(id int) error {
	if id <= 0 {
		return errors.New("invalid role id")
	}
	db := database.GetDB()
	var row model.AdminRole
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		return err
	}
	if row.OwnerRole {
		return errors.New("owner role cannot be deleted")
	}
	if row.BuiltIn {
		return errors.New("built-in roles cannot be deleted")
	}

	var count int64
	if err := db.Model(&model.User{}).Where("role_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("role is assigned to admins")
	}

	res := db.Where("id = ?", id).Delete(&model.AdminRole{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("role not found")
	}
	return nil
}

// usedHashPassword hashes a plaintext password for storage, reused by AdminService.
func usedHashPassword(password string) (string, error) {
	return crypto.HashPasswordAsBcrypt(password)
}
