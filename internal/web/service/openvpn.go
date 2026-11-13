package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"errors"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const (
	defaultEasyRsaDir = "/etc/openvpn/easy-rsa"
	defaultServerDir  = "/etc/openvpn/server"
	defaultServerConf = "/etc/openvpn/server/server.conf"
	defaultCcdDir     = "/etc/openvpn/ccd"
	openvpnUnitName   = "openvpn-server@server.service"
	mgmtAddress       = "127.0.0.1:7505"
)

var (
	validClientNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)
	openvpnTrafficMu      sync.Mutex
	ccdDirOverride        string
	mgmtAddressOverride   string
)

func getMgmtAddress() string {
	if mgmtAddressOverride != "" {
		return mgmtAddressOverride
	}
	return mgmtAddress
}

func getCcdDir() string {
	if ccdDirOverride != "" {
		return ccdDirOverride
	}
	return defaultCcdDir
}

func ensureCcdDir() error {
	return os.MkdirAll(getCcdDir(), 0755)
}

func setClientBlocked(name string, blocked bool) error {
	name = strings.TrimSpace(name)
	if !validClientNameRegexp.MatchString(name) || name == "server" {
		return fmt.Errorf("invalid client name")
	}
	if err := ensureCcdDir(); err != nil {
		return err
	}
	ccdPath := filepath.Join(getCcdDir(), name)
	if blocked {
		return os.WriteFile(ccdPath, []byte("disable\n"), 0644)
	}
	_ = os.Remove(ccdPath)
	return nil
}

// OpenVPNLiveSession represents an active session parsed from OpenVPN management socket.
type OpenVPNLiveSession struct {
	CommonName     string
	RealAddress    string
	BytesReceived  int64
	BytesSent      int64
	ConnectedSince string
}

// OpenVPNConnectedClient represents an active connection shown in UI.
type OpenVPNConnectedClient struct {
	CommonName     string `json:"common_name"`
	RealAddress    string `json:"real_address"`
	BytesReceived  string `json:"bytes_received"`
	BytesSent      string `json:"bytes_sent"`
	ConnectedSince string `json:"connected_since"`
}

// OpenVPNClientInfo represents an issued client certificate with limits and live usage.
type OpenVPNClientInfo struct {
	Name               string   `json:"name"`
	Password           string   `json:"password"`
	Enable             bool     `json:"enable"`
	TotalGB            int64    `json:"total_gb"`
	ExpiryTime         int64    `json:"expiry_time"`
	LimitIP            int      `json:"limit_ip"`
	Up                 int64    `json:"up"`
	Down               int64    `json:"down"`
	TotalUsed          int64    `json:"total_used"`
	TrafficUsedStr     string   `json:"traffic_used_str"`
	TrafficLimitStr    string   `json:"traffic_limit_str"`
	TrafficPercent     float64  `json:"traffic_percent"`
	CreatedAt          string   `json:"created_at"`
	IsConnected        bool     `json:"is_connected"`
	ConnectedIPs       []string `json:"connected_ips"`
	CurrentConnections int      `json:"current_connections"`
	Since              string   `json:"since,omitempty"`
	IsExpired          bool     `json:"is_expired"`
	IsTrafficExceeded  bool     `json:"is_traffic_exceeded"`
	OwnerAdminId       int      `json:"owner_admin_id"`
	OwnerAdminUsername string   `json:"owner_admin_username"`
}

// OpenVPNAccessScope defines the permission boundary for the acting admin.
type OpenVPNAccessScope struct {
	AdminID int
	Mode    int // 0 = None, 1 = Own, 2 = All
	IsOwner bool
}

// OpenVPNAdminUsage contains usage summary and quota for an admin.
type OpenVPNAdminUsage struct {
	AdminID        int     `json:"admin_id"`
	Username       string  `json:"username"`
	RoleName       string  `json:"role_name"`
	ClientCount    int     `json:"client_count"`
	OnlineCount    int     `json:"online_count"`
	TotalUp        int64   `json:"total_up"`
	TotalDown      int64   `json:"total_down"`
	TotalUsed      int64   `json:"total_used"`
	TrafficUsedStr string  `json:"traffic_used_str"`
	DataLimit      int64   `json:"data_limit"`
	DataLimitStr   string  `json:"data_limit_str"`
	UsagePercent   float64 `json:"usage_percent"`
	IsLimited      bool    `json:"is_limited"`
	Status         string  `json:"status"`
}

// OpenVPNStatus contains overall OpenVPN server status and client list.
type OpenVPNStatus struct {
	Installed        bool                     `json:"installed"`
	Running          bool                     `json:"running"`
	Port             int                      `json:"port"`
	Proto            string                   `json:"proto"`
	Subnet           string                   `json:"subnet"`
	DirectIP         string                   `json:"direct_ip"`
	IranIP           string                   `json:"iran_ip"`
	RemoteHost       string                   `json:"remote_host"`
	RemotePort       int                      `json:"remote_port"`
	TotalClients     int                      `json:"total_clients"`
	OnlineClients    int                      `json:"online_clients"`
	TotalUp          int64                    `json:"total_up"`
	TotalDown        int64                    `json:"total_down"`
	TotalUsed        int64                    `json:"total_used"`
	TotalUsedStr     string                   `json:"total_used_str"`
	TotalUpStr       string                   `json:"total_up_str"`
	TotalDownStr     string                   `json:"total_down_str"`
	ConnectedClients []OpenVPNConnectedClient `json:"connected_clients"`
	Clients          []OpenVPNClientInfo      `json:"clients"`
	AdminStats       []OpenVPNAdminUsage      `json:"admin_stats"`
}

// OpenVPNService manages OpenVPN server, easy-rsa certificates, client configs, and accounting.
type OpenVPNService struct {
	lock sync.Mutex
}

func NewOpenVPNService() *OpenVPNService {
	return &OpenVPNService{}
}

func (s *OpenVPNService) execPrivileged(ctx context.Context, name string, args ...string) ([]byte, error) {
	if systemdRun, err := exec.LookPath("systemd-run"); err == nil {
		sdArgs := append([]string{"--quiet", "--pipe", "--wait", name}, args...)
		cmd := exec.CommandContext(ctx, systemdRun, sdArgs...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return out, nil
		}
	}
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

func (s *OpenVPNService) execPrivilegedBash(ctx context.Context, script string) ([]byte, error) {
	tmpFile, err := os.CreateTemp("", "ovpn_script_*.sh")
	if err == nil {
		defer os.Remove(tmpFile.Name())
		if _, err := tmpFile.WriteString(script); err == nil {
			_ = tmpFile.Close()
			_ = os.Chmod(tmpFile.Name(), 0755)

			if systemdRun, err := exec.LookPath("systemd-run"); err == nil {
				cmd := exec.CommandContext(ctx, systemdRun, "--quiet", "--pipe", "--wait", "bash", tmpFile.Name())
				out, err := cmd.CombinedOutput()
				if err == nil {
					return out, nil
				}
			}
			cmd := exec.CommandContext(ctx, "bash", tmpFile.Name())
			return cmd.CombinedOutput()
		}
	}

	if systemdRun, err := exec.LookPath("systemd-run"); err == nil {
		cmd := exec.CommandContext(ctx, systemdRun, "--quiet", "--pipe", "--wait", "bash", "-c", script)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return out, nil
		}
	}
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	return cmd.CombinedOutput()
}

// FormatBytes formats byte counts into human-readable strings (MB/GB).
func FormatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
}

// KillClient disconnects any active OpenVPN session matching common_name via management socket.
func (s *OpenVPNService) KillClient(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}

	sessions, _ := s.QueryManagementSessions()

	conn, err := net.DialTimeout("tcp", getMgmtAddress(), 1*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	reader := bufio.NewReader(conn)
	_, _ = reader.ReadString('\n') // read banner

	// 1. Kill by common name
	_, _ = fmt.Fprintf(conn, "kill %s\n", name)

	// 2. Kill by exact IP:port for all matching sessions
	for _, sess := range sessions {
		if strings.EqualFold(sess.CommonName, name) && sess.RealAddress != "" {
			_, _ = fmt.Fprintf(conn, "kill %s\n", sess.RealAddress)
		}
	}
	return nil
}

// QueryManagementSessions reads all live connected sessions from the OpenVPN management interface.
func (s *OpenVPNService) QueryManagementSessions() ([]OpenVPNLiveSession, error) {
	conn, err := net.DialTimeout("tcp", getMgmtAddress(), 1*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	reader := bufio.NewReader(conn)
	_, _ = reader.ReadString('\n') // read banner

	_, err = fmt.Fprintf(conn, "status 2\n")
	if err != nil {
		return nil, err
	}

	var sessions []OpenVPNLiveSession
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "END" {
			break
		}
		if strings.HasPrefix(line, "CLIENT_LIST,") {
			parts := strings.Split(line, ",")
			// CLIENT_LIST,Common Name,Real Address,Virtual Address,Virtual IPv6 Address,Bytes Received,Bytes Sent,Connected Since,Connected Since (time_t),Username
			if len(parts) >= 8 {
				cn := strings.TrimSpace(parts[1])
				realAddr := strings.TrimSpace(parts[2])
				virtAddr := ""
				if len(parts) >= 4 {
					virtAddr = strings.TrimSpace(parts[3])
				}
				if (cn == "UNDEF" || cn == "") && len(parts) >= 10 {
					uCol := strings.TrimSpace(parts[9])
					if uCol != "" && uCol != "UNDEF" {
						cn = uCol
					}
				}
				if cn == "UNDEF" || cn == "" {
					if db := database.GetDB(); db != nil {
						portPart := ""
						if idx := strings.LastIndex(realAddr, ":"); idx != -1 {
							portPart = realAddr[idx+1:]
						}
						var sName string
						if portPart != "" {
							_ = db.Raw("SELECT name FROM open_vpn_sessions WHERE real_port = ? LIMIT 1", portPart).Scan(&sName)
						}
						if sName == "" && virtAddr != "" {
							_ = db.Raw("SELECT name FROM open_vpn_sessions WHERE virtual_ip = ? LIMIT 1", virtAddr).Scan(&sName)
						}
						if sName != "" {
							cn = sName
						}
					}
				}
				rx, _ := strconv.ParseInt(parts[5], 10, 64)
				tx, _ := strconv.ParseInt(parts[6], 10, 64)
				since := parts[7]
				sessions = append(sessions, OpenVPNLiveSession{
					CommonName:     cn,
					RealAddress:    realAddr,
					BytesReceived:  rx,
					BytesSent:      tx,
					ConnectedSince: since,
				})
			}
		}
	}
	return sessions, nil
}

