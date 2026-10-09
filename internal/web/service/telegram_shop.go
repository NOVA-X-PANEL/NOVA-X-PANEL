package service

import (
	_ "embed"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	tgShopConfigPath  = "/etc/dark-shop/config.json"
	tgShopPlansPath   = "/etc/dark-shop/plans.json"
	tgShopDbPath      = "/etc/dark-shop/shop.db"
	tgShopServiceUnit = "dark-shop-bot.service"
	tgShopScriptPath  = "/usr/local/bin/dark_shop_bot.py"
	mirzaHelperPath   = "/usr/local/bin/mirzabot_helper.php"
)

//go:embed dark_shop_bot.py
var defaultBotScript []byte

// TelegramShopConfig represents settings required to run the bot.
type TelegramShopConfig struct {
	BotToken         string  `json:"bot_token" form:"bot_token"`
	AdminChatID      string  `json:"admin_chat_id" form:"admin_chat_id"`
	CardNumber       string  `json:"card_number" form:"card_number"`
	CardHolder       string  `json:"card_holder" form:"card_holder"`
	BotEnabled       bool    `json:"bot_enabled" form:"bot_enabled"`
	FreeTrialEnabled bool    `json:"free_trial_enabled" form:"free_trial_enabled"`
	FreeTrialGB      float64 `json:"free_trial_gb" form:"free_trial_gb"`
	FreeTrialDays    int     `json:"free_trial_days" form:"free_trial_days"`
	DefaultInboundID int     `json:"default_inbound_id" form:"default_inbound_id"`
	SupportUsername  string  `json:"support_username" form:"support_username"`
	ChannelUsername  string  `json:"channel_username" form:"channel_username"`
}

// TelegramShopStatus returns the operational status of the bot.
type TelegramShopStatus struct {
	Installed       bool   `json:"installed"`
	Running         bool   `json:"running"`
	BotToken        string `json:"bot_token"`
	AdminChatID     string `json:"admin_chat_id"`
	CardNumber      string `json:"card_number"`
	CardHolder      string `json:"card_holder"`
	BotUsername     string `json:"bot_username"`
	BotFirstName    string `json:"bot_first_name"`
	SupportUsername string `json:"support_username"`
	ChannelUsername string `json:"channel_username"`
	TotalTrials     int    `json:"total_trials"`
	TotalOrders     int    `json:"total_orders"`
}

type TelegramShopService struct {
	mu sync.Mutex
}

// GetStatus checks if the Telegram Shop Bot is installed and running.
func (s *TelegramShopService) GetStatus() (*TelegramShopStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := &TelegramShopStatus{
		Installed: false,
		Running:   false,
	}

	// Priority 1: Check MirzaBot native helper
	if _, err := os.Stat(mirzaHelperPath); err == nil {
		out, err := exec.Command(mirzaHelperPath, "status").Output()
		if err == nil {
			var m struct {
				Installed       bool   `json:"installed"`
				Running         bool   `json:"running"`
				BotToken        string `json:"bot_token"`
				AdminChatID     string `json:"admin_chat_id"`
				BotUsername     string `json:"bot_username"`
				Domain          string `json:"domain"`
				RegisteredUsers int    `json:"registered_users"`
				ActiveOrders    int    `json:"active_orders"`
			}
			if json.Unmarshal(out, &m) == nil && m.Installed {
				status.Installed = true
				status.Running = m.Running
				status.BotToken = m.BotToken
				status.AdminChatID = m.AdminChatID
				status.BotUsername = m.BotUsername
				status.TotalTrials = m.RegisteredUsers
				status.TotalOrders = m.ActiveOrders
				if status.BotToken != "" && status.BotUsername == "" {
					s.fetchBotInfo(status.BotToken, status)
				}
				return status, nil
			}
		}
	}

	if _, err := os.Stat(tgShopConfigPath); err != nil {
		return status, nil
	}
	if _, err := os.Stat(tgShopScriptPath); err != nil {
		return status, nil
	}

	status.Installed = true

	// Check systemd status
	out, err := exec.Command("systemctl", "is-active", tgShopServiceUnit).Output()
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		status.Running = true
	}

	// Read config
	data, err := os.ReadFile(tgShopConfigPath)
	if err == nil {
		var cfg TelegramShopConfig
		if json.Unmarshal(data, &cfg) == nil {
			status.BotToken = cfg.BotToken
			status.AdminChatID = cfg.AdminChatID
			status.CardNumber = cfg.CardNumber
			status.CardHolder = cfg.CardHolder
			status.SupportUsername = cfg.SupportUsername
			status.ChannelUsername = cfg.ChannelUsername

			// Fetch bot username from Telegram if token is present
			if cfg.BotToken != "" {
				s.fetchBotInfo(cfg.BotToken, status)
			}
		}
	}

	// Read stats from DB
	if _, err := os.Stat(tgShopDbPath); err == nil {
		db, err := sql.Open("sqlite3", tgShopDbPath)
		if err == nil {
			defer db.Close()
			_ = db.QueryRow("SELECT COUNT(*) FROM trials").Scan(&status.TotalTrials)
			_ = db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&status.TotalOrders)
		}
	}

	return status, nil
}

func (s *TelegramShopService) fetchBotInfo(token string, st *TelegramShopStatus) {
	client := http.Client{Timeout: 3 * time.Second}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", token)
	resp, err := client.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var res struct {
		OK     bool `json:"ok"`
		Result struct {
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
		} `json:"result"`
	}
	if json.Unmarshal(body, &res) == nil && res.OK {
		st.BotUsername = res.Result.Username
		st.BotFirstName = res.Result.FirstName
	}
}

