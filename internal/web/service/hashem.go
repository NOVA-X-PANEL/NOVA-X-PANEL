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
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
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

func getGreInterface() string {
	if iface := strings.TrimSpace(os.Getenv("HASHEM_GRE_IFACE")); iface != "" {
		return iface
	}
	return "gre-tunnel"
}

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
	clientService  ClientService
}

type HashemPortActionResult struct {
	Success     bool   `json:"success"`
	Port        int    `json:"port"`
	Ports       []int  `json:"ports"`
	IranCommand string `json:"iranCommand"`
	Message     string `json:"message"`
}

func (s *HashemService) ensureHashemCore(ctx context.Context) error {
	if _, err := os.Stat(hashemBinPath); err == nil {
		return nil
	}
	if _, err := os.Stat(hashemScriptPath); err == nil {
		_ = os.Chmod(hashemScriptPath, 0o755)
		_ = os.Remove(hashemBinPath)
		_ = os.Symlink(hashemScriptPath, hashemBinPath)
		return nil
	}
	_, err := s.Install()
	return err
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

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func (s *HashemService) execPrivileged(ctx context.Context, name string, args ...string) ([]byte, error) {
	if os.Geteuid() == 0 {
		cmd := exec.CommandContext(ctx, name, args...)
		return cmd.CombinedOutput()
	}
	if systemdRun, err := exec.LookPath("systemd-run"); err == nil {
		sdArgs := append([]string{"--quiet", "--pipe", "--wait", name}, args...)
		cmd := exec.CommandContext(ctx, systemdRun, sdArgs...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return out, nil
		}
		outStr := strings.TrimSpace(string(out))
		if !strings.Contains(outStr, "System has not been booted with systemd") &&
			!strings.Contains(outStr, "Failed to connect to bus") {
			return out, err
		}
	}
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

func (s *HashemService) execPrivilegedBash(ctx context.Context, script string) ([]byte, error) {
	if os.Geteuid() == 0 {
		cmd := exec.CommandContext(ctx, "bash", "-c", script)
		return cmd.CombinedOutput()
	}
	if systemdRun, err := exec.LookPath("systemd-run"); err == nil {
		cmd := exec.CommandContext(ctx, systemdRun, "--quiet", "--pipe", "--wait", "bash", "-c", script)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return out, nil
		}
		outStr := strings.TrimSpace(string(out))
		if !strings.Contains(outStr, "System has not been booted with systemd") &&
			!strings.Contains(outStr, "Failed to connect to bus") {
			return out, err
		}
	}
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	return cmd.CombinedOutput()
}

func (s *HashemService) writePrivilegedFile(path string, content []byte, mode os.FileMode) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tmpFile, err := os.CreateTemp("", "hashem-priv-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(content); err != nil {
		_ = tmpFile.Close()
		return err
	}
	_ = tmpFile.Close()

	cmdStr := fmt.Sprintf("mkdir -p $(dirname %s) && cp %s %s && chmod %o %s",
		shellQuote(path), shellQuote(tmpPath), shellQuote(path), mode, shellQuote(path))
	_, err = s.execPrivilegedBash(ctx, cmdStr)
	return err
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
		status.PingMs = s.measurePing(status.RemoteGreIP, getGreInterface(), status.FrpPort)
		// Upstream Hashem fix: treat ICMP-filtered ping as WARN not FAIL when frp is active/connected
		if status.PingMs <= 0 && !frpConn {
			status.Running = false
		}
	} else {
		status.PingMs = -1
		status.Running = false
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

	modBhClientPath := ""
	modBhServerPath := ""
	if entries, err := os.ReadDir("/root/backhaul-core"); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "kharej") && strings.HasSuffix(e.Name(), ".toml") {
				modBhClientPath = filepath.Join("/root/backhaul-core", e.Name())
				if s.checkServiceStatus(strings.TrimSuffix(e.Name(), ".toml")) == "active" {
					bhClientActive = true
				}
			} else if strings.HasPrefix(e.Name(), "iran") && strings.HasSuffix(e.Name(), ".toml") {
				modBhServerPath = filepath.Join("/root/backhaul-core", e.Name())
				if s.checkServiceStatus(strings.TrimSuffix(e.Name(), ".toml")) == "active" {
					bhServerActive = true
				}
			}
		}
	}

	if bhClientActive || bhServerActive || bhClientExists || bhServerExists || modBhClientPath != "" || modBhServerPath != "" {
		if status.Running {
			status.Engine = "gre-backhaul"
		} else {
			status.Engine = "backhaul"
		}
		if bhClientActive || bhClientExists || modBhClientPath != "" {
			status.Role = "foreign"
			if bhClientExists {
				s.parseBackhaulConfig(backhaulClientTomlPath, status)
			} else if modBhClientPath != "" {
				s.parseBackhaulConfig(modBhClientPath, status)
			}
			if bhClientActive {
				status.BackhaulStatus = "active"
				status.FrpStatus = "active"
				status.Running = true
			} else {
				status.BackhaulStatus = s.checkServiceStatus("backhaul-client")
			}
		} else if bhServerActive || bhServerExists || modBhServerPath != "" {
			status.Role = "iran"
			if bhServerExists {
				s.parseBackhaulConfig(backhaulServerTomlPath, status)
			} else if modBhServerPath != "" {
				s.parseBackhaulConfig(modBhServerPath, status)
			}
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

	// Detect DrSaeedHub Tunnel-Panel (gre-panel)
	s.detectTunnelPanelStatus(status)
	if status.Engine == "tunnel-panel" && token == "" {
		token = "tp-managed"
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
		if status.Engine == "tunnel-panel" {
			status.Bundle = fmt.Sprintf("tp1_%s_%s_%s_%s",
				status.RemotePubIP, status.LocalPubIP, status.RemoteGreIP, portsStr)
			status.SetupCommand = buildTunnelPanelIranScript(
				status.RemotePubIP, status.LocalPubIP, status.RemoteGreIP, status.LocalGreIP, rawPortsStr)
		} else if status.Engine == "backhaul" {
			status.Bundle = fmt.Sprintf("bh1_%s_%d_%s_%s_%s",
				status.RemotePubIP, status.BackhaulPort, status.Transport, token, portsStr)
			_, _, status.SetupCommand = buildIranSetupCommands(
				status.RemotePubIP, status.LocalPubIP, 0, status.BackhaulPort, token, status.Carrier, rawPortsStr, "backhaul", status.Transport,
			)
		} else if status.Engine == "gre-backhaul" {
			status.Bundle = fmt.Sprintf("gh1_%s_%d_%s_%s_%s_%s_%s",
				status.RemotePubIP, status.BackhaulPort, status.RemoteGreIP, status.LocalGreIP, status.Transport, token, portsStr)
			_, _, status.SetupCommand = buildIranSetupCommands(
				status.RemotePubIP, status.LocalPubIP, 0, status.BackhaulPort, token, status.Carrier, rawPortsStr, "gre-backhaul", status.Transport,
			)
		} else {
			status.Bundle = fmt.Sprintf("hsh1_%s_%d_%s_%s_%s_%s",
				status.RemotePubIP, status.FrpPort, status.RemoteGreIP, status.LocalGreIP, token, portsStr)
			_, _, status.SetupCommand = buildIranSetupCommands(
				status.RemotePubIP, status.LocalPubIP, status.FrpPort, 0, token, status.Carrier, rawPortsStr, "frp", status.Transport,
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

	out, err := exec.CommandContext(ctx, "ip", "addr", "show", "dev", getGreInterface()).CombinedOutput()
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
		if sanitizedGre := sanitizeTunnelIP(peerGre); sanitizedGre != "" {
			filter = fmt.Sprintf("dst %s and dport = :%d", sanitizedGre, port)
		}
	}

	cmd := exec.CommandContext(ctx, "ss", "-t", "state", "established", filter)
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

	// 2. Multi-probe fallback (TCP SYN probe to FRP or standard SSH/benchmark ports)
	probePorts := []int{}
	if frpPort > 0 {
		probePorts = append(probePorts, frpPort)
	}
	probePorts = append(probePorts, 22, 7777, 5201)

	for _, p := range probePorts {
		target := net.JoinHostPort(ip, strconv.Itoa(p))
		start := time.Now()
		conn, err := net.DialTimeout("tcp", target, 350*time.Millisecond)
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

	out, err := s.execPrivileged(ctx, hashemBinPath, "carrier", "set", carrier)
	if err != nil {
		logger.Warningf("hashem carrier set failed: %v, out: %s", err, string(out))
		return fmt.Errorf("failed to set carrier: %s", string(out))
	}
	return nil
}

func (s *HashemService) Restart() error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	_ = s.ensureHashemCore(ctx)

	if _, err := os.Stat(hashemBinPath); err == nil {
		out, err := s.execPrivileged(ctx, hashemBinPath, "restart")
		if err == nil {
			return nil
		}
		logger.Warningf("hashem restart CLI failed: %v, out: %s", err, string(out))
	}

	restartCmd := `
systemctl daemon-reload 2>/dev/null || true
for svc in gre-tunnel frpc frps backhaul-client backhaul-server gre-panel; do
    if [ -f "/etc/systemd/system/${svc}.service" ]; then
        systemctl restart "$svc" 2>/dev/null || true
    fi
done
`
	_, _ = s.execPrivilegedBash(ctx, restartCmd)
	return nil
}

func (s *HashemService) SetWatchdog(enabled bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	action := "off"
	if enabled {
		action = "on"
	}
	out, err := s.execPrivileged(ctx, hashemBinPath, "watchdog", action)
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

	var args []string
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

	out, err := s.execPrivileged(ctx, hashemBinPath, args...)
	if err != nil {
		logger.Warningf("hashem setup failed: %v, out: %s", err, string(out))
		return string(out), fmt.Errorf("setup failed: %s", string(out))
	}

	if form.AutoCreateInbound && (form.Role == "" || form.Role == "foreign") {
		inHost := strings.TrimSpace(form.InboundHost)
		if inHost == "" {
			inHost = form.RemotePub
		}
		_ = s.AutoCreateMatchingInbounds(form.Ports, form.RemotePub, inHost)
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
		return nil, fmt.Errorf("هیچ اینباند فعالی برای همگام‌سازی با تانل یافت نشد")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	_ = s.ensureHashemCore(ctx)

	// Dynamic zero-downtime port sync via EditPorts
	if err := s.EditPorts(ports); err == nil {
		return ports, nil
	}

	status, err := s.GetStatus()
	if err != nil {
		return nil, fmt.Errorf("خطا در دریافت وضعیت تانل: %w", err)
	}

	// Fallback to setup-foreign if toml direct edit was not applicable
	if status.Role == "foreign" && status.LocalPubIP != "" && status.RemotePubIP != "" && status.FrpPort > 0 {
		var portStrs []string
		for _, p := range ports {
			portStrs = append(portStrs, strconv.Itoa(p))
		}
		portsArg := strings.Join(portStrs, ", ")

		out, err := s.execPrivileged(ctx, hashemBinPath, "setup-foreign",
			"--local-pub", status.LocalPubIP,
			"--remote-pub", status.RemotePubIP,
			"--frp-port", strconv.Itoa(status.FrpPort),
			"--ports", portsArg,
			"--force",
		)
		if err != nil {
			return nil, fmt.Errorf("همگام‌سازی ناموفق بود: %s", string(out))
		}
		return ports, nil
	}

	return nil, fmt.Errorf("تانل روی این سرور به عنوان سرور خارج (Foreign Client) پیکربندی نشده است. ابتدا تانل را از بخش دستور تک‌خطی یا اتصال SSH راه‌اندازی کنید.")
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
		if _, err := s.execPrivileged(ctx, hashemBinPath, "edit-peer-ports", "--id", "1", "--ports", portsArg); err == nil {
			return nil
		}
	}

	editedAny := false

	// Update gre-panel database if present
	if _, err := os.Stat("/var/lib/gre-panel/panel.db"); err == nil {
		pyScript := fmt.Sprintf(`import sqlite3
try:
    c = sqlite3.connect("/var/lib/gre-panel/panel.db")
    c.execute("DELETE FROM RouteDestination")
    for p in [%s]:
        c.execute("INSERT INTO RouteDestination (Port, IsEnabled) VALUES (?, 1)", (p,))
    c.commit()
except Exception:
    pass
`, strings.Join(portStrs, ","))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = s.execPrivileged(ctx, "python3", "-c", pyScript)
		cancel()
		editedAny = true
	}

	// Update /etc/x-ui/tunnel.json if exists
	if tData, err := os.ReadFile("/etc/x-ui/tunnel.json"); err == nil {
		var doc map[string]any
		if err := json.Unmarshal(tData, &doc); err == nil {
			doc["ports"] = cleanPorts
			if bs, err := json.MarshalIndent(doc, "", "  "); err == nil {
				_ = os.WriteFile("/etc/x-ui/tunnel.json", bs, 0o644)
				editedAny = true
			}
		}
	}

	// 2. Direct edit: update peers.json + rewrite toml + reload frp
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
				if err := os.WriteFile(peersPath, append(outData, '\n'), 0o644); err != nil {
					_ = s.writePrivilegedFile(peersPath, append(outData, '\n'), 0o644)
				}
				editedAny = true
			}
		}
	}

	// Foreign client toml (/etc/frp/frpc.toml)
	if rawToml, err := os.ReadFile(frpcTomlPath); err == nil {
		updated := rewriteTomlPorts(string(rawToml), cleanPorts)
		if err := os.WriteFile(frpcTomlPath, []byte(updated), 0o644); err != nil {
			_ = s.writePrivilegedFile(frpcTomlPath, []byte(updated), 0o644)
		}
		c, cl := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = s.execPrivileged(c, "systemctl", "reload-or-restart", "frpc")
		cl()
		editedAny = true
	}

	// Server toml (/etc/frp/frps.toml)
	if rawToml, err := os.ReadFile(frpsTomlPath); err == nil {
		if strings.Contains(string(rawToml), "[[proxies]]") {
			updated := rewriteTomlPorts(string(rawToml), cleanPorts)
			if err := os.WriteFile(frpsTomlPath, []byte(updated), 0o644); err != nil {
				_ = s.writePrivilegedFile(frpsTomlPath, []byte(updated), 0o644)
			}
		}
		c, cl := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = s.execPrivileged(c, "systemctl", "reload-or-restart", "frps")
		cl()
		editedAny = true
	}

	// Backhaul client toml (/etc/backhaul/client.toml)
	if rawToml, err := os.ReadFile(backhaulClientTomlPath); err == nil {
		updated := rewriteBackhaulPorts(string(rawToml), cleanPorts)
		if err := os.WriteFile(backhaulClientTomlPath, []byte(updated), 0o644); err != nil {
			_ = s.writePrivilegedFile(backhaulClientTomlPath, []byte(updated), 0o644)
		}
		c, cl := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = s.execPrivileged(c, "systemctl", "restart", "backhaul-client")
		cl()
		editedAny = true
	}

	// Backhaul server toml (/etc/backhaul/config.toml)
	if rawToml, err := os.ReadFile(backhaulServerTomlPath); err == nil {
		updated := rewriteBackhaulPorts(string(rawToml), cleanPorts)
		if err := os.WriteFile(backhaulServerTomlPath, []byte(updated), 0o644); err != nil {
			_ = s.writePrivilegedFile(backhaulServerTomlPath, []byte(updated), 0o644)
		}
		c, cl := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = s.execPrivileged(c, "systemctl", "restart", "backhaul-server")
		cl()
		editedAny = true
	}

	// Modified Backhaul tomls (/root/backhaul-core/*.toml)
	if entries, err := os.ReadDir("/root/backhaul-core"); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".toml") {
				p := filepath.Join("/root/backhaul-core", e.Name())
				if rawToml, err := os.ReadFile(p); err == nil {
					updated := rewriteBackhaulPorts(string(rawToml), cleanPorts)
					if err := os.WriteFile(p, []byte(updated), 0o644); err == nil {
						svc := strings.TrimSuffix(e.Name(), ".toml")
						_, _ = exec.Command("systemctl", "restart", svc).CombinedOutput()
						editedAny = true
					}
				}
			}
		}
	}

	// Allow UFW ports if active
	if out, err := exec.Command("ufw", "status").CombinedOutput(); err == nil && strings.Contains(string(out), "Status: active") {
		for _, p := range cleanPorts {
			_, _ = exec.Command("ufw", "allow", fmt.Sprintf("%d/tcp", p)).CombinedOutput()
			_, _ = exec.Command("ufw", "allow", fmt.Sprintf("%d/udp", p)).CombinedOutput()
		}
	}

	if !editedAny {
		return errors.New("neither frp nor backhaul configuration could be found or updated")
	}

	return nil
}