// ApplyLiveSessionDeltas computes byte deltas from active sessions, persists cumulative
// traffic to open_vpn_users, and synchronizes open_vpn_sessions state.
func (s *OpenVPNService) ApplyLiveSessionDeltas(liveSessions []OpenVPNLiveSession) error {
	openvpnTrafficMu.Lock()
	defer openvpnTrafficMu.Unlock()

	db := database.GetDB()
	if db == nil {
		return errors.New("db not initialized")
	}

	var existingSessions []model.OpenVPNSession
	_ = db.Find(&existingSessions).Error
	sessionMap := make(map[string]*model.OpenVPNSession, len(existingSessions))
	for i := range existingSessions {
		sess := &existingSessions[i]
		key := strings.ToLower(sess.Name) + "|" + sess.RealIp + ":" + sess.RealPort
		sessionMap[key] = sess
	}

	activeKeys := make(map[string]bool)
	nowMs := time.Now().UnixMilli()

	type trafficDelta struct {
		up   int64
		down int64
	}
	userDeltas := make(map[string]*trafficDelta)

	for _, sess := range liveSessions {
		cn := strings.TrimSpace(sess.CommonName)
		if cn == "" || cn == "UNDEF" {
			continue
		}
		realAddr := strings.TrimSpace(sess.RealAddress)
		realIp := realAddr
		realPort := ""
		if idx := strings.LastIndex(realAddr, ":"); idx != -1 {
			realIp = realAddr[:idx]
			realPort = realAddr[idx+1:]
		}

		key := strings.ToLower(cn) + "|" + realIp + ":" + realPort
		activeKeys[key] = true

		var deltaRx, deltaTx int64
		existing, found := sessionMap[key]
		if found {
			if sess.BytesReceived >= existing.LastRx {
				deltaRx = sess.BytesReceived - existing.LastRx
			} else {
				deltaRx = sess.BytesReceived
			}
			if sess.BytesSent >= existing.LastTx {
				deltaTx = sess.BytesSent - existing.LastTx
			} else {
				deltaTx = sess.BytesSent
			}

			existing.LastRx = sess.BytesReceived
			existing.LastTx = sess.BytesSent
			existing.UpdatedAt = nowMs
			_ = db.Model(existing).Updates(map[string]any{
				"last_rx":    sess.BytesReceived,
				"last_tx":    sess.BytesSent,
				"updated_at": nowMs,
			}).Error
		} else {
			deltaRx = sess.BytesReceived
			deltaTx = sess.BytesSent
			newSession := &model.OpenVPNSession{
				Name:        cn,
				RealIp:      realIp,
				RealPort:    realPort,
				LastRx:      sess.BytesReceived,
				LastTx:      sess.BytesSent,
				ConnectedAt: nowMs,
				UpdatedAt:   nowMs,
			}
			_ = db.Create(newSession).Error
			sessionMap[key] = newSession
		}

		if deltaRx > 0 || deltaTx > 0 {
			d, exists := userDeltas[cn]
			if !exists {
				d = &trafficDelta{}
				userDeltas[cn] = d
			}
			d.up += deltaRx
			d.down += deltaTx
		}
	}

	for key, existing := range sessionMap {
		if !activeKeys[key] {
			_ = db.Delete(existing).Error
		}
	}

	for name, delta := range userDeltas {
		if delta.up > 0 || delta.down > 0 {
			_ = db.Model(&model.OpenVPNUser{}).
				Where("LOWER(name) = ?", strings.ToLower(name)).
				Updates(map[string]any{
					"up":   gorm.Expr("up + ?", delta.up),
					"down": gorm.Expr("down + ?", delta.down),
				}).Error
		}
	}

	return nil
}

// SyncTraffic scrapes live sessions from the management socket and records traffic deltas in DB.
func (s *OpenVPNService) SyncTraffic() error {
	sessions, err := s.QueryManagementSessions()
	if err != nil {
		return err
	}
	return s.ApplyLiveSessionDeltas(sessions)
}

// SyncAllCCD synchronizes all client CCD files against current database quotas and expiration state.
func (s *OpenVPNService) SyncAllCCD() error {
	db := database.GetDB()
	if db == nil {
		return errors.New("db not initialized")
	}

	var users []model.OpenVPNUser
	if err := db.Find(&users).Error; err != nil {
		return err
	}

	nowMs := time.Now().UnixMilli()

	var allAdmins []model.User
	_ = db.Find(&allAdmins).Error
	adminLimitMap := make(map[int]bool)
	for _, a := range allAdmins {
		if a.DataLimit > 0 && a.UsedBytes >= a.DataLimit {
			adminLimitMap[a.Id] = true
		}
	}

	liveSessions, _ := s.QueryManagementSessions()
	onlineMap := make(map[string]bool)
	for _, sess := range liveSessions {
		if sess.CommonName != "" && sess.CommonName != "UNDEF" {
			onlineMap[strings.ToLower(sess.CommonName)] = true
		}
	}

	for _, u := range users {
		totalUsed := u.Up + u.Down
		isExpired := u.ExpiryTime > 0 && nowMs >= u.ExpiryTime
		isTrafficExceeded := u.TotalGB > 0 && totalUsed >= (u.TotalGB*1024*1024*1024)
		isAdminExceeded := u.OwnerAdminId > 0 && adminLimitMap[u.OwnerAdminId]

		shouldBlock := !u.Enable || isExpired || isTrafficExceeded || isAdminExceeded
		_ = setClientBlocked(u.Name, shouldBlock)

		if shouldBlock && onlineMap[strings.ToLower(u.Name)] {
			_ = s.KillClient(u.Name)
		}
	}

	return nil
}

// EnforceDeviceLimits terminates excess active OpenVPN sessions exceeding user.LimitIP.
func (s *OpenVPNService) EnforceDeviceLimits() {
	db := database.GetDB()
	if db == nil {
		return
	}

	var limitedUsers []*model.OpenVPNUser
	if err := db.Where("limit_ip > 0").Find(&limitedUsers).Error; err != nil || len(limitedUsers) == 0 {
		return
	}

	sessions, err := s.QueryManagementSessions()
	if err != nil || len(sessions) == 0 {
		return
	}

	userLimitMap := make(map[string]int)
	for _, u := range limitedUsers {
		userLimitMap[strings.ToLower(strings.TrimSpace(u.Name))] = u.LimitIP
	}

	// Group sessions by common name
	sessionsByCN := make(map[string][]OpenVPNLiveSession)
	for _, sess := range sessions {
		cn := strings.ToLower(strings.TrimSpace(sess.CommonName))
		if cn == "" || cn == "undef" || cn == "server" {
			continue
		}
		if _, ok := userLimitMap[cn]; ok {
			sessionsByCN[cn] = append(sessionsByCN[cn], sess)
		}
	}

	if len(sessionsByCN) == 0 {
		return
	}

	conn, dialErr := net.DialTimeout("tcp", getMgmtAddress(), 1*time.Second)
	if dialErr != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	for cn, userSessions := range sessionsByCN {
		limit := userLimitMap[cn]
		if limit > 0 && len(userSessions) > limit {
			// Kill excess sessions beyond limit
			for i := limit; i < len(userSessions); i++ {
				if userSessions[i].RealAddress != "" {
					_, _ = fmt.Fprintf(conn, "kill %s\n", userSessions[i].RealAddress)
				}
			}
		}
	}
}

// SyncTrafficAndEnforceLimits performs both delta traffic sync, CCD quota enforcement, and device limit enforcement.
func (s *OpenVPNService) SyncTrafficAndEnforceLimits() {
	_ = s.SyncTraffic()
	_ = s.SyncAllCCD()
	s.EnforceDeviceLimits()
}

// isPortInUseByInbounds checks if a given port is already used by an active Xray inbound.
func isPortInUseByInbounds(port int) (bool, string) {
	db := database.GetDB()
	if db == nil {
		return false, ""
	}
	var inbounds []model.Inbound
	if err := db.Where("enable = ? AND port = ?", true, port).Find(&inbounds).Error; err == nil && len(inbounds) > 0 {
		return true, fmt.Sprintf("پورت %d توسط اینباند فعال «%s» (%s) در حال استفاده است. لطفاً جهت جلوگیری از قطعی VLESS از پورت دیگری استفاده کنید.", port, inbounds[0].Remark, inbounds[0].Protocol)
	}
	return false, ""
}

// PurgeRogueRedirects actively purges any legacy or rogue PREROUTING REDIRECT rules targeting OpenVPN so VLESS / Xray ports and Tunnels are never hijacked.
func (s *OpenVPNService) PurgeRogueRedirects() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	script := `
iptables -t nat -S PREROUTING 2>/dev/null | grep -E -- '(-j REDIRECT|--to-ports)' | while read -r r; do
    del_r=$(echo "$r" | sed 's/^-A/-D/')
    iptables -t nat $del_r 2>/dev/null || true
done
`
	_, _ = s.execPrivilegedBash(ctx, script)
}

