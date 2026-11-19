package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func transferTestDB(t *testing.T) (owner, admin, other *model.User) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	var ownerRole model.AdminRole
	if err := database.GetDB().Where("slug = ?", model.AdminRoleSlugOwner).First(&ownerRole).Error; err != nil {
		t.Fatal(err)
	}
	role := model.AdminRole{
		Name:            "transfer-test-admin",
		Slug:            "transfer-test-admin",
		PermissionsJSON: `{"users":{"read":{"scope":2},"update":{"scope":2}}}`,
		AccessJSON:      `{"allowAllGroups":true,"allowAllInbounds":true}`,
	}
	if err := database.GetDB().Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	owner = &model.User{Username: "transfer-owner", RoleId: ownerRole.Id, Status: model.AdminStatusActive}
	admin = &model.User{Username: "transfer-admin", RoleId: role.Id, Status: model.AdminStatusActive}
	other = &model.User{Username: "transfer-other", RoleId: role.Id, Status: model.AdminStatusActive}
	for _, user := range []*model.User{owner, admin, other} {
		if err := database.GetDB().Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	return owner, admin, other
}

func transferTestCall(t *testing.T, router *gin.Engine, cookies []*http.Cookie, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, nil)
	if len(payload) > 0 {
		req = httptest.NewRequest(method, path, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestOwnerTransfersClientToAdminAndKeepsRelatedData(t *testing.T) {
	owner, from, to := transferTestDB(t)
	db := database.GetDB()
	const email = "move-me@example.test"
	inbound := &model.Inbound{
		UserId: owner.Id, Enable: true, Port: 48211, Tag: "transfer-test-inbound",
		Protocol: model.VLESS,
		Settings: `{"clients":[{"id":"11111111-1111-1111-1111-111111111111","email":"move-me@example.test","enable":true,"ownerAdminId":1,"comment":"keep-this"}]}`,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatal(err)
	}
	record := &model.ClientRecord{
		Email: email, UUID: "11111111-1111-1111-1111-111111111111", SubID: "keep-sub-id",
		OwnerAdminId: int64(from.Id), Enable: true, TotalGB: 123456, Comment: "keep-this",
	}
	if err := db.Create(record).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: record.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.OpenVPNUser{Name: "move-me-ovpn", Email: email, OwnerAdminId: from.Id, Enable: true}).Error; err != nil {
		t.Fatal(err)
	}

	ownerRouter, ownerCookies := ownerGuidRegressionLogin(t, owner)
	response := transferTestCall(t, ownerRouter, ownerCookies, http.MethodPost, "/clients/"+email+"/transfer", map[string]any{"adminId": to.Id})
	var body struct {
		Success bool `json:"success"`
		Obj     struct {
			Email              string `json:"email"`
			OwnerAdminID       int    `json:"ownerAdminId"`
			OwnerAdminUsername string `json:"ownerAdminUsername"`
		} `json:"obj"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || !body.Success {
		t.Fatalf("transfer response=%s decodeErr=%v", response.Body.String(), err)
	}
	if body.Obj.Email != email || body.Obj.OwnerAdminID != to.Id || body.Obj.OwnerAdminUsername != to.Username {
		t.Fatalf("unexpected transfer response: %+v", body.Obj)
	}

	var got model.ClientRecord
	if err := db.Where("email = ?", email).First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.OwnerAdminId != int64(to.Id) || got.UUID != record.UUID || got.SubID != record.SubID || got.TotalGB != record.TotalGB || got.Comment != record.Comment {
		t.Fatalf("client changed unexpectedly after transfer: %+v", got)
	}
	var movedOVPN model.OpenVPNUser
	if err := db.Where("email = ?", email).First(&movedOVPN).Error; err != nil {
		t.Fatal(err)
	}
	if movedOVPN.OwnerAdminId != to.Id {
		t.Fatalf("OpenVPN owner=%d, want %d", movedOVPN.OwnerAdminId, to.Id)
	}
	var movedInbound model.Inbound
	if err := db.Where("id = ?", inbound.Id).First(&movedInbound).Error; err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Clients []struct {
			Email        string `json:"email"`
			OwnerAdminID int    `json:"ownerAdminId"`
			Comment      string `json:"comment"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(movedInbound.Settings), &settings); err != nil {
		t.Fatal(err)
	}
	if len(settings.Clients) != 1 || settings.Clients[0].OwnerAdminID != to.Id || settings.Clients[0].Comment != "keep-this" {
		t.Fatalf("inbound settings were not transferred safely: %+v", settings.Clients)
	}

	// The target admin's scoped list must now include the transferred client.
	targetRouter, targetCookies := ownerGuidRegressionLogin(t, to)
	var page struct {
		Success bool `json:"success"`
		Obj     struct {
			Total int64 `json:"total"`
			Items []struct {
				Email        string `json:"email"`
				OwnerAdminID int64  `json:"ownerAdminId"`
			} `json:"items"`
		} `json:"obj"`
	}
	listed := transferTestCall(t, targetRouter, targetCookies, http.MethodGet, "/clients/list/paged", nil)
	if err := json.Unmarshal(listed.Body.Bytes(), &page); err != nil || !page.Success || page.Obj.Total != 1 || len(page.Obj.Items) != 1 || page.Obj.Items[0].Email != email || page.Obj.Items[0].OwnerAdminID != int64(to.Id) {
		t.Fatalf("target admin cannot see transferred client: %s decodeErr=%v", listed.Body.String(), err)
	}
}

func TestOnlyOwnerCanTransferClient(t *testing.T) {
	owner, from, to := transferTestDB(t)
	db := database.GetDB()
	record := &model.ClientRecord{Email: "forbidden-transfer@example.test", OwnerAdminId: int64(from.Id), Enable: true}
	if err := db.Create(record).Error; err != nil {
		t.Fatal(err)
	}
	adminRouter, adminCookies := ownerGuidRegressionLogin(t, from)
	response := transferTestCall(t, adminRouter, adminCookies, http.MethodPost, "/clients/"+record.Email+"/transfer", map[string]any{"adminId": to.Id})
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-owner status=%d body=%s, want 403", response.Code, response.Body.String())
	}
	var unchanged model.ClientRecord
	if err := db.Where("id = ?", record.Id).First(&unchanged).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.OwnerAdminId != int64(from.Id) {
		t.Fatalf("unauthorized transfer changed owner to %d", unchanged.OwnerAdminId)
	}
	_ = owner
}

