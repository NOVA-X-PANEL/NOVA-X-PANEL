package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"

	"github.com/gin-gonic/gin"
)

func notifyClientsChanged() {
	websocket.BroadcastInvalidate(websocket.MessageTypeClients)
}

func parseInboundIdsQuery(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		if id, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

type ClientController struct {
	clientService  service.ClientService
	inboundService service.InboundService
	xrayService    service.XrayService
	settingService service.SettingService
	happGenerator  service.HappLinkGenerator
}

func NewClientController(g *gin.RouterGroup) *ClientController {
	a := &ClientController{}
	a.happGenerator = service.NewHappService(&a.clientService, &a.settingService)
	a.initRouter(g)
	return a
}

func (a *ClientController) initRouter(g *gin.RouterGroup) {
	// The clients list had no permission gate at all: a role without users.read
	// silently received an empty list instead of a refusal, which is
	// indistinguishable from "there are no clients". Read or read_simple may
	// list; the scope below still narrows the rows.
	// Every route needs a permission gate. Most of these previously had none, so
	// any account that could log in — including a role with no client grants at
	// all — could create, import, enable/disable, reset, detach or delete clients.
	// The gates below match the actions the role editor exposes for "clients"
	// (stored under "users"). Ownership is narrowed separately, inside the
	// handlers, so an "own"-scoped role still only reaches its own records.
	clientsRead := requireAnyPanelPermission(
		panelPermissionRequirement{Section: "clients", Permission: "view"},
		panelPermissionRequirement{Section: "clients", Permission: "viewSimple"},
	)
	clientsCreate := requirePanelPermission("clients", "create")
	clientsUpdate := requirePanelPermission("clients", "update")
	clientsDelete := requirePanelPermission("clients", "delete")
	clientsReset := requirePanelPermission("clients", "resetUsage")

	g.GET("/list", clientsRead, a.list)
	g.GET("/list/paged", clientsRead, a.listPaged)
	g.GET("/get/:email", clientsRead, a.get)
	g.GET("/get/tgId/:tgId", clientsRead, a.getByTgId)
	g.GET("/traffic/:email", clientsRead, a.getTrafficByEmail)
	g.GET("/subLinks/:subId", clientsRead, a.getSubLinks)
	g.GET("/links/:email", clientsRead, a.getClientLinks)
	g.POST("/happLink/:id", clientsRead, a.generateHappLink)

	g.POST("/add", clientsCreate, a.create)
	g.POST("/update/:email", clientsUpdate, a.update)
	g.POST("/del/:email", clientsDelete, a.delete)
	g.POST("/:email/attach", clientsUpdate, a.attach)
	g.POST("/:email/detach", clientsUpdate, a.detach)
	g.POST("/:email/externalLinks", clientsUpdate, a.setExternalLinks)
	g.GET("/export", clientsRead, a.export)
	g.POST("/import", clientsCreate, a.importClients)
	g.POST("/delOrphans", clientsDelete, a.delOrphans)
	g.POST("/resetAllTraffics", clientsReset, a.resetAllTraffics)
	g.POST("/delDepleted", clientsDelete, a.delDepleted)
	g.POST("/bulkAdjust", clientsUpdate, a.bulkAdjust)
	g.POST("/bulkEnable", clientsUpdate, a.bulkEnable)
	g.POST("/bulkDisable", clientsUpdate, a.bulkDisable)
	g.POST("/bulkDel", clientsDelete, a.bulkDelete)
	g.POST("/bulkCreate", clientsCreate, a.bulkCreate)
	g.POST("/bulkAttach", clientsUpdate, a.bulkAttach)
	g.POST("/bulkDetach", clientsUpdate, a.bulkDetach)
	g.POST("/bulkResetTraffic", clientsReset, a.bulkResetTraffic)
	g.POST("/resetTraffic/:email", clientsReset, a.resetTrafficByEmail)
	g.POST("/updateTraffic/:email", clientsUpdate, a.updateTrafficByEmail)
	g.POST("/ips/:email", clientsRead, a.getIps)
	g.POST("/clearIps/:email", clientsUpdate, a.clearIps)
	g.POST("/hwids/:email", clientsRead, a.getHwids)
	g.DELETE("/hwids/:email", clientsUpdate, a.clearHwids)
	g.DELETE("/hwids/:email/:id", clientsUpdate, a.deleteHwid)
	g.POST("/onlines", clientsRead, a.onlines)
	g.POST("/onlinesByGuid", clientsRead, a.onlinesByGuid)
	g.POST("/clientIpsByGuid", clientsRead, a.clientIpsByGuid)
	g.POST("/activeInbounds", clientsRead, a.activeInbounds)
	g.POST("/lastOnline", clientsRead, a.lastOnline)
}

// clientScope resolves the RBAC scope for the acting account. Unit tests mount
// this controller without the session middleware, so a missing user in test
// mode keeps full access; production always resolves through the session.
func (a *ClientController) clientScope(c *gin.Context, permission string) service.ClientAccessScope {
	// Panel-wide API tokens (monitor / node-sync / the settings page) have no
	// panel account, so scoping them by owner would silently turn node sync into
	// a no-op; they keep full scope and are constrained by enforceTokenScope.
	// A token minted by an admin *is* bound to an account, and falls through so
	// that account's role narrows it exactly as it does for a browser session.
	if c.GetBool("api_authed") && c.GetInt("api_token_admin_id") <= 0 {
		return service.ClientAccessScope{Mode: service.ClientAccessAll}
	}

	user := a.loginUser(c)
	if user == nil && gin.Mode() == gin.TestMode {
		return service.ClientAccessScope{Mode: service.ClientAccessAll}
	}
	return a.clientService.ClientScopeFor(user, permission)
}

func (a *ClientController) loginUser(c *gin.Context) *model.User {
	var user *model.User
	func() {
		defer func() {
			if recover() != nil {
				user = nil
			}
		}()
		user = session.GetLoginUser(c)
	}()
	return user
}

// requireGlobalClientScope allows a call that touches every client at once only
// when the acting scope covers every client. An "own" or "none" scoped role must
// not be able to wipe another admin's traffic by calling a bulk endpoint.
func (a *ClientController) requireGlobalClientScope(c *gin.Context, permission string) bool {
	if c.GetBool("api_authed") || gin.Mode() == gin.TestMode {
		return true
	}
	if a.loginUser(c) == nil {
		return true
	}
	if a.clientScope(c, permission).Mode == service.ClientAccessAll {
		return true
	}
	pureJsonMsg(c, http.StatusForbidden, false, "this operation requires access to every client")
	return false
}

// requireClientInScope loads a client by email and aborts when the acting
// account may not touch it, so a scoped admin cannot address another admin's
// client by guessing its address.
func (a *ClientController) requireClientInScope(c *gin.Context, email string, permission string) (*model.ClientRecord, bool) {
	rec, err := a.clientService.RequireClientForScopeByEmail(a.clientScope(c, permission), email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil, false
	}
	return rec, true
}

// scopeEmails narrows a request's email list to what the scope allows.
func (a *ClientController) scopeEmails(c *gin.Context, emails []string, permission string) []string {
	return service.FilterClientEmailsForScope(a.clientScope(c, permission), emails)
}

func (a *ClientController) list(c *gin.Context) {
	rows, err := a.clientService.ListForScope(a.clientScope(c, "view"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, rows, nil)
}

func (a *ClientController) listPaged(c *gin.Context) {
	var params service.ClientPageParams
	if err := c.ShouldBindQuery(&params); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	resp, err := a.clientService.ListPaged(&a.inboundService, &a.settingService, params)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}

	// Narrow the page to the acting scope. An unrestricted owner short-circuits
	// so the common path costs nothing extra.
	scope := a.clientScope(c, "view")
	if scope.Mode != service.ClientAccessAll || (scope.RestrictGroups && !scope.AllowAllGroups) {
		if resp != nil {
			emails := make([]string, 0, len(resp.Items))
			for _, item := range resp.Items {
				emails = append(emails, item.Email)
			}
			allowed := map[string]struct{}{}
			for _, email := range service.FilterClientEmailsForScope(scope, emails) {
				allowed[email] = struct{}{}
			}
			filtered := make([]service.ClientSlim, 0, len(resp.Items))
			for _, item := range resp.Items {
				if _, ok := allowed[item.Email]; ok {
					filtered = append(filtered, item)
				}
			}
			resp.Items = filtered
			resp.Total = len(filtered)
			resp.Filtered = len(filtered)
			resp.Summary.Online = service.FilterClientEmailsForScope(scope, resp.Summary.Online)
			resp.Summary.Depleted = service.FilterClientEmailsForScope(scope, resp.Summary.Depleted)
			resp.Summary.Expiring = service.FilterClientEmailsForScope(scope, resp.Summary.Expiring)
			resp.Summary.Deactive = service.FilterClientEmailsForScope(scope, resp.Summary.Deactive)
			resp.Summary.Total = len(filtered)
		}
	}
	jsonObj(c, resp, nil)
}

func (a *ClientController) buildClientPayload(rec *model.ClientRecord) (gin.H, error) {
	inboundIds, err := a.clientService.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		return nil, err
	}
	externalLinks, err := a.clientService.GetExternalLinksForRecord(rec.Id)
	if err != nil {
		return nil, err
	}
	flow, err := a.clientService.EffectiveFlow(nil, rec.Id)
	if err != nil {
		return nil, err
	}
	rec.Flow = flow
	var usedTraffic int64
	if t, tErr := a.inboundService.GetClientTrafficByEmail(rec.Email); tErr == nil && t != nil {
		usedTraffic = t.Up + t.Down
	}
	tunnelAllowedIPs, err := a.clientService.TunnelAllowedIPsByInbound(&a.inboundService, rec.Email, inboundIds)
	if err != nil {
		return nil, err
	}
	return gin.H{
		"client":           rec,
		"inboundIds":       inboundIds,
		"externalLinks":    externalLinks,
		"usedTraffic":      usedTraffic,
		"tunnelAllowedIPs": tunnelAllowedIPs,
	}, nil
}