const openvpnVerifyPyScript = `#!/usr/bin/env python3
"""
NOVA-X-PANEL OpenVPN Authentication Verifier Hook
Invoked by OpenVPN: auth-user-pass-verify <cred_file_path> via-file
Supports both PostgreSQL (XUI_DB_TYPE=postgres) and SQLite (/etc/x-ui/x-ui.db)
"""
import sys
import os
import time

LOG_PATH = "/var/log/openvpn_auth_exec.log"

def log_debug(msg):
    try:
        with open(LOG_PATH, "a") as f:
            f.write(f"{time.strftime('%Y-%m-%d %H:%M:%S')} [VERIFY] {msg}\n")
    except Exception:
        pass

class DB:
    def __init__(self):
        self.backend, self.conn = self._connect()
        if not self.conn:
            raise RuntimeError("No database connection available")

    def _connect(self):
        db_type = os.environ.get("XUI_DB_TYPE", "")
        dsn = os.environ.get("XUI_DB_DSN", "")
        if os.path.exists("/etc/default/x-ui"):
            try:
                with open("/etc/default/x-ui") as f:
                    for line in f:
                        line = line.strip()
                        if line.startswith("XUI_DB_TYPE="):
                            db_type = line.split("=", 1)[1].strip().strip("\"'")
                        elif line.startswith("XUI_DB_DSN="):
                            dsn = line.split("=", 1)[1].strip().strip("\"'")
            except Exception:
                pass

        if db_type == "postgres" and dsn:
            try:
                import psycopg2
                conn = psycopg2.connect(dsn)
                return "postgres", conn
            except Exception as e:
                log_debug(f"psycopg2 connect failed: {e}")

        db_path = "/etc/x-ui/x-ui.db"
        if os.path.exists(db_path):
            try:
                import sqlite3
                conn = sqlite3.connect(db_path, timeout=5.0)
                return "sqlite", conn
            except Exception as e:
                log_debug(f"sqlite3 connect failed: {e}")

        return None, None

    def query_one(self, sql_pg, sql_lite, params):
        cur = self.conn.cursor()
        try:
            if self.backend == "postgres":
                cur.execute(sql_pg, params)
            else:
                cur.execute(sql_lite, params)
            return cur.fetchone()
        finally:
            cur.close()

    def close(self):
        try:
            self.conn.close()
        except Exception:
            pass

def main():
    if len(sys.argv) < 2:
        log_debug("Rejected: no arguments provided")
        sys.exit(1)

    cred_file = sys.argv[1]
    username = ""
    password = ""
    if os.path.isfile(cred_file):
        try:
            with open(cred_file, "r", encoding="utf-8", errors="ignore") as f:
                lines = f.read().splitlines()
                if len(lines) > 0:
                    username = lines[0].strip()
                if len(lines) > 1:
                    password = lines[1].strip()
        except Exception as e:
            log_debug(f"Failed to read cred_file: {e}")
            sys.exit(1)

    common_name = os.environ.get("common_name", "").strip()
    remote_ip = os.environ.get("untrusted_ip", "").strip()

    # If certificate common_name is present and valid, enforce that username must match common_name
    if common_name and common_name != "UNDEF" and common_name != "server":
        if username and username.lower() != common_name.lower():
            log_debug(f"REJECT: username mismatch! cert cn='{common_name}' != user='{username}' (IP: {remote_ip})")
            sys.exit(1)
        if not username:
            username = common_name

    if not username or username == "server":
        log_debug(f"REJECT: invalid or empty username (cn='{common_name}')")
        sys.exit(1)

    try:
        db = DB()
    except Exception as e:
        log_debug(f"REJECT: Database connection failed: {e}")
        sys.exit(1)

    try:
        row = db.query_one(
            "SELECT password, enable, expiry_time, total_gb, up, down FROM open_vpn_users WHERE LOWER(name) = LOWER(%s);",
            "SELECT password, enable, expiry_time, total_gb, up, down FROM open_vpn_users WHERE LOWER(name) = LOWER(?);",
            (username,)
        )
        db.close()

        if not row:
            log_debug(f"REJECT: User '{username}' NOT found in database")
            sys.exit(1)

        db_password, enable, expiry_time, total_gb, up, down = row

        # 1. Enable check
        if not enable:
            log_debug(f"REJECT: User '{username}' is DISABLED")
            sys.exit(1)

        # 2. Password check:
        # If user has a password in DB, verify exact match
        if db_password and str(db_password).strip() != "":
            if password != str(db_password).strip():
                log_debug(f"REJECT: User '{username}' password mismatch! Provided wrong password from {remote_ip}")
                sys.exit(1)

        # 3. Expiry check
        now_ms = int(time.time() * 1000)
        expiry_val = int(expiry_time or 0)
        if expiry_val > 0 and now_ms >= expiry_val:
            log_debug(f"REJECT: User '{username}' is EXPIRED")
            sys.exit(1)

        # 4. Traffic quota check
        total_used = int(up or 0) + int(down or 0)
        quota_gb = int(total_gb or 0)
        if quota_gb > 0 and total_used >= (quota_gb * 1024 * 1024 * 1024):
            log_debug(f"REJECT: User '{username}' traffic limit exceeded ({total_used} >= {quota_gb}GB)")
            sys.exit(1)

        log_debug(f"ACCEPT: User '{username}' auth verified successfully from {remote_ip}")
        sys.exit(0)

    except Exception as e:
        log_debug(f"ERROR: Exception verifying user '{username}': {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
`

const openvpnAuthPyScript = `#!/usr/bin/env python3
"""
NOVA-X-PANEL OpenVPN Client Connect & Disconnect Hook
Enforces:
  - Account enabled / disabled status
  - Expiry date & traffic quota
  - Mandatory password check if user has password set in DB
  - Concurrent device / IP limit (LimitIP) queried against OpenVPN live management socket
  - Traffic delta accounting on disconnect
Supports both PostgreSQL (XUI_DB_TYPE=postgres) and SQLite (/etc/x-ui/x-ui.db)
"""
import sys
import os
import time
import socket

LOG_PATH = "/var/log/openvpn_auth_exec.log"
MGMT_HOST = "127.0.0.1"
MGMT_PORT = 7505

def log_debug(msg):
    try:
        with open(LOG_PATH, "a") as f:
            f.write(f"{time.strftime('%Y-%m-%d %H:%M:%S')} {msg}\n")
    except Exception:
        pass

def count_live_sessions_mgmt(target_cn, exclude_real_port=None):
    """Query OpenVPN management socket to get live session count for user."""
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.settimeout(1.5)
        s.connect((MGMT_HOST, MGMT_PORT))
        _ = s.recv(512)
        s.sendall(b"status 2\n")
        data = b""
        while True:
            chunk = s.recv(4096)
            if not chunk:
                break
            data += chunk
            if b"END\r\n" in data or b"END\n" in data:
                break
        try:
            s.sendall(b"quit\n")
        except Exception:
            pass
        s.close()

        count = 0
        target = target_cn.lower().strip()
        lines = data.decode("utf-8", errors="ignore").splitlines()
        for line in lines:
            if line.startswith("CLIENT_LIST,"):
                parts = line.split(",")
                if len(parts) >= 3:
                    cn = parts[1].strip().lower()
                    uname = parts[9].strip().lower() if len(parts) >= 10 else ""
                    real_addr = parts[2].strip()
                    port = real_addr.split(":")[-1] if ":" in real_addr else ""
                    if exclude_real_port and port == exclude_real_port:
                        continue
                    if cn == target or (uname and uname == target):
                        count += 1
        return count
    except Exception as e:
        log_debug(f"MGMT query error: {e}")
        return -1

def kill_older_sessions_mgmt(target_cn, exclude_real_port=None):
    """Evict older/stale session for single-device accounts on reconnect."""
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.settimeout(1.5)
        s.connect((MGMT_HOST, MGMT_PORT))
        _ = s.recv(512)
        s.sendall(b"status 2\n")
        data = b""
        while True:
            chunk = s.recv(4096)
            if not chunk:
                break
            data += chunk
            if b"END\r\n" in data or b"END\n" in data:
                break
        target = target_cn.lower().strip()
        lines = data.decode("utf-8", errors="ignore").splitlines()
        for line in lines:
            if line.startswith("CLIENT_LIST,"):
                parts = line.split(",")
                if len(parts) >= 3:
                    cn = parts[1].strip().lower()
                    uname = parts[9].strip().lower() if len(parts) >= 10 else ""
                    real_addr = parts[2].strip()
                    port = real_addr.split(":")[-1] if ":" in real_addr else ""
                    if exclude_real_port and port == exclude_real_port:
                        continue
                    if cn == target or (uname and uname == target):
                        client_id = parts[10].strip() if len(parts) >= 11 else ""
                        if client_id:
                            s.sendall(f"client-kill {client_id}\n".encode())
                        else:
                            s.sendall(f"kill {real_addr}\n".encode())
                        _ = s.recv(256)
        try:
            s.sendall(b"quit\n")
        except Exception:
            pass
        s.close()
    except Exception as e:
        log_debug(f"MGMT kill error: {e}")

class DB:
    def __init__(self):
        self.backend, self.conn = self._connect()
        if not self.conn:
            raise RuntimeError("No database connection available")

    def _connect(self):
        db_type = os.environ.get("XUI_DB_TYPE", "")
        dsn = os.environ.get("XUI_DB_DSN", "")
        if os.path.exists("/etc/default/x-ui"):
            try:
                with open("/etc/default/x-ui") as f:
                    for line in f:
                        line = line.strip()
                        if line.startswith("XUI_DB_TYPE="):
                            db_type = line.split("=", 1)[1].strip().strip("\"'")
                        elif line.startswith("XUI_DB_DSN="):
                            dsn = line.split("=", 1)[1].strip().strip("\"'")
            except Exception:
                pass

        if db_type == "postgres" and dsn:
            try:
                import psycopg2
                conn = psycopg2.connect(dsn)
                return "postgres", conn
            except Exception as e:
                log_debug(f"psycopg2 connect failed: {e}")

        db_path = "/etc/x-ui/x-ui.db"
        if os.path.exists(db_path):
            try:
                import sqlite3
                conn = sqlite3.connect(db_path, timeout=5.0)
                return "sqlite", conn
            except Exception as e:
                log_debug(f"sqlite3 connect failed: {e}")

        return None, None

    def query_one(self, sql_pg, sql_lite, params):
        cur = self.conn.cursor()
        try:
            if self.backend == "postgres":
                cur.execute(sql_pg, params)
            else:
                cur.execute(sql_lite, params)
            return cur.fetchone()
        finally:
            cur.close()

    def query_all(self, sql_pg, sql_lite, params=()):
        cur = self.conn.cursor()
        try:
            if self.backend == "postgres":
                cur.execute(sql_pg, params)
            else:
                cur.execute(sql_lite, params)
            return cur.fetchall()
        finally:
            cur.close()

    def execute(self, sql_pg, sql_lite, params=()):
        cur = self.conn.cursor()
        try:
            if self.backend == "postgres":
                cur.execute(sql_pg, params)
            else:
                cur.execute(sql_lite, params)
            self.conn.commit()
        finally:
            cur.close()

    def close(self):
        try:
            self.conn.close()
        except Exception:
            pass

def main():
    script_type = os.environ.get("script_type", "")
    cn = os.environ.get("common_name", "").strip() or os.environ.get("username", "").strip()
    username_env = os.environ.get("username", "").strip()

    if cn == "server" or not cn or cn == "UNDEF":
        if username_env and username_env != "UNDEF":
            cn = username_env

    t_ip = os.environ.get("trusted_ip", "").strip() or os.environ.get("untrusted_ip", "").strip()
    t_port = os.environ.get("trusted_port", "").strip() or os.environ.get("untrusted_port", "").strip()
    v_ip = os.environ.get("ifconfig_pool_remote_ip", "").strip()
    now_ms = int(time.time() * 1000)

    try:
        db = DB()
    except Exception as e:
        log_debug(f"DB init failed: {e}")
        sys.exit(1)

    # 1. Handle Disconnect
    if script_type == "client-disconnect":
        bytes_rx = os.environ.get("bytes_received")
        bytes_tx = os.environ.get("bytes_sent")
        try:
            if bytes_rx is not None and bytes_tx is not None and cn:
                try:
                    rx = int(bytes_rx)
                    tx = int(bytes_tx)
                    db.execute(
                        "UPDATE open_vpn_users SET up = up + %s, down = down + %s, updated_at = %s WHERE LOWER(name) = LOWER(%s);",
                        "UPDATE open_vpn_users SET up = up + ?, down = down + ?, updated_at = ? WHERE LOWER(name) = LOWER(?);",
                        (rx, tx, now_ms, cn)
                    )
                except Exception as e:
                    log_debug(f"Disconnect accounting error: {e}")
            if t_port:
                db.execute(
                    "DELETE FROM open_vpn_sessions WHERE real_port = %s;",
                    "DELETE FROM open_vpn_sessions WHERE real_port = ?;",
                    (t_port,)
                )
            elif cn:
                db.execute(
                    "DELETE FROM open_vpn_sessions WHERE LOWER(name) = LOWER(%s);",
                    "DELETE FROM open_vpn_sessions WHERE LOWER(name) = LOWER(?);",
                    (cn,)
                )
            db.close()
        except Exception as e:
            log_debug(f"Disconnect DB exception: {e}")
        log_debug(f"[DISCONNECT] cn={cn}, ip={t_ip}:{t_port}, rx={bytes_rx}, tx={bytes_tx}")
        sys.exit(0)

    # 2. Handle Connect
    if script_type == "client-connect" or not script_type:
        try:
            if db.backend == "sqlite":
                db.execute("", '''
                CREATE TABLE IF NOT EXISTS open_vpn_sessions (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    name TEXT NOT NULL,
                    real_ip TEXT NOT NULL,
                    real_port TEXT NOT NULL,
                    virtual_ip TEXT DEFAULT '',
                    last_rx INTEGER DEFAULT 0,
                    last_tx INTEGER DEFAULT 0,
                    connected_at INTEGER NOT NULL,
                    updated_at INTEGER NOT NULL
                );
                ''')

            row = db.query_one(
                "SELECT enable, total_gb, expiry_time, limit_ip, up, down, password FROM open_vpn_users WHERE LOWER(name) = LOWER(%s);",
                "SELECT enable, total_gb, expiry_time, limit_ip, up, down, password FROM open_vpn_users WHERE LOWER(name) = LOWER(?);",
                (cn,)
            )

            if not row:
                db.close()
                log_debug(f"[REJECT-CONNECT] User '{cn}' not in DB")
                sys.exit(1)

            enable, total_gb, expiry_time, limit_ip, up, down, db_password = row

            if not enable:
                db.close()
                log_debug(f"[REJECT-CONNECT] User '{cn}' is disabled")
                sys.exit(1)

            expiry_val = int(expiry_time or 0)
            if expiry_val > 0 and now_ms >= expiry_val:
                db.close()
                log_debug(f"[REJECT-CONNECT] User '{cn}' is expired")
                sys.exit(1)

            total_used = int(up or 0) + int(down or 0)
            quota_gb = int(total_gb or 0)
            if quota_gb > 0 and total_used >= (quota_gb * 1024 * 1024 * 1024):
                db.close()
                log_debug(f"[REJECT-CONNECT] User '{cn}' traffic limit exceeded")
                sys.exit(1)

            # Security check: If user has a password in DB, ensure credentials were provided
            if db_password and str(db_password).strip() != "":
                if not username_env:
                    db.close()
                    log_debug(f"[REJECT-CONNECT] User '{cn}' has password set but client sent no credentials (bypassed auth-user-pass)")
                    sys.exit(1)

            # Check concurrent device / IP limit (LimitIP)
            limit_ip_val = int(limit_ip or 0)
            if limit_ip_val > 0:
                active_count = count_live_sessions_mgmt(cn, exclude_real_port=t_port)
                if active_count < 0:
                    db_rows = db.query_all(
                        "SELECT id, real_port, updated_at FROM open_vpn_sessions WHERE LOWER(name) = LOWER(%s);",
                        "SELECT id, real_port, updated_at FROM open_vpn_sessions WHERE LOWER(name) = LOWER(?);",
                        (cn,)
                    )
                    valid = [r for r in db_rows if (not t_port or str(r[1]) != str(t_port)) and (now_ms - int(r[2] or 0) < 300000)]
                    active_count = len(valid)

                log_debug(f"[LIMIT-CHECK] cn={cn}, active={active_count}, limit={limit_ip_val}, incoming={t_ip}:{t_port}")

                if active_count >= limit_ip_val:
                    if limit_ip_val == 1:
                        log_debug(f"[LIMIT-KILL-OLD] cn={cn} evicting previous session for single-device user")
                        kill_older_sessions_mgmt(cn, exclude_real_port=t_port)
                    else:
                        db.close()
                        log_debug(f"[REJECT-CONNECT] cn={cn} DEVICE LIMIT REACHED! active={active_count} >= limit={limit_ip_val}")
                        sys.exit(1)

            if t_port:
                if db.backend == "postgres":
                    db.execute(
                        "DELETE FROM open_vpn_sessions WHERE real_port = %s;",
                        "",
                        (t_port,)
                    )
                    db.execute(
                        "INSERT INTO open_vpn_sessions (name, real_ip, real_port, virtual_ip, last_rx, last_tx, connected_at, updated_at) VALUES (%s, %s, %s, %s, 0, 0, %s, %s);",
                        "",
                        (cn, t_ip or "", t_port, v_ip or "", now_ms, now_ms)
                    )
                else:
                    db.execute(
                        "",
                        "INSERT OR REPLACE INTO open_vpn_sessions (name, real_ip, real_port, virtual_ip, connected_at, updated_at) VALUES (?, ?, ?, ?, ?, ?);",
                        (cn, t_ip or "", t_port, v_ip or "", now_ms, now_ms)
                    )

            db.close()
            log_debug(f"[ACCEPT-CONNECT] cn={cn}, ip={t_ip}:{t_port}")
            sys.exit(0)

        except Exception as e:
            log_debug(f"[ERROR-CONNECT] Exception: {e}")
            sys.exit(1)

if __name__ == "__main__":
    main()
`

