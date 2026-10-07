package service

import (
	"strings"
	"testing"
)

func TestRewriteTomlPorts(t *testing.T) {
	initial := `log.level = "info"
serverAddr = "10.10.10.2"
serverPort = 48465
auth.method = "token"
auth.token = "test_token"

[[proxies]]
name = "old_tcp_2053"
type = "tcp"
localIP = "127.0.0.1"
localPort = 2053
remotePort = 2053
`
	ports := []int{443, 2083}
	updated := rewriteTomlPorts(initial, ports)

	if !strings.Contains(updated, `serverPort = 48465`) {
		t.Errorf("expected serverPort to be preserved")
	}
	if !strings.Contains(updated, `auth.token = "test_token"`) {
		t.Errorf("expected auth.token to be preserved")
	}
	if strings.Contains(updated, `old_tcp_2053`) {
		t.Errorf("expected old proxy to be replaced")
	}
	if !strings.Contains(updated, `name = "tcp_443"`) || !strings.Contains(updated, `name = "udp_2083"`) {
		t.Errorf("expected new tcp and udp proxies for 443 and 2083")
	}
}

func TestRewriteBackhaulPorts(t *testing.T) {
	initial := `[client]
remote_addr = "10.10.10.2:3080"
transport = "tcpmux"
token = "sec_test_token"
connection_pool = 8
nodelay = true
ports = [
  "8080=8080",
  "8443=8443"
]
`
	ports := []int{2053, 2096}
	updated := rewriteBackhaulPorts(initial, ports)

	if !strings.Contains(updated, `remote_addr = "10.10.10.2:3080"`) {
		t.Errorf("expected remote_addr to be preserved")
	}
	if !strings.Contains(updated, `transport = "tcpmux"`) {
		t.Errorf("expected transport to be preserved")
	}
	if !strings.Contains(updated, `"2053"`) || !strings.Contains(updated, `"2096"`) {
		t.Errorf("expected updated port list")
	}
	if strings.Contains(updated, `"8080=8080"`) {
		t.Errorf("expected old ports to be removed")
	}
}

func TestCalculateScore(t *testing.T) {
	// Optimal: loss=0, rtt=30, jitter=2
	score1 := calculateScore(0, 30, 2)
	if score1 < 90 {
		t.Errorf("expected high score for low latency and 0 loss, got %d", score1)
	}

	// High loss: loss=50, rtt=30, jitter=2
	score2 := calculateScore(50, 30, 2)
	if score2 > 60 {
		t.Errorf("expected low score for 50%% loss, got %d", score2)
	}

	st1 := determineStatus(95, 0)
	if st1 != "healthy" {
		t.Errorf("expected healthy, got %s", st1)
	}

	st2 := determineStatus(25, 20)
	if st2 != "critical" {
		t.Errorf("expected critical for low score, got %s", st2)
	}
}

func TestParsePorts(t *testing.T) {
	svc := &HashemService{}
	ports := svc.parsePorts("8080, 8443, 2053, 8080, abc, 70000, 0, -5")
	if len(ports) != 3 {
		t.Fatalf("expected 3 valid ports, got %d (%v)", len(ports), ports)
	}
	if ports[0] != 8080 || ports[1] != 8443 || ports[2] != 2053 {
		t.Errorf("unexpected parsed ports: %v", ports)
	}
}

func TestGetGreInterface(t *testing.T) {
	// Default
	if iface := getGreInterface(); iface != "gre-tunnel" {
		t.Errorf("expected default gre-tunnel, got %s", iface)
	}

	// Environment variable override
	t.Setenv("HASHEM_GRE_IFACE", "gre-custom0")
	if iface := getGreInterface(); iface != "gre-custom0" {
		t.Errorf("expected gre-custom0 from env, got %s", iface)
	}
}

func TestBuildIranSetupCommands(t *testing.T) {
	t.Setenv("HASHEM_GRE_IFACE", "gre-test")

	// Backhaul engine
	_, _, oneLinerBh := buildIranSetupCommands("1.1.1.1", "2.2.2.2", 48465, 3080, "secret_tok", "direct", "8080,2053", "backhaul", "tcpmux")
	if !strings.Contains(oneLinerBh, "setup-backhaul-iran") {
		t.Errorf("expected setup-backhaul-iran in one-liner, got %s", oneLinerBh)
	}
	if !strings.Contains(oneLinerBh, "fastly.jsdelivr.net") || !strings.Contains(oneLinerBh, "ghproxy.net") || !strings.Contains(oneLinerBh, "gh-proxy.com") {
		t.Errorf("expected multi-mirror fallback chain in one-liner")
	}

	// GRE-Backhaul engine
	_, optGreBh, oneLinerGreBh := buildIranSetupCommands("1.1.1.1", "2.2.2.2", 48465, 3080, "secret_tok", "fou:443", "8080", "gre-backhaul", "xtcpmux")
	if !strings.Contains(oneLinerGreBh, "setup-gre-backhaul-iran") {
		t.Errorf("expected setup-gre-backhaul-iran in one-liner, got %s", oneLinerGreBh)
	}
	if !strings.Contains(optGreBh, "dev gre-test mtu 1380") {
		t.Errorf("expected optimizeCmd to use gre-test interface, got %s", optGreBh)
	}

	// FRP / Classic GRE engine
	peerJsonCmd, optFrp, oneLinerFrp := buildIranSetupCommands("1.1.1.1", "2.2.2.2", 48465, 3080, "secret_tok", "direct", "8080", "frp", "")
	if !strings.Contains(oneLinerFrp, "setup-iran") {
		t.Errorf("expected setup-iran in one-liner, got %s", oneLinerFrp)
	}
	if !strings.Contains(peerJsonCmd, "Nova-Tunnel-") {
		t.Errorf("expected dynamic Nova-Tunnel ID in peerJsonCmd, got %s", peerJsonCmd)
	}
	if !strings.Contains(peerJsonCmd, "gre-test") {
		t.Errorf("expected peerJsonCmd to use gre-test, got %s", peerJsonCmd)
	}
	if !strings.Contains(optFrp, "-o gre-test") {
		t.Errorf("expected optFrp to bind iptables to gre-test, got %s", optFrp)
	}
}

func TestGenerateOneLinerValidation(t *testing.T) {
	svc := &HashemService{}
	_, err := svc.GenerateOneLiner(HashemOneLinerForm{
		IranIP: "",
	})
	if err == nil {
		t.Errorf("expected error for empty IranIP")
	}

	res, err := svc.GenerateOneLiner(HashemOneLinerForm{
		IranIP:    "1.1.1.1",
		Engine:    "backhaul",
		Transport: "anytls",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.OneLinerCommand == "" {
		t.Fatalf("expected valid one-liner result")
	}
	if !strings.Contains(res.OneLinerCommand, "--transport anytls") {
		t.Errorf("expected --transport anytls in one-liner, got %s", res.OneLinerCommand)
	}
}
