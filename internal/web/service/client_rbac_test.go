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
	// A client on inbounds 1 and 2. The inbound dimension is a parameter now, so
	// the cases below state the client's attachments explicitly.
	onInbounds := []int{1, 2}

	all := ClientAccessScope{AdminID: 7, Mode: ClientAccessAll}
	if !ClientRecordAllowed(all, rec, onInbounds) {
		t.Error("all-scope must allow the record")
	}

	own := ClientAccessScope{AdminID: 7, Mode: ClientAccessOwn}
	if !ClientRecordAllowed(own, rec, onInbounds) {
		t.Error("own-scope must allow a record owned by the admin")
	}

	other := ClientAccessScope{AdminID: 9, Mode: ClientAccessOwn}
	if ClientRecordAllowed(other, rec, onInbounds) {
		t.Error("own-scope must reject a record owned by another admin")
	}

	none := ClientAccessScope{AdminID: 7, Mode: ClientAccessNone}
	if ClientRecordAllowed(none, rec, onInbounds) {
		t.Error("none-scope must reject every record")
	}

	restricted := ClientAccessScope{
		AdminID: 7, Mode: ClientAccessAll,
		RestrictGroups: true, AllowedGroups: []string{"free"},
	}
	if ClientRecordAllowed(restricted, rec, onInbounds) {
		t.Error("group restriction must reject a record outside the allowed groups")
	}
}

// The inbound restriction has to hold even when the ownership scope says "all":
// a role that may see every client on ONE inbound must not be able to authorise a
// client that lives on another. Before the inbound dimension existed here, a scope
// restricted to inbound 1 still approved any client by email.
func TestClientRecordAllowedHonoursInboundRestriction(t *testing.T) {
	rec := &model.ClientRecord{Email: "a@b", Group: "vip", OwnerAdminId: 7}

	onAllowed := ClientAccessScope{
		AdminID: 7, Mode: ClientAccessAll,
		RestrictInbounds: true, AllowedInboundIDs: []int{1},
	}
	if !ClientRecordAllowed(onAllowed, rec, []int{1}) {
		t.Error("a client on an allowed inbound must be permitted")
	}
	if ClientRecordAllowed(onAllowed, rec, []int{2}) {
		t.Error("a client on a forbidden inbound must be rejected")
	}
	// Attached to an allowed inbound and a forbidden one. It IS on the allowed
	// inbound, so the role that may see that inbound must see it — the same client
	// appears in that inbound's own client list. Hiding it would make the clients
	// page disagree with the inbound it was filtered by.
	if !ClientRecordAllowed(onAllowed, rec, []int{1, 2}) {
		t.Error("a client also on an allowed inbound must be permitted")
	}
	// No inbound at all is not "on an allowed inbound".
	if ClientRecordAllowed(onAllowed, rec, nil) {
		t.Error("a client on no inbound must be rejected when the scope is restricted")
	}

	// An unrestricted inbound scope keeps approving, so this does not narrow roles
	// that were never restricted.
	unrestricted := ClientAccessScope{AdminID: 7, Mode: ClientAccessAll, AllowAllInbounds: true}
	if !ClientRecordAllowed(unrestricted, rec, nil) {
		t.Error("an unrestricted inbound scope must still permit the client")
	}
}

