package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClientAccessModeFromPermission(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want ClientAccessMode
	}{
		{"bool true", true, ClientAccessAll},
		{"bool false", false, ClientAccessNone},
		{"string all", "all", ClientAccessAll},
		{"string own", "own", ClientAccessOwn},
		{"scope 2", map[string]any{"scope": float64(2)}, ClientAccessAll},
		{"scope 1", map[string]any{"scope": float64(1)}, ClientAccessOwn},
		{"scope 0", map[string]any{"scope": float64(0)}, ClientAccessNone},
		{"nil", nil, ClientAccessNone},
	}
	for _, tc := range cases {
		if got := ClientAccessModeFromPermission(tc.in); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestClientRecordAllowedScopes(t *testing.T) {
	rec := &model.ClientRecord{Email: "a@b", Group: "vip", OwnerAdminId: 7}

	all := ClientAccessScope{AdminID: 7, Mode: ClientAccessAll}
	if !ClientRecordAllowed(all, rec) {
		t.Error("all-scope must allow the record")
	}

	own := ClientAccessScope{AdminID: 7, Mode: ClientAccessOwn}
	if !ClientRecordAllowed(own, rec) {
		t.Error("own-scope must allow a record owned by the admin")
	}

	other := ClientAccessScope{AdminID: 9, Mode: ClientAccessOwn}
	if ClientRecordAllowed(other, rec) {
		t.Error("own-scope must reject a record owned by another admin")
	}

	none := ClientAccessScope{AdminID: 7, Mode: ClientAccessNone}
	if ClientRecordAllowed(none, rec) {
		t.Error("none-scope must reject every record")
	}

	restricted := ClientAccessScope{
		AdminID: 7, Mode: ClientAccessAll,
		RestrictGroups: true, AllowedGroups: []string{"free"},
	}
	if ClientRecordAllowed(restricted, rec) {
		t.Error("group restriction must reject a record outside the allowed groups")
	}
}

func TestRoleAccessScopeDefaults(t *testing.T) {
	// A non-owner role with an empty access document keeps full group access.
	empty := &model.AdminRole{AccessJSON: "{}"}
	_, allowAll, groups := roleGroupAccessScope(empty)
	if !allowAll || len(groups) != 0 {
		t.Errorf("empty access doc should allow all groups, got allowAll=%v groups=%v", allowAll, groups)
	}

	restricted := &model.AdminRole{AccessJSON: `{"allowAllGroups":false,"allowedGroups":["VIP"," Free "]}`}
	_, allowAll, groups = roleGroupAccessScope(restricted)
	if allowAll {
		t.Error("restricted role must not allow all groups")
	}
	if len(groups) != 2 || groups[0] != "vip" || groups[1] != "free" {
		t.Errorf("groups not normalized: %v", groups)
	}
}

func TestClientAccessScopeForAdminCapsNonOwnerAtOwn(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "scope.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	var adminRole model.AdminRole
	if err := db.Where("slug = ?", model.AdminRoleSlugAdministrator).First(&adminRole).Error; err != nil {
		t.Fatalf("load administrator role: %v", err)
	}

	// An administrator whose permission says "all" must still be capped at "own".
	adminRole.PermissionsJSON = `{"users":{"read":{"scope":2}}}`
	if err := db.Model(&model.AdminRole{}).Where("id = ?", adminRole.Id).
		Update("permissions", adminRole.PermissionsJSON).Error; err != nil {
		t.Fatalf("update role: %v", err)
	}

	user := &model.User{Id: 42, Username: "op", RoleId: adminRole.Id, Status: model.AdminStatusActive}
	scope := ClientAccessScopeForAdmin(user, "read")
	if scope.Mode != ClientAccessOwn {
		t.Fatalf("non-owner scope = %q, want %q", scope.Mode, ClientAccessOwn)
	}

	var ownerRole model.AdminRole
	if err := db.Where("slug = ?", model.AdminRoleSlugOwner).First(&ownerRole).Error; err != nil {
		t.Fatalf("load owner role: %v", err)
	}
	owner := &model.User{Id: 1, Username: "root", RoleId: ownerRole.Id, Status: model.AdminStatusActive}
	if got := ClientAccessScopeForAdmin(owner, "read"); got.Mode != ClientAccessAll {
		t.Fatalf("owner scope = %q, want %q", got.Mode, ClientAccessAll)
	}
}
