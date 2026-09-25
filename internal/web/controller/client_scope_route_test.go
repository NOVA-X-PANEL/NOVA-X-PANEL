package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The scoping a role applies has to hold at the route, not only in the service.
//
// These tests exercise the real gin handlers with a real session, because the
// service-level tests cannot see a handler that forgets to ask. The routes that
// address a client by something other than its email had no check at all, which is
// exactly the kind of gap a service test passes over.
func newScopedClientRouter(t *testing.T, user *model.User, generator service.HappLinkGenerator) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	store := cookie.NewStore([]byte("client-scope-test-secret"))
	router.Use(sessions.Sessions("3x-ui", store))
	router.Use(func(c *gin.Context) {
		c.Set("I18n", func(_ locale.I18nType, key string, _ ...string) string { return key })
		c.Next()
	})

	// Log in by writing the session, then continue on the same cookie jar.
	router.GET("/test-login", func(c *gin.Context) {
		if err := session.SetLoginUser(c, user); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	a := &ClientController{happGenerator: generator}
	a.initRouter(router.Group("/clients"))
	return router
}

// seedScopedRouteFixtures builds two inbounds, one client on each, and returns the
// ids along with a role restricted to the first inbound.
func seedScopedRouteFixtures(t *testing.T) (allowedClientID, forbiddenClientID int, restrictedRole *model.AdminRole) {
	t.Helper()
	db := database.GetDB()

	allowed := &model.Inbound{Tag: "route-allowed", Enable: true, Port: 42001}
	forbidden := &model.Inbound{Tag: "route-forbidden", Enable: true, Port: 42002}
	for _, ib := range []*model.Inbound{allowed, forbidden} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", ib.Tag, err)
		}
	}

	mkClient := func(email string, inboundID int) int {
		rec := &model.ClientRecord{Email: email, Enable: true, TotalGB: 0, ExpiryTime: 0}
		if err := db.Create(rec).Error; err != nil {
			t.Fatalf("create client %s: %v", email, err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: inboundID}).Error; err != nil {
			t.Fatalf("attach %s: %v", email, err)
		}
		return rec.Id
	}
	allowedClient := mkClient("allowed@route", allowed.Id)
	forbiddenClient := mkClient("forbidden@route", forbidden.Id)

	role := &model.AdminRole{
		Name:            "route-inbound-only",
		Slug:            "route-inbound-only",
		PermissionsJSON: `{"users":{"read":{"scope":2}},"inbounds":{"read":true}}`,
		AccessJSON:      `{"allowAllGroups":true,"allowAllInbounds":false,"allowed_inbound_ids":[` + itoa(allowed.Id) + `]}`,
	}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	return allowedClient, forbiddenClient, role
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	out := ""
	for n > 0 {
		out = string(rune('0'+n%10)) + out
		n /= 10
	}
	return out
}

func TestHappLinkRouteIsScopedToAllowedInbounds(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	allowedID, forbiddenID, role := seedScopedRouteFixtures(t)

	user := &model.User{
		Username: "scoped-op", RoleId: role.Id, Status: model.AdminStatusActive,
	}
	if err := database.GetDB().Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	fake := &fakeHappLinkGenerator{result: service.HappLinkResult{EncryptedLink: "happ://crypt5/x"}}
	router := newScopedClientRouter(t, user, fake)

	// Establish the session cookie.
	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/test-login", nil))
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d", login.Code)
	}
	cookies := login.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login set no cookie")
	}

	call := func(clientID int) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/clients/happLink/"+itoa(clientID), nil)
		for _, ck := range cookies {
			req.AddCookie(ck)
		}
		router.ServeHTTP(rec, req)
		return rec
	}

	// The client on the allowed inbound is reachable.
	before := fake.calls
	allowedResp := call(allowedID)
	if fake.calls != before+1 {
		t.Errorf("a client on the allowed inbound did not reach the generator; body=%s",
			allowedResp.Body.String())
	}

	// The client on the other inbound is not, and the generator is never called
	// for it — the check has to happen before the work, not after.
	before = fake.calls
	forbiddenResp := call(forbiddenID)
	if fake.calls != before {
		t.Errorf("a client on a forbidden inbound reached the generator; the route "+
			"handler must deny before generating. body=%s", forbiddenResp.Body.String())
	}
	if forbiddenResp.Code == http.StatusOK &&
		forbiddenResp.Body.String() != "" &&
		forbiddenResp.Body.String() == allowedResp.Body.String() {
		t.Error("the forbidden client produced the allowed client's response")
	}
}