func TestOwnerTransferRejectsDisabledTargetWithoutChangingOwner(t *testing.T) {
	owner, from, target := transferTestDB(t)
	target.Status = model.AdminStatusDisabled
	if err := database.GetDB().Model(target).Update("status", target.Status).Error; err != nil {
		t.Fatal(err)
	}
	record := &model.ClientRecord{Email: "disabled-target@example.test", OwnerAdminId: int64(from.Id), Enable: true}
	if err := database.GetDB().Create(record).Error; err != nil {
		t.Fatal(err)
	}
	ownerRouter, ownerCookies := ownerGuidRegressionLogin(t, owner)
	response := transferTestCall(t, ownerRouter, ownerCookies, http.MethodPost, "/clients/"+record.Email+"/transfer", map[string]any{"adminId": target.Id})
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatalf("disabled target unexpectedly accepted: %s", response.Body.String())
	}
	var unchanged model.ClientRecord
	if err := database.GetDB().Where("id = ?", record.Id).First(&unchanged).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.OwnerAdminId != int64(from.Id) {
		t.Fatalf("rejected transfer changed owner to %d", unchanged.OwnerAdminId)
	}
}

func TestOwnerTransferRejectsTargetWithoutInboundAccess(t *testing.T) {
	owner, from, _ := transferTestDB(t)
	db := database.GetDB()
	ib1 := &model.Inbound{Tag: "transfer-visible", Port: 48221, Enable: true, Settings: `{"clients":[{"email":"restricted-target@example.test","ownerAdminId":1}]}`}
	ib2 := &model.Inbound{Tag: "transfer-hidden", Port: 48222, Enable: true, Settings: `{"clients":[]}`}
	for _, ib := range []*model.Inbound{ib1, ib2} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatal(err)
		}
	}
	restrictedRole := &model.AdminRole{
		Name: "transfer-restricted", Slug: "transfer-restricted",
		PermissionsJSON: `{"users":{"read":{"scope":2}}}`,
		AccessJSON:      `{"allowAllGroups":true,"allowAllInbounds":false,"allowed_inbound_ids":[` + itoa(ib2.Id) + `]}`,
	}
	if err := db.Create(restrictedRole).Error; err != nil {
		t.Fatal(err)
	}
	target := &model.User{Username: "restricted-target", RoleId: restrictedRole.Id, Status: model.AdminStatusActive}
	if err := db.Create(target).Error; err != nil {
		t.Fatal(err)
	}
	record := &model.ClientRecord{Email: "restricted-target@example.test", OwnerAdminId: int64(from.Id), Enable: true}
	if err := db.Create(record).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: record.Id, InboundId: ib1.Id}).Error; err != nil {
		t.Fatal(err)
	}
	ownerRouter, ownerCookies := ownerGuidRegressionLogin(t, owner)
	response := transferTestCall(t, ownerRouter, ownerCookies, http.MethodPost, "/clients/"+record.Email+"/transfer", map[string]any{"adminId": target.Id})
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatalf("target without inbound access unexpectedly accepted: %s", response.Body.String())
	}
	var unchanged model.ClientRecord
	if err := db.Where("id = ?", record.Id).First(&unchanged).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.OwnerAdminId != int64(from.Id) {
		t.Fatalf("rejected transfer changed owner to %d", unchanged.OwnerAdminId)
	}
}