func (s *HashemService) AddPort(port int, autoInbound bool) (*HashemPortActionResult, error) {
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("شماره پورت نامعتبر است: %d", port)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	_ = s.ensureHashemCore(ctx)

	status, _ := s.GetStatus()
	currentPorts := []int{}
	if status != nil && len(status.Ports) > 0 {
		currentPorts = append(currentPorts, status.Ports...)
	}

	portExists := false
	for _, p := range currentPorts {
		if p == port {
			portExists = true
			break
		}
	}

	if !portExists {
		currentPorts = append(currentPorts, port)
	}
	sort.Ints(currentPorts)

	if err := s.EditPorts(currentPorts); err != nil {
		logger.Warningf("EditPorts failed during AddPort: %v", err)
	}

	if autoInbound && status != nil {
		host := status.RemotePubIP
		if host == "" {
			host = status.LocalPubIP
		}
		_ = s.AutoCreateMatchingInbounds(strconv.Itoa(port), host, host)
	}

	iranCommand := fmt.Sprintf(
		`bash -c 'PORT="%d"; `+
			`for f in /etc/backhaul/config.toml /etc/backhaul/iran.toml /etc/backhaul/*.toml /root/backhaul-core/*.toml; do `+
			`[ -f "$f" ] && ! grep -q "\"$PORT\"" "$f" && sed -i "/ports\s*=\s*\[/a \ \ \ \ \"$PORT\"," "$f" && systemctl restart backhaul-server backhaul-iran 2>/dev/null; done; `+
			`if [ -f /etc/gre-panel/peers.json ]; then python3 -c "import json; p=\"/etc/gre-panel/peers.json\"; d=json.load(open(p)); [x.setdefault(\"ports\", []).append($PORT) for x in d.get(\"peers\", []) if $PORT not in x.get(\"ports\", [])]; json.dump(d, open(p, \"w\"), indent=2)" 2>/dev/null; fi; `+
			`if [ -f /var/lib/gre-panel/panel.db ]; then python3 -c "import sqlite3; c=sqlite3.connect(\"/var/lib/gre-panel/panel.db\"); c.execute(\"INSERT OR IGNORE INTO RouteDestination (Port, IsEnabled) VALUES (?, 1)\", ($PORT,)); c.commit()" 2>/dev/null; fi; `+
			`if ip link show gre1 >/dev/null 2>&1 || ip link show gre-tunnel >/dev/null 2>&1; then `+
			`iptables -t nat -C PREROUTING -p tcp --dport ${PORT} -j DNAT --to-destination 10.10.10.2:${PORT} 2>/dev/null || iptables -t nat -A PREROUTING -p tcp --dport ${PORT} -j DNAT --to-destination 10.10.10.2:${PORT} 2>/dev/null || true; `+
			`iptables -t nat -C PREROUTING -p udp --dport ${PORT} -j DNAT --to-destination 10.10.10.2:${PORT} 2>/dev/null || iptables -t nat -A PREROUTING -p udp --dport ${PORT} -j DNAT --to-destination 10.10.10.2:${PORT} 2>/dev/null || true; `+
			`fi; `+
			`ufw allow ${PORT}/tcp 2>/dev/null; ufw allow ${PORT}/udp 2>/dev/null; `+
			`iptables -I INPUT -p tcp --dport ${PORT} -j ACCEPT 2>/dev/null; iptables -I INPUT -p udp --dport ${PORT} -j ACCEPT 2>/dev/null; `+
			`echo "✔️ پورت $PORT بدون اختلال در پورت‌های قبلی با موفقیت به تانل سرور ایران متصل شد."'`,
		port,
	)

	return &HashemPortActionResult{
		Success:     true,
		Port:        port,
		Ports:       currentPorts,
		IranCommand: iranCommand,
		Message:     fmt.Sprintf("پورت %d با موفقیت به تانل خارج افزوده شد.", port),
	}, nil
}

