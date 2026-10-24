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

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const (
	defaultEasyRsaDir = "/etc/openvpn/easy-rsa"
	defaultServerDir  = "/etc/openvpn/server"
	defaultServerConf = "/etc/openvpn/server/server.conf"
	openvpnUnitName   = "openvpn-server@server.service"
	mgmtAddress       = "127.0.0.1:7505"
)

var validClientNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

// OpenVPNLiveSession represents an active session parsed from OpenVPN management socket.
type OpenVPNLiveSession struct {
	CommonName    string
	RealAddress   string
	BytesReceived int64
	BytesSent     int64
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
	ConnectedClients []OpenVPNConnectedClient `json:"connected_clients"`
	Clients          []OpenVPNClientInfo      `json:"clients"`
}

// OpenVPNService manages OpenVPN server, easy-rsa certificates, client configs, and accounting.
type OpenVPNService struct {
	lock sync.Mutex
}

func (s *OpenVPNService) execPrivileged(ctx context.Context, name string, args ...string) ([]byte, error) {
	if systemdRun, err := exec.LookPath("systemd-run"); err == nil {
		sdArgs := append([]string{"--quiet", "--pipe", "--wait", name}, args...)
		cmd := exec.CommandContext(ctx, systemdRun, sdArgs...)
		return cmd.CombinedOutput()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

func (s *OpenVPNService) execPrivilegedBash(ctx context.Context, script string) ([]byte, error) {
	if systemdRun, err := exec.LookPath("systemd-run"); err == nil {
		cmd := exec.CommandContext(ctx, systemdRun, "--quiet", "--pipe", "--wait", "bash", "-c", script)
		return cmd.CombinedOutput()
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

	conn, err := net.DialTimeout("tcp", mgmtAddress, 1*time.Second)
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
	conn, err := net.DialTimeout("tcp", mgmtAddress, 1*time.Second)
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
			// CLIENT_LIST,Common Name,Real Address,Virtual Address,Virtual IPv6 Address,Bytes Received,Bytes Sent,Connected Since,Connected Since (time_t)
			if len(parts) >= 8 {
				cn := parts[1]
				realAddr := parts[2]
				rx, _ := strconv.ParseInt(parts[5], 10, 64)
				tx, _ := strconv.ParseInt(parts[6], 10, 64)
				since := parts[7]
				sessions = append(sessions, OpenVPNLiveSession{
					CommonName:    cn,
					RealAddress:   realAddr,
					BytesReceived: rx,
					BytesSent:     tx,
					ConnectedSince: since,
				})
			}
		}
	}
	return sessions, nil
}

// EnsureNAT enables IPv4 forwarding and adds iptables MASQUERADE for the VPN subnet,
// plus dynamic port redirection for remote/tunnel ports to the OpenVPN server port.
func (s *OpenVPNService) EnsureNAT() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	serverPortStr := getOpenVPNSetting("openvpn_server_port", "2083")
	remotePortStr := getOpenVPNSetting("openvpn_remote_port", "")

	script := fmt.Sprintf(`
sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1
iptables -t nat -C POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE
iptables -C FORWARD -s 10.8.0.0/24 -j ACCEPT 2>/dev/null || iptables -A FORWARD -s 10.8.0.0/24 -j ACCEPT
iptables -C FORWARD -d 10.8.0.0/24 -m state --state RELATED,ESTABLISHED -j ACCEPT 2>/dev/null || iptables -A FORWARD -d 10.8.0.0/24 -m state --state RELATED,ESTABLISHED -j ACCEPT

# Ensure tls-verify hook and duplicate-cn are in server.conf for concurrent logins
for c in /etc/openvpn/server/server.conf /etc/openvpn/server.conf; do
    if [ -f "$c" ]; then
        if ! grep -q "tls-verify" "$c"; then
            sed -i '/script-security 2/a tls-verify /usr/local/x-ui/bin/openvpn-verify.py' "$c"
        fi
        if ! grep -q "duplicate-cn" "$c"; then
            echo "duplicate-cn" >> "$c"
        fi
    fi
done

# Dynamic port redirection for tunnel/remote port to OpenVPN server port
SPORT="%s"
RPORT="%s"
if [ -n "$RPORT" ] && [ "$RPORT" != "$SPORT" ] && [ "$RPORT" != "0" ]; then
    iptables -t nat -C PREROUTING -i eth0 -p tcp --dport "$RPORT" -j REDIRECT --to-ports "$SPORT" 2>/dev/null || \
    iptables -t nat -A PREROUTING -i eth0 -p tcp --dport "$RPORT" -j REDIRECT --to-ports "$SPORT"
    iptables -t nat -C PREROUTING -i eth0 -p udp --dport "$RPORT" -j REDIRECT --to-ports "$SPORT" 2>/dev/null || \
    iptables -t nat -A PREROUTING -i eth0 -p udp --dport "$RPORT" -j REDIRECT --to-ports "$SPORT"

    if ip link show gre-a-1 >/dev/null 2>&1; then
        iptables -t nat -C PREROUTING -i gre-a-1 -p tcp --dport "$RPORT" -j REDIRECT --to-ports "$SPORT" 2>/dev/null || \
        iptables -t nat -A PREROUTING -i gre-a-1 -p tcp --dport "$RPORT" -j REDIRECT --to-ports "$SPORT"
        iptables -t nat -C PREROUTING -i gre-a-1 -p udp --dport "$RPORT" -j REDIRECT --to-ports "$SPORT" 2>/dev/null || \
        iptables -t nat -A PREROUTING -i gre-a-1 -p udp --dport "$RPORT" -j REDIRECT --to-ports "$SPORT"
    fi
fi
`, serverPortStr, remotePortStr)
	_, _ = s.execPrivilegedBash(ctx, script)
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

func (s *OpenVPNService) GetStatus() (*OpenVPNStatus, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	status := &OpenVPNStatus{
		Installed:        false,
		Running:          false,
		Port:             2083,
		Proto:            "tcp",
		Subnet:           "10.8.0.0/24",
		DirectIP:         detectServerIP(),
		IranIP:           getIranTunnelIP(),
		ConnectedClients: []OpenVPNConnectedClient{},
		Clients:          []OpenVPNClientInfo{},
	}

	if _, err := os.Stat("/usr/sbin/openvpn"); err == nil {
		status.Installed = true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "systemctl", "is-active", openvpnUnitName).Output()
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		status.Running = true
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
			savedPort = 2083
		}
		_ = setOpenVPNSetting("openvpn_remote_port", strconv.Itoa(savedPort))
	}

	status.RemoteHost = savedHost
	status.RemotePort = savedPort

	// Read live sessions via management socket
	liveSessions, _ := s.QueryManagementSessions()
	var sessionsByCN = make(map[string][]OpenVPNLiveSession)
	for _, sess := range liveSessions {
		sessionsByCN[sess.CommonName] = append(sessionsByCN[sess.CommonName], sess)
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

	// Build full client info list
	for _, u := range dbUsers {
		createdAt := ""
		crtFile := filepath.Join(defaultEasyRsaDir, "pki", "issued", u.Name+".crt")
		if info, err := os.Stat(crtFile); err == nil {
			createdAt = info.ModTime().Format("2006-01-02 15:04")
		} else if u.CreatedAt > 0 {
			createdAt = time.UnixMilli(u.CreatedAt).Format("2006-01-02 15:04")
		}

		sessList := sessionsByCN[u.Name]
		isConnected := len(sessList) > 0
		var liveRx, liveTx int64
		var connectedIPs []string
		since := ""
		for _, sess := range sessList {
			liveRx += sess.BytesReceived
			liveTx += sess.BytesSent
			connectedIPs = append(connectedIPs, sess.RealAddress)
			if since == "" {
				since = sess.ConnectedSince
			}
		}

		totalUsed := u.Up + u.Down + liveRx + liveTx
		isExpired := u.ExpiryTime > 0 && nowMs >= u.ExpiryTime
		isTrafficExceeded := u.TotalGB > 0 && totalUsed >= (u.TotalGB*1024*1024*1024)

		limitStr := "نامحدود"
		var percent float64 = 0
		if u.TotalGB > 0 {
			limitStr = fmt.Sprintf("%d GB", u.TotalGB)
			percent = (float64(totalUsed) / float64(u.TotalGB*1024*1024*1024)) * 100
			if percent > 100 {
				percent = 100
			}
		}

		clientInfo := OpenVPNClientInfo{
			Name:               u.Name,
			Password:           u.Password,
			Enable:             u.Enable,
			TotalGB:            u.TotalGB,
			ExpiryTime:         u.ExpiryTime,
			LimitIP:            u.LimitIP,
			Up:                 u.Up + liveRx,
			Down:               u.Down + liveTx,
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
		}

		status.Clients = append(status.Clients, clientInfo)
	}

	status.TotalClients = len(status.Clients)
	return status, nil
}