// Install sets up the Telegram Shop Bot configuration, script, and systemd service.
func (s *TelegramShopService) Install(cfg *TelegramShopConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg.BotToken = strings.TrimSpace(cfg.BotToken)
	cfg.AdminChatID = strings.TrimSpace(cfg.AdminChatID)

	if cfg.BotToken == "" {
		return fmt.Errorf("bot_token cannot be empty")
	}
	if cfg.AdminChatID == "" {
		return fmt.Errorf("admin_chat_id cannot be empty")
	}

	// Priority 1: MirzaBot native installation & sync
	if _, err := os.Stat(mirzaHelperPath); err == nil {
		st := &TelegramShopStatus{}
		s.fetchBotInfo(cfg.BotToken, st)
		out, err := exec.Command(mirzaHelperPath, "install", cfg.BotToken, cfg.AdminChatID, st.BotUsername).CombinedOutput()
		if err != nil {
			return fmt.Errorf("mirza install error: %s (%v)", string(out), err)
		}
		return nil
	}

	_ = os.MkdirAll("/etc/dark-shop", 0755)

	if cfg.CardNumber == "" {
		cfg.CardNumber = "۶۰۳۷-۹۹۷۴-XXXX-XXXX"
	}
	if cfg.CardHolder == "" {
		cfg.CardHolder = "DARK VVPN"
	}
	cfg.BotEnabled = true
	cfg.FreeTrialEnabled = true
	cfg.FreeTrialGB = 1.0
	cfg.FreeTrialDays = 1
	cfg.DefaultInboundID = 1
	if cfg.SupportUsername == "" {
		cfg.SupportUsername = "TheSilentOnee"
	}
	if cfg.ChannelUsername == "" {
		cfg.ChannelUsername = "DARK_VVPN"
	}

	// Write bot script
	if len(defaultBotScript) > 0 {
		_ = os.WriteFile(tgShopScriptPath, defaultBotScript, 0755)
	}

	// Write config
	cfgBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tgShopConfigPath, cfgBytes, 0644); err != nil {
		return err
	}

	// Default plans if not exists
	if _, err := os.Stat(tgShopPlansPath); os.IsNotExist(err) {
		defaultPlans := `[
  {"id": 1, "title": "یک‌ماهه پایه (۳۰ گیگ)", "traffic_gb": 30, "period_days": 30, "price_tomans": 90000, "enabled": true, "inbound_id": 1},
  {"id": 2, "title": "یک‌ماهه نقره‌ای (۵۰ گیگ)", "traffic_gb": 50, "period_days": 30, "price_tomans": 140000, "enabled": true, "inbound_id": 1},
  {"id": 3, "title": "یک‌ماهه طلایی (۸۰ گیگ)", "traffic_gb": 80, "period_days": 30, "price_tomans": 210000, "enabled": true, "inbound_id": 1},
  {"id": 4, "title": "سه‌ماهه ویژه (۱۲۰ گیگ)", "traffic_gb": 120, "period_days": 90, "price_tomans": 320000, "enabled": true, "inbound_id": 1}
]`
		_ = os.WriteFile(tgShopPlansPath, []byte(defaultPlans), 0644)
	}

	// Systemd unit
	unitContent := `[Unit]
Description=DARK VVPN Telegram Shop Bot Daemon
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/etc/dark-shop
ExecStart=/usr/bin/python3 /usr/local/bin/dark_shop_bot.py
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`
	if err := os.WriteFile("/etc/systemd/system/dark-shop-bot.service", []byte(unitContent), 0644); err != nil {
		return err
	}

	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("systemctl", "enable", "--now", tgShopServiceUnit).Run()
	_ = exec.Command("systemctl", "restart", tgShopServiceUnit).Run()

	// Clear any active Telegram webhook asynchronously so polling getUpdates works without conflict
	go func(token string) {
		cl := http.Client{Timeout: 5 * time.Second}
		resp, err := cl.Get(fmt.Sprintf("https://api.telegram.org/bot%s/deleteWebhook?drop_pending_updates=true", token))
		if err == nil && resp != nil {
			_ = resp.Body.Close()
		}
	}(cfg.BotToken)

	return nil
}

// Action performs management operations (start, stop, restart, uninstall).
func (s *TelegramShopService) Action(action string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Priority 1: MirzaBot native actions
	if _, err := os.Stat(mirzaHelperPath); err == nil {
		out, err := exec.Command(mirzaHelperPath, "action", action).CombinedOutput()
		if err != nil {
			return fmt.Errorf("mirza action error: %s (%v)", string(out), err)
		}
		return nil
	}

	switch action {
	case "start":
		return exec.Command("systemctl", "start", tgShopServiceUnit).Run()
	case "stop":
		return exec.Command("systemctl", "stop", tgShopServiceUnit).Run()
	case "restart":
		return exec.Command("systemctl", "restart", tgShopServiceUnit).Run()
	case "uninstall":
		_ = exec.Command("systemctl", "stop", tgShopServiceUnit).Run()
		_ = exec.Command("systemctl", "disable", tgShopServiceUnit).Run()
		_ = os.Remove("/etc/systemd/system/dark-shop-bot.service")
		_ = os.Remove(tgShopScriptPath)
		_ = os.RemoveAll("/etc/dark-shop")
		_ = exec.Command("systemctl", "daemon-reload").Run()
		return nil
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}