func (a *ClientController) get(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "view"); !ok {
		return
	}
	rec, err := a.clientService.GetRecordByEmail(nil, email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	payload, err := a.buildClientPayload(rec)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, payload, nil)
}

func (a *ClientController) getByTgId(c *gin.Context) {
	tgIdStr := c.Param("tgId")
	tgId, err := strconv.ParseInt(tgIdStr, 10, 64)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	records, err := a.clientService.GetRecordsByTgID(tgId)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	results := make([]gin.H, 0, len(records))
	for _, rec := range records {
		payload, err := a.buildClientPayload(rec)
		if err != nil {
			jsonMsg(c, I18nWeb(c, "get"), err)
			return
		}
		results = append(results, payload)
	}
	jsonObj(c, results, nil)
}

// stampOwnerForPayloads records the acting account as the owner of freshly
// created clients, the same way the single-client create does.
func (a *ClientController) stampOwnerForPayloads(c *gin.Context, payloads []service.ClientCreatePayload) {
	user := a.loginUser(c)
	if user == nil {
		return
	}
	emails := make([]string, 0, len(payloads))
	for _, p := range payloads {
		if p.Client.Email != "" {
			emails = append(emails, p.Client.Email)
		}
	}
	if len(emails) == 0 {
		return
	}
	if err := a.clientService.AssignOwnerAdmin(emails, user.Id); err != nil {
		logger.Warning("failed to stamp client owners:", err)
	}
}

