package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"golang.org/x/crypto/ssh"
)

const (
	hashemBinPath    = "/usr/local/bin/hashem"
	hashemScriptPath = "/usr/local/bin/hashem.sh"
	carrierJsonPath  = "/etc/gre-panel/carrier.json"
	watchdogJsonPath = "/etc/gre-panel/watchdog.json"
	frpcTomlPath     = "/etc/frp/frpc.toml"
	frpsTomlPath     = "/etc/frp/frps.toml"
)

type CarrierConfig struct {
	Mode          string   `json:"mode"`
	ActiveCarrier string   `json:"active_carrier"`
	FouPort1      int      `json:"fou_port1"`
	FouPort2      int      `json:"fou_port2"`
	WssPort       int      `json:"wss_port"`
	Candidates    []string `json:"candidates"`
	LastSwitch    string   `json:"last_switch"`
	SwitchCount   int      `json:"switch_count"`
}

type WatchdogConfig struct {
	Enabled       bool   `json:"enabled"`
	IntervalSec   int    `json:"interval_sec"`
	FailThreshold int    `json:"fail_threshold"`
	LastCheck     string `json:"last_check"`
	ConsecFails   int    `json:"consec_fails"`
}

type HashemStatus struct {
	Installed       bool     `json:"installed"`
	Running         bool     `json:"running"`
	Role            string   `json:"role"`
	Carrier         string   `json:"carrier"`
	ActiveCarrier   string   `json:"activeCarrier"`
	Candidates      []string `json:"candidates"`
	LocalPubIP      string   `json:"localPubIp"`
	RemotePubIP     string   `json:"remotePubIp"`
	LocalGreIP      string   `json:"localGreIp"`
	RemoteGreIP     string   `json:"remoteGreIp"`
	FrpStatus       string   `json:"frpStatus"`
	FrpPort         int      `json:"frpPort"`
	PingMs          float64  `json:"pingMs"`
	Ports           []int    `json:"ports"`
	WatchdogEnabled bool     `json:"watchdogEnabled"`
	Bundle          string   `json:"bundle"`
	SetupCommand    string   `json:"setupCommand"`
}

func (s HashemStatus) MarshalJSON() ([]byte, error) {
	type Alias HashemStatus
	return json.Marshal(&struct {
		Alias
		LocalGreIP  string `json:"localGreIP"`
		RemoteGreIP string `json:"remoteGreIP"`
		PeerGreIP   string `json:"peerGreIP"`
		LocalPubIP  string `json:"localPubIP"`
		RemotePubIP string `json:"remotePubIP"`
	}{
		Alias:       Alias(s),
		LocalGreIP:  s.LocalGreIP,
		RemoteGreIP: s.RemoteGreIP,
		PeerGreIP:   s.RemoteGreIP,
		LocalPubIP:  s.LocalPubIP,
		RemotePubIP: s.RemotePubIP,
	})
}

type HashemSetupForm struct {
	Role      string `json:"role"`
	LocalPub  string `json:"localPub"`
	RemotePub string `json:"remotePub"`
	FrpPort   int    `json:"frpPort"`
	Token     string `json:"token"`
	Ports     string `json:"ports"`
	Carrier   string `json:"carrier"`
	Bundle    string `json:"bundle"`
}

type HashemSSHSetupForm struct {
	IranIP      string `json:"iranIp" form:"iranIp"`
	SSHPort     int    `json:"sshPort" form:"sshPort"`
	SSHUser     string `json:"sshUser" form:"sshUser"`
	SSHPassword string `json:"sshPassword" form:"sshPassword"`
	Ports       string `json:"ports" form:"ports"`
	Carrier     string `json:"carrier" form:"carrier"`
}

type HashemSSHSetupResult struct {
	Success   bool   `json:"success" form:"success"`
	Message   string `json:"message" form:"message"`
	IranIP    string `json:"iranIp" form:"iranIp"`
	ForeignIP string `json:"foreignIp" form:"foreignIp"`
	Ports     string `json:"ports" form:"ports"`
	Log       string `json:"log" form:"log"`
}

