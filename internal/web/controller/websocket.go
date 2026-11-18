package controller

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:    32768,
	WriteBufferSize:   32768,
	EnableCompression: true,
	CheckOrigin:       checkSameOrigin,
}

// checkSameOrigin allows requests with no Origin header (same-origin or non-browser
// clients) and otherwise requires the Origin hostname to match the request hostname.
// Comparison is case-insensitive (RFC 7230 §2.7.3) and ignores port differences
// (the panel often sits behind a reverse proxy on a different port).
func checkSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		return false
	}
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		// IPv6 literals without a port arrive as "[::1]"; net.SplitHostPort
		// fails in that case while url.Hostname() returns the address without
		// brackets. Strip them so same-origin checks pass for bare IPv6 hosts.
		host = r.Host
		if len(host) >= 2 && host[0] == '[' && host[len(host)-1] == ']' {
			host = host[1 : len(host)-1]
		}
	}
	return strings.EqualFold(u.Hostname(), host)
}

// WebSocketController handles the HTTP→WebSocket upgrade for real-time updates.
// All per-connection lifecycle (pumps, hub registration) lives in
// panel.WebSocketService — this controller is HTTP-layer only.
type WebSocketController struct {
	BaseController
	service *panel.WebSocketService
}

// NewWebSocketController creates a controller wired to the given service.
func NewWebSocketController(svc *panel.WebSocketService) *WebSocketController {
	return &WebSocketController{service: svc}
}

// HandleWebSocket authenticates the request, upgrades the HTTP connection, and
// hands ownership of the connection off to the service.
func (w *WebSocketController) HandleWebSocket(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user == nil {
		logger.Warningf("Unauthorized WebSocket connection attempt from %s", getRemoteIp(c))
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("Failed to upgrade WebSocket connection:", err)
		return
	}

	w.service.HandleConnection(conn, getRemoteIp(c), func(msg []byte) []byte {
		return scopeWebSocketMessage(user.Id, user.LoginEpoch, msg)
	})
}

// Every message re-resolves account, role and client membership. Never cache
// authorization on the socket: ownership, access and login epoch may change.
// Unknown event types are dropped for non-owners, rather than accidentally
// exposing a new producer's payload.
func scopeWebSocketMessage(userID int, loginEpoch int64, msg []byte) []byte {
	user, err := apiTokenUser(userID)
	if err != nil || user.LoginEpoch != loginEpoch {
		return nil
	}
	role, err := adminRoleOf(user)
	if err != nil {
		return nil
	}
	if role.OwnerRole {
		return msg
	}
	var event struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
		Time    int64           `json:"time"`
	}
	if json.Unmarshal(msg, &event) != nil {
		return nil
	}
	switch event.Type {
	case "traffic", "client_stats":
		// Match the client REST routes, including simple-read-only roles.
		scope := service.ClientAccessScopeForAdmin(user, "view")
		if scope.Mode == service.ClientAccessNone {
			scope = service.ClientAccessScopeForAdmin(user, "view_simple")
		}
		// A non-owner is never allowed across ownership, even if the role
		// changes between the two lookups above.
		if scope.Mode == service.ClientAccessAll {
			scope.Mode = service.ClientAccessOwn
			scope.AdminID = user.Id
		}
		return scopeClientWebSocketEvent(event.Type, event.Time, event.Payload, scope)
	case "inbounds":
		return wsInvalidate("inbounds", event.Time)
	case "nodes", "outbounds":
		// These snapshots contain panel-wide counts/traffic. Fetch via the
		// permission-gated REST routes instead of broadcasting those totals.
		return wsInvalidate(event.Type, event.Time)
	case "invalidate":
		var p struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(event.Payload, &p) != nil {
			return nil
		}
		switch p.Type {
		case "clients", "inbounds", "traffic", "client_stats", "nodes", "outbounds":
			return wsInvalidate(p.Type, event.Time)
		}
		return nil
	case "status":
		// Keep machine health live, but never forward free-form Xray errors or
		// future, unknown fields that may contain client identities.
		var status service.Status
		if len(event.Payload) == 0 || string(event.Payload) == "null" || json.Unmarshal(event.Payload, &status) != nil {
			return nil
		}
		status.Xray.ErrorMsg = ""
		return wsEvent("status", event.Time, status)
	case "xray_state":
		var state struct {
			State string `json:"state"`
		}
		if json.Unmarshal(event.Payload, &state) != nil || (state.State != "running" && state.State != "stop" && state.State != "error") {
			return nil
		}
		return wsEvent("xray_state", event.Time, map[string]string{"state": state.State, "errorMsg": ""})
	case "notification":
		// Notifications are arbitrary free-form strings. Do not forward them
		// without an audience or client-aware authorization contract.
		return nil
	default:
		return nil
	}
}

func wsEvent(kind string, at int64, payload any) []byte {
	out, err := json.Marshal(map[string]any{"type": kind, "payload": payload, "time": at})
	if err != nil {
		return nil
	}
	return out
}

