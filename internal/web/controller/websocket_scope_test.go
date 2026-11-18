package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type wsFixture struct {
	alice, bob, owner              model.User
	role                           model.AdminRole
	good, wrongGroup, wrongInbound model.ClientRecord
	inbound                        model.Inbound
}

func makeWSFixture(t testing.TB) wsFixture {
	t.Helper()
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "ws.db"))
	db := database.GetDB()
	create := func(v any) {
		t.Helper()
		if err := db.Create(v).Error; err != nil {
			t.Fatal(err)
		}
	}
	ib := model.Inbound{Tag: "ws-ok", Port: 31001, Enable: true}
	otherIB := model.Inbound{Tag: "ws-other", Port: 31002, Enable: true}
	create(&ib)
	create(&otherIB)
	role := model.AdminRole{Name: "ws-user", Slug: "ws-user", PermissionsJSON: `{"users":{"read":{"scope":2}}}`, AccessJSON: fmt.Sprintf(`{"allowedGroups":["red"],"allowed_inbound_ids":[%d]}`, ib.Id)}
	ownerRole := model.AdminRole{Name: "ws-owner", Slug: "ws-owner", OwnerRole: true}
	create(&role)
	create(&ownerRole)
	alice := model.User{Username: "ws-alice", RoleId: role.Id, Status: model.AdminStatusActive, LoginEpoch: 4}
	bob := model.User{Username: "ws-bob", RoleId: role.Id, Status: model.AdminStatusActive, LoginEpoch: 4}
	owner := model.User{Username: "ws-root", RoleId: ownerRole.Id, Status: model.AdminStatusActive, LoginEpoch: 4}
	create(&alice)
	create(&bob)
	create(&owner)
	good := model.ClientRecord{Email: "alice@ws", Group: "red", OwnerAdminId: int64(alice.Id)}
	wrongGroup := model.ClientRecord{Email: "group@ws", Group: "blue", OwnerAdminId: int64(alice.Id)}
	wrongInbound := model.ClientRecord{Email: "inbound@ws", Group: "red", OwnerAdminId: int64(alice.Id)}
	bobClient := model.ClientRecord{Email: "bob@ws", Group: "red", OwnerAdminId: int64(bob.Id)}
	ownerClient := model.ClientRecord{Email: "owner@ws", Group: "red", OwnerAdminId: int64(owner.Id)}
	for _, c := range []*model.ClientRecord{&good, &wrongGroup, &wrongInbound, &bobClient, &ownerClient} {
		create(c)
	}
	for _, pair := range []struct{ client, inbound int }{{good.Id, ib.Id}, {wrongGroup.Id, ib.Id}, {wrongInbound.Id, otherIB.Id}, {bobClient.Id, ib.Id}, {ownerClient.Id, ib.Id}} {
		create(&model.ClientInbound{ClientId: pair.client, InboundId: pair.inbound})
	}
	return wsFixture{alice: alice, bob: bob, owner: owner, role: role, good: good, wrongGroup: wrongGroup, wrongInbound: wrongInbound, inbound: ib}
}