func (s *HashemService) RemovePort(port int) (*HashemPortActionResult, error) {
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("شماره پورت نامعتبر است: %d", port)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	_ = s.ensureHashemCore(ctx)

	status, _ := s.GetStatus()
	var newPorts []int
	if status != nil && len(status.Ports) > 0 {
		for _, p := range status.Ports {
			if p != port {
				newPorts = append(newPorts, p)
			}
		}
	}
	sort.Ints(newPorts)

	if len(newPorts) > 0 {
		if err := s.EditPorts(newPorts); err != nil {
			logger.Warningf("EditPorts failed during RemovePort: %v", err)
		}
	} else {
		// If no ports remain in tunnel, tear down the whole tunnel cleanly
		_ = s.Remove()
	}

	// Sever any active conntrack sessions on this port immediately
	flushPortCmd := fmt.Sprintf("conntrack -D -p tcp --dport %d 2>/dev/null || true; conntrack -D -p udp --dport %d 2>/dev/null || true", port, port)
	_, _ = s.execPrivilegedBash(ctx, flushPortCmd)

	iranCommand := fmt.Sprintf(
		`bash -c 'PORT="%d"; for f in /etc/backhaul/config.toml /root/backhaul-core/*.toml; do [ -f "$f" ] && sed -i "/\"$PORT\"/d" "$f" && systemctl restart backhaul-server 2>/dev/null; done; if [ -f /etc/gre-panel/peers.json ]; then python3 -c "import json; p=\"/etc/gre-panel/peers.json\"; d=json.load(open(p)); [x[\"ports\"].remove($PORT) for x in d.get(\"peers\", []) if \"ports\" in x and $PORT in x[\"ports\"]]; json.dump(d, open(p, \"w\"), indent=2)" 2>/dev/null; fi; if [ -f /var/lib/gre-panel/panel.db ]; then python3 -c "import sqlite3; c=sqlite3.connect(\"/var/lib/gre-panel/panel.db\"); c.execute(\"DELETE FROM RouteDestination WHERE Port = ?\", ($PORT,)); c.commit()" 2>/dev/null; fi; ufw delete allow ${PORT}/tcp 2>/dev/null; ufw delete allow ${PORT}/udp 2>/dev/null; echo "پورت $PORT با موفقیت از تانل حذف شد"'`,
		port,
	)

	return &HashemPortActionResult{
		Success:     true,
		Port:        port,
		Ports:       newPorts,
		IranCommand: iranCommand,
		Message:     fmt.Sprintf("پورت %d از تانل خارج حذف گردید.", port),
	}, nil
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

	scriptCmd := `
set -e
mkdir -p /usr/local/bin /etc/frp /etc/gre-panel /etc/backhaul

# 1. Download hashem.sh
curl -fsSL --connect-timeout 10 --max-time 60 https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /usr/local/bin/hashem.sh 2>/dev/null || \
curl -fsSL --connect-timeout 10 --max-time 60 https://fastly.jsdelivr.net/gh/pdnczone/hashem-panel@main/hashem.sh -o /usr/local/bin/hashem.sh 2>/dev/null || \
curl -fsSL --connect-timeout 10 --max-time 60 https://ghproxy.net/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /usr/local/bin/hashem.sh 2>/dev/null || \
curl -fsSL --connect-timeout 10 --max-time 60 https://gh-proxy.com/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /usr/local/bin/hashem.sh 2>/dev/null || \
curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /usr/local/bin/hashem.sh

chmod +x /usr/local/bin/hashem.sh
ln -sf /usr/local/bin/hashem.sh /usr/local/bin/hashem

# 2. Download FRP binaries if not present
if [ ! -x /usr/local/bin/frpc ] || [ ! -x /usr/local/bin/frps ]; then
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64) FRP_ARCH="amd64" ;;
    aarch64|arm64) FRP_ARCH="arm64" ;;
    *) FRP_ARCH="amd64" ;;
  esac
  TMP_FRP=$(mktemp -d)
  (
    curl -fsSL --connect-timeout 10 --max-time 60 https://github.com/fatedier/frp/releases/download/v0.71.0/frp_0.71.0_linux_${FRP_ARCH}.tar.gz -o "${TMP_FRP}/frp.tar.gz" 2>/dev/null || \
    curl -fsSL --connect-timeout 10 --max-time 60 https://ghproxy.net/https://github.com/fatedier/frp/releases/download/v0.71.0/frp_0.71.0_linux_${FRP_ARCH}.tar.gz -o "${TMP_FRP}/frp.tar.gz" 2>/dev/null
  ) && tar -xzf "${TMP_FRP}/frp.tar.gz" -C "${TMP_FRP}" && \
    cp "${TMP_FRP}/frp_0.71.0_linux_${FRP_ARCH}/frpc" /usr/local/bin/ && \
    cp "${TMP_FRP}/frp_0.71.0_linux_${FRP_ARCH}/frps" /usr/local/bin/ && \
    chmod +x /usr/local/bin/frpc /usr/local/bin/frps || true
  rm -rf "${TMP_FRP}"
fi

echo "hashem core installed successfully"
`

	out, err := s.execPrivilegedBash(ctx, scriptCmd)
	if err != nil {
		logger.Warningf("hashem install failed: %v, out: %s", err, string(out))
		return string(out), fmt.Errorf("install failed: %w (output: %s)", err, string(out))
	}
	if !s.IsInstalled() {
		return string(out), fmt.Errorf("installation failed: binary not found at %s", hashemBinPath)
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

func sanitizeTunnelIP(ipStr string) string {
	ipStr = strings.TrimSpace(ipStr)
	if ip := net.ParseIP(ipStr); ip != nil {
		return ip.String()
	}
	validHost := regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	if validHost.MatchString(ipStr) && !strings.HasPrefix(ipStr, "-") {
		return ipStr
	}
	return ""
}

func sanitizeCarrier(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	switch c {
	case "direct", "mci", "irancell", "rightel", "mokhaberat", "shatel", "asiatech", "hiweb", "afranet":
		return c
	default:
		return "direct"
	}
}

func sanitizeEngine(e string) string {
	e = strings.ToLower(strings.TrimSpace(e))
	switch e {
	case "tunnel-panel", "frp", "backhaul", "gre-backhaul":
		return e
	default:
		return "tunnel-panel"
	}
}

func sanitizeTransport(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "tcpmux", "xtcpmux", "anytls", "wssmux", "ws", "tcp", "tun":
		return t
	default:
		return "tcpmux"
	}
}

func (s *HashemService) resolveTunnelPorts(portsStr string) string {
	cleaned := strings.TrimSpace(portsStr)
	if cleaned != "" {
		parsed := s.parsePorts(cleaned)
		if len(parsed) > 0 {
			var pstrs []string
			for _, p := range parsed {
				pstrs = append(pstrs, strconv.Itoa(p))
			}
			return strings.Join(pstrs, ", ")
		}
	}

	if database.GetDB() == nil {
		return "8080"
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

func (s *HashemService) AutoCreateMatchingInbounds(portsStr string, iranIP string, wsHost ...string) error {
	if database.GetDB() == nil {
		return nil
	}

	ports := s.parsePorts(portsStr)
	if len(ports) == 0 {
		return nil
	}

	iranIP = strings.TrimSpace(iranIP)
	if iranIP == "" {
		iranIP = s.getForeignPubIP()
	}

	hostHeader := iranIP
	if len(wsHost) > 0 && strings.TrimSpace(wsHost[0]) != "" {
		hostHeader = strings.TrimSpace(wsHost[0])
	}

	existingInbounds, _ := s.inboundService.GetInboundsForScope(InboundAccessScope{All: true})
	existingMap := make(map[int]*model.Inbound)

	for _, ib := range existingInbounds {
		existingMap[ib.Port] = ib
	}

	// Resolve acting user ID for newly created inbound
	defaultUserId := 1
	var firstUser model.User
	if err := database.GetDB().First(&firstUser).Error; err == nil && firstUser.Id > 0 {
		defaultUserId = firstUser.Id
	}

	createdAny := false

	for _, port := range ports {
		if _, exists := existingMap[port]; exists {
			continue
		}

		settingsMap := map[string]any{
			"clients":    []any{},
			"decryption": "none",
			"fallbacks":  []any{},
		}
		settingsBytes, _ := json.Marshal(settingsMap)

		streamMap := map[string]any{
			"network":  "ws",
			"security": "none",
			"wsSettings": map[string]any{
				"acceptProxyProtocol": false,
				"path":                "/@NOVA-X-PANEL",
				"host":                hostHeader,
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
			UserId:            defaultUserId,
			Enable:            true,
			Protocol:          model.VLESS,
			Port:              port,
			Tag:               fmt.Sprintf("in-%d-nova", port),
			Remark:            "NOVA-X-PANEL",
			Listen:            "",
			Settings:          string(settingsBytes),
			StreamSettings:    string(streamBytes),
			Sniffing:          string(sniffingBytes),
			ShareAddrStrategy: "custom",
			ShareAddr:         iranIP,
		}

		addedInbound, _, err := s.inboundService.AddInbound(newInbound)
		if err == nil && addedInbound != nil {
			createdAny = true
		} else {
			logger.Warningf("AutoCreateMatchingInbounds failed for port %d: %v", port, err)
		}
	}

	if createdAny {
		_ = s.xrayService.RestartXray(false)
		websocket.BroadcastInvalidate(websocket.MessageTypeInbounds)
		websocket.BroadcastInvalidate(websocket.MessageTypeClients)
	}

	return nil
}

func (s *HashemService) SetupSSH(form HashemSSHSetupForm) (*HashemSSHSetupResult, error) {
	iranIP := sanitizeTunnelIP(form.IranIP)
	if iranIP == "" {
		return nil, fmt.Errorf("آدرس آی‌پی سرور ایران نامعتبر است")
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
	carrier := sanitizeCarrier(form.Carrier)
	engine := sanitizeEngine(form.Engine)
	transport := sanitizeTransport(form.Transport)
	bhPort := form.BackhaulPort
	if bhPort <= 0 {
		bhPort = 3080
	}

	foreignIP := s.getForeignPubIP()
	if foreignIP == "" {
		return nil, fmt.Errorf("امکان تشخیص خودکار آی‌پی سرور خارج وجود ندارد")
	}

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
		return nil, fmt.Errorf("خطا در اتصال SSH به سرور ایران (%s): %w", addr, err)
	}
	defer client.Close()

	// 1. Refresh local hashem.sh if possible to always use upstream latest, then read to transfer over SSH
	_ = exec.Command("bash", "-c", "curl -fsSL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /usr/local/bin/hashem.sh 2>/dev/null && chmod +x /usr/local/bin/hashem.sh || true").Run()
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

	downloadPipeSSH := "if [ -f /opt/frp_backup/frps ] && [ ! -s /usr/local/bin/frps ]; then cp /opt/frp_backup/frps /usr/local/bin/frps && chmod +x /usr/local/bin/frps; fi; " +
		"if [ ! -s /tmp/hashem.sh ]; then " +
		"curl -fsSL https://fastly.jsdelivr.net/gh/pdnczone/hashem-panel@main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || " +
		"curl -fsSL https://ghproxy.net/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || " +
		"curl -fsSL https://gh-proxy.com/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || " +
		"curl -fsSL https://ghproxy.cn/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || " +
		"curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh; " +
		"chmod +x /tmp/hashem.sh; fi; "

	carrierCmd := ""
	if carrier != "direct" && carrier != "" {
		carrierCmd = fmt.Sprintf("bash /tmp/hashem.sh carrier mode %s && ", carrier)
	}

	var iranCmd string
	if engine == "backhaul" {
		iranCmd = fmt.Sprintf(
			"export DEBIAN_FRONTEND=noninteractive; %s"+
				"bash /tmp/hashem.sh setup-backhaul-iran --local-pub %s --remote-pub %s --port %d --transport %s --token %s --ports \"%s\" --force; cp -f /tmp/hashem.sh /usr/local/bin/hashem.sh 2>/dev/null; ln -sf /usr/local/bin/hashem.sh /usr/local/bin/hashem 2>/dev/null; systemctl enable backhaul-server 2>/dev/null || true; rm -f /tmp/hashem.sh",
			downloadPipeSSH, iranIP, foreignIP, bhPort, transport, token, ports,
		)
	} else if engine == "gre-backhaul" {
		iranCmd = fmt.Sprintf(
			"export DEBIAN_FRONTEND=noninteractive; %s"+
				"bash /tmp/hashem.sh setup-gre-backhaul-iran --local-pub %s --remote-pub %s --port %d --transport %s --token %s --ports \"%s\" --force && "+
				carrierCmd+
				"%s; cp -f /tmp/hashem.sh /usr/local/bin/hashem.sh 2>/dev/null; ln -sf /usr/local/bin/hashem.sh /usr/local/bin/hashem 2>/dev/null; systemctl enable gre-tunnel backhaul-server 2>/dev/null || true; rm -f /tmp/hashem.sh",
			downloadPipeSSH, iranIP, foreignIP, bhPort, transport, token, ports, optimizeCmd,
		)
	} else {
		iranCmd = fmt.Sprintf(
			"export DEBIAN_FRONTEND=noninteractive; %s"+
				"bash /tmp/hashem.sh setup-iran --local-pub %s --remote-pub %s --frp-port %d --token %s --local-gre 10.10.10.2 --peer-gre 10.10.10.1 --force && "+
				carrierCmd+
				"%s && %s; cp -f /tmp/hashem.sh /usr/local/bin/hashem.sh 2>/dev/null; ln -sf /usr/local/bin/hashem.sh /usr/local/bin/hashem 2>/dev/null; systemctl enable gre-tunnel frps 2>/dev/null || true; rm -f /tmp/hashem.sh",
			downloadPipeSSH, iranIP, foreignIP, frpPort, token, peerJsonCmd, optimizeCmd,
		)
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("خطا در ایجاد نشست SSH روی سرور ایران: %w", err)
	}
	defer session.Close()

	iranOut, err := session.CombinedOutput(iranCmd)
	if err != nil {
		return nil, fmt.Errorf("اجرای تانل روی سرور ایران با خطا مواجه شد: %w\nخروجی: %s", err, string(iranOut))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var argsForeign []string
	if engine == "backhaul" {
		argsForeign = []string{
			"setup-backhaul-foreign",
			"--local-pub", foreignIP,
			"--remote-pub", iranIP,
			"--port", strconv.Itoa(bhPort),
			"--transport", transport,
			"--token", token,
			"--force",
		}
	} else if engine == "gre-backhaul" {
		argsForeign = []string{
			"setup-gre-backhaul-foreign",
			"--local-pub", foreignIP,
			"--remote-pub", iranIP,
			"--port", strconv.Itoa(bhPort),
			"--transport", transport,
			"--token", token,
			"--force",
		}
	} else {
		argsForeign = []string{
			"setup-foreign",
			"--local-pub", foreignIP,
			"--remote-pub", iranIP,
			"--frp-port", strconv.Itoa(frpPort),
			"--token", token,
			"--ports", ports,
			"--local-gre", "10.10.10.1",
			"--peer-gre", "10.10.10.2",
			"--force",
		}
	}
	fOut, _ := s.execPrivileged(ctx, hashemBinPath, argsForeign...)

	if engine != "backhaul" && carrier != "direct" && carrier != "" {
		_, _ = s.execPrivileged(ctx, hashemBinPath, "carrier", "mode", carrier)
	}
	if engine != "backhaul" {
		s.optimizeForeignNetwork(ports)
	}

	if form.AutoCreateInbound {
		inHost := strings.TrimSpace(form.InboundHost)
		if inHost == "" {
			inHost = iranIP
		}
		_ = s.AutoCreateMatchingInbounds(ports, iranIP, inHost)
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

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
		if err := os.WriteFile(frpcTomlPath, []byte(sData), 0o644); err != nil {
			_ = s.writePrivilegedFile(frpcTomlPath, []byte(sData), 0o644)
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
	if err := os.WriteFile("/etc/systemd/system/frpc.service", []byte(fixFrpcService), 0o644); err != nil {
		_ = s.writePrivilegedFile("/etc/systemd/system/frpc.service", []byte(fixFrpcService), 0o644)
	}

	_, _ = s.execPrivileged(ctx, "systemctl", "daemon-reload")
	_, _ = s.execPrivileged(ctx, "systemctl", "restart", "frpc")

	greIf := getGreInterface()
	_, _ = s.execPrivileged(ctx, "ip", "link", "set", "dev", greIf, "mtu", "1380")
	_, _ = s.execPrivileged(ctx, "iptables", "-t", "mangle", "-C", "POSTROUTING", "-o", greIf, "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--set-mss", "1340")
	_, _ = s.execPrivileged(ctx, "iptables", "-t", "mangle", "-A", "POSTROUTING", "-o", greIf, "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--set-mss", "1340")
	_, _ = s.execPrivileged(ctx, "iptables", "-t", "mangle", "-C", "FORWARD", "-o", greIf, "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--set-mss", "1340")
	_, _ = s.execPrivileged(ctx, "iptables", "-t", "mangle", "-A", "FORWARD", "-o", greIf, "-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--set-mss", "1340")
	if out, err := s.execPrivileged(ctx, hashemBinPath, "optimize"); err != nil {
		logger.Warningf("hashem optimize failed: %v, out: %s", err, string(out))
	}
}

func buildIranSetupCommands(iranIP, foreignIP string, frpPort, bhPort int, token, carrier, ports, engine, transport string) (string, string, string) {
	iranIP = sanitizeTunnelIP(iranIP)
	foreignIP = sanitizeTunnelIP(foreignIP)
	carrier = sanitizeCarrier(carrier)
	engine = sanitizeEngine(engine)
	transport = sanitizeTransport(transport)

	var portList []string
	for _, p := range strings.Split(ports, ",") {
		p = strings.TrimSpace(p)
		if portNum, err := strconv.Atoi(p); err == nil && portNum > 0 && portNum <= 65535 {
			portList = append(portList, strconv.Itoa(portNum))
		}
	}
	portsJson := strings.Join(portList, ", ")
	if portsJson == "" {
		portsJson = "8080"
		portList = []string{"8080"}
	}
	portsSafe := strings.Join(portList, ",")

	greIf := getGreInterface()
	peerJsonCmd := fmt.Sprintf(
		`python3 -c "import json, os; p='/etc/gre-panel/peers.json'; os.makedirs(os.path.dirname(p), exist_ok=True); d={'peers':[]}; (os.path.exists(p) and d.update(json.load(open(p)))) if os.path.exists(p) else 0; peers=d.get('peers',[]); m=[x for x in peers if x.get('remote_pub')=='%s']; nid=m[0].get('id',1) if m else max([x.get('id',0) for x in peers]+[0])+1; lg='10.10.10.2' if nid==1 else '10.10.'+str(nid)+'.2'; pg='10.10.10.1' if nid==1 else '10.10.'+str(nid)+'.1'; entry={'id': nid, 'name': 'Nova-Tunnel-'+str(nid), 'local_pub': '%s', 'remote_pub': '%s', 'peer_pub': '%s', 'local_gre': lg, 'peer_gre': pg, 'ports': [%s], 'frp_port': %d, 'gre_if': '%s', 'frps_svc': 'frps', 'legacy': True}; peers=[x for x in peers if x.get('remote_pub')!='%s']+[entry]; d['peers']=peers; json.dump(d, open(p, 'w'), indent=2)" 2>/dev/null && systemctl restart gre-panel 2>/dev/null || true`,
		foreignIP, iranIP, foreignIP, foreignIP, portsJson, frpPort, greIf, foreignIP,
	)

	optimizeCmd := fmt.Sprintf(
		"ip link set dev %s mtu 1380 2>/dev/null || true; "+
			"iptables -t mangle -C POSTROUTING -o %s -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1340 2>/dev/null || iptables -t mangle -A POSTROUTING -o %s -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1340 2>/dev/null || true; "+
			"iptables -t mangle -C FORWARD -o %s -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1340 2>/dev/null || iptables -t mangle -A FORWARD -o %s -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1340 2>/dev/null || true; "+
			"bash /tmp/hashem.sh optimize 2>/dev/null || true",
		greIf, greIf, greIf, greIf, greIf,
	)

	downloadPipe := "if [ -f /opt/frp_backup/frps ] && [ ! -s /usr/local/bin/frps ]; then cp /opt/frp_backup/frps /usr/local/bin/frps && chmod +x /usr/local/bin/frps; fi; " +
		"curl -fsSL --connect-timeout 5 --max-time 30 https://fastly.jsdelivr.net/gh/pdnczone/hashem-panel@main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || " +
		"curl -fsSL --connect-timeout 5 --max-time 30 https://ghproxy.net/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || " +
		"curl -fsSL --connect-timeout 5 --max-time 30 https://gh-proxy.com/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || " +
		"curl -fsSL --connect-timeout 5 --max-time 30 https://ghproxy.cn/https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || " +
		"curl -fsSL --connect-timeout 5 --max-time 30 https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh 2>/dev/null || " +
		"curl -sL https://raw.githubusercontent.com/pdnczone/hashem-panel/main/hashem.sh -o /tmp/hashem.sh; " +
		"chmod +x /tmp/hashem.sh; " +
		"cp -f /tmp/hashem.sh /usr/local/bin/hashem.sh 2>/dev/null || true; " +
		"chmod +x /usr/local/bin/hashem.sh 2>/dev/null || true; " +
		"ln -sf /usr/local/bin/hashem.sh /usr/local/bin/hashem 2>/dev/null || true;"

	carrierCmd := ""
	if carrier != "direct" && carrier != "" {
		carrierCmd = fmt.Sprintf("bash /tmp/hashem.sh carrier mode %s && ", carrier)
	}

	var oneLiner string
	if engine == "backhaul" {
		oneLiner = fmt.Sprintf(
			"%s bash /tmp/hashem.sh setup-backhaul-iran --local-pub %s --remote-pub %s --port %d --transport %s --token %s --ports \"%s\" --force; systemctl enable backhaul-server 2>/dev/null || true; rm -f /tmp/hashem.sh",
			downloadPipe, iranIP, foreignIP, bhPort, transport, token, portsSafe,
		)
	} else if engine == "gre-backhaul" {
		oneLiner = fmt.Sprintf(
			"%s bash /tmp/hashem.sh setup-gre-backhaul-iran --local-pub %s --remote-pub %s --port %d --transport %s --token %s --ports \"%s\" --force && %s%s; systemctl enable gre-tunnel backhaul-server 2>/dev/null || true; rm -f /tmp/hashem.sh",
			downloadPipe, iranIP, foreignIP, bhPort, transport, token, portsSafe, carrierCmd, optimizeCmd,
		)
	} else {
		portsArg := ""
		if portsSafe != "" {
			portsArg = fmt.Sprintf(" --ports \"%s\"", portsSafe)
		}
		oneLiner = fmt.Sprintf(
			"%s bash /tmp/hashem.sh setup-iran --local-pub %s --remote-pub %s --frp-port %d --token %s%s --local-gre 10.10.10.2 --peer-gre 10.10.10.1 --force && %s%s && %s; systemctl enable gre-tunnel frps 2>/dev/null || true; rm -f /tmp/hashem.sh",
			downloadPipe, iranIP, foreignIP, frpPort, token, portsArg, carrierCmd, peerJsonCmd, optimizeCmd,
		)
	}

	return peerJsonCmd, optimizeCmd, oneLiner
}

func (s *HashemService) GenerateOneLiner(form HashemOneLinerForm) (*HashemOneLinerResult, error) {
	iranIP := sanitizeTunnelIP(form.IranIP)
	if iranIP == "" {
		return nil, fmt.Errorf("آدرس آی‌پی سرور ایران نامعتبر است")
	}

	carrier := sanitizeCarrier(form.Carrier)
	engine := sanitizeEngine(form.Engine)
	transport := sanitizeTransport(form.Transport)
	bhPort := form.BackhaulPort
	if bhPort <= 0 {
		bhPort = 3080
	}

	foreignIP := s.getForeignPubIP()
	if foreignIP == "" {
		return nil, fmt.Errorf("امکان تشخیص خودکار آی‌پی سرور خارج وجود ندارد")
	}

	ports := s.resolveTunnelPorts(form.Ports)
	if engine == "tunnel-panel" {
		return s.generateTunnelPanelOneLiner(form, iranIP, foreignIP, ports)
	}

	frpPort, token := s.genFRPPortAndToken()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = s.ensureHashemCore(ctx)

	var argsForeign []string
	if engine == "backhaul" {
		argsForeign = []string{
			"setup-backhaul-foreign",
			"--local-pub", foreignIP,
			"--remote-pub", iranIP,
			"--port", strconv.Itoa(bhPort),
			"--transport", transport,
			"--token", token,
			"--force",
		}
	} else if engine == "gre-backhaul" {
		argsForeign = []string{
			"setup-gre-backhaul-foreign",
			"--local-pub", foreignIP,
			"--remote-pub", iranIP,
			"--port", strconv.Itoa(bhPort),
			"--transport", transport,
			"--token", token,
			"--force",
		}
	} else {
		argsForeign = []string{
			"setup-foreign",
			"--local-pub", foreignIP,
			"--remote-pub", iranIP,
			"--frp-port", strconv.Itoa(frpPort),
			"--token", token,
			"--ports", ports,
			"--force",
		}
	}
	outForeign, errForeign := s.execPrivileged(ctx, hashemBinPath, argsForeign...)
	if errForeign != nil {
		logger.Warningf("hashem setup-foreign failed: %v, out: %s", errForeign, string(outForeign))
	}

	if engine != "backhaul" {
		if carrier != "" && carrier != "direct" {
			outCarrier, errCarrier := s.execPrivileged(ctx, hashemBinPath, "carrier", "mode", carrier)
			if errCarrier != nil {
				logger.Warningf("hashem carrier mode failed: %v, out: %s", errCarrier, string(outCarrier))
			}
		}
		s.optimizeForeignNetwork(ports)
	}

	if form.AutoCreateInbound {
		inHost := strings.TrimSpace(form.InboundHost)
		if inHost == "" {
			inHost = iranIP
		}
		_ = s.AutoCreateMatchingInbounds(ports, iranIP, inHost)
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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// 1. If hashem core CLI is installed, trigger its native remove-tunnel
	if _, err := os.Stat(hashemBinPath); err == nil {
		_, _ = s.execPrivileged(ctx, hashemBinPath, "remove-tunnel", "--force")
	}

	// 2. Stop and disable all tunnel services
	tunnelServices := []string{
		"frpc", "frps",
		"backhaul-client", "backhaul-server",
		"gre-panel", "tunnel-panel",
		"gre-tunnel", "rathole",
	}
	for _, svc := range tunnelServices {
		_, _ = s.execPrivileged(ctx, "systemctl", "stop", svc)
		_, _ = s.execPrivileged(ctx, "systemctl", "disable", svc)
	}

	// 3. Stop, disable and remove all GRE/GRETAP/Backhaul/FRP systemd unit files
	stopSvcBash := `
for s in /etc/systemd/system/gre-*.service /etc/systemd/system/gretap*.service /etc/systemd/system/bh-*.service /etc/systemd/system/backhaul*.service /etc/systemd/system/frp*.service; do
    [ -f "$s" ] || continue
    unit=$(basename "$s")
    systemctl stop "$unit" 2>/dev/null || true
    systemctl disable "$unit" 2>/dev/null || true
    rm -f "$s"
done
`
	_, _ = s.execPrivilegedBash(ctx, stopSvcBash)

	// 4. Forcefully terminate all tunnel processes
	killBash := `
pkill -9 -f "/usr/local/bin/gre-panel" 2>/dev/null || true
pkill -9 -f "gre-panel" 2>/dev/null || true
pkill -9 -f "frpc" 2>/dev/null || true
pkill -9 -f "frps" 2>/dev/null || true
pkill -9 -f "backhaul" 2>/dev/null || true
pkill -9 -f "rathole" 2>/dev/null || true
pkill -9 -f "tunnel-panel" 2>/dev/null || true
`
	_, _ = s.execPrivilegedBash(ctx, killBash)

	// 5. Cleanly delete all virtual tunnel network interfaces (strip @NONE and @parent properly)
	delIfBash := `
for raw in $(ip -o link show 2>/dev/null | awk -F': ' '{print $2}'); do
    dev=$(echo "$raw" | cut -d'@' -f1)
    case "$dev" in
        gre-a-*|gre-b-*|gre-tunnel*|gretap*|bh-*|backhaul*|tnp-*|hashem-*|fou*)
            ip link set dev "$dev" down 2>/dev/null || true
            ip link del dev "$dev" 2>/dev/null || true
            ;;
    esac
done
for d in gre-a-1 gre-a-2 gre-b-1 gre-b-2 gre-tunnel gretap0 gretap1 gretap2; do
    ip link set dev "$d" down 2>/dev/null || true
    ip link del dev "$d" 2>/dev/null || true
done
`
	_, _ = s.execPrivilegedBash(ctx, delIfBash)

	// 6. Delete FoU listeners
	fouBash := `
ip fou del port 443 2>/dev/null || true
ip fou del port 19998 2>/dev/null || true
ip fou del port 55555 2>/dev/null || true
for p in $(ip fou show 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="port") print $(i+1)}'); do
    ip fou del port "$p" 2>/dev/null || true
done
`
	_, _ = s.execPrivilegedBash(ctx, fouBash)

	// 7. Flush kernel routing table for tunnel subnets
	routeBash := `
ip route flush dev gre-a-1 2>/dev/null || true
ip route flush dev gre-a-2 2>/dev/null || true
ip route flush dev gre-tunnel 2>/dev/null || true
for subnet in 10.10.10.0/24 10.10.1.0/30 10.10.2.0/30 10.10.3.0/30 10.10.4.0/30 10.10.0.0/16; do
    ip route del "$subnet" 2>/dev/null || true
done
`
	_, _ = s.execPrivilegedBash(ctx, routeBash)

	// 8. Remove IPTables mangle TCPMSS, FORWARD, and NAT masquerade rules
	iptablesBash := `
iptables -t mangle -D POSTROUTING -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1340 2>/dev/null || true
iptables -t mangle -D FORWARD -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1340 2>/dev/null || true
for iface in $(ip -o link show 2>/dev/null | awk -F': ' '{print $2}' | cut -d'@' -f1 | grep -E '^gre'); do
    iptables -t mangle -D POSTROUTING -o "$iface" -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1340 2>/dev/null || true
    iptables -t mangle -D FORWARD -o "$iface" -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1340 2>/dev/null || true
done
iptables -t nat -D POSTROUTING -s 10.10.0.0/16 -j MASQUERADE 2>/dev/null || true
`
	_, _ = s.execPrivilegedBash(ctx, iptablesBash)

	// 9. Flush kernel conntrack so existing TCP/UDP sessions drop immediately
	conntrackBash := `
if command -v conntrack >/dev/null 2>&1; then
    conntrack -F 2>/dev/null || true
fi
`
	_, _ = s.execPrivilegedBash(ctx, conntrackBash)

	// 10. Mark tunnels disabled/deleted and wipe records in gre-panel SQLite database
	dbCleanBash := `python3 -c '
import sqlite3, os
p = "/var/lib/gre-panel/panel.db"
if os.path.exists(p):
    try:
        c = sqlite3.connect(p)
        c.execute("UPDATE Tunnel SET IsEnabled=0, IsDeleted=1")
        c.execute("DELETE FROM RouteDestination")
        c.execute("DELETE FROM Tunnel")
        c.commit()
        c.close()
    except Exception:
        pass
' 2>/dev/null || true`
	_, _ = s.execPrivilegedBash(ctx, dbCleanBash)

	// 11. Clean up config files and service units via privileged bash
	cleanCmd := "rm -rf /etc/frp /etc/backhaul /etc/gre-panel /etc/x-ui/tunnel.json /var/lib/gre-panel/* " +
		"/etc/systemd/system/frpc.service /etc/systemd/system/frps.service " +
		"/etc/systemd/system/backhaul-client.service /etc/systemd/system/backhaul-server.service " +
		"/etc/systemd/system/gre-tunnel.service /etc/systemd/system/gre-panel.service /etc/sysctl.d/99-nova-tunnel.conf /etc/sysctl.d/99-gre-panel.conf"
	_, _ = s.execPrivilegedBash(ctx, cleanCmd)

	// 12. Reload systemd daemon
	_, _ = s.execPrivileged(ctx, "systemctl", "daemon-reload")

	logger.Info("Hashem tunnel removed completely from host and all connections severed")
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
	_ = os.MkdirAll("/etc/gre-panel", 0o755)
	if data, err := json.MarshalIndent(cc, "", "  "); err == nil {
		_ = os.WriteFile(carrierJsonPath, data, 0o644)
	}

	rep, _ := s.GetBenchmark()
	if rep != nil {
		rep.AutoPilot.Enabled = enabled
		if enabled {
			rep.AutoPilot.LastTriggered = time.Now().Format("2006-01-02 15:04:05")
		}
		if data, err := json.MarshalIndent(rep, "", "  "); err == nil {
			_ = os.WriteFile(benchmarkReportPath, data, 0o644)
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
		ID:         "direct",
		Name:       "Direct GRE (Layer-3)",
		Type:       "gre",
		Port:       0,
		AvgRTTMs:   math.Round(avg*10) / 10,
		MinRTTMs:   math.Round(minR*10) / 10,
		MaxRTTMs:   math.Round(maxR*10) / 10,
		PacketLoss: math.Round(loss*10) / 10,
		JitterMs:   math.Round(jit*10) / 10,
		Score:      sc,
		Status:     determineStatus(sc, loss),
		IsActive:   status != nil && (status.ActiveCarrier == "direct" || status.Carrier == "direct"),
	})

	// 2. FoU 443
	avg, minR, maxR, loss, jit, _ = probeTCP(net.JoinHostPort(targetPub, "443"), 3, 300*time.Millisecond)
	sc = calculateScore(loss, avg, jit)
	metrics = append(metrics, CarrierMetric{
		ID:         "fou:443",
		Name:       "FoU : 443 (Foo-over-UDP)",
		Type:       "fou",
		Port:       443,
		AvgRTTMs:   math.Round(avg*10) / 10,
		MinRTTMs:   math.Round(minR*10) / 10,
		MaxRTTMs:   math.Round(maxR*10) / 10,
		PacketLoss: math.Round(loss*10) / 10,
		JitterMs:   math.Round(jit*10) / 10,
		Score:      sc,
		Status:     determineStatus(sc, loss),
		IsActive:   status != nil && (status.ActiveCarrier == "fou:443" || status.Carrier == "fou:443"),
	})

	// 3. FoU 55555
	avg, minR, maxR, loss, jit, _ = probeTCP(net.JoinHostPort(targetPub, "55555"), 3, 300*time.Millisecond)
	sc = calculateScore(loss, avg, jit)
	metrics = append(metrics, CarrierMetric{
		ID:         "fou:55555",
		Name:       "FoU : 55555 (Foo-over-UDP High)",
		Type:       "fou",
		Port:       55555,
		AvgRTTMs:   math.Round(avg*10) / 10,
		MinRTTMs:   math.Round(minR*10) / 10,
		MaxRTTMs:   math.Round(maxR*10) / 10,
		PacketLoss: math.Round(loss*10) / 10,
		JitterMs:   math.Round(jit*10) / 10,
		Score:      sc,
		Status:     determineStatus(sc, loss),
		IsActive:   status != nil && (status.ActiveCarrier == "fou:55555" || status.Carrier == "fou:55555"),
	})

	// 4. WSS 8443
	avg, minR, maxR, loss, jit, _ = probeTLS(net.JoinHostPort(targetPub, "8443"), 3, 350*time.Millisecond)
	sc = calculateScore(loss, avg, jit)
	metrics = append(metrics, CarrierMetric{
		ID:         "wss:8443",
		Name:       "WSS : 8443 (Encrypted WebSocket)",
		Type:       "wss",
		Port:       8443,
		AvgRTTMs:   math.Round(avg*10) / 10,
		MinRTTMs:   math.Round(minR*10) / 10,
		MaxRTTMs:   math.Round(maxR*10) / 10,
		PacketLoss: math.Round(loss*10) / 10,
		JitterMs:   math.Round(jit*10) / 10,
		Score:      sc,
		Status:     determineStatus(sc, loss),
		IsActive:   status != nil && (status.ActiveCarrier == "wss:8443" || status.Carrier == "wss:8443"),
	})

	// 5. Backhaul Mux
	bhPort := 3080
	if status != nil && status.BackhaulPort > 0 {
		bhPort = status.BackhaulPort
	}
	avg, minR, maxR, loss, jit, _ = probeTCP(net.JoinHostPort(targetPub, strconv.Itoa(bhPort)), 3, 300*time.Millisecond)
	sc = calculateScore(loss, avg, jit)
	metrics = append(metrics, CarrierMetric{
		ID:         "backhaul:tcp",
		Name:       fmt.Sprintf("Backhaul Mux (Port %d)", bhPort),
		Type:       "backhaul",
		Port:       bhPort,
		AvgRTTMs:   math.Round(avg*10) / 10,
		MinRTTMs:   math.Round(minR*10) / 10,
		MaxRTTMs:   math.Round(maxR*10) / 10,
		PacketLoss: math.Round(loss*10) / 10,
		JitterMs:   math.Round(jit*10) / 10,
		Score:      sc,
		Status:     determineStatus(sc, loss),
		IsActive:   status != nil && (status.Engine == "backhaul" || status.Engine == "gre-backhaul"),
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

	_ = os.MkdirAll("/etc/gre-panel", 0o755)
	if data, err := json.MarshalIndent(report, "", "  "); err == nil {
		_ = os.WriteFile(benchmarkReportPath, data, 0o644)
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

func (s *HashemService) detectTunnelPanelStatus(status *HashemStatus) {
	type TunnelJSON struct {
		Installed bool   `json:"installed"`
		Engine    string `json:"engine"`
		Iface     string `json:"iface"`
		LocalPub  string `json:"local_pub"`
		RemotePub string `json:"remote_pub"`
		LocalGre  string `json:"local_gre"`
		RemoteGre string `json:"remote_gre"`
		Ports     []int  `json:"ports"`
	}

	found := false
	var tDoc TunnelJSON
	if data, err := os.ReadFile("/etc/x-ui/tunnel.json"); err == nil {
		if err := json.Unmarshal(data, &tDoc); err == nil && tDoc.RemotePub != "" {
			found = true
			if tDoc.Iface == "" {
				tDoc.Iface = "gre-a-1"
			}
		}
	}

	if !found {
		greDb := "/var/lib/gre-panel/panel.db"
		if _, err := os.Stat(greDb); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			pyScript := `import sqlite3, json
try:
    c = sqlite3.connect("/var/lib/gre-panel/panel.db")
    row = c.execute("""
        SELECT t.InterfaceName, t.LocalEndpoint, t.RemoteEndpoint, a.Address, a.PeerAddress
        FROM Tunnel t
        JOIN TunnelAddress a ON t.TunnelID = a.TunnelID
        WHERE t.IsEnabled=1 AND t.IsDeleted=0
        ORDER BY t.TunnelID DESC LIMIT 1
    """).fetchone()
    ports = [r[0] for r in c.execute("SELECT Port FROM RouteDestination WHERE IsEnabled=1").fetchall()]
    if row:
        print(json.dumps({
            "found": True,
            "iface": row[0],
            "local_pub": row[1],
            "remote_pub": row[2],
            "local_gre": row[3],
            "remote_gre": row[4],
            "ports": ports
        }))
    else:
        print(json.dumps({"found": False}))
except Exception as e:
    print(json.dumps({"found": False, "error": str(e)}))
`
			out, err := s.execPrivileged(ctx, "python3", "-c", pyScript)
			cancel()
			if err == nil {
				var doc struct {
					Found     bool   `json:"found"`
					Iface     string `json:"iface"`
					LocalPub  string `json:"local_pub"`
					RemotePub string `json:"remote_pub"`
					LocalGre  string `json:"local_gre"`
					RemoteGre string `json:"remote_gre"`
					Ports     []int  `json:"ports"`
				}
				start := bytes.IndexByte(out, '{')
				end := bytes.LastIndexByte(out, '}')
				if start != -1 && end > start && json.Unmarshal(out[start:end+1], &doc) == nil && doc.Found {
					found = true
					tDoc.Iface = doc.Iface
					tDoc.LocalPub = doc.LocalPub
					tDoc.RemotePub = doc.RemotePub
					tDoc.LocalGre = doc.LocalGre
					tDoc.RemoteGre = doc.RemoteGre
					tDoc.Ports = doc.Ports
				}
			}
		}
	}

	if found && tDoc.RemoteGre != "" {
		iface := tDoc.Iface
		if iface == "" {
			iface = "gre-a-1"
		}
		// Confirm the GRE interface actually exists on Linux network stack
		if _, err := os.Stat("/sys/class/net/" + iface); err == nil {
			status.Role = "foreign"
			status.Engine = "tunnel-panel"
			status.LocalPubIP = tDoc.LocalPub
			status.RemotePubIP = tDoc.RemotePub
			status.LocalGreIP = tDoc.LocalGre
			status.RemoteGreIP = tDoc.RemoteGre
			status.Ports = tDoc.Ports
			if status.Carrier == "" {
				status.Carrier = "direct"
			}
			if status.ActiveCarrier == "" {
				status.ActiveCarrier = "direct"
			}
			status.PingMs = s.measurePing(status.RemoteGreIP, iface, 0)
			if status.PingMs <= 0 {
				status.PingMs = s.measurePing(status.RemoteGreIP, "", 0)
			}
			if status.PingMs > 0 {
				status.Running = true
				status.FrpStatus = "active"
				status.BackhaulStatus = "active"
			} else {
				status.Running = false
				status.FrpStatus = "stopped"
				status.BackhaulStatus = "stopped"
				status.PingMs = -1
			}
			return
		}
	}

	// Tunnel was either removed or interface doesn't exist
	if status.Engine == "tunnel-panel" || status.Engine == "" {
		status.Running = false
		status.FrpStatus = "stopped"
		status.BackhaulStatus = "stopped"
		status.PingMs = -1
		if status.ActiveCarrier == "direct" {
			status.ActiveCarrier = ""
		}
		if status.Carrier == "direct" {
			status.Carrier = ""
		}
		if status.Role == "foreign" || status.Role == "" {
			if _, err := os.Stat(frpcTomlPath); err != nil && !fileExists(backhaulClientTomlPath) {
				status.Role = "none"
				status.LocalPubIP = ""
				status.RemotePubIP = ""
				status.LocalGreIP = ""
				status.RemoteGreIP = ""
				status.Ports = nil
				status.Engine = "none"
			}
		}
	}
}

func (s *HashemService) generateTunnelPanelOneLiner(form HashemOneLinerForm, iranIP, foreignIP, ports string) (*HashemOneLinerResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	iface := "gre-a-1"
	localGre := "10.10.1.1"
	peerGre := "10.10.1.2"

	// 1. Configure the Foreign server GRETAP interface immediately in kernel
	foreignSetupBash := fmt.Sprintf(`
set -e
modprobe ip_gre 2>/dev/null || true
sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1 || true
sysctl -w net.ipv4.conf.all.rp_filter=2 >/dev/null 2>&1 || true
sysctl -w net.ipv4.conf.default.rp_filter=2 >/dev/null 2>&1 || true
echo "net.ipv4.ip_forward=1" > /etc/sysctl.d/99-nova-tunnel.conf 2>/dev/null || true
echo "net.ipv4.conf.all.rp_filter=2" >> /etc/sysctl.d/99-nova-tunnel.conf 2>/dev/null || true
IFNAME="%s"
ip link del $IFNAME 2>/dev/null || true
ip link add name $IFNAME type gretap local %s remote %s ttl 255 tos inherit ikey 2749365187 okey 2749365187 nopmtudisc 2>/dev/null || ip link add name $IFNAME type gretap remote %s ttl 255 tos inherit ikey 2749365187 okey 2749365187 nopmtudisc
ip addr add %s/30 dev $IFNAME
ip link set dev $IFNAME mtu 1372
ip link set dev $IFNAME up

cat << "UNIT_EOF" > /etc/systemd/system/${IFNAME}.service
[Unit]
Description=GRETAP Tunnel $IFNAME (NOVA-X-PANEL)
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStartPre=-/usr/sbin/modprobe ip_gre
ExecStartPre=-/usr/sbin/ip link del $IFNAME
ExecStart=/usr/sbin/ip link add name $IFNAME type gretap remote %s ttl 255 tos inherit ikey 2749365187 okey 2749365187 nopmtudisc
ExecStart=/usr/sbin/ip addr add %s/30 dev $IFNAME
ExecStart=/usr/sbin/ip link set dev $IFNAME mtu 1372
ExecStart=/usr/sbin/ip link set dev $IFNAME up
ExecStop=-/usr/sbin/ip link set dev $IFNAME down
ExecStop=-/usr/sbin/ip link del $IFNAME

[Install]
WantedBy=multi-user.target
UNIT_EOF
systemctl daemon-reload >/dev/null 2>&1 || true
systemctl enable ${IFNAME}.service >/dev/null 2>&1 || true
`, iface, foreignIP, iranIP, iranIP, localGre, iranIP, localGre)

	_, _ = s.execPrivileged(ctx, "bash", "-c", foreignSetupBash)

	// Parse ports
	var portList []int
	for _, p := range strings.Split(ports, ",") {
		if pInt, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && pInt > 0 {
			portList = append(portList, pInt)
		}
	}
	if len(portList) == 0 {
		portList = []int{8080}
	}

	// 2. Persist state to /etc/x-ui/tunnel.json
	stateData := map[string]any{
		"installed":  true,
		"engine":     "tunnel-panel",
		"iface":      iface,
		"local_pub":  foreignIP,
		"remote_pub": iranIP,
		"local_gre":  localGre,
		"remote_gre": peerGre,
		"ports":      portList,
		"updated_at": time.Now().Unix(),
	}
	if bs, err := json.MarshalIndent(stateData, "", "  "); err == nil {
		_ = os.WriteFile("/etc/x-ui/tunnel.json", bs, 0o644)
	}

	// 3. Auto create matching inbounds
	if form.AutoCreateInbound {
		inHost := strings.TrimSpace(form.InboundHost)
		if inHost == "" {
			inHost = iranIP
		}
		_ = s.AutoCreateMatchingInbounds(ports, iranIP, inHost)
	}

	// 4. Generate the One-Liner command for Iran
	oneLiner := buildTunnelPanelIranScript(iranIP, foreignIP, peerGre, localGre, ports)

	return &HashemOneLinerResult{
		OneLinerCommand: oneLiner,
		ForeignIP:       foreignIP,
		IranIP:          iranIP,
		Ports:           ports,
		Engine:          "tunnel-panel",
		Transport:       "kernel-fastpath",
		FrpPort:         7777,
	}, nil
}

func buildTunnelPanelIranScript(iranIP, foreignIP, iranGreIP, foreignGreIP, ports string) string {
	var portList []string
	cleaned := strings.TrimSpace(ports)
	if cleaned != "" {
		for _, part := range strings.Split(cleaned, ",") {
			p := strings.TrimSpace(part)
			if p != "" {
				portList = append(portList, p)
			}
		}
	}
	if len(portList) == 0 {
		portList = []string{"8080"}
	}

	var dnatRules, iptablesRules []string
	for _, p := range portList {
		pInt, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || pInt < 1 || pInt > 65535 {
			continue
		}
		// Match dport directly so it works regardless of NAT
		dnatRules = append(dnatRules,
			fmt.Sprintf("        tcp dport %d dnat ip to %s:%d", pInt, foreignGreIP, pInt),
			fmt.Sprintf("        udp dport %d dnat ip to %s:%d", pInt, foreignGreIP, pInt),
		)
		iptablesRules = append(iptablesRules,
			fmt.Sprintf("    iptables -t nat -A PREROUTING -p tcp --dport %d -j DNAT --to-destination %s:%d 2>/dev/null || true", pInt, foreignGreIP, pInt),
			fmt.Sprintf("    iptables -t nat -A PREROUTING -p udp --dport %d -j DNAT --to-destination %s:%d 2>/dev/null || true", pInt, foreignGreIP, pInt),
		)
	}

	dnatStr := strings.Join(dnatRules, "\n")
	iptablesStr := strings.Join(iptablesRules, "\n")
	portsDisplay := strings.Join(portList, ", ")

	return fmt.Sprintf(`bash -c 'set -e; echo "=== NOVA-X-PANEL NOVA-TUNELL Fast Setup ==="; sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1 || true; sysctl -w net.ipv4.conf.all.rp_filter=2 >/dev/null 2>&1 || true; sysctl -w net.ipv4.conf.default.rp_filter=2 >/dev/null 2>&1 || true; echo "net.ipv4.ip_forward=1" > /etc/sysctl.d/99-nova-tunnel.conf 2>/dev/null || true; echo "net.ipv4.conf.all.rp_filter=2" >> /etc/sysctl.d/99-nova-tunnel.conf 2>/dev/null || true; modprobe ip_gre 2>/dev/null || true; IFNAME="gre-b-1"; ip link del $IFNAME 2>/dev/null || true; ip link add name $IFNAME type gretap local %s remote %s ttl 255 tos inherit ikey 2749365187 okey 2749365187 nopmtudisc 2>/dev/null || ip link add name $IFNAME type gretap remote %s ttl 255 tos inherit ikey 2749365187 okey 2749365187 nopmtudisc; ip addr add %s/30 dev $IFNAME; ip link set dev $IFNAME mtu 1372; ip link set dev $IFNAME up; cat << "UNIT_EOF" > /etc/systemd/system/${IFNAME}.service
[Unit]
Description=GRETAP Tunnel $IFNAME (NOVA-X-PANEL)
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStartPre=-/usr/sbin/modprobe ip_gre
ExecStartPre=-/usr/sbin/ip link del $IFNAME
ExecStart=/usr/sbin/ip link add name $IFNAME type gretap remote %s ttl 255 tos inherit ikey 2749365187 okey 2749365187 nopmtudisc
ExecStart=/usr/sbin/ip addr add %s/30 dev $IFNAME
ExecStart=/usr/sbin/ip link set dev $IFNAME mtu 1372
ExecStart=/usr/sbin/ip link set dev $IFNAME up
ExecStop=-/usr/sbin/ip link set dev $IFNAME down
ExecStop=-/usr/sbin/ip link del $IFNAME

[Install]
WantedBy=multi-user.target
UNIT_EOF
systemctl daemon-reload && systemctl enable --now ${IFNAME}.service >/dev/null 2>&1 || true; mkdir -p /var/lib/gre-panel/rules; cat << "NFT_EOF" > /var/lib/gre-panel/rules/gre-panel.nft
table inet gre_panel
flush table inet gre_panel
table inet gre_panel {
    chain prerouting {
        type nat hook prerouting priority dstnat; policy accept;
%s
    }
    chain postrouting {
        type nat hook postrouting priority srcnat; policy accept;
        oifname "gre-b-1" masquerade
    }
    chain forward {
        type filter hook forward priority filter; policy accept;
        ct state established,related accept
        oifname "gre-b-1" accept
        iifname "gre-b-1" accept
    }
    chain mss_clamp {
        type filter hook forward priority mangle; policy accept;
        oifname "gre-b-1" tcp flags syn / syn,rst tcp option maxseg size set rt mtu
    }
}
NFT_EOF
if which nft >/dev/null 2>&1; then nft -f /var/lib/gre-panel/rules/gre-panel.nft 2>/dev/null || true; cat << "UNIT_NFT" > /etc/systemd/system/gre-panel-rules.service
[Unit]
Description=GRETAP port forwarding rules (nftables)
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/sbin/nft -f /var/lib/gre-panel/rules/gre-panel.nft

[Install]
WantedBy=multi-user.target
UNIT_NFT
systemctl daemon-reload && systemctl enable gre-panel-rules.service >/dev/null 2>&1 || true; fi;
%s
iptables -t nat -A POSTROUTING -o gre-b-1 -j MASQUERADE 2>/dev/null || true
iptables -A FORWARD -o gre-b-1 -j ACCEPT 2>/dev/null || true
iptables -A FORWARD -i gre-b-1 -j ACCEPT 2>/dev/null || true
iptables -A FORWARD -m state --state ESTABLISHED,RELATED -j ACCEPT 2>/dev/null || true
echo "=========================================="; echo "[✓] تانل با موفقیت برقرار شد! پورت‌های فعال: %s"; echo "پینگ به سرور خارج (%s):"; ping -c 3 -W 2 %s || true; echo "=========================================="'`,
		iranIP, foreignIP, foreignIP, iranGreIP,
		foreignIP, iranGreIP,
		dnatStr,
		iptablesStr,
		portsDisplay, foreignGreIP, foreignGreIP,
	)
}
