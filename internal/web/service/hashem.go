package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
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
	if err != nil {
		return "inactive"
	}
	return strings.TrimSpace(string(out))
}

func (s *HashemService) measurePing(ip, dev string) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var args []string
	if dev != "" {
		args = []string{"-I", dev, "-c", "2", "-W", "1", ip}
	} else {
		args = []string{"-c", "2", "-W", "1", ip}
	}

	out, err := exec.CommandContext(ctx, "ping", args...).CombinedOutput()
	if err != nil {
		out, err = exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", ip).CombinedOutput()
		if err != nil {
			return -1
		}
	}

	reTime := regexp.MustCompile(`time=([0-9.]+)\s*ms`)
	if m := reTime.FindStringSubmatch(string(out)); len(m) == 2 {
		val, _ := strconv.ParseFloat(m[1], 64)
		return val
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