func (s *OpenVPNService) deployAuthHooks() {
	binDir := "/usr/local/x-ui/bin"
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return
	}
	verifyPath := filepath.Join(binDir, "openvpn-verify.py")
	authPath := filepath.Join(binDir, "openvpn-auth.py")

	_ = os.WriteFile(verifyPath, []byte(openvpnVerifyPyScript), 0755)
	_ = os.WriteFile(authPath, []byte(openvpnAuthPyScript), 0755)
}

// EnsureNAT enables IPv4 forwarding and adds iptables MASQUERADE for the VPN subnet,
// while actively removing any dangerous or legacy port redirection rules that conflict with Xray inbounds or Tunnels.
func (s *OpenVPNService) EnsureNAT() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	s.deployAuthHooks()

	script := `
sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1
IFACE=$(ip route show default 2>/dev/null | awk '{print $5}' | head -n1)
[ -z "$IFACE" ] && IFACE="eth0"

# Strictly scoped MASQUERADE only for OpenVPN subnet 10.8.0.0/24 exiting via default WAN interface
iptables -t nat -C POSTROUTING -s 10.8.0.0/24 -o "$IFACE" -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -s 10.8.0.0/24 -o "$IFACE" -j MASQUERADE

# Remove any un-scoped blanket MASQUERADE that could hijack GRE/tunnel interfaces
iptables -t nat -D POSTROUTING -s 10.8.0.0/24 -j MASQUERADE 2>/dev/null || true

iptables -C FORWARD -s 10.8.0.0/24 -j ACCEPT 2>/dev/null || iptables -A FORWARD -s 10.8.0.0/24 -j ACCEPT
iptables -C FORWARD -d 10.8.0.0/24 -m state --state RELATED,ESTABLISHED -j ACCEPT 2>/dev/null || iptables -A FORWARD -d 10.8.0.0/24 -m state --state RELATED,ESTABLISHED -j ACCEPT

if [ -f /etc/default/x-ui ] && grep -q "postgres" /etc/default/x-ui; then
    if ! python3 -c "import psycopg2" >/dev/null 2>&1; then
        apt-get update -qq >/dev/null 2>&1 && apt-get install -y -qq python3-psycopg2 >/dev/null 2>&1 || true
    fi
fi

# Ensure duplicate-cn, client-config-dir, script-security, and auth verification hooks in server.conf
mkdir -p /etc/openvpn/ccd
mkdir -p /usr/local/x-ui/bin
restart_needed=0

# Ensure tls-crypt key exists
if [ ! -f /etc/openvpn/server/tc.key ]; then
    openvpn --genkey secret /etc/openvpn/server/tc.key 2>/dev/null || true
    chmod 600 /etc/openvpn/server/tc.key 2>/dev/null || true
fi

for c in /etc/openvpn/server/server.conf /etc/openvpn/server.conf; do
    if [ -f "$c" ]; then
        if [ -f /etc/openvpn/server/tc.key ] && ! grep -q "tls-crypt" "$c"; then
            echo "tls-crypt /etc/openvpn/server/tc.key" >> "$c"
            restart_needed=1
        fi
        if ! grep -q "tun-mtu" "$c"; then
            echo "tun-mtu 1400" >> "$c"
            restart_needed=1
        fi
        if ! grep -q "mssfix" "$c"; then
            echo "mssfix 1240" >> "$c"
            restart_needed=1
        fi
        if ! grep -q "tcp-nodelay" "$c"; then
            echo "tcp-nodelay" >> "$c"
            restart_needed=1
        fi
        if ! grep -q "sndbuf" "$c"; then
            echo "sndbuf 524288" >> "$c"
            echo "rcvbuf 524288" >> "$c"
            echo 'push "sndbuf 524288"' >> "$c"
            echo 'push "rcvbuf 524288"' >> "$c"
            restart_needed=1
        fi
        if grep -q 'push "mssfix' "$c"; then
            sed -i '/push "mssfix/d' "$c"
            restart_needed=1
        fi
        if grep -q 'keepalive 10 60' "$c"; then
            sed -i 's/keepalive 10 60/keepalive 10 30/' "$c"
            restart_needed=1
        fi
        if ! grep -q "duplicate-cn" "$c"; then
            echo "duplicate-cn" >> "$c"
            restart_needed=1
        fi
        if ! grep -q "client-config-dir" "$c"; then
            echo "client-config-dir /etc/openvpn/ccd" >> "$c"
            restart_needed=1
        fi
        if ! grep -q "script-security" "$c"; then
            echo "script-security 2" >> "$c"
            restart_needed=1
        fi
        if ! grep -q "auth-user-pass-verify" "$c"; then
            echo "auth-user-pass-verify /usr/local/x-ui/bin/openvpn-verify.py via-file" >> "$c"
            restart_needed=1
        fi
        if grep -q "auth-user-pass-optional" "$c"; then
            sed -i '/auth-user-pass-optional/d' "$c"
            restart_needed=1
        fi
        if ! grep -q "client-connect /usr/local/x-ui/bin/openvpn-auth.py" "$c"; then
            echo "client-connect /usr/local/x-ui/bin/openvpn-auth.py" >> "$c"
            restart_needed=1
        fi
        if ! grep -q "client-disconnect /usr/local/x-ui/bin/openvpn-auth.py" "$c"; then
            echo "client-disconnect /usr/local/x-ui/bin/openvpn-auth.py" >> "$c"
            restart_needed=1
        fi
        sed -i '/username-as-common-name/d' "$c"
        sed -i '/tls-verify/d' "$c"
        # Auto-migrate legacy port 2083 to standard 1194
        if grep -qE '^port 2083$' "$c"; then
            sed -i 's/^port 2083$/port 1194/' "$c"
            restart_needed=1
        fi
    fi
done

if [ "$restart_needed" -eq 1 ]; then
    systemctl stop openvpn@server.service 2>/dev/null || true
    systemctl disable openvpn@server.service 2>/dev/null || true
    systemctl restart openvpn-server@server.service 2>/dev/null || systemctl restart openvpn@server.service 2>/dev/null || systemctl restart openvpn 2>/dev/null || true
fi

# Actively purge any legacy or rogue PREROUTING REDIRECT rules targeting OpenVPN so VLESS / Xray ports are never hijacked
iptables -t nat -S PREROUTING 2>/dev/null | grep -E -- '(-j REDIRECT|--to-ports)' | while read -r r; do
    del_r=$(echo "$r" | sed 's/^-A/-D/')
    iptables -t nat $del_r 2>/dev/null || true
done

# Persist firewall rules so reboot doesn't drop MASQUERADE
netfilter-persistent save >/dev/null 2>&1 || true
`
	_, _ = s.execPrivilegedBash(ctx, script)
	_ = s.SyncAllCCD()
}

