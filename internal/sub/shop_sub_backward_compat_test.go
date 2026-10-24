package sub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestShopSub_VPNClientAcceptHeaderDoesNotServeHTML verifies that a VPN client
// (v2rayNG, Hiddify, Sing-box, Clash, etc.) that advertises "Accept: text/html"
// is NEVER served the HTML Web Portal, guaranteeing backward compatibility
// and preventing config import failures.
func TestShopSub_VPNClientAcceptHeaderDoesNotServeHTML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initSubDB(t)

	subId := "shop-compat-sub-1"
	seedInfoEndpointSub(t, subId, "shop-compat@test.ir")

	router := gin.New()
	NewSUBController(router.Group("/"),
		WithSUBPath("/secret-path/"),
		WithSUBClashEnabled(true),
		WithSUBJsonEnabled(true),
		WithSUBClashAutoDetect(true),
		WithSUBJsonAutoDetect(true),
	)

	vpnUserAgents := []string{
		"v2rayNG/1.8.12",
		"Sing-box/1.9.0",
		"HiddifyNext/1.0.0",
		"ClashMeta/1.18.0",
		"Shadowrocket/1982",
		"Streisand/1.0",
		"Karing/1.0.28",
		"V2Box/1.0",
	}

	for _, ua := range vpnUserAgents {
		t.Run("UA="+ua, func(t *testing.T) {
			// Test both /sub/:subid and legacy /secret-path/:subid
			paths := []string{"/sub/" + subId, "/secret-path/" + subId}
			for _, p := range paths {
				req := httptest.NewRequest(http.MethodGet, p, nil)
				req.Host = "sub.example.com"
				req.Header.Set("User-Agent", ua)
				req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Fatalf("path %s: expected 200 OK for UA %s, got %d; body=%s", p, ua, w.Code, w.Body.String())
				}

				// Crucial check: Content-Type must NOT be text/html
				contentType := w.Header().Get("Content-Type")
				if strings.Contains(contentType, "text/html") {
					t.Fatalf("path %s: UA %s received text/html when requesting subscription configs!", p, ua)
				}

				// Telemetry headers must be present
				if w.Header().Get("Subscription-Userinfo") == "" {
					t.Fatalf("path %s: missing Subscription-Userinfo header for UA %s", p, ua)
				}
				if w.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
					t.Fatalf("path %s: expected X-Robots-Tag to be 'noindex, nofollow', got %q", p, w.Header().Get("X-Robots-Tag"))
				}
			}
		})
	}
}

// TestShopSub_ExplicitFormatOverridesUA checks that explicit format (?format=clash, ?format=plain, etc.)
// always takes precedence over User-Agent and headers.
func TestShopSub_ExplicitFormatOverridesUA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initSubDB(t)

	subId := "shop-override-sub"
	seedInfoEndpointSub(t, subId, "shop-override@test.ir")

	router := gin.New()
	NewSUBController(router.Group("/"),
		WithSUBPath("/legacy-path/"),
		WithSUBClashEnabled(true),
		WithSUBJsonEnabled(true),
		WithSUBClashAutoDetect(true),
		WithSUBJsonAutoDetect(true),
	)

	// A browser requesting ?format=clash gets Clash YAML
	req := httptest.NewRequest(http.MethodGet, "/sub/"+subId+"?format=clash", nil)
	req.Host = "sub.example.com"
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/128.0")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "proxies:") {
		t.Fatalf("expected Clash YAML with proxies:, got %s", w.Body.String())
	}
}
