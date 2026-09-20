package panel

import (
	"errors"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	core "github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"gorm.io/gorm"
)

// RBAC (NOVA X PANEL) — admin account management service.
//
// Panel accounts live in model.User; each is bound to exactly one
// model.AdminRole. The first account (id 1 order) holds the owner role.

type AdminService struct{}

// AdminPayload is the create/update body for a panel account.
type AdminPayload struct {
	Username  string `json:"username" form:"username"`
	Password  string `json:"password" form:"password"`
	RoleId    int    `json:"roleId" form:"roleId"`
	Status    string `json:"status" form:"status"`
	DataLimit int64  `json:"dataLimit" form:"dataLimit"`
}

// AdminView is the API representation of a panel account (password omitted).
type AdminView struct {
	Id        int    `json:"id"`
	Username  string `json:"username"`
	RoleId    int    `json:"roleId"`
	RoleName  string `json:"roleName"`
	RoleSlug  string `json:"roleSlug"`
	OwnerRole bool   `json:"ownerRole"`
	Status    string `json:"status"`
	IsSelf    bool   `json:"isSelf"`
	DataLimit int64  `json:"dataLimit"`
	UsedBytes int64  `json:"usedBytes"`
	TotalUsers int64 `json:"totalUsers"`
	Limited   bool   `json:"limited"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// AdminStats summarises panel accounts for the admins page.
type AdminStats struct {
	TotalAdmins    int64 `json:"totalAdmins"`
	ActiveAdmins   int64 `json:"activeAdmins"`
	DisabledAdmins int64 `json:"disabledAdmins"`
	LimitedAdmins  int64 `json:"limitedAdmins"`
}

// adminIsLimited reports whether an account has exhausted its data quota.
func adminIsLimited(user *model.User) bool {
	return user != nil && user.DataLimit > 0 && user.UsedBytes >= user.DataLimit
}

// adminUsageRow is one grouped aggregate over the clients table.
type adminUsageRow struct {
	AdminID   int
	UsedBytes int64
	Count     int64
}

// adminUsageByOwner aggregates per-admin traffic and owned-client counts.
// Clients carry owner_admin_id; their traffic lives in client_traffics, keyed
// by email.
func adminUsageByOwner(db *gorm.DB) (map[int]int64, map[int]int64, error) {
	used := map[int]int64{}
	counts := map[int]int64{}

	var usage []adminUsageRow
	if err := db.Table("clients AS c").
		Select("c.owner_admin_id AS admin_id, COALESCE(SUM(COALESCE(ct.up, 0) + COALESCE(ct.down, 0)), 0) AS used_bytes, COUNT(*) AS count").
		Joins("LEFT JOIN client_traffics AS ct ON ct.email = c.email").
		Where("c.owner_admin_id > 0").
		Group("c.owner_admin_id").
		Scan(&usage).Error; err != nil {
		return used, counts, err
	}
	for _, row := range usage {
		if row.AdminID <= 0 {
			continue
		}
		used[row.AdminID] = row.UsedBytes
		counts[row.AdminID] = row.Count
	}
	return used, counts, nil
}

func validAdminStatus(status string) bool {
	return status == model.AdminStatusActive || status == model.AdminStatusDisabled
}

func normalizeAdminStatus(status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		return model.AdminStatusActive
	}
	return status
}

func (s *AdminService) roleByID(tx *gorm.DB, roleID int) (*model.AdminRole, error) {
	if roleID <= 0 {
		return nil, errors.New("role is required")
	}
	var role model.AdminRole
	if err := tx.Where("id = ?", roleID).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (s *AdminService) ownerRoleID(tx *gorm.DB) (int, error) {
	var role model.AdminRole
	if err := tx.Where("slug = ?", model.AdminRoleSlugOwner).First(&role).Error; err != nil {
		return 0, err
	}
	return role.Id, nil
}

func adminToView(user *model.User, role *model.AdminRole, selfID int, usedBytes, totalUsers int64) *AdminView {
	view := &AdminView{
		Id:         user.Id,
		Username:   user.Username,
		RoleId:     user.RoleId,
		Status:     user.Status,
		IsSelf:     user.Id == selfID,
		DataLimit:  user.DataLimit,
		UsedBytes:  usedBytes,
		TotalUsers: totalUsers,
		Limited:    adminIsLimited(user),
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
	}
	if role != nil {
		view.RoleName = role.Name
		view.RoleSlug = role.Slug
		view.OwnerRole = role.OwnerRole
	}
	return view
}

// List returns every panel account with its role resolved.
func (s *AdminService) List() ([]*AdminView, error) {
	db := database.GetDB()
	var users []model.User
	if err := db.Order("id ASC").Find(&users).Error; err != nil {
		return nil, err
	}

	roleIDs := make([]int, 0, len(users))
	for _, user := range users {
		roleIDs = append(roleIDs, user.RoleId)
	}

	roles := map[int]*model.AdminRole{}
	if len(roleIDs) > 0 {
		var rows []model.AdminRole
		if err := db.Where("id IN ?", roleIDs).Find(&rows).Error; err != nil {
			return nil, err
		}
		for i := range rows {
			roles[rows[i].Id] = &rows[i]
		}
	}

	used, counts, err := adminUsageByOwner(db)
	if err != nil {
		return nil, err
	}

	out := make([]*AdminView, 0, len(users))
	for i := range users {
		out = append(out, adminToView(&users[i], roles[users[i].RoleId], 0, used[users[i].Id], counts[users[i].Id]))
	}
	return out, nil
}

// ListFor returns the list with IsSelf marked relative to selfID.
func (s *AdminService) ListFor(selfID int) ([]*AdminView, error) {
	rows, err := s.List()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		row.IsSelf = row.Id == selfID
	}
	return rows, nil
}

// Get returns one panel account.
func (s *AdminService) Get(id int) (*AdminView, error) {
	if id <= 0 {
		return nil, errors.New("invalid admin id")
	}
	db := database.GetDB()
	var user model.User
	if err := db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	role, _ := s.roleByID(db, user.RoleId)
	used, counts, err := adminUsageByOwner(db)
	if err != nil {
		return nil, err
	}
	return adminToView(&user, role, 0, used[user.Id], counts[user.Id]), nil
}

// Stats summarises panel accounts, refreshing each account's aggregated usage
// first so the "limited" count reflects live traffic.
func (s *AdminService) Stats() (*AdminStats, error) {
	db := database.GetDB()

	if err := s.SyncAdminUsedBytes(); err != nil {
		return nil, err
	}

	var stats AdminStats
	if err := db.Model(&model.User{}).Count(&stats.TotalAdmins).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&model.User{}).Where("status = ?", model.AdminStatusActive).Count(&stats.ActiveAdmins).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&model.User{}).Where("status = ?", model.AdminStatusDisabled).Count(&stats.DisabledAdmins).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&model.User{}).
		Where("data_limit > 0 AND used_bytes >= data_limit").
		Count(&stats.LimitedAdmins).Error; err != nil {
		return nil, err
	}
	return &stats, nil
}

// SyncAdminUsedBytes recomputes every account's aggregated client traffic and
// writes it back only when it changed.
func (s *AdminService) SyncAdminUsedBytes() error {
	db := database.GetDB()
	used, _, err := adminUsageByOwner(db)
	if err != nil {
		return err
	}

	var users []model.User
	if err := db.Model(&model.User{}).Select("id", "used_bytes").Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		next := used[user.Id]
		if next < 0 {
			next = 0
		}
		if next == user.UsedBytes {
			continue
		}
		if err := db.Model(&model.User{}).Where("id = ?", user.Id).
			Update("used_bytes", next).Error; err != nil {
			return err
		}
	}
	return nil
}

// Create adds a new panel account bound to the given role.
func (s *AdminService) Create(payload AdminPayload) (*AdminView, error) {
	username := strings.TrimSpace(payload.Username)
	if username == "" {
		return nil, errors.New("username is required")
	}
	if strings.TrimSpace(payload.Password) == "" {
		return nil, errors.New("password is required")
	}
	status := normalizeAdminStatus(payload.Status)
	if !validAdminStatus(status) {
		return nil, errors.New("invalid status")
	}

	db := database.GetDB()
	role, err := s.roleByID(db, payload.RoleId)
	if err != nil {
		return nil, err
	}
	if role.OwnerRole {
		return nil, errors.New("cannot create a second owner account")
	}

	var count int64
	if err := db.Model(&model.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("username already exists")
	}

	hashed, err := usedHashPassword(payload.Password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username:  username,
		Password:  hashed,
		RoleId:    payload.RoleId,
		Status:    status,
		DataLimit: payload.DataLimit,
	}
	if err := db.Create(user).Error; err != nil {
		return nil, err
	}
	return adminToView(user, role, 0, 0, 0), nil
}

// Update edits an account; the owner account cannot be demoted or disabled.
func (s *AdminService) Update(id int, payload AdminPayload) (*AdminView, error) {
	if id <= 0 {
		return nil, errors.New("invalid admin id")
	}
	db := database.GetDB()
	var user model.User
	if err := db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}

	currentRole, _ := s.roleByID(db, user.RoleId)
	isOwner := currentRole != nil && currentRole.OwnerRole

	updates := map[string]any{}

	if name := strings.TrimSpace(payload.Username); name != "" && name != user.Username {
		var count int64
		if err := db.Model(&model.User{}).Where("username = ? AND id <> ?", name, id).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("username already exists")
		}
		updates["username"] = name
	}

	if pw := strings.TrimSpace(payload.Password); pw != "" {
		hashed, err := usedHashPassword(pw)
		if err != nil {
			return nil, err
		}
		updates["password"] = hashed
	}

	if payload.DataLimit != user.DataLimit {
		updates["data_limit"] = payload.DataLimit
	}

	if payload.RoleId > 0 && payload.RoleId != user.RoleId {
		if isOwner {
			return nil, errors.New("owner role cannot be changed")
		}
		if _, err := s.roleByID(db, payload.RoleId); err != nil {
			return nil, err
		}
		updates["role_id"] = payload.RoleId
	}

	if strings.TrimSpace(payload.Status) != "" {
		status := normalizeAdminStatus(payload.Status)
		if !validAdminStatus(status) {
			return nil, errors.New("invalid status")
		}
		if isOwner && status != model.AdminStatusActive {
			return nil, errors.New("owner account cannot be disabled")
		}
		updates["status"] = status
	}

	if len(updates) > 0 {
		if err := db.Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.Get(id)
}

// Delete removes an account; the owner account cannot be deleted.
func (s *AdminService) Delete(id int) error {
	if id <= 0 {
		return errors.New("invalid admin id")
	}
	db := database.GetDB()
	var user model.User
	if err := db.Where("id = ?", id).First(&user).Error; err != nil {
		return err
	}
	if role, _ := s.roleByID(db, user.RoleId); role != nil && role.OwnerRole {
		return errors.New("owner account cannot be deleted")
	}
	res := db.Where("id = ?", id).Delete(&model.User{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("admin not found")
	}
	return nil
}

// SetStatus enables or disables an account.
func (s *AdminService) SetStatus(id int, status string) (*AdminView, error) {
	if !validAdminStatus(status) {
		return nil, errors.New("invalid status")
	}
	db := database.GetDB()
	var user model.User
	if err := db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	if role, _ := s.roleByID(db, user.RoleId); role != nil && role.OwnerRole && status != model.AdminStatusActive {
		return nil, errors.New("owner account cannot be disabled")
	}
	if err := db.Model(&model.User{}).Where("id = ?", id).Update("status", status).Error; err != nil {
		return nil, err
	}
	return s.Get(id)
}

// EnsureOwnerRole backfills the owner role on the first account if it is unset.
// Exposed for the CLI/server bootstrap path.
func (s *AdminService) EnsureOwnerRole() error {
	db := database.GetDB()
	if db == nil {
		return errors.New("database is not initialized")
	}
	ownerID, err := s.ownerRoleID(db)
	if err != nil {
		return err
	}
	return db.Model(&model.User{}).
		Where("role_id = 0").
		Order("id ASC").
		Limit(1).
		Update("role_id", ownerID).Error
}

// ---- per-admin client operations -------------------------------------------

// clientEmailsByAdminID lists the emails of the clients an admin owns, with an
// optional enable filter. The admin must exist.
func (s *AdminService) clientEmailsByAdminID(tx *gorm.DB, id int, enabled *bool) ([]string, error) {
	if id <= 0 {
		return nil, errors.New("invalid admin id")
	}
	var existing model.User
	if err := tx.Where("id = ?", id).First(&existing).Error; err != nil {
		return nil, err
	}

	query := tx.Model(&model.ClientRecord{}).Where("owner_admin_id = ?", id)
	if enabled != nil {
		query = query.Where("enable = ?", *enabled)
	}

	var emails []string
	if err := query.Pluck("email", &emails).Error; err != nil {
		return nil, err
	}
	return emails, nil
}

// ResetUsage zeroes the traffic of every client the admin owns and clears the
// admin's aggregate usage counter.
func (s *AdminService) ResetUsage(id int) error {
	db := database.GetDB()
	emails, err := s.clientEmailsByAdminID(db, id, nil)
	if err != nil {
		return err
	}

	if len(emails) > 0 {
		affected, resetErr := (&core.ClientService{}).BulkResetTraffic(&core.InboundService{}, emails)
		if resetErr != nil {
			return resetErr
		}
		if affected > 0 {
			(&core.XrayService{}).SetToNeedRestart()
		}
	}

	res := db.Model(&model.User{}).Where("id = ?", id).Update("used_bytes", 0)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("admin not found")
	}
	return nil
}

// DisableAllActiveUsers disables every enabled client the admin owns.
func (s *AdminService) DisableAllActiveUsers(id int) (int64, error) {
	db := database.GetDB()
	enabled := true
	emails, err := s.clientEmailsByAdminID(db, id, &enabled)
	if err != nil {
		return 0, err
	}
	if len(emails) == 0 {
		return 0, nil
	}

	result, needRestart, err := (&core.ClientService{}).BulkSetEnable(&core.InboundService{}, emails, false)
	if needRestart {
		(&core.XrayService{}).SetToNeedRestart()
	}
	if err != nil {
		return int64(result.Changed), err
	}
	return int64(result.Changed), nil
}

// ActivateAllDisabledUsers enables every disabled client the admin owns.
func (s *AdminService) ActivateAllDisabledUsers(id int) (int64, error) {
	db := database.GetDB()
	disabled := false
	emails, err := s.clientEmailsByAdminID(db, id, &disabled)
	if err != nil {
		return 0, err
	}
	if len(emails) == 0 {
		return 0, nil
	}

	result, needRestart, err := (&core.ClientService{}).BulkSetEnable(&core.InboundService{}, emails, true)
	if needRestart {
		(&core.XrayService{}).SetToNeedRestart()
	}
	if err != nil {
		return int64(result.Changed), err
	}
	return int64(result.Changed), nil
}

// RemoveAllUsers deletes every client the admin owns.
func (s *AdminService) RemoveAllUsers(id int) (int, error) {
	db := database.GetDB()
	emails, err := s.clientEmailsByAdminID(db, id, nil)
	if err != nil {
		return 0, err
	}
	if len(emails) == 0 {
		return 0, nil
	}

	result, needRestart, err := (&core.ClientService{}).BulkDelete(&core.InboundService{}, emails, false)
	if needRestart {
		(&core.XrayService{}).SetToNeedRestart()
	}
	if err != nil {
		return result.Deleted, err
	}
	return result.Deleted, nil
}