type HashemOneLinerForm struct {
	IranIP  string `json:"iranIp" form:"iranIp"`
	Ports   string `json:"ports" form:"ports"`
	Carrier string `json:"carrier" form:"carrier"`
}

type HashemOneLinerResult struct {
	OneLinerCommand string `json:"oneLinerCommand"`
	ForeignIP       string `json:"foreignIp"`
	IranIP          string `json:"iranIp"`
	Ports           string `json:"ports"`
	FrpPort         int    `json:"frpPort"`
	Token           string `json:"token"`
}

type HashemService struct {
	inboundService InboundService
}

func (s *HashemService) IsInstalled() bool {
	if _, err := os.Stat(hashemBinPath); err == nil {
		return true
	}
	if _, err := os.Stat(hashemScriptPath); err == nil {
		return true
	}
	return false
}

func (s *HashemService) GetStatus() (*HashemStatus, error) {
	status := &HashemStatus{
		Installed:  s.IsInstalled(),
		PingMs:     -1,
		Candidates: []string{"direct", "fou:443", "wss:8443"},
	}

	if !status.Installed {
		return status, nil
	}

	if data, err := os.ReadFile(carrierJsonPath); err == nil {
		var cc CarrierConfig
		if err := json.Unmarshal(data, &cc); err == nil {
			status.Carrier = cc.Mode
			status.ActiveCarrier = cc.ActiveCarrier
			if len(cc.Candidates) > 0 {
				status.Candidates = cc.Candidates
			}
		}
	}

	if data, err := os.ReadFile(watchdogJsonPath); err == nil {
		var wc WatchdogConfig
		if err := json.Unmarshal(data, &wc); err == nil {
			status.WatchdogEnabled = wc.Enabled
		}
	}

	s.parseGreInterface(status)

	token := ""
	if _, err := os.Stat(frpcTomlPath); err == nil {
		status.Role = "foreign"
		token = s.parseFrpConfig(frpcTomlPath, status)
	} else if _, err := os.Stat(frpsTomlPath); err == nil {
		status.Role = "iran"
		token = s.parseFrpConfig(frpsTomlPath, status)
	} else {
		status.Role = "none"
	}

	if status.RemoteGreIP != "" {
		status.PingMs = s.measurePing(status.RemoteGreIP, "gre-tunnel", status.FrpPort)
		if status.PingMs <= 0 {
			status.Running = false
		}
	} else {
		status.PingMs = -1
		if !status.Installed {
			status.Running = false
		}
	}

	if status.Role == "foreign" {
		baseStatus := s.checkServiceStatus("frpc")
		if baseStatus == "active" {
			if status.PingMs > 0 && s.isFrpConnected(status.RemoteGreIP, status.FrpPort) {
				status.FrpStatus = "active"
			} else {
				status.FrpStatus = "connecting"
			}
		} else {
			status.FrpStatus = baseStatus
		}
	} else if status.Role == "iran" {
		baseStatus := s.checkServiceStatus("frps")
		if baseStatus == "active" {
			if status.PingMs > 0 && s.isFrpConnected(status.RemoteGreIP, status.FrpPort) {
				status.FrpStatus = "active"
			} else {
				status.FrpStatus = "connecting"
			}
		} else {
			status.FrpStatus = baseStatus
		}
	}

	if status.Role == "foreign" && status.RemotePubIP != "" && status.FrpPort > 0 && token != "" {
		portsStr := ""
		if len(status.Ports) > 0 {
			var pstrs []string
			for _, p := range status.Ports {
				pstrs = append(pstrs, strconv.Itoa(p))
			}
			portsStr = strings.Join(pstrs, "-")
		}
		status.Bundle = fmt.Sprintf("hsh1_%s_%d_%s_%s_%s_%s",
			status.RemotePubIP, status.FrpPort, status.RemoteGreIP, status.LocalGreIP, token, portsStr)
		status.SetupCommand = fmt.Sprintf(
			"curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh | bash -s -- setup-iran --local-pub %s --remote-pub %s --frp-port %d --token %s",
			status.RemotePubIP, status.LocalPubIP, status.FrpPort, token,
		)
	}

	return status, nil
}