// requireInboundsInScope rejects a call that would place or attach a client on an
// inbound the acting role may not use. The role editor's "Allowed Inbounds" is
// documented as restricting exactly this, but nothing enforced it: a role limited
// to one inbound could still create clients on every other one.
func (a *ClientController) requireInboundsInScope(c *gin.Context, permission string, inboundIds []int) bool {
	if len(inboundIds) == 0 {
		return true
	}
	scope := a.clientScope(c, permission)
	if scope.Mode == service.ClientAccessNone {
		// The route's own permission gate already refused this call, so there is
		// nothing extra to decide here.
		return true
	}
	if service.ClientInboundsAllowedForScope(scope, inboundIds) {
		return true
	}
	pureJsonMsg(c, http.StatusForbidden, false, "inbounds not allowed for this role")
	return false
}

func (a *ClientController) create(c *gin.Context) {
	var payload service.ClientCreatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	user := a.loginUser(c)
	if user != nil && !a.clientService.CanCreateClientForAdmin(user) {
		pureJsonMsg(c, http.StatusForbidden, false, "clients.create permission required")
		return
	}
	if !a.requireInboundsInScope(c, "create", payload.InboundIds) {
		return
	}

	needRestart, err := a.clientService.Create(&a.inboundService, &payload)
	// Flagged before the error check: a partly-applied create leaves clients
	// committed on the inbounds that succeeded, and those still need the restart.
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	// A partly-applied call committed real clients; a rejected one touched
	// nothing, and broadcasting those would refetch every panel for nothing.
	if needRestart || err == nil {
		notifyClientsChanged()
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	// Stamp ownership so an "own"-scoped role only ever sees what it created.
	if user != nil && payload.Client.Email != "" {
		if stampErr := a.clientService.AssignOwnerAdmin([]string{payload.Client.Email}, user.Id); stampErr != nil {
			logger.Warning("failed to stamp client owner:", stampErr)
		}
	}

	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientAddSuccess"), pendingNodeObj(a.inboundService.AnyNodePending(payload.InboundIds)), nil)
}

func (a *ClientController) update(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "update"); !ok {
		return
	}
	var req struct {
		model.Client
		LimitHwid int `json:"limitHwid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	inboundFilter := parseInboundIdsQuery(c.Query("inboundIds"))
	needRestart, err := a.clientService.UpdateByEmail(&a.inboundService, email, req.Client, req.LimitHwid, inboundFilter...)
	// Flagged before the error check: a partly-applied edit leaves the change
	// committed on the inbounds that succeeded, and those still need the restart.
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	// A partly-applied call committed real changes; a rejected one touched
	// nothing, and broadcasting those would refetch every panel for nothing.
	if needRestart || err == nil {
		notifyClientsChanged()
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), pendingNodeObj(a.clientService.HasPendingNode(&a.inboundService, email)), nil)
}

func (a *ClientController) delete(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "delete"); !ok {
		return
	}
	keepTraffic := c.Query("keepTraffic") == "1"
	needRestart, err := a.clientService.DeleteByEmail(&a.inboundService, email, keepTraffic)
	// Flagged before the error check: a partly-applied delete already removed
	// the client from the inbounds that succeeded, and those need the restart.
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	// A partly-applied call committed real removals; a rejected one touched
	// nothing, and broadcasting those would refetch every panel for nothing.
	if needRestart || err == nil {
		notifyClientsChanged()
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientDeleteSuccess"), nil)
}

type attachDetachBody struct {
	InboundIds []int `json:"inboundIds"`
}

type externalLinksBody struct {
	ExternalLinks []service.ExternalLinkInput `json:"externalLinks"`
}

func (a *ClientController) attach(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "update"); !ok {
		return
	}
	var body attachDetachBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if !a.requireInboundsInScope(c, "update", body.InboundIds) {
		return
	}
	needRestart, err := a.clientService.AttachByEmail(&a.inboundService, email, body.InboundIds)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	// A partly-applied call committed real clients; a rejected one touched
	// nothing, and broadcasting those would refetch every panel for nothing.
	if needRestart || err == nil {
		notifyClientsChanged()
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientAddSuccess"), pendingNodeObj(a.inboundService.AnyNodePending(body.InboundIds)), nil)
}

func (a *ClientController) setExternalLinks(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "update"); !ok {
		return
	}
	var body externalLinksBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.clientService.SetExternalLinksByEmail(email, body.ExternalLinks); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), nil)
	notifyClientsChanged()
}

func (a *ClientController) resetAllTraffics(c *gin.Context) {
	if !a.requireGlobalClientScope(c, "reset_usage") {
		return
	}
	needRestart, err := a.clientService.ResetAllTraffics()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetAllClientTrafficSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

type bulkAdjustRequest struct {
	Emails    []string `json:"emails"`
	AddDays   int      `json:"addDays"`
	AddBytes  int64    `json:"addBytes"`
	Flow      string   `json:"flow"`
	LimitHwid *int     `json:"limitHwid"`
	AdTag     string   `json:"adTag"`
}

func (a *ClientController) bulkAdjust(c *gin.Context) {
	var req bulkAdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	req.Emails = a.scopeEmails(c, req.Emails, "update")
	result, needRestart, err := a.clientService.BulkAdjust(&a.inboundService, req.Emails, req.AddDays, req.AddBytes, req.Flow, req.LimitHwid, req.AdTag)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

type bulkDeleteRequest struct {
	Emails      []string `json:"emails"`
	KeepTraffic bool     `json:"keepTraffic"`
}

type bulkAttachRequest struct {
	Emails     []string `json:"emails"`
	InboundIds []int    `json:"inboundIds"`
}

func (a *ClientController) bulkAttach(c *gin.Context) {
	var req bulkAttachRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	req.Emails = a.scopeEmails(c, req.Emails, "update")
	if !a.requireInboundsInScope(c, "update", req.InboundIds) {
		return
	}
	result, needRestart, err := a.clientService.BulkAttach(&a.inboundService, req.Emails, req.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

type bulkDetachRequest struct {
	Emails     []string `json:"emails"`
	InboundIds []int    `json:"inboundIds"`
}

func (a *ClientController) bulkDetach(c *gin.Context) {
	var req bulkDetachRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	req.Emails = a.scopeEmails(c, req.Emails, "update")
	result, needRestart, err := a.clientService.BulkDetach(&a.inboundService, req.Emails, req.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

func (a *ClientController) bulkDelete(c *gin.Context) {
	var req bulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	req.Emails = a.scopeEmails(c, req.Emails, "delete")
	result, needRestart, err := a.clientService.BulkDelete(&a.inboundService, req.Emails, req.KeepTraffic)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

type bulkEnableRequest struct {
	Emails []string `json:"emails"`
}

func (a *ClientController) bulkEnable(c *gin.Context) {
	a.bulkSetEnable(c, true)
}

func (a *ClientController) bulkDisable(c *gin.Context) {
	a.bulkSetEnable(c, false)
}

func (a *ClientController) bulkSetEnable(c *gin.Context, enable bool) {
	var req bulkEnableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	req.Emails = a.scopeEmails(c, req.Emails, "update")
	result, needRestart, err := a.clientService.BulkSetEnable(&a.inboundService, req.Emails, enable)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

func (a *ClientController) bulkCreate(c *gin.Context) {
	var payloads []service.ClientCreatePayload
	if err := c.ShouldBindJSON(&payloads); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	result, needRestart, err := a.clientService.BulkCreate(&a.inboundService, payloads)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	// Same ownership stamp as the single create: without it an "own"-scoped
	// admin would create clients it could not then see.
	a.stampOwnerForPayloads(c, payloads)
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

func (a *ClientController) delDepleted(c *gin.Context) {
	if !a.requireGlobalClientScope(c, "delete") {
		return
	}
	deleted, needRestart, err := a.clientService.DelDepleted(&a.inboundService)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"deleted": deleted}, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

// export returns every client as a {client, inboundIds} list in the standard
// envelope. The frontend renders it in a read-only CodeMirror viewer (Copy /
// Download), so this hands back data rather than streaming a file attachment.
func (a *ClientController) export(c *gin.Context) {
	items, err := a.clientService.ExportAll()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, items, nil)
}

type importClientsRequest struct {
	Data string `json:"data"`
}

// importClients accepts the pasted export text as a JSON body { "data": "..." },
// mirroring the inbound import flow. The data string is itself a JSON-encoded
// []ClientCreatePayload, so it is unmarshalled in a second step.
func (a *ClientController) importClients(c *gin.Context) {
	var req importClientsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var items []service.ClientCreatePayload
	if err := json.Unmarshal([]byte(req.Data), &items); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	result, needRestart, err := a.clientService.ImportClients(&a.inboundService, items)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	a.stampOwnerForPayloads(c, items)
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

func (a *ClientController) delOrphans(c *gin.Context) {
	if !a.requireGlobalClientScope(c, "delete") {
		return
	}
	deleted, err := a.clientService.DeleteOrphans()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"deleted": deleted}, nil)
	notifyClientsChanged()
}

func (a *ClientController) resetTrafficByEmail(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "reset_usage"); !ok {
		return
	}
	needRestart, err := a.clientService.ResetTrafficByEmail(&a.inboundService, email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetInboundClientTrafficSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

type trafficUpdateRequest struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}

func (a *ClientController) updateTrafficByEmail(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "update"); !ok {
		return
	}
	var req trafficUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.inboundService.UpdateClientTrafficByEmail(email, req.Upload, req.Download); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), nil)
	notifyClientsChanged()
}

func (a *ClientController) getIps(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "view"); !ok {
		return
	}
	infos, err := a.inboundService.GetClientIpsWithNodes(email)
	jsonObj(c, infos, err)
}

func (a *ClientController) clientIpsByGuid(c *gin.Context) {
	data, err := a.inboundService.GetClientIpsByGuid()
	jsonObj(c, data, err)
}

func (a *ClientController) clearIps(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "update"); !ok {
		return
	}
	if err := a.inboundService.ClearClientIps(email); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.updateSuccess"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.logCleanSuccess"), nil)
}

func (a *ClientController) getHwids(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "view"); !ok {
		return
	}
	infos, err := a.clientService.ListClientHwids(c.Param("email"))
	jsonObj(c, infos, err)
}

func (a *ClientController) clearHwids(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "update"); !ok {
		return
	}
	if err := a.clientService.ClearClientHwids(c.Param("email")); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.updateSuccess"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.logCleanSuccess"), nil)
}

func (a *ClientController) deleteHwid(c *gin.Context) {
	if _, ok := a.requireClientInScope(c, c.Param("email"), "update"); !ok {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.clientService.DeleteClientHwid(c.Param("email"), id); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.clients.hwidDeleted"), nil)
}

func (a *ClientController) onlines(c *gin.Context) {
	jsonObj(c, a.inboundService.GetOnlineClients(), nil)
}

func (a *ClientController) onlinesByGuid(c *gin.Context) {
	jsonObj(c, a.inboundService.GetOnlineClientsByGuid(), nil)
}

func (a *ClientController) activeInbounds(c *gin.Context) {
	jsonObj(c, a.inboundService.GetActiveInboundsByGuid(), nil)
}

func (a *ClientController) lastOnline(c *gin.Context) {
	data, err := a.inboundService.GetClientsLastOnline()
	jsonObj(c, data, err)
}

func (a *ClientController) getTrafficByEmail(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "view"); !ok {
		return
	}
	traffic, err := a.inboundService.GetClientTrafficByEmail(email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.trafficGetError"), err)
		return
	}
	jsonObj(c, traffic, nil)
}

func (a *ClientController) getSubLinks(c *gin.Context) {
	links, err := a.inboundService.GetSubLinks(resolveHost(c), c.Param("subId"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, links, nil)
}

func (a *ClientController) getClientLinks(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "view"); !ok {
		return
	}
	links, err := a.inboundService.GetAllClientLinks(resolveHost(c), c.Param("email"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, links, nil)
}

func (a *ClientController) generateHappLink(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	clientID, err := strconv.Atoi(c.Param("id"))
	if err != nil || clientID < 1 {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), service.ErrHappLinkUnavailable)
		return
	}
	result, err := a.happGenerator.Generate(c.Request.Context(), clientID, c.Request.Host)
	if err != nil {
		if errors.Is(err, service.ErrHappSourceTooLong) {
			// Keep the code exact so clients can localize it without exposing internal error details.
			c.JSON(http.StatusOK, entity.Msg{Success: false, Msg: "happ_source_too_long", Obj: nil})
			return
		}
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), service.ErrHappLinkUnavailable)
		return
	}
	jsonObj(c, result, nil)
}

func (a *ClientController) detach(c *gin.Context) {
	email := c.Param("email")
	if _, ok := a.requireClientInScope(c, email, "update"); !ok {
		return
	}
	var body attachDetachBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	needRestart, err := a.clientService.DetachByEmailMany(&a.inboundService, email, body.InboundIds)
	// Flagged before the error check: a partly-applied detach already removed
	// the client from the inbounds that succeeded, and those need the restart.
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	// A partly-applied call committed real removals; a rejected one touched
	// nothing, and broadcasting those would refetch every panel for nothing.
	if needRestart || err == nil {
		notifyClientsChanged()
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientDeleteSuccess"), pendingNodeObj(a.inboundService.AnyNodePending(body.InboundIds)), nil)
}

type bulkResetRequest struct {
	Emails []string `json:"emails"`
}

func (a *ClientController) bulkResetTraffic(c *gin.Context) {
	var req bulkResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	req.Emails = a.scopeEmails(c, req.Emails, "reset_usage")
	affected, err := a.clientService.BulkResetTraffic(&a.inboundService, req.Emails)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"affected": affected}, nil)
	a.xrayService.SetToNeedRestart()
	notifyClientsChanged()
}
