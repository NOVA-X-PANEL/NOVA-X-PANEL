package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func ownerGuidRegressionDB(t *testing.T) (*model.User, *model.User, *model.User) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	db := database.GetDB()
	var ownerRole model.AdminRole
	if err := db.Where("slug = ?", model.AdminRoleSlugOwner).First(&ownerRole).Error; err != nil {
		t.Fatal(err)
	}
	role := model.AdminRole{Name: "guid-edit-admin", Slug: "guid-edit-admin", PermissionsJSON: `{"users":{"read":{"scope":2},"update":{"scope":2}}}`, AccessJSON: `{"allowAllGroups":true,"allowAllInbounds":true}`}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	owner := &model.User{Username: "guid-edit-owner", RoleId: ownerRole.Id, Status: model.AdminStatusActive}
	admin := &model.User{Username: "guid-edit-admin", RoleId: role.Id, Status: model.AdminStatusActive}
	other := &model.User{Username: "guid-edit-other", RoleId: role.Id, Status: model.AdminStatusActive}
	for _, u := range []*model.User{owner, admin, other} {
		if err := db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}
	return owner, admin, other
}

func ownerGuidRegressionCall(t *testing.T, router *gin.Engine, cookies []*http.Cookie, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func ownerGuidRegressionLogin(t *testing.T, user *model.User) (*gin.Engine, []*http.Cookie) {
	t.Helper()
	router := newScopedClientRouter(t, user, &fakeHappLinkGenerator{})
	login := ownerGuidRegressionCall(t, router, nil, "GET", "/test-login", nil)
	cookies := login.Result().Cookies()
	if login.Code != http.StatusOK || len(cookies) == 0 {
		t.Fatalf("login failed: %s", login.Body.String())
	}
	return router, cookies
}

func TestOwnerClientEditPreservesAssignedOwnershipRegression(t *testing.T) {
	owner, admin, other := ownerGuidRegressionDB(t)
	db := database.GetDB()
	ib := &model.Inbound{UserId: owner.Id, Enable: true, Port: 48121, Tag: "edit-ownership", Protocol: model.VLESS, Settings: `{"clients":[]}`, StreamSettings: `{"network":"tcp","security":"none"}`}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	clientSvc := &service.ClientService{}
	for i, attached := range []bool{true, false, true, false, true} {
		email := fmt.Sprintf("owner-edit-%d@example.com", i)
		client := model.Client{Email: email, ID: fmt.Sprintf("11111111-1111-1111-1111-%012d", i+1), SubID: fmt.Sprintf("owner-edit-sub-%d", i), Enable: true, OwnerAdminId: int64(admin.Id)}
		ids := []int(nil)
		if attached {
			ids = []int{ib.Id}
		} else {
			if err := db.Create(client.ToRecord()).Error; err != nil {
				t.Fatalf("create detached %s: %v", email, err)
			}
			continue
		}
		if _, err := clientSvc.Create(&service.InboundService{}, &service.ClientCreatePayload{Client: client, InboundIds: ids}); err != nil {
			t.Fatalf("create %s: %v", email, err)
		}
	}
	ownerRouter, ownerCookies := ownerGuidRegressionLogin(t, owner)
	adminRouter, adminCookies := ownerGuidRegressionLogin(t, admin)
	otherRouter, otherCookies := ownerGuidRegressionLogin(t, other)

	for _, tc := range []struct {
		name, email string
		router      *gin.Engine
		cookies     []*http.Cookie
		supplied    *int64
		success     bool
	}{
		{"owner attached omitted", "owner-edit-0@example.com", ownerRouter, ownerCookies, nil, true},
		{"owner detached omitted", "owner-edit-1@example.com", ownerRouter, ownerCookies, nil, true},
		{"owner attached spoofed", "owner-edit-2@example.com", ownerRouter, ownerCookies, ptrInt64(int64(other.Id)), true},
		{"owner detached spoofed", "owner-edit-3@example.com", ownerRouter, ownerCookies, ptrInt64(int64(other.Id)), true},
		{"own admin spoofed", "owner-edit-4@example.com", adminRouter, adminCookies, ptrInt64(int64(other.Id)), true},
		{"foreign admin denied", "owner-edit-0@example.com", otherRouter, otherCookies, ptrInt64(int64(other.Id)), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"email": tc.email, "enable": true, "comment": tc.name}
			if tc.supplied != nil {
				body["ownerAdminId"] = *tc.supplied
			}
			w := ownerGuidRegressionCall(t, tc.router, tc.cookies, "POST", "/clients/update/"+tc.email, body)
			var resp struct {
				Success bool `json:"success"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Success != tc.success {
				t.Fatalf("update response %s, error %v", w.Body.String(), err)
			}
			var rec model.ClientRecord
			if err := db.Where("email = ?", tc.email).First(&rec).Error; err != nil {
				t.Fatal(err)
			}
			if rec.OwnerAdminId != int64(admin.Id) {
				t.Fatalf("owner=%d, want %d", rec.OwnerAdminId, admin.Id)
			}
			if tc.success && rec.Comment != tc.name {
				t.Fatalf("comment %q was not saved", rec.Comment)
			}
		})
	}
	// The attached client must retain its owner in the inbound settings, not
	// only in the clients table, so a subsequent sync cannot undo the fix.
	storedIB, err := (&service.InboundService{}).GetInbound(ib.Id)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := (&service.InboundService{}).GetClients(storedIB)
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range clients {
		if client.OwnerAdminId != int64(admin.Id) {
			t.Fatalf("inbound settings changed owner for %s: %d", client.Email, client.OwnerAdminId)
		}
	}
	var page struct {
		Success bool                       `json:"success"`
		Obj     service.ClientPageResponse `json:"obj"`
	}
	w := ownerGuidRegressionCall(t, adminRouter, adminCookies, "GET", "/clients/list/paged", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || !page.Success || page.Obj.Total != 5 {
		t.Fatalf("owner edits removed admin clients from list: %s, %v", w.Body.String(), err)
	}
	// Unassigned remains unassigned on an ordinary owner edit.
	unassigned := model.Client{Email: "unassigned-edit@example.com", ID: "22222222-2222-2222-2222-222222222222", SubID: "unassigned-edit-sub", Enable: true}
	if _, err := clientSvc.Create(&service.InboundService{}, &service.ClientCreatePayload{Client: unassigned, InboundIds: []int{ib.Id}}); err != nil {
		t.Fatal(err)
	}
	w = ownerGuidRegressionCall(t, ownerRouter, ownerCookies, "POST", "/clients/update/"+unassigned.Email, map[string]any{"email": unassigned.Email, "enable": true, "ownerAdminId": other.Id})
	var resp struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || !resp.Success {
		t.Fatalf("unassigned edit: %s (%v)", w.Body.String(), err)
	}
	var unassignedRow model.ClientRecord
	if err := db.Where("email = ?", unassigned.Email).First(&unassignedRow).Error; err != nil || unassignedRow.OwnerAdminId != 0 {
		t.Fatalf("unassigned owner = %d (error %v)", unassignedRow.OwnerAdminId, err)
	}
	// A panel-wide API token is also used for node synchronization. Its incoming
	// ownership field must still travel down that legacy path without an actor.
	tokenRouter := gin.New()
	tokenRouter.Use(func(c *gin.Context) { c.Set("api_authed", true); c.Next() })
	(&ClientController{}).initRouter(tokenRouter.Group("/clients"))
	w = ownerGuidRegressionCall(t, tokenRouter, nil, "POST", "/clients/update/"+unassigned.Email, map[string]any{"email": unassigned.Email, "enable": true, "ownerAdminId": admin.Id})
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || !resp.Success {
		t.Fatalf("node-sync edit: %s (%v)", w.Body.String(), err)
	}
	if err := db.Where("email = ?", unassigned.Email).First(&unassignedRow).Error; err != nil || unassignedRow.OwnerAdminId != int64(admin.Id) {
		t.Fatalf("node-sync owner = %d, want %d (error %v)", unassignedRow.OwnerAdminId, admin.Id, err)
	}
}

func ptrInt64(v int64) *int64 { return &v }

func TestNodeGuidIPAndActiveInboundOwnershipRegression(t *testing.T) {
	owner, admin, other := ownerGuidRegressionDB(t)
	db := database.GetDB()
	localGUID, err := (&service.SettingService{}).GetPanelGuid()
	if err != nil {
		t.Fatal(err)
	}
	node1, node2, node3 := 101, 102, 103
	inbounds := []model.Inbound{
		{Tag: "own-local", Port: 48131},
		{Tag: "foreign-local", Port: 48132},
		{Tag: "own-remote", Port: 48133, NodeID: &node1, OriginNodeGuid: "remote-guid"},
		{Tag: "foreign-remote", Port: 48134, NodeID: &node1, OriginNodeGuid: "remote-guid"},
		{Tag: "own-inactive", Port: 48135, NodeID: &node2, OriginNodeGuid: "inactive-guid"},
		{Tag: "foreign-active", Port: 48136, NodeID: &node2, OriginNodeGuid: "inactive-guid"},
		{Tag: "foreign-only", Port: 48137, NodeID: &node2, OriginNodeGuid: "foreign-guid"},
		{Tag: "own-fallback", Port: 48138, NodeID: &node3},
		{Tag: "own-no-signal", Port: 48139, NodeID: &node3, OriginNodeGuid: "silent-guid"},
	}
	for i := range inbounds {
		if err := db.Create(&inbounds[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	own := model.ClientRecord{Email: "own-guid@example.com", UUID: "fake-client-guid", OwnerAdminId: int64(admin.Id), Enable: true}
	foreign := model.ClientRecord{Email: "foreign-guid@example.com", UUID: "foreign-client-uuid", OwnerAdminId: int64(other.Id), Enable: true}
	for _, rec := range []*model.ClientRecord{&own, &foreign} {
		if err := db.Create(rec).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i, ib := range inbounds {
		clientID := foreign.Id
		if i == 0 || i == 2 || i == 4 || i == 7 || i == 8 {
			clientID = own.Id
		}
		if err := db.Create(&model.ClientInbound{ClientId: clientID, InboundId: ib.Id}).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Unix()
	ips := &service.InboundService{}
	for guid, perEmail := range map[string]map[string][]model.ClientIpEntry{
		"remote-guid":      {own.Email: {{IP: "192.0.2.1", Timestamp: now}}, foreign.Email: {{IP: "192.0.2.2", Timestamp: now}}},
		"fake-client-guid": {foreign.Email: {{IP: "192.0.2.3", Timestamp: now}}},
		"foreign-guid":     {foreign.Email: {{IP: "192.0.2.4", Timestamp: now}}},
	} {
		if err := ips.RecordLocalClientIps(guid, perEmail); err != nil {
			t.Fatal(err)
		}
	}
	process := xray.NewTestProcess(nil, "")
	process.SetNodeActiveInboundTree(1, map[string][]string{
		localGUID:          {"own-local", "foreign-local"},
		"remote-guid":      {"own-remote", "foreign-remote", "own-local"},
		"inactive-guid":    {"foreign-active"},
		"foreign-guid":     {"foreign-only"},
		"fake-client-guid": {"foreign-only"},
		"node:103":         {"own-fallback"},
	})
	restore := service.SetXrayProcessForTest(process)
	t.Cleanup(restore)

	for _, tc := range []struct {
		name  string
		user  *model.User
		owner bool
	}{{"admin", admin, false}, {"owner", owner, true}} {
		t.Run(tc.name, func(t *testing.T) {
			router, cookies := ownerGuidRegressionLogin(t, tc.user)
			w := ownerGuidRegressionCall(t, router, cookies, "POST", "/clients/clientIpsByGuid", nil)
			var got struct {
				Success bool                                        `json:"success"`
				Obj     map[string]map[string][]model.ClientIpEntry `json:"obj"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || !got.Success {
				t.Fatalf("IP response: %s (%v)", w.Body.String(), err)
			}
			if tc.owner {
				if len(got.Obj) != 3 || len(got.Obj["remote-guid"]) != 2 || len(got.Obj["fake-client-guid"][foreign.Email]) != 1 {
					t.Fatalf("owner IPs: %v", got.Obj)
				}
			} else {
				want := map[string]map[string][]model.ClientIpEntry{"remote-guid": {own.Email: {{IP: "192.0.2.1", Timestamp: now}}}}
				if !reflect.DeepEqual(got.Obj, want) {
					t.Fatalf("scoped IPs = %#v, want %#v", got.Obj, want)
				}
			}
			w = ownerGuidRegressionCall(t, router, cookies, "POST", "/clients/activeInbounds", nil)
			var active struct {
				Success bool                `json:"success"`
				Obj     map[string][]string `json:"obj"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &active); err != nil || !active.Success {
				t.Fatalf("active response: %s (%v)", w.Body.String(), err)
			}
			if tc.owner {
				if len(active.Obj) != 6 || len(active.Obj["remote-guid"]) != 3 {
					t.Fatalf("owner active inbounds: %v", active.Obj)
				}
			} else {
				want := map[string][]string{localGUID: {"own-local"}, "remote-guid": {"own-remote"}, "inactive-guid": {}, "node:103": {"own-fallback"}}
				if !reflect.DeepEqual(active.Obj, want) {
					t.Fatalf("scoped active inbounds = %#v, want %#v", active.Obj, want)
				}
			}
		})
	}
	// The client is on several inbounds, but an inbound-restricted role may see
	// status for only the one it is allowed to use.
	access := fmt.Sprintf(`{"allowAllGroups":true,"allowed_inbound_ids":[%d]}`, inbounds[2].Id)
	if err := db.Model(&model.AdminRole{}).Where("id = ?", admin.RoleId).Update("access", access).Error; err != nil {
		t.Fatal(err)
	}
	router, cookies := ownerGuidRegressionLogin(t, admin)
	w := ownerGuidRegressionCall(t, router, cookies, "POST", "/clients/activeInbounds", nil)
	var restricted struct {
		Success bool                `json:"success"`
		Obj     map[string][]string `json:"obj"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &restricted); err != nil || !restricted.Success || !reflect.DeepEqual(restricted.Obj, map[string][]string{"remote-guid": {"own-remote"}}) {
		t.Fatalf("inbound-restricted response %s (error %v)", w.Body.String(), err)
	}
}
