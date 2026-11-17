package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
)

func TestOpenVPNSubscriptionDomainPersistenceAndURL(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))

	if err := SetOpenVPNSubDomain("  SUB.Example.com. "); err != nil {
		t.Fatalf("save subscription domain: %v", err)
	}
	if got, want := GetOpenVPNSubDomain(), "sub.example.com"; got != want {
		t.Fatalf("GetOpenVPNSubDomain() = %q, want %q", got, want)
	}

	service := &OpenVPNService{}
	if got, want := service.BuildClientSubUrl("client-token"), "https://sub.example.com:2096/sub/client-token"; got != want {
		t.Fatalf("BuildClientSubUrl() = %q, want %q", got, want)
	}

	for _, invalid := range []string{"https://sub.example.com", "sub.example.com/path", "sub.example.com:8443"} {
		if err := SetOpenVPNSubDomain(invalid); err == nil {
			t.Errorf("SetOpenVPNSubDomain(%q) succeeded; want validation error", invalid)
		}
	}

	if err := SetOpenVPNSubDomain(""); err != nil {
		t.Fatalf("clear subscription domain: %v", err)
	}
	if got := GetOpenVPNSubDomain(); got != "" {
		t.Fatalf("GetOpenVPNSubDomain() after clear = %q, want empty", got)
	}
}
