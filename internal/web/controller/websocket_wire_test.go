package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
	hubws "github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
)

func TestWebSocketWireScopesQueuedEventsAfterOwnershipChange(t *testing.T) {
	f := makeWSFixture(t)
	gin.SetMode(gin.TestMode)
	hub := hubws.NewHub()
	go hub.Run()
	defer hub.Stop()
	router := gin.New()
	router.Use(sessions.Sessions("ws-test", cookie.NewStore([]byte("ws-test-secret-key"))))
	users := map[string]*model.User{"alice": &f.alice, "bob": &f.bob, "owner": &f.owner}
	router.GET("/login/:name", func(c *gin.Context) {
		u := users[c.Param("name")]
		if u == nil {
			c.Status(http.StatusNotFound)
			return
		}
		if err := session.SetLoginUser(c, u); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})
	router.GET("/ws", NewWebSocketController(panel.NewWebSocketService(hub)).HandleWebSocket)
	server := httptest.NewServer(router)
	defer server.Close()
	connect := func(name string) *ws.Conn {
		t.Helper()
		response, err := http.Get(server.URL + "/login/" + name)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatal(response.Status)
		}
		header := http.Header{}
		for _, ck := range response.Cookies() {
			header.Add("Cookie", ck.Name+"="+ck.Value)
		}
		conn, _, err := ws.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", header)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	alice, bob, owner := connect("alice"), connect("bob"), connect("owner")
	deadline := time.Now().Add(time.Second)
	for hub.GetClientCount() != 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if hub.GetClientCount() != 3 {
		t.Fatal("websocket clients failed to register")
	}
	payload := map[string]any{"clientTraffics": []map[string]any{{"email": "alice@ws", "up": 17}, {"email": "bob@ws", "up": 29}}, "onlineClients": []string{"alice@ws", "bob@ws"}}
	read := func(conn *ws.Conn) []byte {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		return msg
	}
	hub.Broadcast(hubws.MessageTypeTraffic, payload)
	a, b, o := read(alice), read(bob), read(owner)
	if !bytes.Contains(a, []byte("alice@ws")) || bytes.Contains(a, []byte("bob@ws")) {
		t.Fatalf("alice: %s", a)
	}
	if !bytes.Contains(b, []byte("bob@ws")) || bytes.Contains(b, []byte("alice@ws")) {
		t.Fatalf("bob: %s", b)
	}
	if !bytes.Contains(o, []byte("alice@ws")) || !bytes.Contains(o, []byte("bob@ws")) {
		t.Fatalf("owner: %s", o)
	}
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("id = ?", f.good.Id).Update("owner_admin_id", f.bob.Id).Error; err != nil {
		t.Fatal(err)
	}
	hub.Broadcast(hubws.MessageTypeTraffic, payload)
	a, b, o = read(alice), read(bob), read(owner)
	if bytes.Contains(a, []byte("alice@ws")) {
		t.Fatalf("alice retained transferred client: %s", a)
	}
	if !bytes.Contains(b, []byte("alice@ws")) || !bytes.Contains(o, []byte("alice@ws")) {
		t.Fatalf("new owner missed client: bob=%s owner=%s", b, o)
	}
	if err := database.GetDB().Model(&model.User{}).Where("id = ?", f.bob.Id).Update("login_epoch", 5).Error; err != nil {
		t.Fatal(err)
	}
	hub.Broadcast(hubws.MessageTypeTraffic, payload)
	_ = read(alice)
	_ = read(owner)
	_ = bob.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, msg, err := bob.ReadMessage(); err == nil {
		t.Fatal(fmt.Sprintf("revoked socket received %s", msg))
	}
}