// An unrestricted caller still reaches the route, and now without a lookup: a
// scope that hides nothing has nothing to deny. The existing Happ tests cover the
// unrestricted case through the same handler, and this states the intent.
func TestHappLinkRouteAllowsUnrestrictedCaller(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	ownerRole := &model.AdminRole{
		Name: "route-owner", Slug: "route-owner", OwnerRole: true,
		PermissionsJSON: `{}`, AccessJSON: `{"allowAllGroups":true,"allowAllInbounds":true}`,
	}
	if err := database.GetDB().Create(ownerRole).Error; err != nil {
		t.Fatalf("create owner role: %v", err)
	}
	owner := &model.User{Username: "root", RoleId: ownerRole.Id, Status: model.AdminStatusActive}
	if err := database.GetDB().Create(owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}

	fake := &fakeHappLinkGenerator{result: service.HappLinkResult{EncryptedLink: "happ://crypt5/y"}}
	router := newScopedClientRouter(t, owner, fake)

	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/test-login", nil))
	cookies := login.Result().Cookies()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/clients/happLink/7", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	router.ServeHTTP(rec, req)

	if fake.calls != 1 {
		t.Errorf("an unrestricted caller must reach the generator, got %d calls", fake.calls)
	}
}

// The live-status reads are lists of client identities, so they have to be scoped
// too. Each was reachable by any signed-in admin regardless of role: the online
// list returned every client's email, the last-online map was keyed by email, and
// three endpoints keyed by a client's uuid returned its addresses and the inbounds
// it was using. A role that hides a client from the clients list must not be able
// to read it from any of them.
func TestLiveStatusReadsAreScoped(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	_, _, role := seedScopedRouteFixtures(t)
	user := &model.User{Username: "scoped-status", RoleId: role.Id, Status: model.AdminStatusActive}
	if err := database.GetDB().Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	// last_online lives on the traffic counters, one row per email.
	db := database.GetDB()
	for i, email := range []string{"allowed@route", "forbidden@route"} {
		row := &xray.ClientTraffic{Email: email, LastOnline: int64(1_700_000_000_000 + i)}
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed traffic %s: %v", email, err)
		}
	}

	fake := &fakeHappLinkGenerator{}
	router := newScopedClientRouter(t, user, fake)

	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/test-login", nil))
	cookies := login.Result().Cookies()

	post := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		for _, ck := range cookies {
			req.AddCookie(ck)
		}
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("lastOnline is keyed by email", func(t *testing.T) {
		rec := post("/clients/lastOnline")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var envelope struct {
			Obj map[string]int64 `json:"obj"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("decode %s: %v", rec.Body.String(), err)
		}
		if _, ok := envelope.Obj["allowed@route"]; !ok {
			t.Errorf("the in-scope client is missing: %v", envelope.Obj)
		}
		if _, leaked := envelope.Obj["forbidden@route"]; leaked {
			t.Errorf("a client on a forbidden inbound leaked into lastOnline: %v", envelope.Obj)
		}
	})

	// onlines, onlinesByGuid and clientIpsByGuid read the running core, which is
	// absent in a unit test, so they return empty for every caller and cannot be
	// distinguished here. What is asserted is that the scoped path does not fail
	// the request — the filtering itself is covered by UUIDsForScope's test.
	for _, path := range []string{"/clients/onlines", "/clients/onlinesByGuid", "/clients/activeInbounds", "/clients/clientIpsByGuid"} {
		t.Run("responds for a scoped caller "+path, func(t *testing.T) {
			rec := post(path)
			if rec.Code != http.StatusOK {
				t.Errorf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
			}
		})
	}
}