// GetStatus checks server state, active connections, database records, and issued client certs.
func getOpenVPNSetting(key, fallback string) string {
	db := database.GetDB()
	if db == nil {
		return fallback
	}
	var s model.Setting
	if err := db.Where("key = ?", key).First(&s).Error; err == nil && s.Value != "" {
		return s.Value
	}
	return fallback
}

func setOpenVPNSetting(key, val string) error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	var s model.Setting
	if err := db.Where("key = ?", key).First(&s).Error; err == nil {
		return db.Model(&s).Update("value", val).Error
	}
	return db.Create(&model.Setting{Key: key, Value: val}).Error
}

func detectServerIP() string {
	client := http.Client{Timeout: 2 * time.Second}
	for _, u := range []string{"https://api.ipify.org", "https://ifconfig.me/ip", "https://icanhazip.com"} {
		if resp, err := client.Get(u); err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			ip := strings.TrimSpace(string(b))
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	if conn, err := net.Dial("udp", "1.1.1.1:80"); err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return addr.IP.String()
		}
	}
	return "127.0.0.1"
}

func getIranTunnelIP() string {
	if data, err := os.ReadFile("/etc/x-ui/tunnel.json"); err == nil {
		var tj struct {
			RemotePub string `json:"remote_pub"`
		}
		if err := json.Unmarshal(data, &tj); err == nil && tj.RemotePub != "" {
			return tj.RemotePub
		}
	}
	if data, err := os.ReadFile("/etc/frp/frpc.toml"); err == nil {
		re := regexp.MustCompile(`serverAddr\s*=\s*"([^"]+)"`)
		if m := re.FindStringSubmatch(string(data)); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

func (s *OpenVPNService) GetStatus(scopes ...OpenVPNAccessScope) (*OpenVPNStatus, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	scope := OpenVPNAccessScope{Mode: 2, IsOwner: true}
	if len(scopes) > 0 {
		scope = scopes[0]
	}

	status := &OpenVPNStatus{
		Installed:        false,
		Running:          false,
		Port:             1194,
		Proto:            "tcp",
		Subnet:           "10.8.0.0/24",
		DirectIP:         detectServerIP(),
		IranIP:           getIranTunnelIP(),
		ConnectedClients: []OpenVPNConnectedClient{},
		Clients:          []OpenVPNClientInfo{},
		AdminStats:       []OpenVPNAdminUsage{},
	}

	hasOpenvpn := false
	if _, err := exec.LookPath("openvpn"); err == nil {
		hasOpenvpn = true
	} else if _, err := os.Stat("/usr/sbin/openvpn"); err == nil {
		hasOpenvpn = true
	}

	hasServerConf := false
	if _, err := os.Stat(defaultServerConf); err == nil {
		hasServerConf = true
	} else if _, err := os.Stat("/etc/openvpn/server.conf"); err == nil {
		hasServerConf = true
	}

	hasCerts := false
	if _, err := os.Stat(filepath.Join(defaultServerDir, "ca.crt")); err == nil {
		if _, err2 := os.Stat(filepath.Join(defaultServerDir, "server.crt")); err2 == nil {
			hasCerts = true
		}
	} else if _, err := os.Stat(filepath.Join(defaultEasyRsaDir, "pki", "ca.crt")); err == nil {
		hasCerts = true
	}

	status.Installed = hasOpenvpn && hasServerConf && hasCerts

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "systemctl", "is-active", openvpnUnitName).Output()
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		status.Running = true
	} else {
		outAlt, errAlt := exec.CommandContext(ctx, "systemctl", "is-active", "openvpn@server.service").Output()
		if errAlt == nil && strings.TrimSpace(string(outAlt)) == "active" {
			status.Running = true
		}
	}

	confPath := defaultServerConf
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		confPath = "/etc/openvpn/server.conf"
	}
	if data, err := os.ReadFile(confPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "port ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					if p, err := strconv.Atoi(parts[1]); err == nil {
						status.Port = p
					}
				}
			} else if strings.HasPrefix(line, "proto ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					status.Proto = strings.TrimPrefix(parts[1], "tcp-server")
					if status.Proto == "" {
						status.Proto = "tcp"
					}
				}
			}
		}
	}

	savedHost := getOpenVPNSetting("openvpn_remote_host", "")
	savedPortStr := getOpenVPNSetting("openvpn_remote_port", "")
	savedPort, _ := strconv.Atoi(savedPortStr)

	if savedHost == "" {
		if status.IranIP != "" {
			savedHost = status.IranIP
		} else {
			savedHost = status.DirectIP
		}
		_ = setOpenVPNSetting("openvpn_remote_host", savedHost)
	}

	if savedPort <= 0 {
		savedPort = status.Port
		if savedPort <= 0 {
			savedPort = 1194
		}
		_ = setOpenVPNSetting("openvpn_remote_port", strconv.Itoa(savedPort))
	}

	status.RemoteHost = savedHost
	status.RemotePort = savedPort

	// Read live sessions via management socket and sync deltas to DB
	liveSessions, _ := s.QueryManagementSessions()
	_ = s.ApplyLiveSessionDeltas(liveSessions)

	var sessionsByCN = make(map[string][]OpenVPNLiveSession)
	for _, sess := range liveSessions {
		key := strings.ToLower(strings.TrimSpace(sess.CommonName))
		if key != "" {
			sessionsByCN[key] = append(sessionsByCN[key], sess)
		}
		status.ConnectedClients = append(status.ConnectedClients, OpenVPNConnectedClient{
			CommonName:     sess.CommonName,
			RealAddress:    sess.RealAddress,
			BytesReceived:  FormatBytes(sess.BytesReceived),
			BytesSent:      FormatBytes(sess.BytesSent),
			ConnectedSince: sess.ConnectedSince,
		})
	}
	status.OnlineClients = len(sessionsByCN)

	// Fetch DB records
	db := database.GetDB()
	var dbUsers []*model.OpenVPNUser
	_ = db.Find(&dbUsers).Error
	var userMap = make(map[string]*model.OpenVPNUser)
	for _, u := range dbUsers {
		userMap[u.Name] = u
	}

	// Also discover any physical certs created via easyrsa that are not yet in DB
	issuedPattern := filepath.Join(defaultEasyRsaDir, "pki", "issued", "*.crt")
	certs, _ := filepath.Glob(issuedPattern)
	for _, crt := range certs {
		name := strings.TrimSuffix(filepath.Base(crt), ".crt")
		if name == "server" {
			continue
		}
		if _, exists := userMap[name]; !exists {
			newUser := &model.OpenVPNUser{
				Name:       name,
				Enable:     true,
				TotalGB:    0,
				ExpiryTime: 0,
				LimitIP:    0,
			}
			_ = db.Create(newUser).Error
			userMap[name] = newUser
			dbUsers = append(dbUsers, newUser)
		}
	}

	nowMs := time.Now().UnixMilli()

	adminNameMap := make(map[int]string)
	adminRoleMap := make(map[int]string)
	var allAdmins []model.User
	_ = db.Find(&allAdmins).Error
	var allRoles []model.AdminRole
	_ = db.Find(&allRoles).Error
	roleNameMap := make(map[int]string)
	for _, r := range allRoles {
		roleNameMap[r.Id] = r.Name
	}
	adminLimitExceededMap := make(map[int]bool)
	for _, a := range allAdmins {
		adminNameMap[a.Id] = a.Username
		adminRoleMap[a.Id] = roleNameMap[a.RoleId]
		if a.DataLimit > 0 && a.UsedBytes >= a.DataLimit {
			adminLimitExceededMap[a.Id] = true
		}
	}

	type adminUsageAgg struct {
		clientCount int
		onlineCount int
		totalUp     int64
		totalDown   int64
	}
	adminAggs := make(map[int]*adminUsageAgg)
	for _, a := range allAdmins {
		adminAggs[a.Id] = &adminUsageAgg{}
	}

	// Build full client info list
	for _, u := range dbUsers {
		createdAt := ""
		crtFile := filepath.Join(defaultEasyRsaDir, "pki", "issued", u.Name+".crt")
		if info, err := os.Stat(crtFile); err == nil {
			createdAt = info.ModTime().Format("2006-01-02 15:04")
		} else if u.CreatedAt > 0 {
			createdAt = time.UnixMilli(u.CreatedAt).Format("2006-01-02 15:04")
		}

		nameKey := strings.ToLower(strings.TrimSpace(u.Name))
		sessList := sessionsByCN[nameKey]
		isConnected := len(sessList) > 0
		var connectedIPs []string
		since := ""
		for _, sess := range sessList {
			connectedIPs = append(connectedIPs, sess.RealAddress)
			if since == "" {
				since = sess.ConnectedSince
			}
		}

		totalUsed := u.Up + u.Down
		isExpired := u.ExpiryTime > 0 && nowMs >= u.ExpiryTime
		isTrafficExceeded := (u.TotalGB > 0 && totalUsed >= (u.TotalGB*1024*1024*1024)) ||
			(u.OwnerAdminId > 0 && adminLimitExceededMap[u.OwnerAdminId])

		limitStr := "نامحدود"
		var percent float64 = 0
		if u.TotalGB > 0 {
			limitStr = fmt.Sprintf("%d GB", u.TotalGB)
			percent = (float64(totalUsed) / float64(u.TotalGB*1024*1024*1024)) * 100
			if percent > 100 {
				percent = 100
			}
		}

		// Update admin stats aggregator
		agg, exists := adminAggs[u.OwnerAdminId]
		if !exists {
			agg = &adminUsageAgg{}
			adminAggs[u.OwnerAdminId] = agg
		}
		agg.clientCount++
		if isConnected {
			agg.onlineCount++
		}
		agg.totalUp += u.Up
		agg.totalDown += u.Down

		// Scope check: an admin with ScopeOwn only sees their own clients
		if !scope.IsOwner && scope.Mode != 2 {
			if scope.Mode == 0 || u.OwnerAdminId != scope.AdminID {
				continue
			}
		}

		clientInfo := OpenVPNClientInfo{
			Name:               u.Name,
			Password:           u.Password,
			Enable:             u.Enable,
			TotalGB:            u.TotalGB,
			ExpiryTime:         u.ExpiryTime,
			LimitIP:            u.LimitIP,
			Up:                 u.Up,
			Down:               u.Down,
			TotalUsed:          totalUsed,
			TrafficUsedStr:     FormatBytes(totalUsed),
			TrafficLimitStr:    limitStr,
			TrafficPercent:     percent,
			CreatedAt:          createdAt,
			IsConnected:        isConnected,
			ConnectedIPs:       connectedIPs,
			CurrentConnections: len(connectedIPs),
			Since:              since,
			IsExpired:          isExpired,
			IsTrafficExceeded:  isTrafficExceeded,
			OwnerAdminId:       u.OwnerAdminId,
			OwnerAdminUsername: adminNameMap[u.OwnerAdminId],
		}

		status.Clients = append(status.Clients, clientInfo)
	}

	// Populate AdminStats
	for _, a := range allAdmins {
		if !scope.IsOwner && scope.Mode != 2 && a.Id != scope.AdminID {
			continue
		}
		agg := adminAggs[a.Id]
		if agg == nil {
			agg = &adminUsageAgg{}
		}
		totalUsed := agg.totalUp + agg.totalDown
		limitStr := "نامحدود"
		var percent float64 = 0
		if a.DataLimit > 0 {
			limitStr = FormatBytes(a.DataLimit)
			percent = (float64(a.UsedBytes) / float64(a.DataLimit)) * 100
			if percent > 100 {
				percent = 100
			}
		}

		status.AdminStats = append(status.AdminStats, OpenVPNAdminUsage{
			AdminID:        a.Id,
			Username:       a.Username,
			RoleName:       adminRoleMap[a.Id],
			ClientCount:    agg.clientCount,
			OnlineCount:    agg.onlineCount,
			TotalUp:        agg.totalUp,
			TotalDown:      agg.totalDown,
			TotalUsed:      totalUsed,
			TrafficUsedStr: FormatBytes(totalUsed),
			DataLimit:      a.DataLimit,
			DataLimitStr:   limitStr,
			UsagePercent:   percent,
			IsLimited:      a.DataLimit > 0 && a.UsedBytes >= a.DataLimit,
			Status:         a.Status,
		})
	}

	onlinePermitted := 0
	var overallUp int64
	var overallDown int64
	for _, c := range status.Clients {
		if c.IsConnected {
			onlinePermitted++
		}
		overallUp += c.Up
		overallDown += c.Down
	}
	overallUsed := overallUp + overallDown
	status.TotalClients = len(status.Clients)
	status.OnlineClients = onlinePermitted
	status.TotalUp = overallUp
	status.TotalDown = overallDown
	status.TotalUsed = overallUsed
	status.TotalUsedStr = FormatBytes(overallUsed)
	status.TotalUpStr = FormatBytes(overallUp)
	status.TotalDownStr = FormatBytes(overallDown)
	return status, nil
}

