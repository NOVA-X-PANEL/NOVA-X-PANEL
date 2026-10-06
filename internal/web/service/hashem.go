package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"golang.org/x/crypto/ssh"
)

const (
	hashemBinPath          = "/usr/local/bin/hashem"
	hashemScriptPath       = "/usr/local/bin/hashem.sh"
	carrierJsonPath        = "/etc/gre-panel/carrier.json"
	watchdogJsonPath       = "/etc/gre-panel/watchdog.json"
	benchmarkReportPath    = "/etc/gre-panel/benchmark_report.json"
	backhaulClientTomlPath = "/etc/backhaul/client.toml"
	backhaulServerTomlPath = "/etc/backhaul/config.toml"
	frpcTomlPath           = "/etc/frp/frpc.toml"
	frpsTomlPath           = "/etc/frp/frps.toml"
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
	AutoPilot     bool     `json:"auto_pilot,omitempty"`
}

type WatchdogConfig struct {
	Enabled       bool   `json:"enabled"`
	IntervalSec   int    `json:"interval_sec"`
	FailThreshold int    `json:"fail_threshold"`
	LastCheck     string `json:"last_check"`
	ConsecFails   int    `json:"consec_fails"`
}

type CarrierMetric struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Port          int     `json:"port"`
	AvgRTTMs      float64 `json:"avgRttMs"`
	MinRTTMs      float64 `json:"minRttMs"`
	MaxRTTMs      float64 `json:"maxRttMs"`
	PacketLoss    float64 `json:"packetLoss"`
	JitterMs      float64 `json:"jitterMs"`
	Score         int     `json:"score"`
	Status        string  `json:"status"`
	IsActive      bool    `json:"isActive"`
	IsRecommended bool    `json:"isRecommended"`
	ErrorDetail   string  `json:"errorDetail,omitempty"`
}

type AutoPilotStatus struct {
	Enabled       bool    `json:"enabled"`
	ThresholdLoss float64 `json:"thresholdLoss"`
	LastTriggered string  `json:"lastTriggered"`
	TriggerCount  int     `json:"triggerCount"`
}

type BenchmarkReport struct {
	Timestamp      string          `json:"timestamp"`
	DurationSec    float64         `json:"durationSec"`
	PeerURL        string          `json:"peerUrl,omitempty"`
	PeerInternalIP string          `json:"peerInternalIp,omitempty"`
	ActiveCarrier  string          `json:"activeCarrier"`
	BestCarrier    string          `json:"bestCarrier"`
	AutoPilot      AutoPilotStatus `json:"autoPilot"`
	Metrics        []CarrierMetric `json:"metrics"`
}

type HashemStatus struct {
	Installed       bool             `json:"installed"`
	Running         bool             `json:"running"`
	Role            string           `json:"role"`
	Engine          string           `json:"engine"`
	Transport       string           `json:"transport"`
	Snappy          bool             `json:"snappy"`
	BackhaulPort    int              `json:"backhaulPort"`
	Carrier         string           `json:"carrier"`
	ActiveCarrier   string           `json:"activeCarrier"`
	Candidates      []string         `json:"candidates"`
	LocalPubIP      string           `json:"localPubIp"`
	RemotePubIP     string           `json:"remotePubIp"`
	LocalGreIP      string           `json:"localGreIp"`
	RemoteGreIP     string           `json:"remoteGreIp"`
	FrpStatus       string           `json:"frpStatus"`
	BackhaulStatus  string           `json:"backhaulStatus"`
	FrpPort         int              `json:"frpPort"`
	PingMs          float64          `json:"pingMs"`
	Ports           []int            `json:"ports"`
	WatchdogEnabled bool             `json:"watchdogEnabled"`
	AutoPilot       bool             `json:"autoPilot"`
	Benchmark       *BenchmarkReport `json:"benchmark,omitempty"`
	Bundle          string           `json:"bundle"`
	SetupCommand    string           `json:"setupCommand"`
	BackhaulToken   string           `json:"backhaulToken,omitempty"`
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
	Role              string `json:"role"`
	Engine            string `json:"engine"`
	Transport         string `json:"transport"`
	BackhaulPort      int    `json:"backhaulPort"`
	Snappy            bool   `json:"snappy"`
	LocalPub          string `json:"localPub"`
	RemotePub         string `json:"remotePub"`
	FrpPort           int    `json:"frpPort"`
	Token             string `json:"token"`
	Ports             string `json:"ports"`
	Carrier           string `json:"carrier"`
	Bundle            string `json:"bundle"`
	AutoCreateInbound bool   `json:"autoCreateInbound"`
	InboundHost       string `json:"inboundHost"`
}

type HashemSSHSetupForm struct {
	Engine            string `json:"engine" form:"engine"`
	Transport         string `json:"transport" form:"transport"`
	BackhaulPort      int    `json:"backhaulPort" form:"backhaulPort"`
	Snappy            bool   `json:"snappy" form:"snappy"`
	IranIP            string `json:"iranIp" form:"iranIp"`
	SSHPort           int    `json:"sshPort" form:"sshPort"`
	SSHUser           string `json:"sshUser" form:"sshUser"`
	SSHPassword       string `json:"sshPassword" form:"sshPassword"`
	Ports             string `json:"ports" form:"ports"`
	Carrier           string `json:"carrier" form:"carrier"`
	AutoCreateInbound bool   `json:"autoCreateInbound" form:"autoCreateInbound"`
	InboundHost       string `json:"inboundHost" form:"inboundHost"`
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
	Engine            string `json:"engine" form:"engine"`
	Transport         string `json:"transport" form:"transport"`
	BackhaulPort      int    `json:"backhaulPort" form:"backhaulPort"`
	Snappy            bool   `json:"snappy" form:"snappy"`
	IranIP            string `json:"iranIp" form:"iranIp"`
	Ports             string `json:"ports" form:"ports"`
	Carrier           string `json:"carrier" form:"carrier"`
	AutoCreateInbound bool   `json:"autoCreateInbound" form:"autoCreateInbound"`
	InboundHost       string `json:"inboundHost" form:"inboundHost"`
}