func TestOwnerTransferRollsBackWhenAttachedInboundSettingsAreInvalid(t *testing.T) {
	owner, from, target := transferTestDB(t)
	db := database.GetDB()
	inbound := &model.Inbound{Tag: "transfer-invalid-settings", Port: 48231, Enable: true, Settings: "{"}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatal(err)
	}
	record := &model.ClientRecord{Email: "invalid-settings@example.test", OwnerAdminId: int64(from.Id), Enable: true}
	if err := db.Create(record).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: record.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatal(err)
	}
	ownerRouter, ownerCookies := ownerGuidRegressionLogin(t, owner)
	response := transferTestCall(t, ownerRouter, ownerCookies, http.MethodPost, "/clients/"+record.Email+"/transfer", map[string]any{"adminId": target.Id})
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatalf("transfer unexpectedly succeeded with malformed settings: %s", response.Body.String())
	}
	var unchanged model.ClientRecord
	if err := db.Where("id = ?", record.Id).First(&unchanged).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.OwnerAdminId != int64(from.Id) {
		t.Fatalf("failed transaction changed owner to %d", unchanged.OwnerAdminId)
	}
	var unchangedInbound model.Inbound
	if err := db.Where("id = ?", inbound.Id).First(&unchangedInbound).Error; err != nil {
		t.Fatal(err)
	}
	if unchangedInbound.Settings != "{" {
		t.Fatalf("failed transaction modified inbound settings to %q", unchangedInbound.Settings)
	}
}

func TestOwnerTransferHonorsTargetMaxUsersLimit(t *testing.T) {
	owner, from, target := transferTestDB(t)
	db := database.GetDB()
	target.PermissionOverridesJSON = `{"max_users":1}`
	if err := db.Model(target).Update("permission_overrides", target.PermissionOverridesJSON).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientRecord{Email: "already-owned@example.test", OwnerAdminId: int64(target.Id), Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	requested := &model.ClientRecord{Email: "over-limit-transfer@example.test", OwnerAdminId: int64(from.Id), Enable: true}
	if err := db.Create(requested).Error; err != nil {
		t.Fatal(err)
	}
	ownerRouter, ownerCookies := ownerGuidRegressionLogin(t, owner)
	response := transferTestCall(t, ownerRouter, ownerCookies, http.MethodPost, "/clients/"+requested.Email+"/transfer", map[string]any{"adminId": target.Id})
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatalf("transfer exceeded target's max_users limit: %s", response.Body.String())
	}
	var unchanged model.ClientRecord
	if err := db.Where("id = ?", requested.Id).First(&unchanged).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.OwnerAdminId != int64(from.Id) {
		t.Fatalf("over-limit transfer changed owner to %d", unchanged.OwnerAdminId)
	}
}