// CanManageClient verifies if the acting scope permits mutating/viewing this client.
func (s *OpenVPNService) CanManageClient(scope OpenVPNAccessScope, name string) error {
	if scope.IsOwner || scope.Mode == 2 {
		return nil
	}
	if scope.Mode == 0 {
		return errors.New("شما اجازه دسترسی به کلاینت‌های OpenVPN را ندارید")
	}
	db := database.GetDB()
	var user model.OpenVPNUser
	if err := db.Where("name = ?", name).First(&user).Error; err != nil {
		return fmt.Errorf("کلاینت '%s' یافت نشد", name)
	}
	if user.OwnerAdminId != scope.AdminID {
		return errors.New("شما فقط مجاز به مدیریت کلاینت‌های اختصاصی خود هستید")
	}
	return nil
}

// AddClient builds a new client certificate using Easy-RSA and saves limits to DB.
func (s *OpenVPNService) AddClient(name string, password string, totalGB int64, expiryTime int64, limitIP int, ownerAdminId ...int) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	name = strings.TrimSpace(name)
	password = strings.TrimSpace(password)
	if !validClientNameRegexp.MatchString(name) {
		return fmt.Errorf("invalid client name '%s' (only letters, numbers, dash and underscore allowed, max 32 chars)", name)
	}
	if name == "server" {
		return fmt.Errorf("client name 'server' is reserved")
	}

	db := database.GetDB()
	var count int64
	db.Model(&model.OpenVPNUser{}).Where("name = ?", name).Count(&count)
	if count > 0 {
		return fmt.Errorf("client '%s' already exists in database", name)
	}

	if err := s.ensureEasyRSALocked(); err != nil {
		return fmt.Errorf("failed to initialize openvpn easy-rsa environment: %w", err)
	}

	crtFile := filepath.Join(defaultEasyRsaDir, "pki", "issued", name+".crt")
	if _, err := os.Stat(crtFile); err == nil {
		return fmt.Errorf("client '%s' already exists in certificates", name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := fmt.Sprintf("cd %s && ./easyrsa --batch build-client-full %s nopass", defaultEasyRsaDir, name)
	out, err := s.execPrivilegedBash(ctx, cmd)
	if err != nil || func() bool { _, e := os.Stat(crtFile); return e != nil }() {
		opensslCmd := fmt.Sprintf(`
mkdir -p /etc/openvpn/easy-rsa/pki/reqs /etc/openvpn/easy-rsa/pki/issued /etc/openvpn/easy-rsa/pki/private /etc/openvpn/easy-rsa/pki/inline
[ -f /etc/openvpn/easy-rsa/vars ] || touch /etc/openvpn/easy-rsa/vars
openssl req -new -nodes -newkey rsa:2048 \
    -keyout /etc/openvpn/easy-rsa/pki/private/%s.key \
    -out /tmp/%s.csr -subj "/CN=%s" >/dev/null 2>&1
cat << 'EOF_EXT' > /tmp/%s.ext
basicConstraints = CA:FALSE
keyUsage = digitalSignature
extendedKeyUsage = clientAuth
EOF_EXT
openssl x509 -req -in /tmp/%s.csr \
    -CA /etc/openvpn/server/ca.crt -CAkey /etc/openvpn/server/ca.key \
    -CAcreateserial -out /etc/openvpn/easy-rsa/pki/issued/%s.crt -days 3650 -extfile /tmp/%s.ext >/dev/null 2>&1
rm -f /tmp/%s.csr /tmp/%s.ext
`, name, name, name, name, name, name, name, name, name)
		if outOss, errOss := s.execPrivilegedBash(ctx, opensslCmd); errOss != nil {
			return fmt.Errorf("failed to create client: %v (output: %s / openssl: %s)", err, string(out), string(outOss))
		}
	}

	uOwner := 0
	if len(ownerAdminId) > 0 {
		uOwner = ownerAdminId[0]
	}
	if uOwner > 0 {
		var adminUser model.User
		if err := db.Where("id = ?", uOwner).First(&adminUser).Error; err == nil {
			if adminUser.DataLimit > 0 && adminUser.UsedBytes >= adminUser.DataLimit {
				return errors.New("سهمیه ترافیک مجاز این ادمین به پایان رسیده است و امکان ساخت کلاینت جدید وجود ندارد")
			}
		}
	}

	if _, err := os.Stat(crtFile); err != nil {
		return fmt.Errorf("certificate was not generated: %v", err)
	}

	user := &model.OpenVPNUser{
		Name:         name,
		Password:     password,
		Enable:       true,
		TotalGB:      totalGB,
		ExpiryTime:   expiryTime,
		LimitIP:      limitIP,
		OwnerAdminId: uOwner,
	}
	if err := db.Create(user).Error; err != nil {
		return fmt.Errorf("client cert created but failed to save to database: %w", err)
	}

	return nil
}

// UpdateClient updates limits and enable status of an OpenVPN client.
func (s *OpenVPNService) UpdateClient(name string, password string, totalGB int64, expiryTime int64, limitIP int, enable bool) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	name = strings.TrimSpace(name)
	password = strings.TrimSpace(password)
	db := database.GetDB()
	var user model.OpenVPNUser
	if err := db.Where("name = ?", name).First(&user).Error; err != nil {
		return fmt.Errorf("client '%s' not found", name)
	}

	if password != "" {
		user.Password = password
	}

	if enable && user.OwnerAdminId > 0 {
		var adminUser model.User
		if err := db.Where("id = ?", user.OwnerAdminId).First(&adminUser).Error; err == nil {
			if adminUser.DataLimit > 0 && adminUser.UsedBytes >= adminUser.DataLimit {
				return errors.New("سهمیه ترافیک مجاز این ادمین به پایان رسیده است و امکان فعال‌سازی کلاینت وجود ندارد")
			}
		}
	}

	// Commit any pending session bytes before updating limits or kicking
	_ = s.SyncTraffic()
	_ = db.Where("name = ?", name).First(&user)

	user.TotalGB = totalGB
	user.ExpiryTime = expiryTime
	user.LimitIP = limitIP
	user.Enable = enable

	if err := db.Save(&user).Error; err != nil {
		return err
	}

	nowMs := time.Now().UnixMilli()
	totalUsed := user.Up + user.Down
	isExpired := user.ExpiryTime > 0 && nowMs >= user.ExpiryTime
	isTrafficExceeded := user.TotalGB > 0 && totalUsed >= (user.TotalGB*1024*1024*1024)

	shouldBlock := !user.Enable || isExpired || isTrafficExceeded
	_ = setClientBlocked(name, shouldBlock)

	// If disabled, expired, or over traffic, kick active session immediately
	if shouldBlock {
		_ = s.KillClient(name)
	} else if user.LimitIP > 0 {
		// Enforce concurrent limit immediately on active sessions
		if sessions, err := s.QueryManagementSessions(); err == nil {
			var matching []OpenVPNLiveSession
			for _, sess := range sessions {
				if strings.EqualFold(sess.CommonName, name) {
					matching = append(matching, sess)
				}
			}
			if len(matching) > user.LimitIP {
				conn, dialErr := net.DialTimeout("tcp", getMgmtAddress(), 1*time.Second)
				if dialErr == nil {
					for i := user.LimitIP; i < len(matching); i++ {
						if matching[i].RealAddress != "" {
							_, _ = fmt.Fprintf(conn, "kill %s\n", matching[i].RealAddress)
						}
					}
					_ = conn.Close()
				}
			}
		}
	}

	return nil
}