func wsPayload(t testing.TB, got []byte) map[string]any {
	t.Helper()
	if got == nil {
		t.Fatal("event unexpectedly dropped")
	}
	var env struct {
		Type    string         `json:"type"`
		Time    int64          `json:"time"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(got, &env); err != nil || env.Time != 123 {
		t.Fatalf("bad envelope: %s (%v)", got, err)
	}
	return env.Payload
}

func TestWebSocketScopedClientEventsAndRevocation(t *testing.T) {
	f := makeWSFixture(t)
	traffic := []byte(`{"type":"traffic","time":123,"payload":{"clientTraffics":[{"email":"alice@ws","up":100,"down":200,"uuid":"secret","subId":"secret","inboundId":999,"nested":{"email":"bob@ws"}},{"email":"bob@ws","up":9},{"email":"group@ws","up":8},{"email":"inbound@ws","up":7}],"clientTrafficSource":"tuic","clientTrafficIntervalMs":4000,"onlineClients":["alice@ws","bob@ws","group@ws","inbound@ws","owner@ws"],"onlineByGuid":{"node-a":["alice@ws","bob@ws"],"foreign-node":["bob@ws"]},"lastOnlineMap":{"alice@ws":22,"bob@ws":33},"activeInbounds":{"node-a":["foreign-tag"]},"traffics":[{"Tag":"global","Up":999}],"nodeTraffics":[{"Up":999}],"extra":"bob@ws"}}`)
	stats := []byte(`{"type":"client_stats","time":123,"payload":{"snapshot":true,"inbounds":[{"id":1,"up":999}],"clients":[{"email":"alice@ws","up":10,"down":20,"total":100,"expiryTime":7,"enable":true,"lastOnline":33,"uuid":"secret","subId":"secret","inboundId":123},{"email":"bob@ws","up":30},{"email":"group@ws"},{"email":"inbound@ws"}]}}`)
	if got := scopeWebSocketMessage(f.owner.Id, 4, traffic); !bytes.Equal(got, traffic) {
		t.Fatalf("owner event modified: %s", got)
	}
	got := scopeWebSocketMessage(f.alice.Id, 4, traffic)
	p := wsPayload(t, got)
	for _, forbidden := range []string{"bob@ws", "group@ws", "inbound@ws", "owner@ws", "secret", "foreign-tag", "foreign-node", "traffics", "nodeTraffics", "activeInbounds", "extra"} {
		if bytes.Contains(got, []byte(forbidden)) {
			t.Errorf("traffic leaked %q in %s", forbidden, got)
		}
	}
	if p["clientTrafficSource"] != "tuic" || p["clientTrafficIntervalMs"] != float64(4000) {
		t.Fatalf("speed sampling metadata missing: %s", got)
	}
	rows := p["clientTraffics"].([]any)
	if len(rows) != 1 {
		t.Fatalf("traffic rows: %s", got)
	}
	row := rows[0].(map[string]any)
	if len(row) != 3 || row["email"] != "alice@ws" || row["up"] != float64(100) || row["down"] != float64(200) {
		t.Fatalf("speed row: %s", got)
	}
	if len(p["onlineClients"].([]any)) != 1 || len(p["onlineByGuid"].(map[string]any)) != 1 || len(p["lastOnlineMap"].(map[string]any)) != 1 {
		t.Fatalf("online scope: %s", got)
	}
	statsGot := scopeWebSocketMessage(f.alice.Id, 4, stats)
	s := wsPayload(t, statsGot)
	if bytes.Contains(statsGot, []byte("bob@ws")) || bytes.Contains(statsGot, []byte("secret")) || bytes.Contains(statsGot, []byte("inbounds")) {
		t.Fatalf("stats leaked fields: %s", statsGot)
	}
	clientRows := s["clients"].([]any)
	if len(clientRows) != 1 {
		t.Fatalf("stats rows: %s", statsGot)
	}
	r := clientRows[0].(map[string]any)
	if len(r) != 7 || r["email"] != "alice@ws" || r["enable"] != true || r["total"] != float64(100) || r["lastOnline"] != float64(33) {
		t.Fatalf("stats fields: %s", statsGot)
	}
	bob := scopeWebSocketMessage(f.bob.Id, 4, traffic)
	if !bytes.Contains(bob, []byte("bob@ws")) || bytes.Contains(bob, []byte("alice@ws")) {
		t.Fatalf("bob scope: %s", bob)
	}
	// Already open sockets must stop sending an ownership-transferred client immediately.
	db := database.GetDB()
	if err := db.Model(&model.ClientRecord{}).Where("id = ?", f.good.Id).Update("owner_admin_id", f.bob.Id).Error; err != nil {
		t.Fatal(err)
	}
	if got := scopeWebSocketMessage(f.alice.Id, 4, traffic); bytes.Contains(got, []byte("alice@ws")) {
		t.Fatalf("stale owner: %s", got)
	}
	if got := scopeWebSocketMessage(f.bob.Id, 4, traffic); !bytes.Contains(got, []byte("alice@ws")) {
		t.Fatalf("new owner not visible: %s", got)
	}
	// Role access and permission changes apply without reconnecting.
	if err := db.Model(&model.AdminRole{}).Where("id = ?", f.role.Id).Update("access", `{"allowedGroups":[]}`).Error; err != nil {
		t.Fatal(err)
	}
	if got := scopeWebSocketMessage(f.bob.Id, 4, traffic); bytes.Contains(got, []byte("alice@ws")) || bytes.Contains(got, []byte("bob@ws")) {
		t.Fatalf("stale role: %s", got)
	}
	if err := db.Model(&model.User{}).Where("id = ?", f.bob.Id).Update("login_epoch", 5).Error; err != nil {
		t.Fatal(err)
	}
	if got := scopeWebSocketMessage(f.bob.Id, 4, traffic); got != nil {
		t.Fatalf("stale epoch: %s", got)
	}
	if err := db.Model(&model.User{}).Where("id = ?", f.alice.Id).Update("status", model.AdminStatusDisabled).Error; err != nil {
		t.Fatal(err)
	}
	if got := scopeWebSocketMessage(f.alice.Id, 4, traffic); got != nil {
		t.Fatalf("disabled account: %s", got)
	}
}

func TestWebSocketFailsClosedAndInvalidates(t *testing.T) {
	f := makeWSFixture(t)
	check := func(msg, kind string) {
		t.Helper()
		got := scopeWebSocketMessage(f.alice.Id, 4, []byte(msg))
		p := wsPayload(t, got)
		if p["type"] != kind || strings.Contains(string(got), "bob@ws") {
			t.Fatalf("invalidate: %s", got)
		}
	}
	check(`{"type":"inbounds","time":123,"payload":[{"email":"bob@ws"}]}`, "inbounds")
	check(`{"type":"nodes","time":123,"payload":[{"onlineCount":900}]}`, "nodes")
	check(`{"type":"outbounds","time":123,"payload":[{"up":900}]}`, "outbounds")
	check(`{"type":"invalidate","time":123,"payload":{"type":"traffic","extra":"bob@ws"}}`, "traffic")
	for _, raw := range []string{`{"type":"unknown","time":123,"payload":{"email":"bob@ws"}}`, `{"type":"traffic","time":123,"payload":{"clientTraffics":"invalid"}}`, `{"type":"invalidate","time":123,"payload":{"type":"unexpected"}}`, `not-json`} {
		if got := scopeWebSocketMessage(f.alice.Id, 4, []byte(raw)); got != nil {
			t.Fatalf("must drop %s: %s", raw, got)
		}
	}
	db := database.GetDB()
	if err := db.Model(&model.AdminRole{}).Where("id = ?", f.role.Id).Updates(map[string]any{"permissions": `{"users":{"read_simple":{"scope":1}}}`, "access": fmt.Sprintf(`{"allowedGroups":["red"],"allowed_inbound_ids":[%d]}`, f.inbound.Id)}).Error; err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"type":"traffic","time":123,"payload":{"onlineClients":["alice@ws"]}}`)
	if got := scopeWebSocketMessage(f.alice.Id, 4, raw); !bytes.Contains(got, []byte("alice@ws")) {
		t.Fatalf("simple read: %s", got)
	}
	if err := db.Model(&model.AdminRole{}).Where("id = ?", f.role.Id).Update("permissions", `{}`).Error; err != nil {
		t.Fatal(err)
	}
	if got := scopeWebSocketMessage(f.alice.Id, 4, raw); bytes.Contains(got, []byte("alice@ws")) {
		t.Fatalf("revoked permission: %s", got)
	}
	if err := db.Delete(&f.role).Error; err != nil {
		t.Fatal(err)
	}
	if got := scopeWebSocketMessage(f.alice.Id, 4, raw); got != nil {
		t.Fatalf("missing role: %s", got)
	}
}

