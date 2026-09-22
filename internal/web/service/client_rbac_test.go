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

func TestClientAccessScopeForAdminHonoursStoredScope(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "scope.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	var adminRole model.AdminRole
	if err := db.Where("slug = ?", model.AdminRoleSlugAdministrator).First(&adminRole).Error; err != nil {
		t.Fatalf("load administrator role: %v", err)
	}

	// "all" means all: every client on the panel, including other admins'.
	// Downgrading this to "own" made a role that granted "all" show an empty
	// list whenever the admin had not created clients themselves.
	adminRole.PermissionsJSON = `{"users":{"read":{"scope":2}}}`
	if err := db.Model(&model.AdminRole{}).Where("id = ?", adminRole.Id).
		Update("permissions", adminRole.PermissionsJSON).Error; err != nil {
		t.Fatalf("update role: %v", err)
	}
	user := &model.User{Id: 42, Username: "op", RoleId: adminRole.Id, Status: model.AdminStatusActive}
	if scope := ClientAccessScopeForAdmin(user, "read"); scope.Mode != ClientAccessAll {
		t.Fatalf("stored scope 2 resolved to %q, want %q", scope.Mode, ClientAccessAll)
	}

	// "own" stays inside the admin's own records.
	adminRole.PermissionsJSON = `{"users":{"read":{"scope":1}}}`
	if err := db.Model(&model.AdminRole{}).Where("id = ?", adminRole.Id).
		Update("permissions", adminRole.PermissionsJSON).Error; err != nil {
		t.Fatalf("update role: %v", err)
	}
	if scope := ClientAccessScopeForAdmin(user, "read"); scope.Mode != ClientAccessOwn {
		t.Fatalf("stored scope 1 resolved to %q, want %q", scope.Mode, ClientAccessOwn)
	}

	// No grant at all is a refusal, not a wider scope.
	adminRole.PermissionsJSON = `{"users":{}}`
	if err := db.Model(&model.AdminRole{}).Where("id = ?", adminRole.Id).
		Update("permissions", adminRole.PermissionsJSON).Error; err != nil {
		t.Fatalf("update role: %v", err)
	}
	if scope := ClientAccessScopeForAdmin(user, "read"); scope.Mode != ClientAccessNone {
		t.Fatalf("missing grant resolved to %q, want %q", scope.Mode, ClientAccessNone)
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

// The inbound list used to be filtered by the legacy user_id column alone, so a
// role that granted an inbound still showed nothing. These cases pin the
// resolution order that replaced it.
func TestInboundAccessScopeForAdmin(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "inbound-scope.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	// No account at all (API token, unit mount) keeps full access.
	if got := InboundAccessScopeForAdmin(nil); !got.All {
		t.Fatalf("nil user should list every inbound, got %+v", got)
	}

	var ownerRole model.AdminRole
	if err := db.Where("slug = ?", model.AdminRoleSlugOwner).First(&ownerRole).Error; err != nil {
		t.Fatalf("load owner role: %v", err)
	}
	if got := InboundAccessScopeForAdmin(&model.User{Id: 1, RoleId: ownerRole.Id}); !got.All {
		t.Fatalf("owner should list every inbound, got %+v", got)
	}

	role := &model.AdminRole{Name: "scoped", Slug: "scoped", PermissionsJSON: `{}`}
	role.AccessJSON = `{"allowAllGroups":true,"allowed_inbound_ids":[3,5]}`
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	user := &model.User{Id: 9, RoleId: role.Id}
	got := InboundAccessScopeForAdmin(user)
	if got.All || len(got.IDs) != 2 || got.IDs[0] != 3 || got.IDs[1] != 5 {
		t.Fatalf("id list should restrict to [3 5], got %+v", got)
	}

	// An empty selection means "no restriction" — the role editor states
	// "leave empty to allow all inbounds".
	if err := db.Model(&model.AdminRole{}).Where("id = ?", role.Id).
		Update("access", `{"allowAllGroups":true,"allowed_inbound_ids":[]}`).Error; err != nil {
		t.Fatalf("update access: %v", err)
	}
	if got := InboundAccessScopeForAdmin(user); !got.All {
		t.Fatalf("empty id list should allow every inbound, got %+v", got)
	}

	// A role with no usable access document keeps the legacy rule instead of
	// silently widening to everything.
	if err := db.Model(&model.AdminRole{}).Where("id = ?", role.Id).
		Update("access", `{}`).Error; err != nil {
		t.Fatalf("update access: %v", err)
	}
	if got := InboundAccessScopeForAdmin(user); got.All || got.LegacyUserID != 9 {
		t.Fatalf("empty access doc should fall back to the legacy owner filter, got %+v", got)
	}

	// An account pointing at a missing role cannot widen either.
	if got := InboundAccessScopeForAdmin(&model.User{Id: 11, RoleId: 99999}); got.All || got.LegacyUserID != 11 {
		t.Fatalf("missing role should fall back to the legacy owner filter, got %+v", got)
	}
}

func TestApplyInboundAccessScopeSQL(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "inbound-sql.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()
	rows := []model.Inbound{
		{UserId: 1, Remark: "mine", Tag: "mine", Port: 20001, Protocol: model.VLESS},
		{UserId: 2, Remark: "theirs", Tag: "theirs", Port: 20002, Protocol: model.VLESS},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed inbound: %v", err)
		}
	}

	list := func(scope InboundAccessScope) []string {
		var got []model.Inbound
		if err := applyInboundAccessScope(db.Model(model.Inbound{}), scope).
			Order("id ASC").Find(&got).Error; err != nil {
			t.Fatalf("query: %v", err)
		}
		out := make([]string, 0, len(got))
		for _, r := range got {
			out = append(out, r.Remark)
		}
		return out
	}

	if got := list(InboundAccessScope{All: true}); len(got) != 2 {
		t.Fatalf("all scope should return both inbounds, got %v", got)
	}
	if got := list(InboundAccessScope{LegacyUserID: 1}); len(got) != 1 || got[0] != "mine" {
		t.Fatalf("legacy scope should return only the account's own inbound, got %v", got)
	}
	id := rows[0].Id
	if got := list(InboundAccessScope{IDs: []int{id}}); len(got) != 1 || got[0] != "mine" {
		t.Fatalf("id scope should return the listed inbound, got %v", got)
	}
	if got := list(InboundAccessScope{}); len(got) != 0 {
		t.Fatalf("empty scope must match nothing, got %v", got)
	}
}
