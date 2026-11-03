package service

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestOpenVPNClientBlockingCCD(t *testing.T) {
	tempCcd := t.TempDir()
	ccdDirOverride = tempCcd
	t.Cleanup(func() { ccdDirOverride = "" })

	clientName := "testuser"

	// 1. Block client
	if err := setClientBlocked(clientName, true); err != nil {
		t.Fatalf("setClientBlocked(true) failed: %v", err)
	}

	ccdFile := filepath.Join(tempCcd, clientName)
	content, err := os.ReadFile(ccdFile)
	if err != nil {
		t.Fatalf("expected ccd file %s to exist: %v", ccdFile, err)
	}
	if string(content) != "disable\n" {
		t.Fatalf("expected 'disable\\n', got %q", string(content))
	}

	// 2. Unblock client
	if err := setClientBlocked(clientName, false); err != nil {
		t.Fatalf("setClientBlocked(false) failed: %v", err)
	}
	if _, err := os.Stat(ccdFile); !os.IsNotExist(err) {
		t.Fatalf("expected ccd file %s to be removed, but it exists", ccdFile)
	}
}

func TestOpenVPNTrafficDeltaSync(t *testing.T) {
	tmpCcd := t.TempDir()
	ccdDirOverride = tmpCcd
	defer func() { ccdDirOverride = "" }()

	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	db := database.GetDB()

	user := &model.OpenVPNUser{
		Name:    "ramin",
		Enable:  true,
		TotalGB: 1,
		Up:      0,
		Down:    0,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	svc := &OpenVPNService{}

	// First poll: client used 2 MB up, 8 MB down
	sessions1 := []OpenVPNLiveSession{
		{
			CommonName:     "ramin",
			RealAddress:    "1.2.3.4:5000",
			BytesReceived:  2 * 1024 * 1024,
			BytesSent:      8 * 1024 * 1024,
			ConnectedSince: "2026-11-02 10:00:00",
		},
	}
	if err := svc.ApplyLiveSessionDeltas(sessions1); err != nil {
		t.Fatalf("applyLiveSessionDeltas 1: %v", err)
	}

	var u1 model.OpenVPNUser
	_ = db.Where("name = ?", "ramin").First(&u1)
	if u1.Up != 2*1024*1024 || u1.Down != 8*1024*1024 {
		t.Fatalf("expected up=2MB, down=8MB; got up=%d, down=%d", u1.Up, u1.Down)
	}

	// Second poll: client transferred more bytes (cumulative 2.5 MB up, 10.5 MB down)
	sessions2 := []OpenVPNLiveSession{
		{
			CommonName:     "ramin",
			RealAddress:    "1.2.3.4:5000",
			BytesReceived:  2500000,
			BytesSent:      10500000,
			ConnectedSince: "2026-11-02 10:00:00",
		},
	}
	if err := svc.ApplyLiveSessionDeltas(sessions2); err != nil {
		t.Fatalf("applyLiveSessionDeltas 2: %v", err)
	}

	var u2 model.OpenVPNUser
	_ = db.Where("name = ?", "ramin").First(&u2)
	if u2.Up != 2500000 || u2.Down != 10500000 {
		t.Fatalf("expected up=2500000, down=10500000; got up=%d, down=%d", u2.Up, u2.Down)
	}

	// Third poll: client disconnected / killed (sessions list empty)
	if err := svc.ApplyLiveSessionDeltas(nil); err != nil {
		t.Fatalf("applyLiveSessionDeltas 3 (empty): %v", err)
	}

	var u3 model.OpenVPNUser
	_ = db.Where("name = ?", "ramin").First(&u3)
	// TRAFFIC MUST NOT ZERO OUT!
	if u3.Up != 2500000 || u3.Down != 10500000 {
		t.Fatalf("CRITICAL BUG: traffic zeroed out on disconnect! got up=%d, down=%d", u3.Up, u3.Down)
	}

	// Session row should be cleaned up
	var sessCount int64
	_ = db.Model(&model.OpenVPNSession{}).Where("name = ?", "ramin").Count(&sessCount)
	if sessCount != 0 {
		t.Fatalf("expected 0 active sessions after disconnect, got %d", sessCount)
	}

	// Admin disables client (مسدود)
	if err := svc.UpdateClient("ramin", "", u3.TotalGB, u3.ExpiryTime, u3.LimitIP, false); err != nil {
		t.Fatalf("UpdateClient disable failed: %v", err)
	}

	// Verify CCD block exists
	ccdPath := filepath.Join(tmpCcd, "ramin")
	data, err := os.ReadFile(ccdPath)
	if err != nil || string(data) != "disable\n" {
		t.Fatalf("expected CCD disable file, got %q (err: %v)", string(data), err)
	}

	// Verify traffic is STILL preserved after disabling
	var uDisabled model.OpenVPNUser
	_ = db.Where("name = ?", "ramin").First(&uDisabled)
	if uDisabled.Up != 2500000 || uDisabled.Down != 10500000 {
		t.Fatalf("CRITICAL BUG: traffic lost after disable! got up=%d, down=%d", uDisabled.Up, uDisabled.Down)
	}

	// Admin re-enables client (فعال)
	if err := svc.UpdateClient("ramin", "", uDisabled.TotalGB, uDisabled.ExpiryTime, uDisabled.LimitIP, true); err != nil {
		t.Fatalf("UpdateClient re-enable failed: %v", err)
	}

	// Verify CCD block is removed
	if _, err := os.Stat(ccdPath); !os.IsNotExist(err) {
		t.Fatalf("expected CCD file removed after re-enable, but it still exists")
	}

	// Verify traffic is STILL preserved after re-enabling
	var uReEnabled model.OpenVPNUser
	_ = db.Where("name = ?", "ramin").First(&uReEnabled)
	if uReEnabled.Up != 2500000 || uReEnabled.Down != 10500000 {
		t.Fatalf("CRITICAL BUG: traffic lost after re-enable! got up=%d, down=%d", uReEnabled.Up, uReEnabled.Down)
	}
}

func TestOpenVPNEnforceDeviceLimits(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	db := database.GetDB()

	user := &model.OpenVPNUser{
		Name:    "soroosh",
		Enable:  true,
		LimitIP: 1, // Max 1 device!
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Spin up mock OpenVPN management listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	mgmtAddressOverride = ln.Addr().String()
	defer func() { mgmtAddressOverride = "" }()

	var killedCommands []string
	var serverDone = make(chan struct{})

	go func() {
		defer close(serverDone)
		// Connection 1: QueryManagementSessions (status 2)
		conn1, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn1.Close()

		_, _ = conn1.Write([]byte(">INFO:OpenVPN Management Interface\r\n"))
		buf := make([]byte, 1024)
		n, _ := conn1.Read(buf)
		req := string(buf[:n])
		if strings.Contains(req, "status 2") {
			statusResp := "TITLE,OpenVPN Mock\r\n" +
				"TIME,2026-11-03 10:00:00,1793698800\r\n" +
				"HEADER,CLIENT_LIST,Common Name,Real Address,Virtual Address,Virtual IPv6 Address,Bytes Received,Bytes Sent,Connected Since,Connected Since (time_t),Username,Client ID,Peer ID,Data Channel Cipher\r\n" +
				"CLIENT_LIST,soroosh,1.1.1.1:1001,10.8.0.2,,1000,2000,2026-11-03 09:50:00,1793698200,soroosh,1,0,AES-256-GCM\r\n" +
				"CLIENT_LIST,soroosh,2.2.2.2:2002,10.8.0.3,,500,800,2026-11-03 09:55:00,1793698500,soroosh,2,1,AES-256-GCM\r\n" +
				"HEADER,ROUTING_TABLE,Virtual Address,Common Name,Real Address,Last Ref,Last Ref (time_t)\r\n" +
				"GLOBAL_STATS,Max bcast/mcast queue length,0\r\n" +
				"END\r\n"
			_, _ = conn1.Write([]byte(statusResp))
		}

		// Connection 2: kill excess session
		conn2, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn2.Close()

		_, _ = conn2.Write([]byte(">INFO:OpenVPN Management Interface\r\n"))
		n2, _ := conn2.Read(buf)
		cmd := string(buf[:n2])
		killedCommands = append(killedCommands, strings.TrimSpace(cmd))
	}()

	svc := &OpenVPNService{}
	svc.EnforceDeviceLimits()

	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for mock management server")
	}

	if len(killedCommands) == 0 {
		t.Fatalf("expected kill command to be issued for 2nd device exceeding LimitIP=1, but none was issued")
	}
	if !strings.Contains(killedCommands[0], "kill 2.2.2.2:2002") {
		t.Fatalf("expected kill command for 2.2.2.2:2002, got: %q", killedCommands[0])
	}
}
