package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestClientSummaryAndOnlineRoutesByOwner(t *testing.T) {
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
	role := model.AdminRole{Name: "all-client-admin", Slug: "all-client-admin", PermissionsJSON: `{"users":{"read":{"scope":2}}}`, AccessJSON: `{"allowAllGroups":true,"allowAllInbounds":true}`}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	owner := model.User{Username: "summary-owner", RoleId: ownerRole.Id, Status: model.AdminStatusActive}
	admin := model.User{Username: "summary-admin", RoleId: role.Id, Status: model.AdminStatusActive}
	other := model.User{Username: "summary-other", RoleId: role.Id, Status: model.AdminStatusActive}
	for _, u := range []*model.User{&owner, &admin, &other} {
		if err := db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows := []struct {
		email   string
		owner   int
		expired bool
	}{
		{"owner-online@x", owner.Id, false}, {"own-online@x", admin.Id, false}, {"other-online@x", other.Id, false},
		{"own-expired@x", admin.Id, true}, {"other-expired@x", other.Id, true}, {"unowned@x", 0, false},
	}
	for _, row := range rows {
		rec := model.ClientRecord{Email: row.email, OwnerAdminId: int64(row.owner), Enable: true}
		if row.expired {
			rec.ExpiryTime = time.Now().Add(-24 * time.Hour).UnixMilli()
		}
		if err := db.Create(&rec).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&xray.ClientTraffic{Email: row.email, LastOnline: 123}).Error; err != nil {
			t.Fatal(err)
		}
	}
	process := xray.NewTestProcess(nil, "")
	process.SetNodeOnlineTree(1, map[string][]string{"node-guid": {"owner-online@x", "own-online@x", "other-online@x"}})
	restore := service.SetXrayProcessForTest(process)
	t.Cleanup(restore)

	for _, tc := range []struct {
		name                    string
		user                    *model.User
		total, online, depleted int
		onlineEmails            []string
	}{
		{"owner", &owner, 6, 3, 2, []string{"owner-online@x", "own-online@x", "other-online@x"}},
		{"admin with all grant", &admin, 2, 1, 1, []string{"own-online@x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := newScopedClientRouter(t, tc.user, &fakeHappLinkGenerator{})
			login := httptest.NewRecorder()
			router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/test-login", nil))
			cookies := login.Result().Cookies()
			if len(cookies) == 0 {
				t.Fatal("login failed")
			}
			call := func(method, path string) []byte {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(method, path, nil)
				for _, ck := range cookies {
					req.AddCookie(ck)
				}
				router.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("%s: status %d: %s", path, rec.Code, rec.Body.String())
				}
				return rec.Body.Bytes()
			}
			var page struct {
				Success bool                       `json:"success"`
				Obj     service.ClientPageResponse `json:"obj"`
			}
			if err := json.Unmarshal(call("GET", "/clients/list/paged?pageSize=1&filter=depleted"), &page); err != nil {
				t.Fatal(err)
			}
			if !page.Success || page.Obj.Total != tc.total || page.Obj.Filtered != tc.depleted || page.Obj.Summary.Total != tc.total || page.Obj.Summary.OnlineCount != tc.online || page.Obj.Summary.DepletedCount != tc.depleted {
				t.Fatalf("page = %+v", page)
			}
			var online struct {
				Obj []string `json:"obj"`
			}
			if err := json.Unmarshal(call("POST", "/clients/onlines"), &online); err != nil {
				t.Fatal(err)
			}
			slices.Sort(online.Obj)
			slices.Sort(tc.onlineEmails)
			if !slices.Equal(online.Obj, tc.onlineEmails) {
				t.Fatalf("onlines = %v, want %v", online.Obj, tc.onlineEmails)
			}
			var byGuid struct {
				Obj map[string][]string `json:"obj"`
			}
			if err := json.Unmarshal(call("POST", "/clients/onlinesByGuid"), &byGuid); err != nil {
				t.Fatal(err)
			}
			slices.Sort(byGuid.Obj["node-guid"])
			if !slices.Equal(byGuid.Obj["node-guid"], tc.onlineEmails) {
				t.Fatalf("onlinesByGuid = %v, want %v", byGuid.Obj, tc.onlineEmails)
			}
			if tc.user == &admin {
				for _, path := range []string{"/clients/list", "/clients/lastOnline"} {
					method := "GET"
					if strings.HasSuffix(path, "lastOnline") {
						method = "POST"
					}
					body := call(method, path)
					if strings.Contains(string(body), "other-") || strings.Contains(string(body), "owner-online") || strings.Contains(string(body), "unowned@x") {
						t.Fatalf("%s leaked identities: %s", path, body)
					}
				}
				body := call("GET", "/clients/get/other-online@x")
				var obj struct {
					Success bool `json:"success"`
				}
				if err := json.Unmarshal(body, &obj); err != nil || obj.Success {
					t.Fatalf("other client's detail was exposed: %s", body)
				}
			}
		})
	}

	// The route also accepts simple view; it must not turn that grant into an
	// empty result, nor turn an all grant into visibility of another owner.
	if err := db.Model(&role).Update("permissions", `{"users":{"read_simple":{"scope":2}}}`).Error; err != nil {
		t.Fatal(err)
	}
	simpleRouter := newScopedClientRouter(t, &admin, &fakeHappLinkGenerator{})
	login := httptest.NewRecorder()
	simpleRouter.ServeHTTP(login, httptest.NewRequest("GET", "/test-login", nil))
	req := httptest.NewRequest("GET", "/clients/list/paged", nil)
	for _, ck := range login.Result().Cookies() {
		req.AddCookie(ck)
	}
	simpleResponse := httptest.NewRecorder()
	simpleRouter.ServeHTTP(simpleResponse, req)
	var simplePage struct {
		Success bool                       `json:"success"`
		Obj     service.ClientPageResponse `json:"obj"`
	}
	if err := json.Unmarshal(simpleResponse.Body.Bytes(), &simplePage); err != nil || !simplePage.Success || simplePage.Obj.Total != 2 {
		t.Fatalf("simple-view page: %s, error %v", simpleResponse.Body.String(), err)
	}
	if err := db.Model(&role).Update("permissions", `{"users":{"read":{"scope":2}}}`).Error; err != nil {
		t.Fatal(err)
	}

	// An admin-bound token must use the same account scope as a browser.
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("api_authed", true)
		c.Set("api_token_admin_id", admin.Id)
		session.SetAPIAuthUser(c, &admin)
		c.Next()
	})
	(&ClientController{}).initRouter(router.Group("/clients"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/clients/list/paged", nil))
	var tokenPage struct {
		Success bool                       `json:"success"`
		Obj     service.ClientPageResponse `json:"obj"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tokenPage); err != nil || !tokenPage.Success || tokenPage.Obj.Total != 2 || tokenPage.Obj.Summary.OnlineCount != 1 {
		t.Fatalf("token page: %s, error %v", rec.Body.String(), err)
	}

	// A bound token is not the panel-wide system token exception and cannot
	// invoke an operation that requires access to every client.
	controller := &ClientController{}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("api_authed", true)
	ctx.Set("api_token_admin_id", admin.Id)
	session.SetAPIAuthUser(ctx, &admin)
	if controller.requireGlobalClientScope(ctx, "read") {
		t.Fatal("admin-bound token bypassed the global scope check")
	}
	adminController := &AdminController{}
	if adminController.canManageAdminUsers(ctx, other.Id, "read") {
		t.Fatal("admin-bound token could manage another admin's clients")
	}
	if !adminController.canManageAdminUsers(ctx, admin.Id, "read") {
		t.Fatal("admin-bound token cannot manage its own clients")
	}
	systemCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	systemCtx.Set("api_authed", true)
	if !controller.requireGlobalClientScope(systemCtx, "read") {
		t.Fatal("panel-wide system token lost global scope")
	}
	if !adminController.canManageAdminUsers(systemCtx, other.Id, "read") {
		t.Fatal("panel-wide system token lost access to admin management")
	}
}

func TestScopedWebSocketDoesNotBroadcastOtherClients(t *testing.T) {
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
	role := model.AdminRole{Name: "ws-admin", Slug: "ws-admin", PermissionsJSON: `{"users":{"read":{"scope":2}}}`}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "ws-reader", RoleId: role.Id, Status: model.AdminStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"traffic", "client_stats", "inbounds"} {
		msg := []byte(`{"type":"` + kind + `","payload":{"onlineClients":["private@x"]},"time":123}`)
		got := scopeWebSocketMessage(user.Id, user.LoginEpoch, msg)
		var event struct {
			Type    string `json:"type"`
			Payload struct {
				Type string `json:"type"`
			} `json:"payload"`
		}
		err := json.Unmarshal(got, &event)
		wantType := kind
		if kind == "inbounds" {
			wantType = "invalidate"
		}
		if err != nil || event.Type != wantType || (kind == "inbounds" && event.Payload.Type != "inbounds") || strings.Contains(string(got), "private@x") {
			t.Fatalf("%s exposed message: %s (%v)", kind, got, err)
		}
	}
	msg := []byte(`{"type":"traffic","payload":{"onlineClients":["private@x"]}}`)
	if err := db.Model(&user).Update("role_id", ownerRole.Id).Error; err != nil {
		t.Fatal(err)
	}
	if got := scopeWebSocketMessage(user.Id, user.LoginEpoch, msg); string(got) != string(msg) {
		t.Fatalf("owner should retain live traffic: %s", got)
	}
	if err := db.Model(&user).Update("role_id", role.Id).Error; err != nil {
		t.Fatal(err)
	}
	if got := scopeWebSocketMessage(user.Id, user.LoginEpoch, msg); strings.Contains(string(got), "private@x") {
		t.Fatalf("demoted owner leaked identities: %s", got)
	}
	originalEpoch := user.LoginEpoch
	if err := db.Model(&user).Update("login_epoch", originalEpoch+1).Error; err != nil {
		t.Fatal(err)
	}
	if got := scopeWebSocketMessage(user.Id, originalEpoch, msg); got != nil {
		t.Fatalf("revoked session received %s", got)
	}
}
