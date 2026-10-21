package sub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNexoraParity_DirectFormatEndpointsAndAutoDetect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initSubDB(t)
	seedInfoEndpointSub(t, "nexora-sub", "nexora@test.ir")

	router := gin.New()
	NewSUBController(router.Group("/"),
		WithSUBPath("/secret-path/"),
		WithSUBClashEnabled(true),
		WithSUBJsonEnabled(true),
		WithSUBClashAutoDetect(true),
		WithSUBJsonAutoDetect(true),
	)

	// 1. Universal /sub/:subid/plain
	req := httptest.NewRequest(http.MethodGet, "/sub/nexora-sub/plain", nil)
	req.Host = "sub.example.com"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/sub/:subid/plain status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.HasPrefix(w.Body.String(), "vless://") {
		t.Fatalf("/sub/:subid/plain body did not start with vless://: %s", w.Body.String())
	}

	// 2. Custom path /secret-path/:subid/plain
	req = httptest.NewRequest(http.MethodGet, "/secret-path/nexora-sub/plain", nil)
	req.Host = "sub.example.com"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/secret-path/:subid/plain status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.HasPrefix(w.Body.String(), "vless://") {
		t.Fatalf("/secret-path/:subid/plain body did not start with vless://: %s", w.Body.String())
	}

	// 3. /sub/:subid/info
	req = httptest.NewRequest(http.MethodGet, "/sub/nexora-sub/info", nil)
	req.Host = "sub.example.com"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/sub/:subid/info status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var info map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatalf("/sub/:subid/info body not valid json: %v", err)
	}
	if info["sId"] != "nexora-sub" {
		t.Fatalf("/sub/:subid/info sId = %v, want nexora-sub", info["sId"])
	}

	// 4. ?format=plain
	req = httptest.NewRequest(http.MethodGet, "/sub/nexora-sub?format=plain", nil)
	req.Host = "sub.example.com"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("?format=plain status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.HasPrefix(w.Body.String(), "vless://") {
		t.Fatalf("?format=plain body did not start with vless://: %s", w.Body.String())
	}

	// 5. Smart UA Auto-Detect Clash
	req = httptest.NewRequest(http.MethodGet, "/sub/nexora-sub", nil)
	req.Host = "sub.example.com"
	req.Header.Set("User-Agent", "ClashMeta/1.18.0")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Clash auto-detect status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "proxies:") {
		t.Fatalf("Clash auto-detect body did not contain proxies: %s", w.Body.String())
	}

	// 6. Direct /sub/:subid/clash
	req = httptest.NewRequest(http.MethodGet, "/sub/nexora-sub/clash", nil)
	req.Host = "sub.example.com"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/sub/:subid/clash status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "proxies:") {
		t.Fatalf("/sub/:subid/clash body did not contain proxies: %s", w.Body.String())
	}

	// 7. Direct /sub/:subid/singbox
	req = httptest.NewRequest(http.MethodGet, "/sub/nexora-sub/singbox", nil)
	req.Host = "sub.example.com"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/sub/:subid/singbox status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var sbConfig any
	if err := json.Unmarshal(w.Body.Bytes(), &sbConfig); err != nil {
		t.Fatalf("/sub/:subid/singbox body not valid JSON: %v; body=%s", err, w.Body.String())
	}
}
