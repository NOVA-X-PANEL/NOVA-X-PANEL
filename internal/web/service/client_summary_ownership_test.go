package service

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestClientSummaryOwnershipAndStatusBuckets(t *testing.T) {
	svc, inboundSvc, settingSvc := setupPagingServices(t)
	db := database.GetDB()
	now := time.Now().UnixMilli()
	process := useOnlineTestProcess(t)

	for _, owner := range []int64{11, 22} {
		for _, status := range []string{"active", "expired", "quota", "near-time", "near-traffic", "disabled"} {
			email := fmt.Sprintf("%d-%s@x", owner, status)
			rec := model.ClientRecord{Email: email, OwnerAdminId: owner, Enable: true, Group: "allowed"}
			var used int64
			switch status {
			case "expired":
				rec.ExpiryTime = now - pagingDay
			case "quota":
				rec.TotalGB, used = 10*pagingGB, 10*pagingGB
			case "near-time":
				rec.ExpiryTime = now + pagingDay
			case "near-traffic":
				rec.TotalGB, used = 10*pagingGB, 10*pagingGB-pagingGB/2
			}
			if err := db.Create(&rec).Error; err != nil {
				t.Fatalf("create %s: %v", email, err)
			}
			if status == "disabled" {
				if err := db.Model(&rec).Update("enable", false).Error; err != nil {
					t.Fatalf("disable %s: %v", email, err)
				}
			}
			if err := db.Create(&xray.ClientTraffic{Email: email, Up: used, Enable: true}).Error; err != nil {
				t.Fatalf("traffic for %s: %v", email, err)
			}
		}
	}
	process.SetNodeOnlineTree(1, map[string][]string{"node": {"11-active@x", "22-active@x", "11-disabled@x", "missing@x"}})

	cases := []struct {
		name   string
		scope  ClientAccessScope
		owners []int64
	}{
		{"owner sees all", allClientScope(), []int64{11, 22}},
		{"admin sees own", ClientAccessScope{Mode: ClientAccessOwn, AdminID: 11}, []int64{11}},
		{"other admin sees own", ClientAccessScope{Mode: ClientAccessOwn, AdminID: 22}, []int64{22}},
		{"none sees nothing", ClientAccessScope{Mode: ClientAccessNone}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Filtering and pagination affect the rows, not the summary's scoped set.
			resp, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{Filter: "depleted", PageSize: 1}, tc.scope)
			if err != nil {
				t.Fatal(err)
			}
			n := len(tc.owners)
			s := resp.Summary
			if resp.Total != 6*n || resp.Filtered != 2*n || s.Total != 6*n || s.Active != n || s.DepletedCount != 2*n || s.ExpiringCount != 2*n || s.DeactiveCount != n || s.OnlineCount != n {
				t.Fatalf("total=%d filtered=%d summary=%+v for owners %v", resp.Total, resp.Filtered, s, tc.owners)
			}
			var online, depleted, expiring, deactive []string
			for _, owner := range tc.owners {
				online = append(online, fmt.Sprintf("%d-active@x", owner))
				depleted = append(depleted, fmt.Sprintf("%d-expired@x", owner), fmt.Sprintf("%d-quota@x", owner))
				expiring = append(expiring, fmt.Sprintf("%d-near-time@x", owner), fmt.Sprintf("%d-near-traffic@x", owner))
				deactive = append(deactive, fmt.Sprintf("%d-disabled@x", owner))
			}
			if !slices.Equal(s.Online, online) || !slices.Equal(s.Depleted, depleted) || !slices.Equal(s.Expiring, expiring) || !slices.Equal(s.Deactive, deactive) {
				t.Fatalf("unexpected identity buckets: %+v", s)
			}
			for filter, count := range map[string]int{"online": n, "active": n, "depleted": 2 * n, "expiring": 2 * n, "deactive": n} {
				page, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{Filter: filter, PageSize: 50}, tc.scope)
				if err != nil || page.Filtered != count {
					t.Fatalf("filter %s: page=%+v err=%v want %d", filter, page, err, count)
				}
			}
		})
	}

	// Additional scope dimensions must apply to online counts, not just ownership.
	for _, scope := range []ClientAccessScope{
		{Mode: ClientAccessOwn, AdminID: 11, RestrictGroups: true, AllowedGroups: []string{"other"}},
		{Mode: ClientAccessOwn, AdminID: 11, RestrictInbounds: true, AllowedInboundIDs: []int{99}},
	} {
		resp, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{}, scope)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Total != 0 || resp.Summary.OnlineCount != 0 || len(resp.Summary.Online) != 0 {
			t.Fatalf("restrictions ignored: %+v", resp)
		}
	}
	// An all-owner scope can still be inbound-restricted. The unpaged list must
	// not skip that restriction just because its ownership and groups are open.
	rows, err := svc.ListForScope(ClientAccessScope{Mode: ClientAccessAll, RestrictInbounds: true, AllowedInboundIDs: []int{99}})
	if err != nil || len(rows) != 0 {
		t.Fatalf("unpaged inbound-restricted list = %+v, err %v", rows, err)
	}
}