func (s *HashemService) parseFrpConfig(path string, status *HashemStatus) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	token := ""
	portMap := make(map[int]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "serverPort") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				p, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
				status.FrpPort = p
			}
		} else if strings.HasPrefix(line, "auth.token") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				token = strings.Trim(strings.TrimSpace(parts[1]), "\"")
			}
		} else if strings.HasPrefix(line, "remotePort") || strings.HasPrefix(line, "localPort") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				p, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
				if p > 0 {
					portMap[p] = true
				}
			}
		}
	}

	for p := range portMap {
		status.Ports = append(status.Ports, p)
	}
	return token
}

func (s *HashemService) parseGreInterface(status *HashemStatus) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "ip", "addr", "show", "dev", "gre-tunnel").CombinedOutput()
	if err != nil {
		status.Running = false
		return
	}

	status.Running = true
	output := string(out)

	reLink := regexp.MustCompile(`link/gre\s+([0-9.]+)\s+peer\s+([0-9.]+)`)
	if m := reLink.FindStringSubmatch(output); len(m) == 3 {
		status.LocalPubIP = m[1]
		status.RemotePubIP = m[2]
	}

	reInet := regexp.MustCompile(`inet\s+([0-9.]+)/[0-9]+`)
	if m := reInet.FindStringSubmatch(output); len(m) == 2 {
		status.LocalGreIP = m[1]
		if strings.HasSuffix(status.LocalGreIP, ".1") {
			status.RemoteGreIP = strings.TrimSuffix(status.LocalGreIP, ".1") + ".2"
		} else if strings.HasSuffix(status.LocalGreIP, ".2") {
			status.RemoteGreIP = strings.TrimSuffix(status.LocalGreIP, ".2") + ".1"
		} else if status.LocalGreIP == "10.10.10.1" {
			status.RemoteGreIP = "10.10.10.2"
		} else if status.LocalGreIP == "10.10.10.2" {
			status.RemoteGreIP = "10.10.10.1"
		}
	}
}

func (s *HashemService) checkServiceStatus(svc string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "systemctl", "is-active", svc).Output()
	st := strings.TrimSpace(string(out))
	if err != nil || (st != "active" && st != "activating") {
		if svc == "frpc" {
			go func() {
				c, cl := context.WithTimeout(context.Background(), 3*time.Second)
				defer cl()
				_ = exec.CommandContext(c, "systemctl", "restart", svc).Run()
			}()
		}
		return "inactive"
	}
	return st
}

func (s *HashemService) isFrpConnected(peerGre string, port int) bool {
	if port <= 0 {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	filter := fmt.Sprintf("dport = :%d", port)
	if peerGre != "" {
		filter = fmt.Sprintf("dst %s and dport = :%d", peerGre, port)
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("ss -t state established '%s' 2>/dev/null", filter))
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		return len(lines) > 1
	}
	return false
}