type HashemOneLinerResult struct {
	OneLinerCommand string `json:"oneLinerCommand"`
	ForeignIP       string `json:"foreignIp"`
	IranIP          string `json:"iranIp"`
	Ports           string `json:"ports"`
	Engine          string `json:"engine"`
	Transport       string `json:"transport"`
	FrpPort         int    `json:"frpPort"`
	BackhaulPort    int    `json:"backhaulPort"`
	Token           string `json:"token"`
}

type HashemService struct {
	inboundService InboundService
	xrayService    XrayService
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

	frpConn := false
	if status.RemoteGreIP != "" && status.FrpPort > 0 {
		frpConn = s.isFrpConnected(status.RemoteGreIP, status.FrpPort)
	}

	if status.RemoteGreIP != "" {
		status.PingMs = s.measurePing(status.RemoteGreIP, "gre-tunnel", status.FrpPort)
		// Upstream Hashem fix: treat ICMP-filtered ping as WARN not FAIL when frp is active/connected
		if status.PingMs <= 0 && !frpConn {
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
			if status.PingMs > 0 || frpConn {
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
			if status.PingMs > 0 || frpConn {
				status.FrpStatus = "active"
			} else {
				status.FrpStatus = "connecting"
			}
		} else {
			status.FrpStatus = baseStatus
		}
	}

	// Detect Engine (frp / backhaul / gre-backhaul)
	status.Engine = "frp"
	status.Transport = "tcpmux"
	status.BackhaulPort = 3080

	bhClientActive := s.checkServiceStatus("backhaul-client") == "active"
	bhServerActive := s.checkServiceStatus("backhaul-server") == "active"
	bhClientExists := fileExists(backhaulClientTomlPath)
	bhServerExists := fileExists(backhaulServerTomlPath)

	if bhClientActive || bhServerActive || bhClientExists || bhServerExists {
		if status.Running {
			status.Engine = "gre-backhaul"
		} else {
			status.Engine = "backhaul"
		}
		if bhClientActive || bhClientExists {
			status.Role = "foreign"
			s.parseBackhaulConfig(backhaulClientTomlPath, status)
			if bhClientActive {
				status.BackhaulStatus = "active"
				status.FrpStatus = "active"
				status.Running = true
			} else {
				status.BackhaulStatus = s.checkServiceStatus("backhaul-client")
			}
		} else if bhServerActive || bhServerExists {
			status.Role = "iran"
			s.parseBackhaulConfig(backhaulServerTomlPath, status)
			if bhServerActive {
				status.BackhaulStatus = "active"
				status.FrpStatus = "active"
				status.Running = true
			} else {
				status.BackhaulStatus = s.checkServiceStatus("backhaul-server")
			}
		}
	}

	if token == "" && status.BackhaulToken != "" {
		token = status.BackhaulToken
	}

	// AutoPilot & Benchmark report
	if rep, err := s.GetBenchmark(); err == nil && rep != nil {
		status.Benchmark = rep
		status.AutoPilot = rep.AutoPilot.Enabled
	}

	if status.Role == "foreign" && status.RemotePubIP != "" && token != "" {
		if len(status.Ports) == 0 {
			if inbounds, err := s.inboundService.GetInboundsForScope(InboundAccessScope{All: true}); err == nil {
				for _, in := range inbounds {
					if in.Enable && in.Port > 0 {
						status.Ports = append(status.Ports, in.Port)
					}
				}
				sort.Ints(status.Ports)
			}
		}

		portsStr := ""
		rawPortsStr := ""
		if len(status.Ports) > 0 {
			var pstrs []string
			for _, p := range status.Ports {
				pstrs = append(pstrs, strconv.Itoa(p))
			}
			portsStr = strings.Join(pstrs, "-")
			rawPortsStr = strings.Join(pstrs, ",")
		}
		if status.Engine == "backhaul" {
			status.Bundle = fmt.Sprintf("bh1_%s_%d_%s_%s_%s",
				status.RemotePubIP, status.BackhaulPort, status.Transport, token, portsStr)
			status.SetupCommand = fmt.Sprintf(
				"curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh | bash -s -- setup-backhaul-iran --local-pub %s --remote-pub %s --port %d --transport %s --token %s --ports \"%s\"",
				status.RemotePubIP, status.LocalPubIP, status.BackhaulPort, status.Transport, token, rawPortsStr,
			)
		} else if status.Engine == "gre-backhaul" {
			status.Bundle = fmt.Sprintf("gh1_%s_%d_%s_%s_%s_%s_%s",
				status.RemotePubIP, status.BackhaulPort, status.RemoteGreIP, status.LocalGreIP, status.Transport, token, portsStr)
			status.SetupCommand = fmt.Sprintf(
				"curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh | bash -s -- setup-gre-backhaul-iran --local-pub %s --remote-pub %s --port %d --transport %s --token %s --ports \"%s\"",
				status.RemotePubIP, status.LocalPubIP, status.BackhaulPort, status.Transport, token, rawPortsStr,
			)
		} else {
			status.Bundle = fmt.Sprintf("hsh1_%s_%d_%s_%s_%s_%s",
				status.RemotePubIP, status.FrpPort, status.RemoteGreIP, status.LocalGreIP, token, portsStr)
			status.SetupCommand = fmt.Sprintf(
				"curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh | bash -s -- setup-iran --local-pub %s --remote-pub %s --frp-port %d --token %s",
				status.RemotePubIP, status.LocalPubIP, status.FrpPort, token,
			)
		}
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

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (s *HashemService) parseBackhaulConfig(path string, status *HashemStatus) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	inPorts := false
	portMap := make(map[int]bool)
	for _, p := range status.Ports {
		portMap[p] = true
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "transport") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				status.Transport = strings.Trim(strings.TrimSpace(parts[1]), `"`)
			}
		}
		if strings.HasPrefix(line, "remote_addr") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				addr := strings.Trim(strings.TrimSpace(parts[1]), `"`)
				h, p, err := net.SplitHostPort(addr)
				if err == nil {
					status.RemotePubIP = h
					if portNum, err := strconv.Atoi(p); err == nil {
						status.BackhaulPort = portNum
					}
				}
			}
		}
		if strings.HasPrefix(line, "bind_addr") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				addr := strings.Trim(strings.TrimSpace(parts[1]), `"`)
				_, p, err := net.SplitHostPort(addr)
				if err == nil {
					if portNum, err := strconv.Atoi(p); err == nil {
						status.BackhaulPort = portNum
					}
				}
			}
		}
		if strings.HasPrefix(line, "token") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				status.BackhaulToken = strings.Trim(strings.TrimSpace(parts[1]), `"`)
			}
		}
		if strings.HasPrefix(line, "snappy") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				status.Snappy = strings.TrimSpace(parts[1]) == "true"
			}
		}
		if strings.HasPrefix(line, "ports") && strings.Contains(line, "[") {
			inPorts = true
		}
		if inPorts {
			re := regexp.MustCompile(`"(\d+)(?:[=:-].*?)?"`)
			matches := re.FindAllStringSubmatch(line, -1)
			for _, m := range matches {
				if len(m) > 1 {
					if p, err := strconv.Atoi(m[1]); err == nil && !portMap[p] {
						portMap[p] = true
						status.Ports = append(status.Ports, p)
					}
				}
			}
			if strings.Contains(line, "]") {
				inPorts = false
			}
		}
	}
	sort.Ints(status.Ports)
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

	engine := strings.TrimSpace(strings.ToLower(form.Engine))
	transport := strings.TrimSpace(strings.ToLower(form.Transport))
	if transport == "" {
		transport = "tcpmux"
	}
	bhPort := form.BackhaulPort
	if bhPort <= 0 {
		bhPort = 3080
	}

	args := []string{}
	if form.Bundle != "" {
		if strings.HasPrefix(form.Bundle, "bh1_") {
			args = []string{"setup-backhaul-foreign", "--bundle", form.Bundle, "--force"}
		} else if strings.HasPrefix(form.Bundle, "gh1_") {
			args = []string{"setup-gre-backhaul-foreign", "--bundle", form.Bundle, "--force"}
		} else {
			args = []string{"setup-foreign", "--bundle", form.Bundle, "--force"}
		}
	} else if engine == "backhaul" {
		if form.Role == "iran" {
			args = []string{"setup-backhaul-iran", "--local-pub", form.LocalPub, "--remote-pub", form.RemotePub, "--port", strconv.Itoa(bhPort), "--transport", transport, "--force"}
			if form.Token != "" {
				args = append(args, "--token", form.Token)
			}
			if form.Ports != "" {
				args = append(args, "--ports", form.Ports)
			}
		} else {
			args = []string{"setup-backhaul-foreign", "--local-pub", form.LocalPub, "--remote-pub", form.RemotePub, "--port", strconv.Itoa(bhPort), "--transport", transport, "--force"}
			if form.Token != "" {
				args = append(args, "--token", form.Token)
			}
		}
	} else if engine == "gre-backhaul" {
		if form.Role == "iran" {
			args = []string{"setup-gre-backhaul-iran", "--local-pub", form.LocalPub, "--remote-pub", form.RemotePub, "--port", strconv.Itoa(bhPort), "--transport", transport, "--force"}
			if form.Token != "" {
				args = append(args, "--token", form.Token)
			}
			if form.Ports != "" {
				args = append(args, "--ports", form.Ports)
			}
		} else {
			args = []string{"setup-gre-backhaul-foreign", "--local-pub", form.LocalPub, "--remote-pub", form.RemotePub, "--port", strconv.Itoa(bhPort), "--transport", transport, "--force"}
			if form.Token != "" {
				args = append(args, "--token", form.Token)
			}
		}
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

	if form.AutoCreateInbound && (form.Role == "" || form.Role == "foreign") {
		inHost := strings.TrimSpace(form.InboundHost)
		if inHost == "" {
			inHost = form.RemotePub
		}
		_ = s.AutoCreateMatchingInbounds(form.Ports, inHost)
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
	if err != nil || (!status.Installed) {
		return nil, fmt.Errorf("hashem tunnel not configured")
	}

	// Dynamic zero-downtime port sync via EditPorts
	if err := s.EditPorts(ports); err == nil {
		return ports, nil
	}

	// Fallback to setup-foreign if toml direct edit was not applicable
	if status.Role == "foreign" && status.LocalPubIP != "" && status.RemotePubIP != "" && status.FrpPort > 0 {
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

	return nil, fmt.Errorf("hashem tunnel not configured as foreign client")
}

// EditPorts dynamically updates forwarded ports in frpc/frps without dropping the tunnel.
func (s *HashemService) EditPorts(ports []int) error {
	if len(ports) == 0 {
		return errors.New("ports list is empty")
	}

	portMap := make(map[int]bool)
	var cleanPorts []int
	for _, p := range ports {
		if p < 1 || p > 65535 {
			return fmt.Errorf("invalid port %d", p)
		}
		if !portMap[p] {
			portMap[p] = true
			cleanPorts = append(cleanPorts, p)
		}
	}
	sort.Ints(cleanPorts)

	// 1. Try hashem CLI edit-peer-ports if available
	var portStrs []string
	for _, p := range cleanPorts {
		portStrs = append(portStrs, strconv.Itoa(p))
	}
	portsArg := strings.Join(portStrs, ",")

	if _, err := os.Stat(hashemBinPath); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, hashemBinPath, "edit-peer-ports", "--id", "1", "--ports", portsArg)
		cmd.Env = append(os.Environ(), "GRE_SKIP_PANEL=1")
		if _, err := cmd.CombinedOutput(); err == nil {
			return nil
		}
	}

	// 2. Direct edit: update peers.json + rewrite toml + reload frp
	editedAny := false

	peersPath := "/etc/gre-panel/peers.json"
	if data, err := os.ReadFile(peersPath); err == nil {
		var doc struct {
			Peers []map[string]any `json:"peers"`
		}
		if err := json.Unmarshal(data, &doc); err == nil && len(doc.Peers) > 0 {
			for i := range doc.Peers {
				doc.Peers[i]["ports"] = cleanPorts
			}
			if outData, err := json.MarshalIndent(doc, "", "  "); err == nil {
				_ = os.WriteFile(peersPath, append(outData, '\n'), 0644)
				editedAny = true
			}
		}
	}

	// Foreign client toml (/etc/frp/frpc.toml)
	if rawToml, err := os.ReadFile(frpcTomlPath); err == nil {
		updated := rewriteTomlPorts(string(rawToml), cleanPorts)
		if err := os.WriteFile(frpcTomlPath, []byte(updated), 0644); err == nil {
			exec.Command("systemctl", "reload-or-restart", "frpc").CombinedOutput()
			editedAny = true
		}
	}

	// Server toml (/etc/frp/frps.toml)
	if rawToml, err := os.ReadFile(frpsTomlPath); err == nil {
		if strings.Contains(string(rawToml), "[[proxies]]") {
			updated := rewriteTomlPorts(string(rawToml), cleanPorts)
			_ = os.WriteFile(frpsTomlPath, []byte(updated), 0644)
		}
		exec.Command("systemctl", "reload-or-restart", "frps").CombinedOutput()
		editedAny = true
	}

	// Backhaul client toml (/etc/backhaul/client.toml)
	if rawToml, err := os.ReadFile(backhaulClientTomlPath); err == nil {
		updated := rewriteBackhaulPorts(string(rawToml), cleanPorts)
		if err := os.WriteFile(backhaulClientTomlPath, []byte(updated), 0644); err == nil {
			exec.Command("systemctl", "restart", "backhaul-client").CombinedOutput()
			editedAny = true
		}
	}

	// Backhaul server toml (/etc/backhaul/config.toml)
	if rawToml, err := os.ReadFile(backhaulServerTomlPath); err == nil {
		updated := rewriteBackhaulPorts(string(rawToml), cleanPorts)
		if err := os.WriteFile(backhaulServerTomlPath, []byte(updated), 0644); err == nil {
			exec.Command("systemctl", "restart", "backhaul-server").CombinedOutput()
			editedAny = true
		}
	}

	// Allow UFW ports if active
	if out, err := exec.Command("ufw", "status").CombinedOutput(); err == nil && strings.Contains(string(out), "Status: active") {
		for _, p := range cleanPorts {
			exec.Command("ufw", "allow", fmt.Sprintf("%d/tcp", p)).CombinedOutput()
			exec.Command("ufw", "allow", fmt.Sprintf("%d/udp", p)).CombinedOutput()
		}
	}

	if !editedAny {
		return errors.New("neither frp nor backhaul configuration could be found or updated")
	}

	return nil
}

