package service

import (
	"fmt"
	"strings"
	"testing"
)

func TestOpenVPNClientConfigFormat(t *testing.T) {
	// Sample mock data
	caData := "-----BEGIN CERTIFICATE-----\nCA_SAMPLE\n-----END CERTIFICATE-----"
	cleanCert := "-----BEGIN CERTIFICATE-----\nCLIENT_SAMPLE\n-----END CERTIFICATE-----"
	keyData := "-----BEGIN PRIVATE KEY-----\nKEY_SAMPLE\n-----END PRIVATE KEY-----"
	remoteIP := "45.142.189.34"
	remotePort := 1194
	proto := "tcp"
	authPassBlock := ""

	ovpn := fmt.Sprintf(`client
dev tun
proto %s
remote %s %d
resolv-retry infinite
nobind
persist-key
persist-tun
;remote-cert-tls server
data-ciphers AES-256-GCM:AES-128-GCM:CHACHA20-POLY1305
data-ciphers-fallback AES-256-GCM
cipher AES-256-GCM
auth SHA256
verb 3
%s
<ca>
%s
</ca>
<cert>
%s
</cert>
<key>
%s
</key>
`, proto, remoteIP, remotePort, authPassBlock, strings.TrimSpace(caData), strings.TrimSpace(cleanCert), strings.TrimSpace(keyData))

	if strings.Contains(ovpn, "\nremote-cert-tls server\n") {
		t.Fatalf("expected remote-cert-tls server to be disabled/commented to prevent Android peer certificate verification failure")
	}

	if !strings.Contains(ovpn, ";remote-cert-tls server") {
		t.Fatalf("expected ;remote-cert-tls server in ovpn profile")
	}

	if !strings.Contains(ovpn, "data-ciphers AES-256-GCM:AES-128-GCM:CHACHA20-POLY1305") {
		t.Fatalf("expected modern data-ciphers negotiation in ovpn profile")
	}

	if !strings.Contains(ovpn, "remote 45.142.189.34 1194") {
		t.Fatalf("expected remote address and port formatted correctly")
	}
}