func wsInvalidate(kind string, at int64) []byte {
	out, err := json.Marshal(map[string]any{"type": "invalidate", "payload": map[string]string{"type": kind}, "time": at})
	if err != nil {
		return nil
	}
	return out
}

// Deliberately whitelist fields: a client traffic row can carry a subscription
// token, UUID or inbound id; none is needed for the live counters/speed UI.
type wsClientRow struct {
	Email      string `json:"email"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
	Total      int64  `json:"total"`
	ExpiryTime int64  `json:"expiryTime"`
	Enable     bool   `json:"enable"`
	LastOnline int64  `json:"lastOnline"`
}
type wsSpeedRow struct {
	Email string `json:"email"`
	Up    int64  `json:"up"`
	Down  int64  `json:"down"`
}

type wsClientPayload struct {
	ClientTraffics          []wsSpeedRow        `json:"clientTraffics"`
	ClientTrafficSource     string              `json:"clientTrafficSource"`
	ClientTrafficIntervalMs int64               `json:"clientTrafficIntervalMs"`
	OnlineClients           []string            `json:"onlineClients"`
	OnlineByGuid            map[string][]string `json:"onlineByGuid"`
	LastOnlineMap           map[string]int64    `json:"lastOnlineMap"`
	Clients                 []wsClientRow       `json:"clients"`
	Snapshot                bool                `json:"snapshot"`
}

func scopeClientWebSocketEvent(kind string, at int64, raw json.RawMessage, scope service.ClientAccessScope) []byte {
	var p wsClientPayload
	if len(raw) == 0 || string(raw) == "null" || json.Unmarshal(raw, &p) != nil {
		return nil
	}
	// Query the union once per bounded chunk, not one query per client/field.
	// FilterClientEmailsForScope applies the same ownership/group/inbound SQL
	// as REST. No cached memberships, even for a long-lived connection.
	emails := make(map[string]struct{})
	add := func(e string) {
		if e != "" {
			emails[e] = struct{}{}
		}
	}
	for _, r := range p.ClientTraffics {
		add(r.Email)
	}
	for _, r := range p.Clients {
		add(r.Email)
	}
	for _, e := range p.OnlineClients {
		add(e)
	}
	for _, list := range p.OnlineByGuid {
		for _, e := range list {
			add(e)
		}
	}
	for e := range p.LastOnlineMap {
		add(e)
	}
	all := make([]string, 0, len(emails))
	for e := range emails {
		all = append(all, e)
	}
	allowed := make(map[string]struct{})
	const chunkSize = 500 // below SQLite's traditional bind limit, incl. scope args
	for start := 0; start < len(all); start += chunkSize {
		end := start + chunkSize
		if end > len(all) {
			end = len(all)
		}
		for _, e := range service.FilterClientEmailsForScope(scope, all[start:end]) {
			allowed[e] = struct{}{}
		}
	}
	permits := func(e string) bool { _, ok := allowed[e]; return ok }
	out := map[string]any{}
	if kind == "traffic" {
		if p.ClientTraffics != nil {
			rows := make([]wsSpeedRow, 0)
			for _, r := range p.ClientTraffics {
				if permits(r.Email) {
					rows = append(rows, r)
				}
			}
			out["clientTraffics"] = rows
			if p.ClientTrafficSource == "tuic" || p.ClientTrafficSource == "xray" {
				out["clientTrafficSource"] = p.ClientTrafficSource
			}
			if p.ClientTrafficIntervalMs > 0 {
				out["clientTrafficIntervalMs"] = p.ClientTrafficIntervalMs
			}
		}
		if p.OnlineClients != nil {
			list := make([]string, 0)
			for _, e := range p.OnlineClients {
				if permits(e) {
					list = append(list, e)
				}
			}
			out["onlineClients"] = list
		}
		if p.OnlineByGuid != nil {
			groups := map[string][]string{}
			for guid, list := range p.OnlineByGuid {
				for _, e := range list {
					if permits(e) {
						groups[guid] = append(groups[guid], e)
					}
				}
			}
			out["onlineByGuid"] = groups
		}
		if p.LastOnlineMap != nil {
			last := map[string]int64{}
			for e, ts := range p.LastOnlineMap {
				if permits(e) {
					last[e] = ts
				}
			}
			out["lastOnlineMap"] = last
		}
		// traffics, nodeTraffics and activeInbounds are panel-wide, not
		// attributable to a particular client. Do not send them.
	} else {
		rows := make([]wsClientRow, 0)
		for _, r := range p.Clients {
			if permits(r.Email) {
				rows = append(rows, r)
			}
		}
		out["clients"] = rows
		out["snapshot"] = p.Snapshot
		// inbounds contains global per-inbound aggregates.
	}
	data, err := json.Marshal(map[string]any{"type": kind, "payload": out, "time": at})
	if err != nil {
		return nil
	}
	return data
}