func rewriteBackhaulPorts(content string, ports []int) string {
	re := regexp.MustCompile(`(?s)ports\s*=\s*\[[^\]]*\]`)
	var pstrs []string
	for _, p := range ports {
		pstrs = append(pstrs, fmt.Sprintf("    \"%d\",", p))
	}
	replacement := fmt.Sprintf("ports = [\n%s\n]", strings.Join(pstrs, "\n"))
	if re.MatchString(content) {
		return re.ReplaceAllString(content, replacement)
	}
	return content + "\n" + replacement + "\n"
}

// rewriteTomlPorts rebuilds the [[proxies]] sections of an frps/frpc TOML file
// with the new port list, preserving configuration headers intact.
func rewriteTomlPorts(src string, ports []int) string {
	var header strings.Builder
	inProxy := false
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[[proxies]]") {
			inProxy = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "[[proxies]]") {
			inProxy = false
		}
		if !inProxy {
			header.WriteString(line)
			header.WriteByte('\n')
		}
	}
	result := strings.TrimRight(header.String(), "\n") + "\n"
	for _, port := range ports {
		result += fmt.Sprintf("\n[[proxies]]\nname = \"tcp_%d\"\ntype = \"tcp\"\nlocalIP = \"127.0.0.1\"\nlocalPort = %d\nremotePort = %d\n\n[[proxies]]\nname = \"udp_%d\"\ntype = \"udp\"\nlocalIP = \"127.0.0.1\"\nlocalPort = %d\nremotePort = %d\n", port, port, port, port, port, port)
	}
	return result
}