// ResetClientTraffic resets cumulative traffic (up & down) for a client.
func (s *OpenVPNService) ResetClientTraffic(name string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	name = strings.TrimSpace(name)
	db := database.GetDB()

	_ = s.SyncTraffic()

	if err := db.Model(&model.OpenVPNUser{}).Where("name = ?", name).Updates(map[string]any{
		"up":   0,
		"down": 0,
	}).Error; err != nil {
		return err
	}

	if sessions, err := s.QueryManagementSessions(); err == nil {
		for _, sess := range sessions {
			if strings.EqualFold(sess.CommonName, name) {
				realAddr := strings.TrimSpace(sess.RealAddress)
				realIp := realAddr
				realPort := ""
				if idx := strings.LastIndex(realAddr, ":"); idx != -1 {
					realIp = realAddr[:idx]
					realPort = realAddr[idx+1:]
				}
				_ = db.Model(&model.OpenVPNSession{}).
					Where("LOWER(name) = ? AND real_ip = ? AND real_port = ?", strings.ToLower(name), realIp, realPort).
					Updates(map[string]any{
						"last_rx": sess.BytesReceived,
						"last_tx": sess.BytesSent,
					}).Error
			}
		}
	}

	var u model.OpenVPNUser
	if err := db.Where("name = ?", name).First(&u).Error; err == nil {
		nowMs := time.Now().UnixMilli()
		isExpired := u.ExpiryTime > 0 && nowMs >= u.ExpiryTime
		if u.Enable && !isExpired {
			_ = setClientBlocked(name, false)
		}
	}

	return nil
}

// DeleteClient revokes and deletes a client certificate and removes it from DB.
func (s *OpenVPNService) DeleteClient(name string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	name = strings.TrimSpace(name)
	if !validClientNameRegexp.MatchString(name) || name == "server" {
		return fmt.Errorf("invalid client name")
	}

	// Kick active connection
	_ = s.KillClient(name)
	_ = setClientBlocked(name, false)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := fmt.Sprintf("mkdir -p %s/pki/reqs %s/pki/inline && [ -f %s/vars ] || touch %s/vars && cd %s && (./easyrsa --batch revoke %s >/dev/null 2>&1 || true) && rm -f pki/issued/%s.crt pki/private/%s.key pki/reqs/%s.req pki/inline/%s.inline",
		defaultEasyRsaDir, defaultEasyRsaDir, defaultEasyRsaDir, defaultEasyRsaDir, defaultEasyRsaDir, name, name, name, name, name)
	_, _ = s.execPrivilegedBash(ctx, cmd)

	db := database.GetDB()
	_ = db.Where("LOWER(name) = ?", strings.ToLower(name)).Delete(&model.OpenVPNSession{}).Error
	if err := db.Where("name = ?", name).Delete(&model.OpenVPNUser{}).Error; err != nil {
		return fmt.Errorf("failed to delete client from database: %w", err)
	}

	return nil
}

// GetClientConfig returns the complete .ovpn profile content for the client.
func (s *OpenVPNService) GetClientConfig(name string, remoteIP string, remotePort int, inlineAuth bool) (string, error) {
	name = strings.TrimSpace(name)
	if !validClientNameRegexp.MatchString(name) || name == "server" {
		return "", fmt.Errorf("invalid client name")
	}

	db := database.GetDB()
	var user model.OpenVPNUser
	_ = db.Where("name = ?", name).First(&user)

	if remoteIP == "" {
		remoteIP = getOpenVPNSetting("openvpn_remote_host", detectServerIP())
	}
	if remotePort <= 0 {
		savedPort, _ := strconv.Atoi(getOpenVPNSetting("openvpn_remote_port", "1194"))
		if savedPort <= 0 {
			savedPort = 1194
		}
		remotePort = savedPort
	}

	caPath := filepath.Join(defaultServerDir, "ca.crt")
	if _, err := os.Stat(caPath); os.IsNotExist(err) {
		caPath = filepath.Join(defaultEasyRsaDir, "pki", "ca.crt")
	}
	crtPath := filepath.Join(defaultEasyRsaDir, "pki", "issued", name+".crt")
	keyPath := filepath.Join(defaultEasyRsaDir, "pki", "private", name+".key")

	caData, err := os.ReadFile(caPath)
	if err != nil {
		return "", fmt.Errorf("failed to read ca.crt: %w", err)
	}
	crtRaw, err := os.ReadFile(crtPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s.crt: %w", name, err)
	}
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s.key: %w", name, err)
	}

	certRegex := regexp.MustCompile(`(?s)-----BEGIN CERTIFICATE-----.*?-----END CERTIFICATE-----`)
	matches := certRegex.FindAllString(string(crtRaw), -1)
	cleanCert := string(crtRaw)
	if len(matches) > 0 {
		cleanCert = matches[0]
	}

	authPassBlock := ""
	pwd := user.Password
	if pwd == "" {
		pwd = "nopassword"
	}
	authPassBlock = "auth-user-pass\n"
	if inlineAuth {
		authPassBlock = fmt.Sprintf("auth-user-pass\n<auth-user-pass>\n%s\n%s\n</auth-user-pass>\n", user.Name, pwd)
	}

	proto := getOpenVPNSetting("openvpn_proto", "tcp")
	if proto != "udp" && proto != "tcp" {
		proto = "tcp"
	}

	// Read optional tls-crypt key to obfuscate DPI handshakes
	tlsCryptBlock := ""
	for _, tcPath := range []string{"/etc/openvpn/server/tc.key", "/etc/openvpn/tc.key"} {
		if tcData, err := os.ReadFile(tcPath); err == nil && len(tcData) > 0 {
			tlsCryptBlock = fmt.Sprintf("\n<tls-crypt>\n%s\n</tls-crypt>", strings.TrimSpace(string(tcData)))
			break
		}
	}

	ovpn := fmt.Sprintf(`client
dev tun
proto %s
remote %s %d
resolv-retry infinite
nobind
persist-key
persist-tun
;remote-cert-tls server
tun-mtu 1400
mssfix 1240
tcp-nodelay
sndbuf 524288
rcvbuf 524288
auth-nocache
block-outside-dns
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
</key>%s
`, proto, remoteIP, remotePort, authPassBlock, strings.TrimSpace(string(caData)), strings.TrimSpace(cleanCert), strings.TrimSpace(string(keyData)), tlsCryptBlock)

	return ovpn, nil
}

