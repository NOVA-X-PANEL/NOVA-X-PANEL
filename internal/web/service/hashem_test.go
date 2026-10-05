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