func (s *HashemService) measurePing(ip, dev string, frpPort int) float64 {
	if ip == "" {
		return -1
	}

	// 1. Real active ICMP ping directly to peer GRE IP (1 probe, 1s timeout)
	ctx1, cancel1 := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel1()

	var args []string
	if dev != "" {
		args = []string{"-c", "1", "-W", "1", "-I", dev, ip}
	} else {
		args = []string{"-c", "1", "-W", "1", ip}
	}

	out, err := exec.CommandContext(ctx1, "ping", args...).CombinedOutput()
	if err == nil {
		reTime := regexp.MustCompile(`time=([0-9.]+)\s*ms`)
		if m := reTime.FindStringSubmatch(string(out)); len(m) == 2 {
			if val, err := strconv.ParseFloat(m[1], 64); err == nil && val > 0 {
				return val
			}
		}
	}

	// 2. Quick fallback TCP SYN probe (500ms timeout)
	if frpPort > 0 {
		target := net.JoinHostPort(ip, strconv.Itoa(frpPort))
		start := time.Now()
		conn, err := net.DialTimeout("tcp", target, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			latency := float64(time.Since(start).Microseconds()) / 1000.0
			if latency > 0 {
				return latency
			}
		}
	}

	return -1
}

func (s *HashemService) SetCarrier(carrier string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, hashemBinPath, "carrier", "set", carrier)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warningf("hashem carrier set failed: %v, out: %s", err, string(out))
		return fmt.Errorf("failed to set carrier: %s", string(out))
	}
	return nil
}

func (s *HashemService) Restart() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, hashemBinPath, "restart")
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warningf("hashem restart failed: %v, out: %s", err, string(out))
		return fmt.Errorf("failed to restart tunnel: %s", string(out))
	}
	return nil
}

func (s *HashemService) SetWatchdog(enabled bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	action := "off"
	if enabled {
		action = "on"
	}
	cmd := exec.CommandContext(ctx, hashemBinPath, "watchdog", action)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warningf("hashem watchdog %s failed: %v, out: %s", action, err, string(out))
		return fmt.Errorf("failed to set watchdog: %s", string(out))
	}
	return nil
}

func (s *HashemService) Setup(form HashemSetupForm) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{}
	if form.Bundle != "" {
		args = []string{"setup-foreign", "--bundle", form.Bundle, "--force"}
	} else if form.Role == "iran" {
		args = []string{"setup-iran", "--local-pub", form.LocalPub, "--remote-pub", form.RemotePub, "--force"}
		if form.FrpPort > 0 {
			args = append(args, "--frp-port", strconv.Itoa(form.FrpPort))
		}
		if form.Token != "" {
			args = append(args, "--token", form.Token)
		}
	} else {
		args = []string{"setup-foreign", "--local-pub", form.LocalPub, "--remote-pub", form.RemotePub, "--force"}
		if form.FrpPort > 0 {
			args = append(args, "--frp-port", strconv.Itoa(form.FrpPort))
		}
		if form.Token != "" {
			args = append(args, "--token", form.Token)
		}
		if form.Ports != "" {
			args = append(args, "--ports", form.Ports)
		}
	}

	cmd := exec.CommandContext(ctx, hashemBinPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warningf("hashem setup failed: %v, out: %s", err, string(out))
		return string(out), fmt.Errorf("setup failed: %s", string(out))
	}
	return string(out), nil
}

func (s *HashemService) SyncInbounds() ([]int, error) {
	inbounds, err := s.inboundService.GetInboundsForScope(InboundAccessScope{All: true})
	if err != nil {
		return nil, err
	}

	var ports []int
	for _, in := range inbounds {
		if in.Enable && in.Port > 0 {
			ports = append(ports, in.Port)
		}
	}

	if len(ports) == 0 {
		return nil, fmt.Errorf("no active inbounds found")
	}

	status, err := s.GetStatus()
	if err != nil || status.Role != "foreign" {
		return nil, fmt.Errorf("hashem tunnel not configured as foreign client")
	}

	var portStrs []string
	for _, p := range ports {
		portStrs = append(portStrs, strconv.Itoa(p))
	}
	portsArg := strings.Join(portStrs, ", ")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, hashemBinPath, "setup-foreign",
		"--local-pub", status.LocalPubIP,
		"--remote-pub", status.RemotePubIP,
		"--frp-port", strconv.Itoa(status.FrpPort),
		"--ports", portsArg,
		"--force",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sync failed: %s", string(out))
	}

	return ports, nil
}