// AddClient builds a new client certificate using Easy-RSA and saves limits to DB.
func (s *OpenVPNService) AddClient(name string, password string, totalGB int64, expiryTime int64, limitIP int) error {
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

	crtFile := filepath.Join(defaultEasyRsaDir, "pki", "issued", name+".crt")
	if _, err := os.Stat(crtFile); err == nil {
		return fmt.Errorf("client '%s' already exists in certificates", name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := fmt.Sprintf("cd %s && ./easyrsa --batch build-client-full %s nopass", defaultEasyRsaDir, name)
	out, err := s.execPrivilegedBash(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to create client: %v (output: %s)", err, string(out))
	}

	if _, err := os.Stat(crtFile); err != nil {
		return fmt.Errorf("certificate was not generated: %v", err)
	}

	user := &model.OpenVPNUser{
		Name:       name,
		Password:   password,
		Enable:     true,
		TotalGB:    totalGB,
		ExpiryTime: expiryTime,
		LimitIP:    limitIP,
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

	// If disabled, expired, or over traffic, kick active session immediately
	if !user.Enable || isExpired || isTrafficExceeded {
		_ = s.KillClient(name)
	}

	return nil
}

// ResetClientTraffic resets cumulative traffic (up & down) for a client.
func (s *OpenVPNService) ResetClientTraffic(name string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	name = strings.TrimSpace(name)
	db := database.GetDB()
	return db.Model(&model.OpenVPNUser{}).Where("name = ?", name).Updates(map[string]any{
		"up":   0,
		"down": 0,
	}).Error
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := fmt.Sprintf("cd %s && ./easyrsa --batch revoke %s && rm -f pki/issued/%s.crt pki/private/%s.key pki/reqs/%s.req pki/inline/%s.inline", defaultEasyRsaDir, name, name, name, name, name)
	out, err := s.execPrivilegedBash(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to delete client: %v (output: %s)", err, string(out))
	}

	db := database.GetDB()
	_ = db.Where("name = ?", name).Delete(&model.OpenVPNUser{}).Error

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
		savedPort, _ := strconv.Atoi(getOpenVPNSetting("openvpn_remote_port", "2083"))
		if savedPort <= 0 {
			savedPort = 2083
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
	if user.Password != "" {
		authPassBlock = "auth-user-pass\n"
		if inlineAuth {
			authPassBlock = fmt.Sprintf("auth-user-pass\n<auth-user-pass>\n%s\n%s\n</auth-user-pass>\n", user.Name, user.Password)
		}
	}

	ovpn := fmt.Sprintf(`client
dev tun
proto tcp
remote %s %d
resolv-retry infinite
nobind
persist-key
persist-tun
remote-cert-tls server
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
`, remoteIP, remotePort, authPassBlock, strings.TrimSpace(string(caData)), strings.TrimSpace(cleanCert), strings.TrimSpace(string(keyData)))

	return ovpn, nil
}

// RestartService restarts OpenVPN and ensures firewall rules.
func (s *OpenVPNService) RestartService() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s.EnsureNAT()

	out, err := s.execPrivileged(ctx, "systemctl", "restart", openvpnUnitName)
	if err != nil {
		return fmt.Errorf("restart failed: %v (output: %s)", err, string(out))
	}
	return nil
}

// SaveSettings permanently stores endpoint and core port settings in DB and updates server.conf if port changed.
func (s *OpenVPNService) SaveSettings(remoteHost string, remotePort int, serverPort int) error {
	s.lock.Lock()
	defer s.lock.Unlock()

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
		confPath := defaultServerConf
		if _, err := os.Stat(confPath); os.IsNotExist(err) {
			confPath = "/etc/openvpn/server.conf"
		}
		if data, err := os.ReadFile(confPath); err == nil {
			content := string(data)
			re := regexp.MustCompile(`(?m)^port\s+\d+`)
			if re.MatchString(content) {
				newContent := re.ReplaceAllString(content, fmt.Sprintf("port %d", serverPort))
				_ = os.WriteFile(confPath, []byte(newContent), 0644)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			s.EnsureNAT()
			_, _ = s.execPrivileged(ctx, "systemctl", "restart", openvpnUnitName)
		}
	}

	// Always refresh NAT & port redirects whenever settings are saved
	s.EnsureNAT()

	return nil
}

// AutoInstallOpenVPN performs a one-click automated installation of OpenVPN and Easy-RSA on fresh servers.
func (s *OpenVPNService) AutoInstallOpenVPN(port int, proto string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if port <= 0 || port > 65535 {
		port = 2083
	}
	if proto == "" {
		proto = "tcp"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	script := fmt.Sprintf(`
export DEBIAN_FRONTEND=noninteractive
apt-get update -y && apt-get install -y openvpn easy-rsa iptables
mkdir -p /etc/openvpn/easy-rsa
cp -rf /usr/share/easy-rsa/* /etc/openvpn/easy-rsa/ 2>/dev/null || true
cd /etc/openvpn/easy-rsa
if [ ! -d "pki" ]; then
    ./easyrsa init-pki
    echo -e "\n" | ./easyrsa --batch build-ca nopass
    ./easyrsa --batch build-server-full server nopass
    ./easyrsa gen-dh
fi
mkdir -p /etc/openvpn/server
cp -f pki/ca.crt /etc/openvpn/server/
cp -f pki/issued/server.crt /etc/openvpn/server/
cp -f pki/private/server.key /etc/openvpn/server/
cp -f pki/dh.pem /etc/openvpn/server/
cat << 'EOF_CONF' > /etc/openvpn/server/server.conf
port %d
proto %s-server
dev tun
topology subnet

ca /etc/openvpn/server/ca.crt
cert /etc/openvpn/server/server.crt
key /etc/openvpn/server/server.key
dh /etc/openvpn/server/dh.pem

server 10.8.0.0 255.255.255.0
ifconfig-pool-persist /etc/openvpn/server/ipp.txt

push "redirect-gateway def1 bypass-dhcp"
push "dhcp-option DNS 1.1.1.1"
push "dhcp-option DNS 8.8.8.8"

keepalive 10 60
data-ciphers AES-256-GCM:AES-128-GCM:AES-256-CBC:AES-128-CBC
cipher AES-256-GCM
auth SHA256

persist-key
persist-tun
status /var/log/openvpn-status.log
log /var/log/openvpn.log
verb 3
management 127.0.0.1 7505

script-security 2
tls-verify /usr/local/x-ui/bin/openvpn-verify.py
client-connect /usr/local/x-ui/bin/openvpn-auth.py
client-disconnect /usr/local/x-ui/bin/openvpn-auth.py

verify-client-cert optional
username-as-common-name
auth-user-pass-verify /usr/local/x-ui/bin/openvpn-verify.py via-file
auth-user-pass-optional
duplicate-cn
EOF_CONF

sysctl -w net.ipv4.ip_forward=1
sed -i '/net.ipv4.ip_forward/d' /etc/sysctl.conf
echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf

systemctl daemon-reload
systemctl enable --now openvpn-server@server
`, port, proto)

	out, err := s.execPrivilegedBash(ctx, script)
	if err != nil {
		return fmt.Errorf("auto-install failed: %v (output: %s)", err, string(out))
	}

	s.EnsureNAT()
	return nil
}