func TestWebSocketSystemEventsDoNotForwardFreeText(t *testing.T) {
	f := makeWSFixture(t)
	for _, raw := range []string{
		`{"type":"status","time":123,"payload":{"cpu":2,"xray":{"state":"error","errorMsg":"bob@ws"},"unknown":"bob@ws"}}`,
		`{"type":"xray_state","time":123,"payload":{"state":"error","errorMsg":"bob@ws"}}`,
	} {
		got := scopeWebSocketMessage(f.alice.Id, 4, []byte(raw))
		wsPayload(t, got)
		if bytes.Contains(got, []byte("bob@ws")) {
			t.Fatalf("free-form error leaked: %s", got)
		}
		if got := scopeWebSocketMessage(f.owner.Id, 4, []byte(raw)); !bytes.Equal(got, []byte(raw)) {
			t.Fatalf("owner changed: %s", got)
		}
	}
	if got := scopeWebSocketMessage(f.alice.Id, 4, []byte(`{"type":"notification","time":123,"payload":{"message":"bob@ws"}}`)); got != nil {
		t.Fatalf("free-form notification: %s", got)
	}
}

// Benchmark the per-connection authorization path with a mixed 1,000-client
// snapshot. Setup is excluded; the result includes JSON and all SQL lookups.
func BenchmarkWebSocketScopedSnapshot1000(b *testing.B) {
	f := makeWSFixture(b)
	db := database.GetDB()
	records := make([]model.ClientRecord, 1000)
	wireRows := make([]map[string]any, 1000)
	for i := range records {
		owner := f.bob.Id
		if i%2 == 0 {
			owner = f.alice.Id
		}
		email := fmt.Sprintf("bench-%04d@ws", i)
		records[i] = model.ClientRecord{Email: email, Group: "red", OwnerAdminId: int64(owner)}
		wireRows[i] = map[string]any{"email": email, "up": i, "down": i * 2, "enable": true}
	}
	if err := db.CreateInBatches(&records, 200).Error; err != nil {
		b.Fatal(err)
	}
	links := make([]model.ClientInbound, 0, len(records))
	for _, rec := range records {
		links = append(links, model.ClientInbound{ClientId: rec.Id, InboundId: f.inbound.Id})
	}
	if err := db.CreateInBatches(&links, 200).Error; err != nil {
		b.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{"type": "client_stats", "time": 123, "payload": map[string]any{"snapshot": true, "clients": wireRows}})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := scopeWebSocketMessage(f.alice.Id, 4, raw); got == nil {
			b.Fatal("dropped snapshot")
		}
	}
}