// The scope has to reach SQL as well as the per-record predicate, or the list
// would still hand back rows the predicate would reject.
func TestApplyClientAccessScopeFiltersByInbound(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "inbound-scope.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	db := database.GetDB()

	mkInbound := func(tag string) int {
		in := &model.Inbound{Tag: tag, Enable: true, Port: 10000 + len(tag)}
		if err := db.Create(in).Error; err != nil {
			t.Fatalf("create inbound %s: %v", tag, err)
		}
		return in.Id
	}
	inA, inB := mkInbound("a"), mkInbound("b")

	mkClient := func(email string, inboundIDs []int) int {
		rec := &model.ClientRecord{Email: email, Enable: true, TotalGB: 0, ExpiryTime: 0}
		if err := db.Create(rec).Error; err != nil {
			t.Fatalf("create client %s: %v", email, err)
		}
		for _, id := range inboundIDs {
			link := &model.ClientInbound{ClientId: rec.Id, InboundId: id}
			if err := db.Create(link).Error; err != nil {
				t.Fatalf("attach %s to %d: %v", email, id, err)
			}
		}
		return rec.Id
	}
	mkClient("on-a@x", []int{inA})
	mkClient("on-b@x", []int{inB})
	mkClient("on-both@x", []int{inA, inB})
	mkClient("no-inbound@x", nil)

	emailsFor := func(scope ClientAccessScope) []string {
		var ids []int
		if err := applyClientAccessScope(db.Model(&model.ClientRecord{}), scope).
			Order("email").Pluck("id", &ids).Error; err != nil {
			t.Fatalf("scoped query: %v", err)
		}
		var out []string
		for _, id := range ids {
			var rec model.ClientRecord
			if err := db.First(&rec, id).Error; err != nil {
				t.Fatalf("load %d: %v", id, err)
			}
			out = append(out, rec.Email)
		}
		return out
	}

	restricted := ClientAccessScope{
		Mode:              ClientAccessAll,
		RestrictInbounds:  true,
		AllowedInboundIDs: []int{inA},
	}
	got := emailsFor(restricted)
	// on-a is attached only to the allowed inbound; on-both is attached to it too,
	// so both are reachable through it. on-b is attached only to the forbidden
	// inbound, and no-inbound is on none.
	want := []string{"on-a@x", "on-both@x"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("inbound-restricted scope returned %v, want %v", got, want)
	}
	// The SQL filter and the per-record predicate must agree, or the list could
	// hand back a row that every other check rejects. This asserts the same set
	// through both.
	for _, email := range []string{"on-a@x", "on-b@x", "on-both@x", "no-inbound@x"} {
		var rec model.ClientRecord
		if err := db.Where("email = ?", email).First(&rec).Error; err != nil {
			t.Fatalf("load %s: %v", email, err)
		}
		var ids []int
		if err := db.Table(model.ClientInbound{}.TableName()).
			Where("client_id = ?", rec.Id).
			Pluck("inbound_id", &ids).Error; err != nil {
			t.Fatalf("attachments for %s: %v", email, err)
		}
		inSQL := false
		for _, e := range got {
			if e == email {
				inSQL = true
			}
		}
		inPredicate := ClientRecordAllowed(restricted, &rec, ids)
		if inSQL != inPredicate {
			t.Errorf("%s: SQL says visible=%v but the predicate says %v — the two must agree",
				email, inSQL, inPredicate)
		}
	}

	// And an unrestricted scope still returns everything, so roles that were never
	// restricted are unaffected.
	unrestricted := ClientAccessScope{Mode: ClientAccessAll, AllowAllInbounds: true}
	if got := emailsFor(unrestricted); len(got) != 4 {
		t.Errorf("unrestricted scope returned %v, want all four clients", got)
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

// The routes that address a client by something other than its email have to apply
// the scope too, or the checks on the email-addressed routes are decorative.
//
// Each of these was reachable without any scope check: the export button returned
// every client in a file, the subscription-link route rendered a client's links
// from its sub id, the Happ-link route took a numeric client id, and the telegram
// lookup resolved a client from an id. A role that hides a client from the clients
// list must not be able to read it through any of them.
func TestNonEmailAddressingRoutesHonourTheScope(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "bypass.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	db := database.GetDB()

	allowed := &model.Inbound{Tag: "allowed", Enable: true, Port: 41001}
	other := &model.Inbound{Tag: "other", Enable: true, Port: 41002}
	for _, ib := range []*model.Inbound{allowed, other} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", ib.Tag, err)
		}
	}

	mkClient := func(email, subID string, tgID int64, inboundIDs []int) *model.ClientRecord {
		rec := &model.ClientRecord{
			Email: email, SubID: subID, TgID: tgID, Enable: true, TotalGB: 0, ExpiryTime: 0,
		}
		if err := db.Create(rec).Error; err != nil {
			t.Fatalf("create client %s: %v", email, err)
		}
		for _, id := range inboundIDs {
			if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: id}).Error; err != nil {
				t.Fatalf("attach %s: %v", email, err)
			}
		}
		return rec
	}
	mine := mkClient("mine@x", "sub-mine", 777, []int{allowed.Id})
	theirs := mkClient("theirs@x", "sub-theirs", 777, []int{other.Id})

	scope := ClientAccessScope{
		Mode:              ClientAccessAll,
		RestrictInbounds:  true,
		AllowedInboundIDs: []int{allowed.Id},
	}
	svc := &ClientService{}

	t.Run("sub id", func(t *testing.T) {
		if _, err := svc.RequireClientForScopeBySubID(scope, "sub-mine"); err != nil {
			t.Errorf("a client on an allowed inbound must be reachable by sub id: %v", err)
		}
		if _, err := svc.RequireClientForScopeBySubID(scope, "sub-theirs"); err == nil {
			t.Error("a client on a forbidden inbound must not be reachable by sub id")
		}
	})

	t.Run("numeric id", func(t *testing.T) {
		if _, err := svc.RequireClientForScopeByID(scope, mine.Id); err != nil {
			t.Errorf("a client on an allowed inbound must be reachable by id: %v", err)
		}
		if _, err := svc.RequireClientForScopeByID(scope, theirs.Id); err == nil {
			t.Error("a client on a forbidden inbound must not be reachable by id")
		}
	})

	t.Run("telegram id narrows instead of failing", func(t *testing.T) {
		// Both clients share tgId 777. The lookup must return only the permitted one
		// and not fail the whole request, which would be a worse answer than a
		// partial list for a real user.
		records, err := svc.GetRecordsByTgID(777)
		if err != nil {
			t.Fatalf("GetRecordsByTgID: %v", err)
		}
		if len(records) != 2 {
			t.Fatalf("expected both clients to share the tg id, got %d", len(records))
		}
		filtered := FilterRecordsForScope(scope, records)
		if len(filtered) != 1 || filtered[0].Email != "mine@x" {
			t.Errorf("filtered = %v, want just mine@x", recordEmails(filtered))
		}
	})

	t.Run("export", func(t *testing.T) {
		items, err := svc.ExportForScope(scope)
		if err != nil {
			t.Fatalf("ExportForScope: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("export returned %d clients, want 1", len(items))
		}
		if items[0].Client.Email != "mine@x" {
			t.Errorf("export returned %s, want mine@x", items[0].Client.Email)
		}

		// ExportAll keeps its meaning for internal callers.
		all, err := svc.ExportAll()
		if err != nil {
			t.Fatalf("ExportAll: %v", err)
		}
		if len(all) != 2 {
			t.Errorf("ExportAll returned %d, want both clients", len(all))
		}
	})

	t.Run("an unrestricted scope reaches everything", func(t *testing.T) {
		open := ClientAccessScope{Mode: ClientAccessAll, AllowAllGroups: true, AllowAllInbounds: true}
		if _, err := svc.RequireClientForScopeBySubID(open, "sub-theirs"); err != nil {
			t.Errorf("unrestricted scope must reach any client: %v", err)
		}
		if _, err := svc.RequireClientForScopeByID(open, theirs.Id); err != nil {
			t.Errorf("unrestricted scope must reach any client by id: %v", err)
		}
	})
}

func recordEmails(records []*model.ClientRecord) []string {
	out := make([]string, 0, len(records))
	for _, r := range records {
		if r != nil {
			out = append(out, r.Email)
		}
	}
	return out
}
