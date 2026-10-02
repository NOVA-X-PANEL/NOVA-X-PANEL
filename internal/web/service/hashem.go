package service

import (
	"bufio"
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

	token := ""
	if _, err := os.Stat(frpcTomlPath); err == nil {
		status.Role = "foreign"
		status.FrpStatus = s.checkServiceStatus("frpc")
		token = s.parseFrpConfig(frpcTomlPath, status)
	} else if _, err := os.Stat(frpsTomlPath); err == nil {
		status.Role = "iran"
		status.FrpStatus = s.checkServiceStatus("frps")
		token = s.parseFrpConfig(frpsTomlPath, status)
	} else {
		status.Role = "none"
	}

	s.parseGreInterface(status)

	if status.RemoteGreIP != "" {
		status.PingMs = s.measurePing(status.RemoteGreIP, "gre-tunnel")
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

	reInet := regexp.MustCompile(`inet\s+([0-9.]+)/[0-9]+.*scope global gre-tunnel`)
	if m := reInet.FindStringSubmatch(output); len(m) == 2 {
		status.LocalGreIP = m[1]
		if status.LocalGreIP == "10.10.10.1" {
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

func (s *HashemService) measurePing(ip, dev string) float64 {
	// 1. Try reading real-time TCP RTT directly from active FRP tunnel socket in kernel
	ctx1, cancel1 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel1()
	cmd := exec.CommandContext(ctx1, "bash", "-c", "ss -tin '( dport = :30000:60000 or sport = :30000:60000 )' 2>/dev/null | grep -oP '(minrtt|rtt):\\K[0-9.]+' | head -n 1")
	if out, err := cmd.Output(); err == nil {
		str := strings.TrimSpace(string(out))
		if val, err := strconv.ParseFloat(str, 64); err == nil && val > 0 {
			return val
		}
	}

	// 2. Fallback to ICMP ping bound to device
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	var args []string
	if dev != "" {
		args = []string{"-I", dev, "-c", "2", "-W", "1", ip}
	} else {
		args = []string{"-c", "2", "-W", "1", ip}
	}

	out, err := exec.CommandContext(ctx2, "ping", args...).CombinedOutput()
	if err == nil {
		reTime := regexp.MustCompile(`time=([0-9.]+)\s*ms`)
		if m := reTime.FindStringSubmatch(string(out)); len(m) == 2 {
			val, _ := strconv.ParseFloat(m[1], 64)
			return val
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
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         20 * time.Second,
	}

	addr := net.JoinHostPort(iranIP, strconv.Itoa(sshPort))
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "timed out") {
			return nil, fmt.Errorf("اتصال SSH به پورت %d سرور ایران با تایم‌اوت مواجه شد. به دلیل مسدود بودن پورت‌های SSH از خارج توسط دیتاسنترهای ایران، لطفاً از تب دوم «دستور تک‌خطی سرور ایران» استفاده فرمایید.", sshPort)
		}
		return nil, fmt.Errorf("خطا در اتصال SSH به سرور ایران (%s): %v", addr, err)
	}
	defer client.Close()

	iranCmd := fmt.Sprintf(
		"curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh && "+
			"bash /tmp/hashem.sh setup-iran --local-pub %s --remote-pub %s --frp-port %d --token %s --force && "+
			"bash /tmp/hashem.sh carrier mode %s && rm -f /tmp/hashem.sh",
		iranIP, foreignIP, frpPort, token, carrier,
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
	fOut, _ := cmdForeign.CombinedOutput()

	cmdCarrier := exec.CommandContext(ctx, hashemBinPath, "carrier", "mode", carrier)
	_ = cmdCarrier.Run()

	if data, err := os.ReadFile(frpcTomlPath); err == nil {
		sData := string(data)
		if !strings.Contains(sData, "transport.poolCount") {
			sData = strings.Replace(sData, "transport.tcpMux = true", "transport.tcpMux = true\ntransport.poolCount = 10", 1)
			_ = os.WriteFile(frpcTomlPath, []byte(sData), 0644)
		}
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

	return &HashemSSHSetupResult{
		Success:   true,
		Message:   "تانل با موفقیت از طریق SSH روی سرور ایران و خارج پیاده‌سازی شد.",
		IranIP:    iranIP,
		ForeignIP: foreignIP,
		Ports:     ports,
		Log:       fmt.Sprintf("Iran Setup:\n%s\n\nForeign Setup:\n%s", string(iranOut), string(fOut)),
	}, nil
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

	if data, err := os.ReadFile(frpcTomlPath); err == nil {
		sData := string(data)
		if !strings.Contains(sData, "transport.poolCount") {
			sData = strings.Replace(sData, "transport.tcpMux = true", "transport.tcpMux = true\ntransport.poolCount = 10", 1)
			_ = os.WriteFile(frpcTomlPath, []byte(sData), 0644)
		}
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

	oneLiner := fmt.Sprintf(
		"curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh && bash /tmp/hashem.sh setup-iran --local-pub %s --remote-pub %s --frp-port %d --token %s --force && bash /tmp/hashem.sh carrier mode %s && rm -f /tmp/hashem.sh",
		iranIP, foreignIP, frpPort, token, carrier,
	)

	return &HashemOneLinerResult{
		OneLinerCommand: oneLiner,
		ForeignIP:       foreignIP,
		IranIP:          iranIP,
		Ports:           ports,
		FrpPort:         frpPort,
		Token:           token,
	}, nil
}