func (s *HashemService) Install() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", "curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh | bash")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("install failed: %s", string(out))
	}
	return string(out), nil
}

func (s *HashemService) getForeignPubIP() string {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "curl", "-sSL", "--max-time", "3", "https://api.ipify.org")
	if out, err := cmd.Output(); err == nil {
		ip := strings.TrimSpace(string(out))
		if len(ip) >= 7 && len(ip) <= 15 {
			return ip
		}
	}

	cmd = exec.CommandContext(ctx, "bash", "-c", "ip route get 1.1.1.1 2>/dev/null | awk '/src/ {for (i=1; i<=NF; i++) if ($i==\"src\") {print $(i+1); exit}}'")
	if out, err := cmd.Output(); err == nil {
		ip := strings.TrimSpace(string(out))
		if len(ip) >= 7 && len(ip) <= 15 {
			return ip
		}
	}

	return ""
}

func (s *HashemService) resolveTunnelPorts(portsStr string) string {
	cleaned := strings.TrimSpace(portsStr)
	if cleaned != "" {
		return cleaned
	}

	inbounds, err := s.inboundService.GetInboundsForScope(InboundAccessScope{All: true})
	if err == nil && len(inbounds) > 0 {
		var pstrs []string
		for _, in := range inbounds {
			if in.Enable && in.Port > 0 {
				pstrs = append(pstrs, strconv.Itoa(in.Port))
			}
		}
		if len(pstrs) > 0 {
			return strings.Join(pstrs, ", ")
		}
	}

	return "8080"
}

func (s *HashemService) genFRPPortAndToken() (int, string) {
	var b [2]byte
	_, _ = rand.Read(b[:])
	port := 35000 + int(b[0])<<8 + int(b[1])
	if port > 58000 {
		port = 35000 + (port % 23000)
	}

	var tb [16]byte
	_, _ = rand.Read(tb[:])
	token := hex.EncodeToString(tb[:])

	return port, token
}