func (s *HashemService) Install() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", "curl -sL --max-time 15 https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh | bash || curl -sL --max-time 15 https://ghfast.top/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh | bash")
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

func (s *HashemService) parsePorts(portsStr string) []int {
	var result []int
	seen := make(map[int]bool)
	for _, p := range strings.Split(portsStr, ",") {
		p = strings.TrimSpace(p)
		if port, err := strconv.Atoi(p); err == nil && port > 0 && port <= 65535 {
			if !seen[port] {
				seen[port] = true
				result = append(result, port)
			}
		}
	}
	return result
}

func (s *HashemService) AutoCreateMatchingInbounds(portsStr string, host string) error {
	ports := s.parsePorts(portsStr)
	if len(ports) == 0 {
		return nil
	}

	host = strings.TrimSpace(host)
	if host == "" {
		host = s.getForeignPubIP()
	}

	existingInbounds, _ := s.inboundService.GetInboundsForScope(InboundAccessScope{All: true})
	existingMap := make(map[int]*model.Inbound)
	var templateClients []model.Client

	for _, ib := range existingInbounds {
		existingMap[ib.Port] = ib
		if len(templateClients) == 0 && ib.Protocol == model.VLESS {
			if cls, err := s.inboundService.GetClients(ib); err == nil && len(cls) > 0 {
				templateClients = cls
			}
		}
	}

	createdAny := false

	for _, port := range ports {
		if existing, exists := existingMap[port]; exists {
			if existing.Protocol == model.VLESS {
				var stream map[string]any
				if err := json.Unmarshal([]byte(existing.StreamSettings), &stream); err == nil && stream != nil {
					if stream["network"] == "ws" {
						wsSettings, ok := stream["wsSettings"].(map[string]any)
						if !ok || wsSettings == nil {
							wsSettings = make(map[string]any)
						}
						wsSettings["host"] = host
						if _, ok := wsSettings["path"]; !ok || wsSettings["path"] == "" {
							wsSettings["path"] = "/@DARK_VVPN"
						}
						stream["wsSettings"] = wsSettings
						if b, err := json.Marshal(stream); err == nil {
							existing.StreamSettings = string(b)
							_, _, _ = s.inboundService.UpdateInbound(existing)
							createdAny = true
						}
					}
				}
			}
			continue
		}

		clientsToUse := templateClients
		if len(clientsToUse) == 0 {
			now := time.Now().Unix() * 1000
			clientsToUse = []model.Client{
				{
					ID:        uuid.NewString(),
					Email:     fmt.Sprintf("dark_vip_%d", port),
					SubID:     uuid.NewString(),
					Enable:    true,
					CreatedAt: now,
					UpdatedAt: now,
				},
			}
		}

		settingsMap := map[string]any{
			"clients":    clientsToUse,
			"decryption": "none",
			"fallbacks":  []any{},
		}
		settingsBytes, _ := json.Marshal(settingsMap)

		streamMap := map[string]any{
			"network":  "ws",
			"security": "none",
			"wsSettings": map[string]any{
				"acceptProxyProtocol": false,
				"path":                "/@DARK_VVPN",
				"host":                host,
				"headers":             map[string]any{},
				"heartbeatPeriod":     0,
			},
		}
		streamBytes, _ := json.Marshal(streamMap)

		sniffingMap := map[string]any{
			"enabled": false,
		}
		sniffingBytes, _ := json.Marshal(sniffingMap)

		newInbound := &model.Inbound{
			Enable:            true,
			Protocol:          model.VLESS,
			Port:              port,
			Tag:               fmt.Sprintf("in-%d-tcp", port),
			Remark:            "⚡️┃𝑫𝑨𝑹𝑲_𝑽𝑽𝑷𝑵┃",
			Listen:            "",
			Settings:          string(settingsBytes),
			StreamSettings:    string(streamBytes),
			Sniffing:          string(sniffingBytes),
			ShareAddrStrategy: "listen",
		}

		_, _, err := s.inboundService.AddInbound(newInbound)
		if err == nil {
			createdAny = true
		} else {
			logger.Warningf("AutoCreateMatchingInbounds failed for port %d: %v", port, err)
		}
	}

	if createdAny {
		_ = s.xrayService.RestartXray(false)
	}

	return nil
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
	engine := strings.TrimSpace(strings.ToLower(form.Engine))
	if engine == "" {
		engine = "frp"
	}
	transport := strings.TrimSpace(strings.ToLower(form.Transport))
	if transport == "" {
		transport = "tcpmux"
	}
	bhPort := form.BackhaulPort
	if bhPort <= 0 {
		bhPort = 3080
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

	peerJsonCmd, optimizeCmd, _ := buildIranSetupCommands(iranIP, foreignIP, frpPort, bhPort, token, carrier, ports, engine, transport)

	var iranCmd string
	if engine == "backhaul" {
		iranCmd = fmt.Sprintf(
			"export DEBIAN_FRONTEND=noninteractive; "+
				"if [ ! -s /tmp/hashem.sh ]; then "+
				"  curl -fsSL https://fastly.jsdelivr.net/gh/pdnczone/hashem-panel/hashem.sh -o /tmp/hashem.sh 2>/dev/null || "+
				"  curl -fsSL https://ghproxy.net/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || "+
				"  curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh; "+
				"  chmod +x /tmp/hashem.sh; "+
				"fi; "+
				"bash /tmp/hashem.sh setup-backhaul-iran --local-pub %s --remote-pub %s --port %d --transport %s --token %s --ports \"%s\" --force && rm -f /tmp/hashem.sh",
			iranIP, foreignIP, bhPort, transport, token, ports,
		)
	} else if engine == "gre-backhaul" {
		iranCmd = fmt.Sprintf(
			"export DEBIAN_FRONTEND=noninteractive; "+
				"if [ ! -s /tmp/hashem.sh ]; then "+
				"  curl -fsSL https://fastly.jsdelivr.net/gh/pdnczone/hashem-panel/hashem.sh -o /tmp/hashem.sh 2>/dev/null || "+
				"  curl -fsSL https://ghproxy.net/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || "+
				"  curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh; "+
				"  chmod +x /tmp/hashem.sh; "+
				"fi; "+
				"bash /tmp/hashem.sh setup-gre-backhaul-iran --local-pub %s --remote-pub %s --port %d --transport %s --token %s --ports \"%s\" --force && "+
				"bash /tmp/hashem.sh carrier mode %s && "+
				"%s; rm -f /tmp/hashem.sh",
			iranIP, foreignIP, bhPort, transport, token, ports, carrier, optimizeCmd,
		)
	} else {
		iranCmd = fmt.Sprintf(
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
	}

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

	var cmdForeign *exec.Cmd
	if engine == "backhaul" {
		cmdForeign = exec.CommandContext(ctx, hashemBinPath, "setup-backhaul-foreign",
			"--local-pub", foreignIP,
			"--remote-pub", iranIP,
			"--port", strconv.Itoa(bhPort),
			"--transport", transport,
			"--token", token,
			"--force",
		)
	} else if engine == "gre-backhaul" {
		cmdForeign = exec.CommandContext(ctx, hashemBinPath, "setup-gre-backhaul-foreign",
			"--local-pub", foreignIP,
			"--remote-pub", iranIP,
			"--port", strconv.Itoa(bhPort),
			"--transport", transport,
			"--token", token,
			"--force",
		)
	} else {
		cmdForeign = exec.CommandContext(ctx, hashemBinPath, "setup-foreign",
			"--local-pub", foreignIP,
			"--remote-pub", iranIP,
			"--frp-port", strconv.Itoa(frpPort),
			"--token", token,
			"--ports", ports,
			"--force",
		)
	}
	fOut, _ := cmdForeign.CombinedOutput()

	if engine != "backhaul" {
		cmdCarrier := exec.CommandContext(ctx, hashemBinPath, "carrier", "mode", carrier)
		_ = cmdCarrier.Run()
		s.optimizeForeignNetwork(ports)
	}

	if form.AutoCreateInbound {
		inHost := strings.TrimSpace(form.InboundHost)
		if inHost == "" {
			inHost = iranIP
		}
		_ = s.AutoCreateMatchingInbounds(ports, inHost)
	}

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

func buildIranSetupCommands(iranIP, foreignIP string, frpPort, bhPort int, token, carrier, ports, engine, transport string) (string, string, string) {
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

	var oneLiner string
	if engine == "backhaul" {
		oneLiner = fmt.Sprintf(
			"curl -fsSL https://fastly.jsdelivr.net/gh/pdnczone/hashem-panel/hashem.sh -o /tmp/hashem.sh 2>/dev/null || curl -fsSL https://ghproxy.net/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh; bash /tmp/hashem.sh setup-backhaul-iran --local-pub %s --remote-pub %s --port %d --transport %s --token %s --ports \"%s\" --force && rm -f /tmp/hashem.sh",
			iranIP, foreignIP, bhPort, transport, token, ports,
		)
	} else if engine == "gre-backhaul" {
		oneLiner = fmt.Sprintf(
			"curl -fsSL https://fastly.jsdelivr.net/gh/pdnczone/hashem-panel/hashem.sh -o /tmp/hashem.sh 2>/dev/null || curl -fsSL https://ghproxy.net/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh; bash /tmp/hashem.sh setup-gre-backhaul-iran --local-pub %s --remote-pub %s --port %d --transport %s --token %s --ports \"%s\" --force && bash /tmp/hashem.sh carrier mode %s && %s && rm -f /tmp/hashem.sh",
			iranIP, foreignIP, bhPort, transport, token, ports, carrier, optimizeCmd,
		)
	} else {
		oneLiner = fmt.Sprintf(
			"curl -fsSL https://fastly.jsdelivr.net/gh/pdnczone/hashem-panel/hashem.sh -o /tmp/hashem.sh 2>/dev/null || curl -fsSL https://ghproxy.net/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh; bash /tmp/hashem.sh setup-iran --local-pub %s --remote-pub %s --frp-port %d --token %s --force && bash /tmp/hashem.sh carrier mode %s && %s && %s && rm -f /tmp/hashem.sh",
			iranIP, foreignIP, frpPort, token, carrier, peerJsonCmd, optimizeCmd,
		)
	}

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
	engine := strings.TrimSpace(strings.ToLower(form.Engine))
	if engine == "" {
		engine = "frp"
	}
	transport := strings.TrimSpace(strings.ToLower(form.Transport))
	if transport == "" {
		transport = "tcpmux"
	}
	bhPort := form.BackhaulPort
	if bhPort <= 0 {
		bhPort = 3080
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

	var cmdForeign *exec.Cmd
	if engine == "backhaul" {
		cmdForeign = exec.CommandContext(ctx, hashemBinPath, "setup-backhaul-foreign",
			"--local-pub", foreignIP,
			"--remote-pub", iranIP,
			"--port", strconv.Itoa(bhPort),
			"--transport", transport,
			"--token", token,
			"--force",
		)
	} else if engine == "gre-backhaul" {
		cmdForeign = exec.CommandContext(ctx, hashemBinPath, "setup-gre-backhaul-foreign",
			"--local-pub", foreignIP,
			"--remote-pub", iranIP,
			"--port", strconv.Itoa(bhPort),
			"--transport", transport,
			"--token", token,
			"--force",
		)
	} else {
		cmdForeign = exec.CommandContext(ctx, hashemBinPath, "setup-foreign",
			"--local-pub", foreignIP,
			"--remote-pub", iranIP,
			"--frp-port", strconv.Itoa(frpPort),
			"--token", token,
			"--ports", ports,
			"--force",
		)
	}
	_ = cmdForeign.Run()

	if engine != "backhaul" {
		cmdCarrier := exec.CommandContext(ctx, hashemBinPath, "carrier", "mode", carrier)
		_ = cmdCarrier.Run()
		s.optimizeForeignNetwork(ports)
	}

	if form.AutoCreateInbound {
		inHost := strings.TrimSpace(form.InboundHost)
		if inHost == "" {
			inHost = iranIP
		}
		_ = s.AutoCreateMatchingInbounds(ports, inHost)
	}

	_, _, oneLiner := buildIranSetupCommands(iranIP, foreignIP, frpPort, bhPort, token, carrier, ports, engine, transport)

	return &HashemOneLinerResult{
		OneLinerCommand: oneLiner,
		ForeignIP:       foreignIP,
		IranIP:          iranIP,
		Ports:           ports,
		Engine:          engine,
		Transport:       transport,
		FrpPort:         frpPort,
		BackhaulPort:    bhPort,
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

	// 2. Stop and disable Backhaul services
	_ = exec.CommandContext(ctx, "systemctl", "stop", "backhaul-client").Run()
	_ = exec.CommandContext(ctx, "systemctl", "disable", "backhaul-client").Run()
	_ = exec.CommandContext(ctx, "systemctl", "stop", "backhaul-server").Run()
	_ = exec.CommandContext(ctx, "systemctl", "disable", "backhaul-server").Run()
	_ = os.Remove("/etc/systemd/system/backhaul-client.service")
	_ = os.Remove("/etc/systemd/system/backhaul-server.service")
	_ = os.RemoveAll("/etc/backhaul")

	// 3. Delete GRE tunnel interface
	_ = exec.CommandContext(ctx, "ip", "link", "set", "gre-tunnel", "down").Run()
	_ = exec.CommandContext(ctx, "ip", "link", "del", "gre-tunnel").Run()

	// 4. Remove FoU listeners
	_ = exec.CommandContext(ctx, "ip", "fou", "del", "port", "443").Run()
	_ = exec.CommandContext(ctx, "ip", "fou", "del", "port", "19998").Run()
	_ = exec.CommandContext(ctx, "ip", "fou", "del", "port", "55555").Run()

	// 5. Remove gre-panel config directory
	_ = os.RemoveAll("/etc/gre-panel")

	// 6. Reload systemd daemon
	_ = exec.CommandContext(ctx, "systemctl", "daemon-reload").Run()

	logger.Info("Hashem tunnel removed completely from host")
	return nil
}

// Carrier Benchmark & Auto-Pilot Implementation

func (s *HashemService) GetBenchmark() (*BenchmarkReport, error) {
	if data, err := os.ReadFile(benchmarkReportPath); err == nil {
		var rep BenchmarkReport
		if err := json.Unmarshal(data, &rep); err == nil {
			return &rep, nil
		}
	}
	return nil, nil
}

func (s *HashemService) SetAutoPilot(enabled bool) error {
	var cc CarrierConfig
	if data, err := os.ReadFile(carrierJsonPath); err == nil {
		_ = json.Unmarshal(data, &cc)
	}
	cc.AutoPilot = enabled
	_ = os.MkdirAll("/etc/gre-panel", 0755)
	if data, err := json.MarshalIndent(cc, "", "  "); err == nil {
		_ = os.WriteFile(carrierJsonPath, data, 0644)
	}

	rep, _ := s.GetBenchmark()
	if rep != nil {
		rep.AutoPilot.Enabled = enabled
		if enabled {
			rep.AutoPilot.LastTriggered = time.Now().Format("2006-01-02 15:04:05")
		}
		if data, err := json.MarshalIndent(rep, "", "  "); err == nil {
			_ = os.WriteFile(benchmarkReportPath, data, 0644)
		}
	}
	return nil
}

func (s *HashemService) RunBenchmark() (*BenchmarkReport, error) {
	status, _ := s.GetStatus()
	targetPub := ""
	if status != nil {
		targetPub = status.RemotePubIP
	}
	if targetPub == "" {
		targetPub = "127.0.0.1"
	}
	targetGre := ""
	if status != nil {
		targetGre = status.RemoteGreIP
	}
	if targetGre == "" {
		targetGre = "10.10.10.2"
	}

	start := time.Now()
	var metrics []CarrierMetric

	// 1. Direct GRE
	avg, minR, maxR, loss, jit, err := probeTCP(net.JoinHostPort(targetGre, "8080"), 3, 300*time.Millisecond)
	if err != nil {
		avg, minR, maxR, loss, jit, _ = probePing(targetGre, 3)
	}
	sc := calculateScore(loss, avg, jit)
	metrics = append(metrics, CarrierMetric{
		ID:            "direct",
		Name:          "Direct GRE (Layer-3)",
		Type:          "gre",
		Port:          0,
		AvgRTTMs:      math.Round(avg*10) / 10,
		MinRTTMs:      math.Round(minR*10) / 10,
		MaxRTTMs:      math.Round(maxR*10) / 10,
		PacketLoss:    math.Round(loss*10) / 10,
		JitterMs:      math.Round(jit*10) / 10,
		Score:         sc,
		Status:        determineStatus(sc, loss),
		IsActive:      status != nil && (status.ActiveCarrier == "direct" || status.Carrier == "direct"),
	})

	// 2. FoU 443
	avg, minR, maxR, loss, jit, _ = probeTCP(net.JoinHostPort(targetPub, "443"), 3, 300*time.Millisecond)
	sc = calculateScore(loss, avg, jit)
	metrics = append(metrics, CarrierMetric{
		ID:            "fou:443",
		Name:          "FoU : 443 (Foo-over-UDP)",
		Type:          "fou",
		Port:          443,
		AvgRTTMs:      math.Round(avg*10) / 10,
		MinRTTMs:      math.Round(minR*10) / 10,
		MaxRTTMs:      math.Round(maxR*10) / 10,
		PacketLoss:    math.Round(loss*10) / 10,
		JitterMs:      math.Round(jit*10) / 10,
		Score:         sc,
		Status:        determineStatus(sc, loss),
		IsActive:      status != nil && (status.ActiveCarrier == "fou:443" || status.Carrier == "fou:443"),
	})

	// 3. FoU 55555
	avg, minR, maxR, loss, jit, _ = probeTCP(net.JoinHostPort(targetPub, "55555"), 3, 300*time.Millisecond)
	sc = calculateScore(loss, avg, jit)
	metrics = append(metrics, CarrierMetric{
		ID:            "fou:55555",
		Name:          "FoU : 55555 (Foo-over-UDP High)",
		Type:          "fou",
		Port:          55555,
		AvgRTTMs:      math.Round(avg*10) / 10,
		MinRTTMs:      math.Round(minR*10) / 10,
		MaxRTTMs:      math.Round(maxR*10) / 10,
		PacketLoss:    math.Round(loss*10) / 10,
		JitterMs:      math.Round(jit*10) / 10,
		Score:         sc,
		Status:        determineStatus(sc, loss),
		IsActive:      status != nil && (status.ActiveCarrier == "fou:55555" || status.Carrier == "fou:55555"),
	})

	// 4. WSS 8443
	avg, minR, maxR, loss, jit, _ = probeTLS(net.JoinHostPort(targetPub, "8443"), 3, 350*time.Millisecond)
	sc = calculateScore(loss, avg, jit)
	metrics = append(metrics, CarrierMetric{
		ID:            "wss:8443",
		Name:          "WSS : 8443 (Encrypted WebSocket)",
		Type:          "wss",
		Port:          8443,
		AvgRTTMs:      math.Round(avg*10) / 10,
		MinRTTMs:      math.Round(minR*10) / 10,
		MaxRTTMs:      math.Round(maxR*10) / 10,
		PacketLoss:    math.Round(loss*10) / 10,
		JitterMs:      math.Round(jit*10) / 10,
		Score:         sc,
		Status:        determineStatus(sc, loss),
		IsActive:      status != nil && (status.ActiveCarrier == "wss:8443" || status.Carrier == "wss:8443"),
	})

	// 5. Backhaul Mux
	bhPort := 3080
	if status != nil && status.BackhaulPort > 0 {
		bhPort = status.BackhaulPort
	}
	avg, minR, maxR, loss, jit, _ = probeTCP(net.JoinHostPort(targetPub, strconv.Itoa(bhPort)), 3, 300*time.Millisecond)
	sc = calculateScore(loss, avg, jit)
	metrics = append(metrics, CarrierMetric{
		ID:            "backhaul:tcp",
		Name:          fmt.Sprintf("Backhaul Mux (Port %d)", bhPort),
		Type:          "backhaul",
		Port:          bhPort,
		AvgRTTMs:      math.Round(avg*10) / 10,
		MinRTTMs:      math.Round(minR*10) / 10,
		MaxRTTMs:      math.Round(maxR*10) / 10,
		PacketLoss:    math.Round(loss*10) / 10,
		JitterMs:      math.Round(jit*10) / 10,
		Score:         sc,
		Status:        determineStatus(sc, loss),
		IsActive:      status != nil && (status.Engine == "backhaul" || status.Engine == "gre-backhaul"),
	})

	// Choose Best Carrier
	bestIdx := -1
	bestScore := 0
	for i, m := range metrics {
		if m.Score > bestScore {
			bestScore = m.Score
			bestIdx = i
		}
	}
	if bestIdx >= 0 {
		metrics[bestIdx].IsRecommended = true
	}

	duration := time.Since(start).Seconds()

	autoPilotEnabled := false
	if data, err := os.ReadFile(carrierJsonPath); err == nil {
		var cc struct {
			AutoPilot bool `json:"auto_pilot"`
		}
		if json.Unmarshal(data, &cc) == nil {
			autoPilotEnabled = cc.AutoPilot
		}
	}

	report := &BenchmarkReport{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		DurationSec:    math.Round(duration*100) / 100,
		PeerInternalIP: targetGre,
		ActiveCarrier:  "",
		BestCarrier:    metrics[bestIdx].ID,
		AutoPilot: AutoPilotStatus{
			Enabled:       autoPilotEnabled,
			ThresholdLoss: 20.0,
			LastTriggered: time.Now().Format("2006-01-02 15:04:05"),
		},
		Metrics: metrics,
	}
	if status != nil {
		report.ActiveCarrier = status.ActiveCarrier
	}

	_ = os.MkdirAll("/etc/gre-panel", 0755)
	if data, err := json.MarshalIndent(report, "", "  "); err == nil {
		_ = os.WriteFile(benchmarkReportPath, data, 0644)
	}

	// Auto-pilot trigger
	if autoPilotEnabled && status != nil && status.ActiveCarrier != report.BestCarrier && metrics[bestIdx].Score > 50 {
		_ = s.SetCarrier(report.BestCarrier)
	}

	return report, nil
}

func calculateScore(loss, rtt, jitter float64) int {
	if loss >= 100.0 {
		return 0
	}
	effectiveRTT := rtt
	if effectiveRTT > 300 {
		effectiveRTT = 300
	}
	effectiveJitter := jitter
	if effectiveJitter > 100 {
		effectiveJitter = 100
	}

	raw := 100.0 - (loss * 1.5) - (effectiveRTT * 0.15) - (effectiveJitter * 0.2)
	if raw < 1 {
		raw = 1
	}
	if raw > 100 {
		raw = 100
	}
	return int(math.Round(raw))
}

func determineStatus(score int, loss float64) string {
	if loss >= 100.0 || score == 0 {
		return "down"
	}
	if score >= 80 {
		return "healthy"
	}
	if score >= 60 {
		return "good"
	}
	if score >= 35 {
		return "warning"
	}
	return "critical"
}

func probeTCP(addr string, count int, timeout time.Duration) (avgRTT, minRTT, maxRTT, loss, jitter float64, err error) {
	var rtts []float64
	var failed int

	for i := 0; i < count; i++ {
		start := time.Now()
		conn, dialErr := net.DialTimeout("tcp", addr, timeout)
		if dialErr != nil {
			failed++
			continue
		}
		rtt := float64(time.Since(start).Microseconds()) / 1000.0
		_ = conn.Close()
		rtts = append(rtts, rtt)
		time.Sleep(25 * time.Millisecond)
	}

	loss = (float64(failed) / float64(count)) * 100.0
	if len(rtts) == 0 {
		return 0, 0, 0, 100.0, 0, fmt.Errorf("all %d probes failed to %s", count, addr)
	}

	minRTT = rtts[0]
	maxRTT = rtts[0]
	total := 0.0
	for _, r := range rtts {
		if r < minRTT {
			minRTT = r
		}
		if r > maxRTT {
			maxRTT = r
		}
		total += r
	}
	avgRTT = total / float64(len(rtts))

	if len(rtts) > 1 {
		var diffSum float64
		for i := 1; i < len(rtts); i++ {
			diffSum += math.Abs(rtts[i] - rtts[i-1])
		}
		jitter = diffSum / float64(len(rtts)-1)
	}

	return avgRTT, minRTT, maxRTT, loss, jitter, nil
}

func probeTLS(addr string, count int, timeout time.Duration) (avgRTT, minRTT, maxRTT, loss, jitter float64, err error) {
	var rtts []float64
	var failed int

	for i := 0; i < count; i++ {
		start := time.Now()
		dialer := &net.Dialer{Timeout: timeout}
		conf := &tls.Config{InsecureSkipVerify: true}
		conn, dialErr := tls.DialWithDialer(dialer, "tcp", addr, conf)
		if dialErr != nil {
			failed++
			continue
		}
		rtt := float64(time.Since(start).Microseconds()) / 1000.0
		_ = conn.Close()
		rtts = append(rtts, rtt)
		time.Sleep(25 * time.Millisecond)
	}

	loss = (float64(failed) / float64(count)) * 100.0
	if len(rtts) == 0 {
		return 0, 0, 0, 100.0, 0, fmt.Errorf("all %d probes failed to %s", count, addr)
	}

	minRTT = rtts[0]
	maxRTT = rtts[0]
	total := 0.0
	for _, r := range rtts {
		if r < minRTT {
			minRTT = r
		}
		if r > maxRTT {
			maxRTT = r
		}
		total += r
	}
	avgRTT = total / float64(len(rtts))

	if len(rtts) > 1 {
		var diffSum float64
		for i := 1; i < len(rtts); i++ {
			diffSum += math.Abs(rtts[i] - rtts[i-1])
		}
		jitter = diffSum / float64(len(rtts)-1)
	}

	return avgRTT, minRTT, maxRTT, loss, jitter, nil
}

func probePing(host string, count int) (avgRTT, minRTT, maxRTT, loss, jitter float64, err error) {
	out, err := exec.Command("ping", "-c", strconv.Itoa(count), "-W", "1", host).CombinedOutput()
	if err != nil && len(out) == 0 {
		return 0, 0, 0, 100, 0, err
	}
	s := string(out)
	reLoss := regexp.MustCompile(`(\d+(?:\.\d+)?)%\s*packet\s*loss`)
	if match := reLoss.FindStringSubmatch(s); len(match) > 1 {
		loss, _ = strconv.ParseFloat(match[1], 64)
	}
	reRTT := regexp.MustCompile(`rtt\s+min/avg/max/mdev\s*=\s*([0-9.]+)/([0-9.]+)/([0-9.]+)/([0-9.]+)`)
	if match := reRTT.FindStringSubmatch(s); len(match) > 4 {
		minRTT, _ = strconv.ParseFloat(match[1], 64)
		avgRTT, _ = strconv.ParseFloat(match[2], 64)
		maxRTT, _ = strconv.ParseFloat(match[3], 64)
		jitter, _ = strconv.ParseFloat(match[4], 64)
	}
	return avgRTT, minRTT, maxRTT, loss, jitter, nil
}
