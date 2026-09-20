package database

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestRBACSeedCreatesDefaultRoles(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "rbac.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	db := GetDB()

	var roles []model.AdminRole
	if err := db.Order("id ASC").Find(&roles).Error; err != nil {
		t.Fatalf("load roles: %v", err)
	}
	if len(roles) != 3 {
		t.Fatalf("expected 3 default roles, got %d", len(roles))
	}

	bySlug := map[string]model.AdminRole{}
	for _, r := range roles {
		bySlug[r.Slug] = r
	}
	for _, slug := range []string{model.AdminRoleSlugOwner, model.AdminRoleSlugAdministrator, model.AdminRoleSlugOperator} {
		if _, ok := bySlug[slug]; !ok {
			t.Fatalf("missing default role %q", slug)
		}
	}
	if !bySlug[model.AdminRoleSlugOwner].OwnerRole {
		t.Fatal("owner role must have OwnerRole=true")
	}
	if bySlug[model.AdminRoleSlugOwner].PermissionsJSON == "" {
		t.Fatal("owner role permissions must not be empty")
	}

	// The first account must be bound to the owner role.
	var users []model.User
	if err := db.Order("id ASC").Find(&users).Error; err != nil {
		t.Fatalf("load users: %v", err)
	}
	if len(users) == 0 {
		t.Fatal("expected the default admin account to exist")
	}
	if users[0].RoleId != bySlug[model.AdminRoleSlugOwner].Id {
		t.Fatalf("first account role_id = %d, want owner %d", users[0].RoleId, bySlug[model.AdminRoleSlugOwner].Id)
	}
	if users[0].Status != model.AdminStatusActive {
		t.Fatalf("first account status = %q, want active", users[0].Status)
	}

	// Idempotent: running the seeder again must not create duplicates.
	if err := seedAdminRBAC(); err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	var count int64
	if err := db.Model(&model.AdminRole{}).Count(&count).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if count != 3 {
		t.Fatalf("after re-seed expected 3 roles, got %d", count)
	}
}