func (s *HashemService) SetupSSH(form HashemSSHSetupForm) (*HashemSSHSetupResult, error) {
	if strings.TrimSpace(form.IranIP) == "" {
		return nil, fmt.Errorf("آدرس آی‌پی سرور ایران الزامی است")
	}
	if strings.TrimSpace(form.SSHPassword) == "" {
		return nil, fmt.Errorf("رمز عبور سرور ایران الزامی است")
	}

	sshPort := form.SSHPort
	if sshPort <= 0 {
		sshPort = 22
	}
	sshUser := strings.TrimSpace(form.SSHUser)
	if sshUser == "" {
		sshUser = "root"
	}
	carrier := strings.TrimSpace(form.Carrier)
	if carrier == "" {
		carrier = "fou:443"
	}

	foreignIP := s.getForeignPubIP()
	if foreignIP == "" {
		return nil, fmt.Errorf("امکان تشخیص خودکار آی‌پی سرور خارج وجود ندارد")
	}

	iranIP := strings.TrimSpace(form.IranIP)
	ports := s.resolveTunnelPorts(form.Ports)
	frpPort, token := s.genFRPPortAndToken()

	sshConfig := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(form.SSHPassword),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = form.SSHPassword
				}
				return answers, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         25 * time.Second,
	}

	addr := net.JoinHostPort(iranIP, strconv.Itoa(sshPort))
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "timed out") || strings.Contains(errStr, "i/o timeout") {
			return nil, fmt.Errorf("اتصال SSH به پورت %d سرور ایران تایم‌اوت شد. به دلیل مسدود بودن پیش‌فرض پورت ۲۲ توسط اکثر دیتاسنترهای ایران، لطفاً پورت SSH سرور ایران را تغییر دهید یا از تب «دستور تک‌خطی» استفاده فرمایید.", sshPort)
		}
		if strings.Contains(errStr, "unable to authenticate") || strings.Contains(errStr, "auth failed") {
			return nil, fmt.Errorf("احراز هویت SSH ناموفق بود. نام کاربری (%s) یا رمز عبور وارد شده برای سرور ایران نادرست است.", sshUser)
		}
		if strings.Contains(errStr, "connection refused") {
			return nil, fmt.Errorf("اتصال به سرور ایران رد شد (Connection Refused). لطفاً بررسی کنید سرویس SSH روی پورت %d فعال باشد.", sshPort)
		}
		return nil, fmt.Errorf("خطا در اتصال SSH به سرور ایران (%s): %v", addr, err)
	}
	defer client.Close()

	// 1. Read local hashem.sh to transfer directly over SSH, bypassing Iranian GitHub blocks
	scriptContent, _ := os.ReadFile(hashemScriptPath)
	if len(scriptContent) == 0 {
		scriptContent, _ = os.ReadFile(hashemBinPath)
	}

	if len(scriptContent) > 0 {
		uploadSession, uErr := client.NewSession()
		if uErr == nil {
			uploadSession.Stdin = bytes.NewReader(scriptContent)
			_ = uploadSession.Run("cat > /tmp/hashem.sh && chmod +x /tmp/hashem.sh && cp -f /tmp/hashem.sh /usr/local/bin/hashem 2>/dev/null || true")
			uploadSession.Close()
		}
	}

	peerJsonCmd, optimizeCmd, _ := buildIranSetupCommands(iranIP, foreignIP, frpPort, token, carrier, ports)
	iranCmd := fmt.Sprintf(
		"export DEBIAN_FRONTEND=noninteractive; "+
			"if [ ! -s /tmp/hashem.sh ]; then "+
			"  curl -fsSL https://fastly.jsdelivr.net/gh/pdnczone/hashem-panel/hashem.sh -o /tmp/hashem.sh 2>/dev/null || "+
			"  curl -fsSL https://ghproxy.net/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || "+
			"  curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh; "+
			"  chmod +x /tmp/hashem.sh; "+
			"fi; "+
			"bash /tmp/hashem.sh setup-iran --local-pub %s --remote-pub %s --frp-port %d --token %s --force && "+
			"bash /tmp/hashem.sh carrier mode %s && "+
			"%s && %s; rm -f /tmp/hashem.sh",
		iranIP, foreignIP, frpPort, token, carrier, peerJsonCmd, optimizeCmd,
	)

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("خطا در ایجاد نشست SSH روی سرور ایران: %v", err)
	}
	defer session.Close()

	iranOut, err := session.CombinedOutput(iranCmd)
	if err != nil {
		return nil, fmt.Errorf("اجرای تانل روی سرور ایران با خطا مواجه شد: %v\nخروجی: %s", err, string(iranOut))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmdForeign := exec.CommandContext(ctx, hashemBinPath, "setup-foreign",
		"--local-pub", foreignIP,
		"--remote-pub", iranIP,
		"--frp-port", strconv.Itoa(frpPort),
		"--token", token,
		"--ports", ports,
		"--force",
	)
	fOut, _ := cmdForeign.CombinedOutput()

	cmdCarrier := exec.CommandContext(ctx, hashemBinPath, "carrier", "mode", carrier)
	_ = cmdCarrier.Run()

	s.optimizeForeignNetwork(ports)

	return &HashemSSHSetupResult{
		Success:   true,
		Message:   "تانل با موفقیت از طریق SSH روی سرور ایران و خارج پیاده‌سازی شد.",
		IranIP:    iranIP,
		ForeignIP: foreignIP,
		Ports:     ports,
		Log:       fmt.Sprintf("Iran Setup:\n%s\n\nForeign Setup:\n%s", string(iranOut), string(fOut)),
	}, nil
}

