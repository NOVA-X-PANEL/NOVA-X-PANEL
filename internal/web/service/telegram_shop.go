package service

import (
	_ "embed"
	"context"
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

//go:embed mirzabot_helper.php
var defaultMirzaHelper []byte

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
	Domain          string `json:"domain"`
	TotalTrials     int    `json:"total_trials"`
	TotalOrders     int    `json:"total_orders"`
}

type TelegramShopService struct {
	mu sync.Mutex
}

func (s *TelegramShopService) ensureMirzaHelper() {
	if len(defaultMirzaHelper) == 0 {
		return
	}
	if _, err := os.Stat(mirzaHelperPath); err != nil {
		_ = os.WriteFile(mirzaHelperPath, defaultMirzaHelper, 0755)
	}
}

// GetStatus checks if the Telegram Shop Bot is installed and running.
func (s *TelegramShopService) GetStatus() (*TelegramShopStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureMirzaHelper()

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
				status.Domain = m.Domain
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

func (s *TelegramShopService) execPrivilegedBash(ctx context.Context, script string) ([]byte, error) {
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

func (s *TelegramShopService) getPanelDetails() (token string, port string, inboundId int) {
	port = "2053"
	inboundId = 1
	db, err := sql.Open("sqlite3", "/etc/x-ui/x-ui.db")
	if err != nil {
		return "", port, inboundId
	}
	defer db.Close()

	_ = db.QueryRow("SELECT value FROM settings WHERE key = 'webPort' LIMIT 1").Scan(&port)
	if port == "" {
		port = "2053"
	}

	_ = db.QueryRow("SELECT id FROM inbounds WHERE enable = 1 ORDER BY id ASC LIMIT 1").Scan(&inboundId)
	if inboundId <= 0 {
		inboundId = 1
	}

	_ = db.QueryRow("SELECT token FROM api_tokens WHERE name = 'mirzabot' AND enabled = 1 LIMIT 1").Scan(&token)
	if token == "" {
		_ = db.QueryRow("SELECT token FROM api_tokens WHERE (scope = 'admin' OR scope = 'admin:full') AND enabled = 1 LIMIT 1").Scan(&token)
	}

	if token == "" {
		token = fmt.Sprintf("%x%x", time.Now().UnixNano(), time.Now().Unix())
		now := time.Now().Unix()
		_, _ = db.Exec("INSERT INTO api_tokens (name, token, enabled, created_at, scope, expires_at, admin_id) VALUES ('mirzabot', ?, 1, ?, 'admin', 0, 0)", token, now)
	}

	return token, port, inboundId
}

func (s *TelegramShopService) installMirzaBotNativeStack(ctx context.Context, cfg *TelegramShopConfig) error {
	token, port, inboundId := s.getPanelDetails()

	installScript := fmt.Sprintf(`
set -e
export DEBIAN_FRONTEND=noninteractive

# 1. Install PHP, MariaDB, curl, tar, unzip
if command -v apt-get >/dev/null 2>&1; then
    apt-get update -qq || true
    apt-get install -y -qq php-cli php-fpm php-mysql php-curl php-mbstring php-zip php-gd mariadb-server curl tar unzip || true
elif command -v dnf >/dev/null 2>&1; then
    dnf install -y -q php-cli php-fpm php-mysqlnd php-curl php-mbstring php-zip php-gd mariadb-server curl tar unzip || true
elif command -v yum >/dev/null 2>&1; then
    yum install -y -q php-cli php-fpm php-mysqlnd php-curl php-mbstring php-zip php-gd mariadb-server curl tar unzip || true
fi

# 2. Ensure MySQL / MariaDB service running
systemctl enable --now mariadb 2>/dev/null || systemctl enable --now mysql 2>/dev/null || true

# 3. Create database and dedicated user
mysql -e "CREATE DATABASE IF NOT EXISTS mirzabot CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" 2>/dev/null || true
mysql -e "CREATE USER IF NOT EXISTS 'mirzauser'@'localhost' IDENTIFIED BY 'MirzaDbSecure_Nova!';" 2>/dev/null || true
mysql -e "GRANT ALL PRIVILEGES ON mirzabot.* TO 'mirzauser'@'localhost'; FLUSH PRIVILEGES;" 2>/dev/null || true

# 4. Deploy Mirza Bot source directly from NOVA-X-PANEL/mirzabot repository
mkdir -p /var/www/mirzabot
if [ ! -f /var/www/mirzabot/index.php ]; then
    TMP_DIR="/tmp/mirzabot_setup_$$"
    rm -rf "$TMP_DIR"
    mkdir -p "$TMP_DIR"
    curl -fsSL --retry 3 --max-time 60 "https://github.com/NOVA-X-PANEL/mirzabot/releases/download/v1.0.1/mirzabot.tar.gz" -o "$TMP_DIR/mirzabot.tar.gz" || \
    curl -fsSL --retry 3 --max-time 60 "https://github.com/NOVA-X-PANEL/mirzabot/archive/refs/heads/main.tar.gz" -o "$TMP_DIR/mirzabot.tar.gz"
    tar -xzf "$TMP_DIR/mirzabot.tar.gz" -C /var/www/mirzabot || true
    if [ -d /var/www/mirzabot/mirzabot-main ]; then
        cp -rn /var/www/mirzabot/mirzabot-main/* /var/www/mirzabot/
        rm -rf /var/www/mirzabot/mirzabot-main
    fi
    rm -rf "$TMP_DIR"
fi

# 5. Write base config.php
cat << 'EOF' > /var/www/mirzabot/config.php
<?php
// Mirza Bot (NOVA-X-PANEL Edition) Configuration
$request_exec_timeout = null;
$dbhost = getenv('DB_HOST') ?: 'localhost';
$dbname = getenv('DB_NAME') ?: 'mirzabot';
$usernamedb = getenv('DB_USER') ?: 'mirzauser';
$passworddb = getenv('DB_PASS') ?: 'MirzaDbSecure_Nova!';

$options = [
    PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
    PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
    PDO::ATTR_EMULATE_PREPARES => false,
    PDO::MYSQL_ATTR_INIT_COMMAND => "SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci",
];
$dsn = "mysql:host=$dbhost;dbname=$dbname;charset=utf8mb4";
try {
    $pdo = new PDO($dsn, $usernamedb, $passworddb, $options);
} catch (\PDOException $e) {
    error_log("Database connection failed: " . $e->getMessage());
}

$APIKEY = '%s';
$adminnumber = '%s';
$domainhosts = '';
$usernamebot = '';
?>
EOF
chmod 644 /var/www/mirzabot/config.php

# 6. Initialize database tables via table.php
if [ -f /var/www/mirzabot/table.php ]; then
    php /var/www/mirzabot/table.php >/dev/null 2>&1 || true
fi

# 7. Register NOVA-X-PANEL as local panel in Mirza Bot DB
mysql -u mirzauser -p'MirzaDbSecure_Nova!' mirzabot -e "
INSERT INTO marzban_panel (
    code_panel, name_panel, status, url_panel, password_panel, type, inboundid, inbounds, conecton, on_hold_test, version_panel
) VALUES (
    'nova_panel', 'NOVA-X-PANEL', 'active', 'http://127.0.0.1:%s', '%s', 'x-ui_single', '[%d]', '[%d]', 'onconecton', '0', '3'
) ON DUPLICATE KEY UPDATE url_panel = VALUES(url_panel), password_panel = VALUES(password_panel);
" 2>/dev/null || true

# 8. Setup systemd unit for background poller (mirzabot-poller.service)
cat << 'EOF' > /etc/systemd/system/mirzabot-poller.service
[Unit]
Description=Mirza Bot Polling Daemon for NOVA-X-PANEL
After=network.target mariadb.service mysql.service

[Service]
Type=simple
User=root
WorkingDirectory=/var/www/mirzabot
ExecStart=/usr/bin/php /var/www/mirzabot/poll.php
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now mirzabot-poller.service 2>/dev/null || true
`, cfg.BotToken, cfg.AdminChatID, port, token, inboundId, inboundId)

	out, err := s.execPrivilegedBash(ctx, installScript)
	if err != nil {
		return fmt.Errorf("script execution error: %s (%v)", string(out), err)
	}
	return nil
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

	s.ensureMirzaHelper()

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// Stop legacy python dark-shop-bot if running
	_, _ = s.execPrivilegedBash(ctx, "systemctl stop dark-shop-bot.service 2>/dev/null; systemctl disable dark-shop-bot.service 2>/dev/null || true")

	// Check if Mirza Bot is installed; if not, perform automated native deployment
	if _, err := os.Stat("/var/www/mirzabot/config.php"); os.IsNotExist(err) {
		if err := s.installMirzaBotNativeStack(ctx, cfg); err != nil {
			return fmt.Errorf("native mirzabot install error: %v", err)
		}
	}

	st := &TelegramShopStatus{}
	s.fetchBotInfo(cfg.BotToken, st)
	if st.BotUsername == "" {
		st.BotUsername = "ShopBot"
	}

	// Priority 1: MirzaBot native installation & sync
	if _, err := os.Stat(mirzaHelperPath); err == nil {
		out, err := exec.Command(mirzaHelperPath, "install", cfg.BotToken, cfg.AdminChatID, st.BotUsername).CombinedOutput()
		if err != nil {
			return fmt.Errorf("mirza install error: %s (%v)", string(out), err)
		}
		return nil
	}

	return fmt.Errorf("mirzabot helper CLI not found at %s", mirzaHelperPath)
}

// Action performs management operations (start, stop, restart, uninstall).
func (s *TelegramShopService) Action(action string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureMirzaHelper()

	// Priority 1: MirzaBot native actions
	if _, err := os.Stat(mirzaHelperPath); err == nil {
		out, err := exec.Command(mirzaHelperPath, "action", action).CombinedOutput()
		if err != nil {
			return fmt.Errorf("mirza action error: %s (%v)", string(out), err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	switch action {
	case "start", "stop", "restart":
		out, err := s.execPrivilegedBash(ctx, fmt.Sprintf("systemctl %s %s", action, tgShopServiceUnit))
		if err != nil {
			return fmt.Errorf("action %s error: %s (%v)", action, string(out), err)
		}
		return nil
	case "uninstall":
		cmd := `
systemctl stop dark-shop-bot.service 2>/dev/null || true
systemctl disable dark-shop-bot.service 2>/dev/null || true
rm -f /etc/systemd/system/dark-shop-bot.service
rm -f /usr/local/bin/dark_shop_bot.py
rm -rf /etc/dark-shop
systemctl daemon-reload
`
		out, err := s.execPrivilegedBash(ctx, cmd)
		if err != nil {
			return fmt.Errorf("uninstall error: %s (%v)", string(out), err)
		}
		return nil
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

// InstallDependencies installs system packages required for Mirza Bot.
func (s *TelegramShopService) InstallDependencies() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	cmd := `
set -e
export DEBIAN_FRONTEND=noninteractive
if command -v apt-get >/dev/null 2>&1; then
    apt-get update -qq || true
    apt-get install -y -qq php-cli php-fpm php-mysql php-curl php-mbstring php-zip php-gd mariadb-server curl tar unzip python3 python3-requests python3-qrcode || true
elif command -v dnf >/dev/null 2>&1; then
    dnf install -y -q php-cli php-fpm php-mysqlnd php-curl php-mbstring php-zip php-gd mariadb-server curl tar unzip python3 python3-requests python3-qrcode || true
elif command -v yum >/dev/null 2>&1; then
    yum install -y -q php-cli php-fpm php-mysqlnd php-curl php-mbstring php-zip php-gd mariadb-server curl tar unzip python3 python3-requests python3-qrcode || true
fi
systemctl enable --now mariadb 2>/dev/null || systemctl enable --now mysql 2>/dev/null || true
`
	out, err := s.execPrivilegedBash(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to install dependencies: %s (%v)", string(out), err)
	}
	return nil
}