// RestartService restarts OpenVPN and ensures firewall rules.
func (s *OpenVPNService) RestartService() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Ensure certificates and server.conf exist BEFORE attempting restart
	if err := s.ensureEasyRSALocked(); err != nil {
		return fmt.Errorf("failed to prepare OpenVPN configuration: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	s.EnsureNAT()

	_, _ = s.execPrivileged(ctx, "systemctl", "stop", "openvpn@server.service")
	_, _ = s.execPrivileged(ctx, "systemctl", "disable", "openvpn@server.service")
	_, err := s.execPrivileged(ctx, "systemctl", "restart", openvpnUnitName)
	if err != nil {
		_, errAlt := s.execPrivileged(ctx, "systemctl", "restart", "openvpn@server.service")
		if errAlt != nil {
			diagCmd := "cat /var/log/openvpn/openvpn.log 2>&1 | tail -n 12 || journalctl -u openvpn-server@server -u openvpn@server -n 12 --no-pager 2>&1"
			diagOut, _ := s.execPrivilegedBash(ctx, diagCmd)
			return fmt.Errorf("restart failed: %v\nDiagnostic logs:\n%s", errAlt, strings.TrimSpace(string(diagOut)))
		}
	}
	return nil
}

// SaveSettings permanently stores endpoint and core port settings in DB and updates server.conf if port changed.
func (s *OpenVPNService) SaveSettings(remoteHost string, remotePort int, serverPort int) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if serverPort > 0 && serverPort <= 65535 {
		if inUse, reason := isPortInUseByInbounds(serverPort); inUse {
			return errors.New(reason)
		}
	}

	remoteHost = strings.TrimSpace(remoteHost)
	if remoteHost != "" {
		if err := setOpenVPNSetting("openvpn_remote_host", remoteHost); err != nil {
			return fmt.Errorf("failed to save remote host: %w", err)
		}
	}

	if remotePort > 0 && remotePort <= 65535 {
		if err := setOpenVPNSetting("openvpn_remote_port", strconv.Itoa(remotePort)); err != nil {
			return fmt.Errorf("failed to save remote port: %w", err)
		}
	}

	if serverPort > 0 && serverPort <= 65535 {
		_ = setOpenVPNSetting("openvpn_server_port", strconv.Itoa(serverPort))
		for _, confPath := range []string{"/etc/openvpn/server/server.conf", "/etc/openvpn/server.conf"} {
			if data, err := os.ReadFile(confPath); err == nil {
				content := string(data)
				re := regexp.MustCompile(`(?m)^port\s+\d+`)
				if re.MatchString(content) {
					newContent := re.ReplaceAllString(content, fmt.Sprintf("port %d", serverPort))
					_ = os.WriteFile(confPath, []byte(newContent), 0644)
				}
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		s.EnsureNAT()
		_, _ = s.execPrivileged(ctx, "systemctl", "stop", "openvpn@server.service")
		_, _ = s.execPrivileged(ctx, "systemctl", "disable", "openvpn@server.service")
		_, _ = s.execPrivileged(ctx, "systemctl", "restart", openvpnUnitName)
	}

	// Always refresh NAT & purge legacy redirects whenever settings are saved
	s.EnsureNAT()

	return nil
}

func (s *OpenVPNService) ensureEasyRSALocked() error {
	serverConf := defaultServerConf
	caCrt := filepath.Join(defaultServerDir, "ca.crt")
	serverCrt := filepath.Join(defaultServerDir, "server.crt")
	serverKey := filepath.Join(defaultServerDir, "server.key")

	if _, err1 := os.Stat(serverConf); err1 == nil {
		if _, err2 := os.Stat(caCrt); err2 == nil {
			if _, err3 := os.Stat(serverCrt); err3 == nil {
				if _, err4 := os.Stat(serverKey); err4 == nil {
					return nil
				}
			}
		}
	}

	savedPort, _ := strconv.Atoi(getOpenVPNSetting("openvpn_server_port", "1194"))
	if savedPort <= 0 {
		savedPort = 1194
	}
	return s.autoInstallOpenVPNInternal(savedPort, "tcp")
}

// AutoInstallOpenVPN performs a one-click automated installation of OpenVPN and Easy-RSA on fresh servers.
func (s *OpenVPNService) AutoInstallOpenVPN(port int, proto string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.autoInstallOpenVPNInternal(port, proto)
}

func (s *OpenVPNService) autoInstallOpenVPNInternal(port int, proto string) error {
	if port <= 0 || port > 65535 {
		port = 1194
	}
	if inUse, reason := isPortInUseByInbounds(port); inUse {
		return errors.New(reason)
	}
	if proto == "" {
		proto = "tcp"
	}

	directIP := detectServerIP()
	iranIP := getIranTunnelIP()
	bypassRoutes := ""
	if directIP != "" && directIP != "127.0.0.1" {
		bypassRoutes += fmt.Sprintf("push \"route %s 255.255.255.255 net_gateway\"\n", directIP)
	}
	if iranIP != "" && iranIP != directIP {
		bypassRoutes += fmt.Sprintf("push \"route %s 255.255.255.255 net_gateway\"\n", iranIP)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	script := fmt.Sprintf(`
export DEBIAN_FRONTEND=noninteractive

# 1. Install required packages if missing
if ! command -v openvpn >/dev/null 2>&1 || ! command -v openssl >/dev/null 2>&1; then
    if command -v apt-get >/dev/null 2>&1; then
        apt-get update -y || true
        apt-get install -y openvpn easy-rsa openssl iptables || true
    elif command -v dnf >/dev/null 2>&1; then
        dnf install -y epel-release || true
        dnf install -y openvpn easy-rsa openssl iptables || true
    elif command -v yum >/dev/null 2>&1; then
        yum install -y epel-release || true
        yum install -y openvpn easy-rsa openssl iptables || true
    elif command -v pacman >/dev/null 2>&1; then
        pacman -Sy --noconfirm openvpn easy-rsa openssl iptables || true
    fi
fi

# 2. Ensure system folders & TUN device
mkdir -p /etc/openvpn/server /etc/openvpn/easy-rsa/pki/reqs /etc/openvpn/easy-rsa/pki/issued /etc/openvpn/easy-rsa/pki/private /etc/openvpn/easy-rsa/pki/inline /run/openvpn /run/openvpn-server /var/log/openvpn
[ -f /etc/openvpn/easy-rsa/vars ] || touch /etc/openvpn/easy-rsa/vars
if [ ! -c /dev/net/tun ]; then
    mkdir -p /dev/net
    mknod /dev/net/tun c 10 200 2>/dev/null || true
    chmod 666 /dev/net/tun 2>/dev/null || true
fi

# 3. Setup easy-rsa folder if available
if [ ! -f "/etc/openvpn/easy-rsa/easyrsa" ]; then
    if [ -d "/usr/share/easy-rsa" ]; then
        cp -rf /usr/share/easy-rsa/* /etc/openvpn/easy-rsa/ 2>/dev/null || true
    elif [ -d "/usr/local/share/easy-rsa" ]; then
        cp -rf /usr/local/share/easy-rsa/* /etc/openvpn/easy-rsa/ 2>/dev/null || true
    fi
fi

# 4. Generate CA + Server Cert using OpenSSL (instant, bulletproof, zero prompts)
if [ ! -f "/etc/openvpn/server/ca.crt" ] || [ ! -f "/etc/openvpn/server/server.crt" ] || [ ! -f "/etc/openvpn/server/server.key" ]; then
    openssl req -new -x509 -days 3650 -nodes -newkey rsa:2048 \
        -keyout /etc/openvpn/server/ca.key -out /etc/openvpn/server/ca.crt \
        -subj "/CN=OpenVPN-CA" >/dev/null 2>&1
    openssl req -new -nodes -newkey rsa:2048 \
        -keyout /etc/openvpn/server/server.key -out /tmp/ovpn_server.csr \
        -subj "/CN=server" >/dev/null 2>&1
    cat << 'EOF_EXT' > /tmp/ovpn_server.ext
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = DNS:server
EOF_EXT
    openssl x509 -req -in /tmp/ovpn_server.csr -CA /etc/openvpn/server/ca.crt -CAkey /etc/openvpn/server/ca.key \
        -CAcreateserial -out /etc/openvpn/server/server.crt -days 3650 -extfile /tmp/ovpn_server.ext >/dev/null 2>&1
    rm -f /tmp/ovpn_server.csr /tmp/ovpn_server.ext
fi

# Sync certs into Easy-RSA PKI directory so client generation and verify work
mkdir -p /etc/openvpn/easy-rsa/pki/issued /etc/openvpn/easy-rsa/pki/private
cp -f /etc/openvpn/server/ca.crt /etc/openvpn/easy-rsa/pki/ca.crt 2>/dev/null || true
cp -f /etc/openvpn/server/ca.key /etc/openvpn/easy-rsa/pki/private/ca.key 2>/dev/null || true
cp -f /etc/openvpn/server/server.crt /etc/openvpn/easy-rsa/pki/issued/server.crt 2>/dev/null || true
cp -f /etc/openvpn/server/server.key /etc/openvpn/easy-rsa/pki/private/server.key 2>/dev/null || true

# Ensure tls-crypt key exists
if [ ! -f /etc/openvpn/server/tc.key ]; then
    openvpn --genkey secret /etc/openvpn/server/tc.key 2>/dev/null || true
    chmod 600 /etc/openvpn/server/tc.key 2>/dev/null || true
fi

# 5. Write server.conf
cat << 'EOF_CONF' > /etc/openvpn/server/server.conf
port %d
proto %s
dev tun
topology subnet

ca /etc/openvpn/server/ca.crt
cert /etc/openvpn/server/server.crt
key /etc/openvpn/server/server.key
dh none
tls-crypt /etc/openvpn/server/tc.key

server 10.8.0.0 255.255.255.0
ifconfig-pool-persist /etc/openvpn/server/ipp.txt

push "redirect-gateway def1 bypass-dhcp"
%spush "dhcp-option DNS 1.1.1.1"
push "dhcp-option DNS 8.8.8.8"

keepalive 10 30
data-ciphers AES-256-GCM:AES-128-GCM:AES-256-CBC:AES-128-CBC
cipher AES-256-GCM
auth SHA256

tun-mtu 1400
mssfix 1240
tcp-nodelay
sndbuf 524288
rcvbuf 524288
push "sndbuf 524288"
push "rcvbuf 524288"

persist-key
persist-tun
status /var/log/openvpn/openvpn-status.log 10
status-version 2
log-append /var/log/openvpn/openvpn.log
verb 3
management 127.0.0.1 7505
duplicate-cn
client-config-dir /etc/openvpn/ccd
script-security 2
auth-user-pass-verify /usr/local/x-ui/bin/openvpn-verify.py via-file
client-connect /usr/local/x-ui/bin/openvpn-auth.py
client-disconnect /usr/local/x-ui/bin/openvpn-auth.py
EOF_CONF

cp -f /etc/openvpn/server/server.conf /etc/openvpn/server.conf 2>/dev/null || true

# 6. Enable routing & systemd service
sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1 || true
sed -i '/net.ipv4.ip_forward/d' /etc/sysctl.conf 2>/dev/null || true
echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf 2>/dev/null || true

systemctl daemon-reload >/dev/null 2>&1 || true
systemctl stop openvpn@server >/dev/null 2>&1 || true
systemctl disable openvpn@server >/dev/null 2>&1 || true
systemctl enable openvpn-server@server >/dev/null 2>&1 || true
systemctl restart openvpn-server@server >/dev/null 2>&1 || true
`, port, proto, bypassRoutes)

	_, err := s.execPrivilegedBash(ctx, script)
	if err != nil {
		diagCmd := "journalctl -u openvpn-server@server -u openvpn@server -n 12 --no-pager 2>&1 || cat /var/log/openvpn/openvpn.log 2>&1 | tail -n 12"
		diagOut, _ := s.execPrivilegedBash(ctx, diagCmd)
		return fmt.Errorf("auto-install failed: %v (diag: %s)", err, strings.TrimSpace(string(diagOut)))
	}

	s.EnsureNAT()
	return nil
}

// GetLog returns recent journal or log file records for the OpenVPN service.
func (s *OpenVPNService) GetLog() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := "cat /var/log/openvpn/openvpn.log 2>/dev/null | tail -n 100"
	out, err := s.execPrivilegedBash(ctx, cmd)
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return string(out), nil
	}

	cmd = "journalctl -u openvpn-server@server -u openvpn@server -n 100 --no-pager 2>/dev/null"
	out, err = s.execPrivilegedBash(ctx, cmd)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