func (s *HashemService) optimizeForeignNetwork(ports string) {
	if data, err := os.ReadFile(frpcTomlPath); err == nil {
		sData := string(data)
		if strings.Contains(sData, "transport.poolCount") {
			re := regexp.MustCompile(`transport\.poolCount\s*=\s*\d+`)
			sData = re.ReplaceAllString(sData, "transport.poolCount = 20")
		} else {
			sData = strings.Replace(sData, "transport.tcpMux = true", "transport.tcpMux = true\ntransport.poolCount = 20", 1)
		}
		reUdp := regexp.MustCompile(`(?s)\[\[proxies\]\]\s*\nname\s*=\s*"udp_[^"]+"\s*\ntype\s*=\s*"udp"[^\[]*`)
		sData = reUdp.ReplaceAllString(sData, "")
		_ = os.WriteFile(frpcTomlPath, []byte(sData), 0644)
	}

	fixFrpcService := `[Unit]
Description=FRP Client Reverse Service
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
User=root
Restart=always
RestartSec=3s
ExecStart=/usr/local/bin/frpc -c /etc/frp/frpc.toml

[Install]
WantedBy=multi-user.target
`
	_ = os.WriteFile("/etc/systemd/system/frpc.service", []byte(fixFrpcService), 0644)
	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("systemctl", "restart", "frpc").Run()

	_ = exec.Command("ip", "link", "set", "dev", "gre-tunnel", "mtu", "1220").Run()
	_ = exec.Command("iptables", "-t", "mangle", "-C", "POSTROUTING", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--set-mss", "1140").Run()
	_ = exec.Command("iptables", "-t", "mangle", "-A", "POSTROUTING", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--set-mss", "1140").Run()
	_ = exec.Command("iptables", "-t", "mangle", "-C", "FORWARD", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--set-mss", "1140").Run()
	_ = exec.Command("iptables", "-t", "mangle", "-A", "FORWARD", "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--set-mss", "1140").Run()
	_ = exec.Command("ethtool", "-K", "gre-tunnel", "tso", "off", "gso", "off", "gro", "off").Run()
	_ = exec.Command("ethtool", "-K", "eth0", "tso", "off", "gso", "off", "gro", "off").Run()
	_ = exec.Command(hashemBinPath, "optimize").Run()
}

func buildIranSetupCommands(iranIP, foreignIP string, frpPort int, token, carrier, ports string) (string, string, string) {
	var portList []string
	for _, p := range strings.Split(ports, ",") {
		p = strings.TrimSpace(p)
		if _, err := strconv.Atoi(p); err == nil {
			portList = append(portList, p)
		}
	}
	portsJson := strings.Join(portList, ", ")
	if portsJson == "" {
		portsJson = "8080"
	}

	peerJsonCmd := fmt.Sprintf(
		`python3 -c 'import json, os; p="/etc/gre-panel/peers.json"; os.makedirs(os.path.dirname(p), exist_ok=True); json.dump({"peers": [{"id": 1, "name": "German-Nova", "local_pub": "%s", "remote_pub": "%s", "peer_pub": "%s", "local_gre": "10.10.10.2", "peer_gre": "10.10.10.1", "ports": [%s], "frp_port": %d, "gre_if": "gre-tunnel", "frps_svc": "frps", "legacy": True}]}, open(p, "w"), indent=2)' 2>/dev/null && systemctl restart gre-panel 2>/dev/null || true`,
		iranIP, foreignIP, foreignIP, portsJson, frpPort,
	)

	optimizeCmd := "ip link set dev gre-tunnel mtu 1220 2>/dev/null || true; " +
		"iptables -t mangle -C POSTROUTING -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1140 2>/dev/null || iptables -t mangle -A POSTROUTING -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1140 2>/dev/null || true; " +
		"iptables -t mangle -C FORWARD -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1140 2>/dev/null || iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1140 2>/dev/null || true; " +
		"ethtool -K eth0 tso off gso off gro off 2>/dev/null || true; " +
		"ethtool -K gre-tunnel tso off gso off gro off 2>/dev/null || true; " +
		"bash /tmp/hashem.sh optimize 2>/dev/null || true"

	oneLiner := fmt.Sprintf(
		"curl -fsSL https://fastly.jsdelivr.net/gh/pdnczone/hashem-panel/hashem.sh -o /tmp/hashem.sh 2>/dev/null || curl -fsSL https://ghproxy.net/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh; bash /tmp/hashem.sh setup-iran --local-pub %s --remote-pub %s --frp-port %d --token %s --force && bash /tmp/hashem.sh carrier mode %s && %s && %s && rm -f /tmp/hashem.sh",
		iranIP, foreignIP, frpPort, token, carrier, peerJsonCmd, optimizeCmd,
	)

	return peerJsonCmd, optimizeCmd, oneLiner
}

func (s *HashemService) GenerateOneLiner(form HashemOneLinerForm) (*HashemOneLinerResult, error) {
	if strings.TrimSpace(form.IranIP) == "" {
		return nil, fmt.Errorf("آدرس آی‌پی سرور ایران الزامی است")
	}

	carrier := strings.TrimSpace(form.Carrier)
	if carrier == "" {
		carrier = "fou:443"
	}

	foreignIP := s.getForeignPubIP()
	if foreignIP == "" {
		return nil, fmt.Errorf("امکان تشخیص خودکار آی‌پی سرور خارج وجود ندارد")
	}

	iranIP := strings.TrimSpace(form.IranIP)
	ports := s.resolveTunnelPorts(form.Ports)
	frpPort, token := s.genFRPPortAndToken()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmdForeign := exec.CommandContext(ctx, hashemBinPath, "setup-foreign",
		"--local-pub", foreignIP,
		"--remote-pub", iranIP,
		"--frp-port", strconv.Itoa(frpPort),
		"--token", token,
		"--ports", ports,
		"--force",
	)
	_ = cmdForeign.Run()

	cmdCarrier := exec.CommandContext(ctx, hashemBinPath, "carrier", "mode", carrier)
	_ = cmdCarrier.Run()

	s.optimizeForeignNetwork(ports)

	_, _, oneLiner := buildIranSetupCommands(iranIP, foreignIP, frpPort, token, carrier, ports)

	return &HashemOneLinerResult{
		OneLinerCommand: oneLiner,
		ForeignIP:       foreignIP,
		IranIP:          iranIP,
		Ports:           ports,
		FrpPort:         frpPort,
		Token:           token,
	}, nil
}

func (s *HashemService) Remove() error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 1. Stop and disable FRP services
	_ = exec.CommandContext(ctx, "systemctl", "stop", "frpc").Run()
	_ = exec.CommandContext(ctx, "systemctl", "disable", "frpc").Run()
	_ = exec.CommandContext(ctx, "systemctl", "stop", "frps").Run()
	_ = exec.CommandContext(ctx, "systemctl", "disable", "frps").Run()
	_ = os.Remove("/etc/systemd/system/frpc.service")
	_ = os.Remove("/etc/systemd/system/frps.service")
	_ = os.RemoveAll("/etc/frp")

	// 2. Delete GRE tunnel interface
	_ = exec.CommandContext(ctx, "ip", "link", "set", "gre-tunnel", "down").Run()
	_ = exec.CommandContext(ctx, "ip", "link", "del", "gre-tunnel").Run()

	// 3. Remove FoU listeners
	_ = exec.CommandContext(ctx, "ip", "fou", "del", "port", "443").Run()
	_ = exec.CommandContext(ctx, "ip", "fou", "del", "port", "19998").Run()
	_ = exec.CommandContext(ctx, "ip", "fou", "del", "port", "55555").Run()

	// 4. Remove gre-panel config directory
	_ = os.RemoveAll("/etc/gre-panel")

	// 5. Reload systemd daemon
	_ = exec.CommandContext(ctx, "systemctl", "daemon-reload").Run()

	logger.Info("Hashem tunnel removed completely from host")
	return nil
}
