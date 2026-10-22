package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestBulkAdjust_SkipUnlimited(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	now := time.Now().UnixMilli()
	dayMs := int64(24 * 60 * 60 * 1000)

	// 1. Regular limited client
	cNorm := model.Client{Email: "norm@x", ID: "11111111-1111-1111-1111-111111111111", SubID: "norm", Enable: true, TotalGB: 50, ExpiryTime: now + 10*dayMs}
	// 2. Unlimited expiry, limited traffic
	cUnlimExp := model.Client{Email: "unlim_exp@x", ID: "22222222-2222-2222-2222-222222222222", SubID: "unlim_exp", Enable: true, TotalGB: 50, ExpiryTime: 0}
	// 3. Limited expiry, unlimited traffic
	cUnlimTraffic := model.Client{Email: "unlim_traf@x", ID: "33333333-3333-3333-3333-333333333333", SubID: "unlim_traf", Enable: true, TotalGB: 0, ExpiryTime: now + 10*dayMs}
	// 4. Unlimited both
	cUnlimBoth := model.Client{Email: "unlim_both@x", ID: "44444444-4444-4444-4444-444444444444", SubID: "unlim_both", Enable: true, TotalGB: 0, ExpiryTime: 0}

	clients := []model.Client{cNorm, cUnlimExp, cUnlimTraffic, cUnlimBoth}
	ib := mkInbound(t, 52099, model.VLESS, clientsSettings(t, clients))
	if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("seed: %v", err)
	}

	for _, c := range clients {
		mkTraffic(t, ib.Id, c.Email, 0, 0, c.TotalGB, c.ExpiryTime, true)
	}

	emails := []string{"norm@x", "unlim_exp@x", "unlim_traf@x", "unlim_both@x"}

	// When skipUnlimited is true:
	res, _, err := svc.BulkAdjust(inboundSvc, emails, 5, 10, "", nil, "", true)
	if err != nil {
		t.Fatalf("BulkAdjust skipUnlimited: %v", err)
	}

	if res.Adjusted != 1 {
		t.Fatalf("expected 1 adjusted client (norm@x), got %d; res=%+v", res.Adjusted, res)
	}

	if len(res.Skipped) != 3 {
		t.Fatalf("expected 3 skipped clients, got %d; res=%+v", len(res.Skipped), res)
	}

	skippedMap := make(map[string]string)
	for _, s := range res.Skipped {
		skippedMap[s.Email] = s.Reason
	}

	if skippedMap["unlim_exp@x"] != "unlimited expiry" {
		t.Errorf("unlim_exp reason: want 'unlimited expiry', got %q", skippedMap["unlim_exp@x"])
	}
	if skippedMap["unlim_traf@x"] != "unlimited traffic" {
		t.Errorf("unlim_traf reason: want 'unlimited traffic', got %q", skippedMap["unlim_traf@x"])
	}
	if skippedMap["unlim_both@x"] != "unlimited traffic and expiry" {
		t.Errorf("unlim_both reason: want 'unlimited traffic and expiry', got %q", skippedMap["unlim_both@x"])
	}

	// Verify norm@x got adjusted
	normTraffic := trafficOf(t, "norm@x")
	if normTraffic.Total != 60 {
		t.Errorf("norm@x traffic total: want 60, got %d", normTraffic.Total)
	}
	if normTraffic.ExpiryTime != now+15*dayMs {
		t.Errorf("norm@x expiry: want %d, got %d", now+15*dayMs, normTraffic.ExpiryTime)
	}

	// Verify unlim_exp@x remained untouched
	expTraffic := trafficOf(t, "unlim_exp@x")
	if expTraffic.Total != 50 || expTraffic.ExpiryTime != 0 {
		t.Errorf("unlim_exp changed: total=%d, expiry=%d", expTraffic.Total, expTraffic.ExpiryTime)
	}
}
