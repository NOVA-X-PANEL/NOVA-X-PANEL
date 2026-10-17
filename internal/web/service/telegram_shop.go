package service

import (
	"context"
	"crypto/tls"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	tgShopConfigPath = "/etc/dark-shop/config.json"
	mirzaBotDir      = "/var/www/mirzabot"
	mirzaConfigPath  = "/var/www/mirzabot/config.php"
	mirzaHelperPath  = "/usr/local/bin/mirzabot_helper.php"
	mirzaPollerUnit  = "mirzabot-poller.service"
)

//go:embed mirzabot.tar.gz
var defaultMirzaBotTarGz []byte

//go:embed mirzabot_helper.php
var defaultMirzaHelper []byte

// TelegramShopConfig represents settings required to run the bot.
type TelegramShopConfig struct {
	BotToken         string  `json:"bot_token" form:"bot_token"`
	AdminChatID      string  `json:"admin_chat_id" form:"admin_chat_id"`
	Domain           string  `json:"domain" form:"domain"`
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
	tmpHelper := fmt.Sprintf("/tmp/mirza_helper_%d.php", time.Now().UnixNano())
	if err := os.WriteFile(tmpHelper, defaultMirzaHelper, 0o755); err == nil {
		defer os.Remove(tmpHelper)
		_, _ = s.execPrivilegedBash(context.Background(), fmt.Sprintf("cp -f %s %s && chmod 755 %s", tmpHelper, mirzaHelperPath, mirzaHelperPath))
	}
}

func (s *TelegramShopService) EnsureWebUnit() {
	if _, err := os.Stat("/var/www/mirzabot/index.php"); os.IsNotExist(err) && len(defaultMirzaBotTarGz) > 0 {
		_ = os.MkdirAll("/var/www/mirzabot", 0o755)
		tmpTar := fmt.Sprintf("/tmp/novabot_sync_%d.tar.gz", time.Now().UnixNano())
		if err := os.WriteFile(tmpTar, defaultMirzaBotTarGz, 0o644); err == nil {
			defer os.Remove(tmpTar)
			cmd := fmt.Sprintf("tar --exclude='config.php' -zxf %s -C /var/www/mirzabot >/dev/null 2>&1 && chown -R www-data:www-data /var/www/mirzabot && php /var/www/mirzabot/table.php >/dev/null 2>&1 || true", tmpTar)
			_, _ = s.execPrivilegedBash(context.Background(), cmd)
		}
	}
	if _, err := os.Stat("/var/www/mirzabot"); os.IsNotExist(err) {
		return
	}

	// Guarantee config.php exists so PHP never throws fatal error
	if _, err := os.Stat("/var/www/mirzabot/config.php"); os.IsNotExist(err) {
		defaultConfig := `<?php
$request_exec_timeout = null;
$dbhost = getenv('DB_HOST') ?: '127.0.0.1';
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
    try {
        $pdo = new PDO("mysql:host=127.0.0.1;dbname=$dbname;charset=utf8mb4", 'root', '', $options);
    } catch (\PDOException $e2) {
        $pdo = null;
    }
}
$APIKEY = '';
$adminnumber = '';
$domainhosts = '';
$usernamebot = '';
`
		_ = os.WriteFile("/var/www/mirzabot/config.php", []byte(defaultConfig), 0o644)
	}

	script := `
if [ ! -f /etc/systemd/system/novabot-web.service ] || ! systemctl is-active --quiet novabot-web.service; then
    PHP_BIN=""
    if [ -x /usr/bin/php ]; then PHP_BIN="/usr/bin/php";
    elif [ -x /usr/local/bin/php ]; then PHP_BIN="/usr/local/bin/php";
    elif [ -x /usr/bin/php8.3 ]; then PHP_BIN="/usr/bin/php8.3";
    elif [ -x /usr/bin/php8.2 ]; then PHP_BIN="/usr/bin/php8.2";
    elif [ -x /usr/bin/php8.1 ]; then PHP_BIN="/usr/bin/php8.1";
    else PHP_BIN="$(command -v php 2>/dev/null || true)"; fi

    if [ -z "$PHP_BIN" ] || [ ! -x "$PHP_BIN" ]; then
        if command -v apt-get >/dev/null 2>&1; then
            apt-get update -qq && apt-get install -y -qq php-cli php-fpm php-mysql php-curl php-mbstring || true
        elif command -v dnf >/dev/null 2>&1; then
            dnf install -y -q php-cli php-mysqlnd php-curl php-mbstring || true
        fi
        PHP_BIN="$(command -v php 2>/dev/null || echo '/usr/bin/php')"
    fi

    # Ensure systemd drop-in for x-ui permissions
    mkdir -p /etc/systemd/system/x-ui.service.d
    cat <<EOF > /etc/systemd/system/x-ui.service.d/30-novabot.conf
[Service]
ReadWritePaths=-/var/www -/etc/dark-shop -/etc/systemd/system -/run/systemd/system
ReadWriteDirectories=-/var/www -/etc/dark-shop -/etc/systemd/system -/run/systemd/system
EOF

    cat <<EOF > /etc/systemd/system/novabot-web.service
[Unit]
Description=NOVA Bot Web Service (Local)
After=network.target mariadb.service mysql.service

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/var/www/mirzabot
Environment="PHP_CLI_SERVER_WORKERS=8"
ExecStart=${PHP_BIN} -d memory_limit=256M -d max_execution_time=60 -S 127.0.0.1:9099 -t /var/www/mirzabot
Restart=always
RestartSec=2
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
    chmod 644 /etc/systemd/system/novabot-web.service
    systemctl daemon-reload
    systemctl enable --now novabot-web.service 2>/dev/null || true
    systemctl restart novabot-web.service 2>/dev/null || true
fi
`
	_, _ = s.execPrivilegedBash(context.Background(), script)

	// Check if port 9099 is listening now. If not, start direct process fallback
	conn, err := net.DialTimeout("tcp", "127.0.0.1:9099", 250*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return
	}

	// Direct fallback: if systemd couldn't start it, start via exec.Command in background
	phpBin := "/usr/bin/php"
	if _, err := os.Stat(phpBin); os.IsNotExist(err) {
		if path, err := exec.LookPath("php"); err == nil {
			phpBin = path
		}
	}
	cmd := exec.Command(phpBin, "-d", "memory_limit=256M", "-d", "max_execution_time=60", "-S", "127.0.0.1:9099", "-t", "/var/www/mirzabot")
	cmd.Dir = "/var/www/mirzabot"
	cmd.Env = append(os.Environ(), "PHP_CLI_SERVER_WORKERS=8")
	_ = cmd.Start()
}

func (s *TelegramShopService) EnsurePollerUnit() {
	if _, err := os.Stat("/var/www/mirzabot"); os.IsNotExist(err) {
		return
	}
	script := `
if [ ! -f /etc/systemd/system/mirzabot-poller.service ]; then
    PHP_BIN=""
    if [ -x /usr/bin/php ]; then PHP_BIN="/usr/bin/php";
    elif [ -x /usr/local/bin/php ]; then PHP_BIN="/usr/local/bin/php";
    elif [ -x /usr/bin/php8.3 ]; then PHP_BIN="/usr/bin/php8.3";
    elif [ -x /usr/bin/php8.2 ]; then PHP_BIN="/usr/bin/php8.2";
    elif [ -x /usr/bin/php8.1 ]; then PHP_BIN="/usr/bin/php8.1";
    else PHP_BIN="$(command -v php 2>/dev/null || true)"; fi

    if [ -z "$PHP_BIN" ] || [ ! -x "$PHP_BIN" ]; then
        PHP_BIN="$(command -v php 2>/dev/null || echo '/usr/bin/php')"
    fi

    cat <<EOF > /etc/systemd/system/mirzabot-poller.service
[Unit]
Description=NOVA Bot Polling Daemon for NOVA-X-PANEL
After=network.target mariadb.service mysql.service

[Service]
Type=simple
User=root
WorkingDirectory=/var/www/mirzabot
ExecStart=${PHP_BIN} /var/www/mirzabot/poll.php
Restart=always
RestartSec=3
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
    chmod 644 /etc/systemd/system/mirzabot-poller.service
    systemctl daemon-reload
fi
`
	_, _ = s.execPrivilegedBash(context.Background(), script)
}

// purgeLegacyBot completely terminates, disables, and removes any previous python dark-shop-bot remnants.
func (s *TelegramShopService) purgeLegacyBot() {
	_, _ = s.execPrivilegedBash(context.Background(), `
systemctl stop dark-shop-bot.service 2>/dev/null || true
systemctl disable dark-shop-bot.service 2>/dev/null || true
rm -f /etc/systemd/system/dark-shop-bot.service
rm -f /usr/local/bin/dark_shop_bot.py
pkill -9 -f dark_shop_bot.py 2>/dev/null || true
systemctl daemon-reload 2>/dev/null || true
`)
}

// GetStatus checks if Mirza Bot is installed and running natively.
func (s *TelegramShopService) GetStatus() (*TelegramShopStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.purgeLegacyBot()
	s.ensureMirzaHelper()
	s.EnsureWebUnit()

	status := &TelegramShopStatus{
		Installed: false,
		Running:   false,
	}

	// Read backup credentials if any (to prefill inputs on fresh install)
	if data, err := os.ReadFile(tgShopConfigPath); err == nil {
		var cfg TelegramShopConfig
		if json.Unmarshal(data, &cfg) == nil {
			status.BotToken = cfg.BotToken
			status.AdminChatID = cfg.AdminChatID
			status.CardNumber = cfg.CardNumber
			status.CardHolder = cfg.CardHolder
			status.SupportUsername = cfg.SupportUsername
			status.ChannelUsername = cfg.ChannelUsername
		}
	}

	// Check if Mirza Bot is installed
	if _, err := os.Stat(mirzaConfigPath); err != nil {
		// Not installed
		if status.BotToken != "" {
			s.fetchBotInfo(status.BotToken, status)
		}
		return status, nil
	}

	// Check if config.php contains real credentials or placeholder
	if cfgData, err := os.ReadFile(mirzaConfigPath); err == nil {
		cfgStr := string(cfgData)
		if strings.Contains(cfgStr, "{API_KEY}") || strings.Contains(cfgStr, "$APIKEY = '';") {
			status.Installed = false
			status.Running = false
			status.BotToken = ""
			status.AdminChatID = ""
			status.BotUsername = ""
			return status, nil
		}
	}

	// Mirza Bot IS installed
	status.Installed = true

	// Check systemd status
	out, err := s.execPrivilegedBash(context.Background(), "systemctl is-active "+mirzaPollerUnit)
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		status.Running = true
	}

	// Read detailed stats via helper CLI
	if _, err := os.Stat(mirzaHelperPath); err == nil {
		out, err := s.execPrivilegedBash(context.Background(), mirzaHelperPath+" status")
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
			if json.Unmarshal(out, &m) == nil {
				status.Installed = m.Installed
				if !status.Installed {
					status.Running = false
					status.BotToken = ""
					status.AdminChatID = ""
					status.BotUsername = ""
					return status, nil
				}
				if m.BotToken != "" {
					status.BotToken = m.BotToken
				}
				if m.AdminChatID != "" {
					status.AdminChatID = m.AdminChatID
				}
				if m.BotUsername != "" {
					status.BotUsername = m.BotUsername
				}
				if m.Domain != "" {
					cleanDisp := m.Domain
					if idx := strings.Index(cleanDisp, "/"); idx != -1 {
						cleanDisp = cleanDisp[:idx]
					}
					if idx := strings.Index(cleanDisp, ":"); idx != -1 {
						cleanDisp = cleanDisp[:idx]
					}
					status.Domain = cleanDisp
				}
				status.TotalTrials = m.RegisteredUsers
				status.TotalOrders = m.ActiveOrders
				if m.Running {
					status.Running = true
				}
			}
		}
	}

	if !status.Running {
		if out, err := s.execPrivilegedBash(context.Background(), "pgrep -f 'poll.php'"); err == nil && len(strings.TrimSpace(string(out))) > 0 {
			status.Running = true
		}
	}

	if status.BotToken != "" && status.BotUsername == "" {
		s.fetchBotInfo(status.BotToken, status)
	}

	if status.Installed {
		_, _ = s.execPrivilegedBash(context.Background(), "if ! systemctl is-active --quiet novabot-web.service 2>/dev/null; then systemctl enable --now novabot-web.service 2>/dev/null; fi")
	}

	return status, nil
}

func (s *TelegramShopService) fetchBotInfo(token string, st *TelegramShopStatus) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: tr, Timeout: 5 * time.Second}
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

func (s *TelegramShopService) formatBotDomain(rawDomain string) string {
	d := strings.TrimSpace(rawDomain)
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimPrefix(d, "http://")
	d = strings.TrimRight(d, "/")

	_, port, _ := s.getPanelDetails()
	if d == "" {
		db, err := sql.Open("sqlite3", "/etc/x-ui/x-ui.db")
		if err == nil {
			var webDomain string
			_ = db.QueryRow("SELECT value FROM settings WHERE key = 'webDomain' LIMIT 1").Scan(&webDomain)
			db.Close()
			d = strings.TrimSpace(webDomain)
		}
	}
	if d == "" {
		return ""
	}
	if !strings.Contains(d, ":") && port != "" && port != "443" && port != "80" {
		d = fmt.Sprintf("%s:%s", d, port)
	}
	if !strings.Contains(d, "/botpanel") && !strings.Contains(d, "/panel") {
		d = fmt.Sprintf("%s/botpanel", d)
	}
	return d
}

func (s *TelegramShopService) installMirzaBotNativeStack(ctx context.Context, cfg *TelegramShopConfig) error {
	// 1. Ensure /var/www/mirzabot directory
	_ = os.MkdirAll(mirzaBotDir, 0o755)

	// 2. Extract embedded Mirza Bot source archive directly from memory (100% offline, zero network dependencies)
	tmpTar := fmt.Sprintf("/tmp/mirzabot_%d.tar.gz", time.Now().UnixNano())
	if err := os.WriteFile(tmpTar, defaultMirzaBotTarGz, 0o644); err != nil {
		return fmt.Errorf("failed to write embedded mirzabot archive: %v", err)
	}
	defer os.Remove(tmpTar)

	unpackScript := fmt.Sprintf("tar -xzf %s -C %s && chown -R www-data:www-data %s 2>/dev/null || true", tmpTar, mirzaBotDir, mirzaBotDir)
	if out, err := s.execPrivilegedBash(ctx, unpackScript); err != nil {
		return fmt.Errorf("failed to extract mirzabot archive: %s (%v)", string(out), err)
	}

	// 3. Ensure PHP, MariaDB, and required extensions are installed
	depScript := `
set -e
export DEBIAN_FRONTEND=noninteractive

need_install=0
if ! command -v php >/dev/null 2>&1 || (! command -v mysql >/dev/null 2>&1 && ! command -v mariadb >/dev/null 2>&1); then
    need_install=1
fi
if command -v php >/dev/null 2>&1; then
    if ! php -m 2>/dev/null | grep -qi curl || ! php -m 2>/dev/null | grep -qi pdo_mysql; then
        need_install=1
    fi
fi

if [ "$need_install" -eq 1 ]; then
    if command -v apt-get >/dev/null 2>&1; then
        apt-get update -qq || true
        apt-get install -y -qq php-cli php-fpm php-mysql php-curl php-mbstring php-zip php-gd mariadb-server curl tar || true
    elif command -v dnf >/dev/null 2>&1; then
        dnf install -y -q php-cli php-fpm php-mysqlnd php-curl php-mbstring php-zip php-gd mariadb-server curl tar || true
    elif command -v yum >/dev/null 2>&1; then
        yum install -y -q php-cli php-fpm php-mysqlnd php-curl php-mbstring php-zip php-gd mariadb-server curl tar || true
    fi
fi

systemctl enable --now mariadb 2>/dev/null || systemctl enable --now mysql 2>/dev/null || true

for i in $(seq 1 15); do
    if mysqladmin ping >/dev/null 2>&1 || mariadb-admin ping >/dev/null 2>&1; then
        break
    fi
    sleep 1
done
`
	if out, err := s.execPrivilegedBash(ctx, depScript); err != nil {
		return fmt.Errorf("failed to setup PHP/MariaDB dependencies: %s (%v)", string(out), err)
	}

	// 4. Setup MySQL Database and dedicated user
	dbScript := `
SQL_BIN="mysql"
if ! command -v mysql >/dev/null 2>&1 && command -v mariadb >/dev/null 2>&1; then
    SQL_BIN="mariadb"
fi

$SQL_BIN -e "CREATE DATABASE IF NOT EXISTS mirzabot CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" 2>/dev/null || true
$SQL_BIN -e "CREATE USER IF NOT EXISTS 'mirzauser'@'localhost' IDENTIFIED BY 'MirzaDbSecure_Nova!';" 2>/dev/null || true
$SQL_BIN -e "CREATE USER IF NOT EXISTS 'mirzauser'@'127.0.0.1' IDENTIFIED BY 'MirzaDbSecure_Nova!';" 2>/dev/null || true
$SQL_BIN -e "CREATE USER IF NOT EXISTS 'mirzauser'@'%' IDENTIFIED BY 'MirzaDbSecure_Nova!';" 2>/dev/null || true
$SQL_BIN -e "ALTER USER 'mirzauser'@'localhost' IDENTIFIED BY 'MirzaDbSecure_Nova!';" 2>/dev/null || true
$SQL_BIN -e "ALTER USER 'mirzauser'@'127.0.0.1' IDENTIFIED BY 'MirzaDbSecure_Nova!';" 2>/dev/null || true
$SQL_BIN -e "GRANT ALL PRIVILEGES ON mirzabot.* TO 'mirzauser'@'localhost';" 2>/dev/null || true
$SQL_BIN -e "GRANT ALL PRIVILEGES ON mirzabot.* TO 'mirzauser'@'127.0.0.1';" 2>/dev/null || true
$SQL_BIN -e "GRANT ALL PRIVILEGES ON mirzabot.* TO 'mirzauser'@'%';" 2>/dev/null || true
$SQL_BIN -e "FLUSH PRIVILEGES;" 2>/dev/null || true

has_admin=$($SQL_BIN -u mirzauser -p'MirzaDbSecure_Nova!' -e "SELECT 1 FROM mirzabot.admin LIMIT 1;" 2>/dev/null || true)
if [ -z "$has_admin" ] && [ -f /var/www/mirzabot/mirzabot_init.sql ]; then
    $SQL_BIN -u mirzauser -p'MirzaDbSecure_Nova!' mirzabot < /var/www/mirzabot/mirzabot_init.sql 2>/dev/null || \
    $SQL_BIN mirzabot < /var/www/mirzabot/mirzabot_init.sql 2>/dev/null || true
fi
`
	_, _ = s.execPrivilegedBash(ctx, dbScript)

	// 5. Write base config.php
	configPHP := fmt.Sprintf(`<?php
// Mirza Bot (NOVA-X-PANEL Edition) Configuration
$request_exec_timeout = null;
$dbhost = getenv('DB_HOST') ?: '127.0.0.1';
$dbname = getenv('DB_NAME') ?: 'mirzabot';
$usernamedb = getenv('DB_USER') ?: 'mirzauser';
$passworddb = getenv('DB_PASS') ?: 'MirzaDbSecure_Nova!';

$options = [
    PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
    PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
    PDO::ATTR_EMULATE_PREPARES => false,
    PDO::MYSQL_ATTR_INIT_COMMAND => "SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci",
];

try {
    $dsn = "mysql:host=$dbhost;dbname=$dbname;charset=utf8mb4";
    $pdo = new PDO($dsn, $usernamedb, $passworddb, $options);
} catch (\PDOException $e1) {
    try {
        $dsn = "mysql:host=localhost;dbname=$dbname;charset=utf8mb4";
        $pdo = new PDO($dsn, $usernamedb, $passworddb, $options);
    } catch (\PDOException $e2) {
        error_log("Database connection failed: " . $e2->getMessage());
        $pdo = null;
    }
}

$APIKEY = '%s';
$adminnumber = '%s';
$domainhosts = '%s';
$usernamebot = '';
?>
`, cfg.BotToken, cfg.AdminChatID, s.formatBotDomain(cfg.Domain))
	_ = os.WriteFile(mirzaConfigPath, []byte(configPHP), 0o644)

	// 6. Initialize database tables via table.php
	initTablesScript := `
if [ -f /var/www/mirzabot/table.php ]; then
    php /var/www/mirzabot/table.php >/dev/null 2>&1 || true
fi
`
	_, _ = s.execPrivilegedBash(ctx, initTablesScript)

	phpBin, err := exec.LookPath("php")
	if err != nil || phpBin == "" {
		phpBin = "/usr/bin/php"
	}

	// 8. Setup systemd unit for background poller (mirzabot-poller.service)
	pollerUnit := fmt.Sprintf(`[Unit]
Description=NOVA Bot Polling Daemon for NOVA-X-PANEL
After=network.target mariadb.service mysql.service

[Service]
Type=simple
User=root
WorkingDirectory=/var/www/mirzabot
ExecStart=%s /var/www/mirzabot/poll.php
Restart=always
RestartSec=3
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, phpBin)
	tmpPoller := fmt.Sprintf("/tmp/poller_%d.service", time.Now().UnixNano())
	if err := os.WriteFile(tmpPoller, []byte(pollerUnit), 0o644); err == nil {
		defer os.Remove(tmpPoller)
		startScript := fmt.Sprintf(`
cp -f %s /etc/systemd/system/mirzabot-poller.service
chmod 644 /etc/systemd/system/mirzabot-poller.service
systemctl daemon-reload
systemctl enable --now mirzabot-poller.service 2>/dev/null || true
systemctl restart mirzabot-poller.service 2>/dev/null || true
`, tmpPoller)
		if out, err := s.execPrivilegedBash(ctx, startScript); err != nil {
			return fmt.Errorf("failed to start mirzabot poller: %s (%v)", string(out), err)
		}
	}

	// 9. Setup systemd unit for local web service (novabot-web.service) on port 9099
	webUnit := fmt.Sprintf(`[Unit]
Description=NOVA Bot Web Service (Local)
After=network.target mariadb.service mysql.service

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/var/www/mirzabot
Environment="PHP_CLI_SERVER_WORKERS=8"
ExecStart=%s -d memory_limit=256M -d max_execution_time=60 -S 127.0.0.1:9099 -t /var/www/mirzabot
Restart=always
RestartSec=2
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, phpBin)
	tmpWeb := fmt.Sprintf("/tmp/novabot_web_%d.service", time.Now().UnixNano())
	if err := os.WriteFile(tmpWeb, []byte(webUnit), 0o644); err == nil {
		defer os.Remove(tmpWeb)
		startWebScript := fmt.Sprintf(`
cp -f %s /etc/systemd/system/novabot-web.service
chmod 644 /etc/systemd/system/novabot-web.service
systemctl daemon-reload
systemctl enable --now novabot-web.service 2>/dev/null || true
systemctl restart novabot-web.service 2>/dev/null || true
`, tmpWeb)
		_, _ = s.execPrivilegedBash(ctx, startWebScript)
	}

	return nil
}

// Install sets up the Mirza Bot configuration, database, and systemd poller.
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

	s.purgeLegacyBot()
	s.ensureMirzaHelper()

	// Save persistent backup config
	_ = os.MkdirAll("/etc/dark-shop", 0o755)
	if data, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		_ = os.WriteFile(tgShopConfigPath, data, 0o644)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// Deploy embedded Mirza Bot stack only if not already installed
	needsFullInstall := false
	if _, err := os.Stat(mirzaConfigPath); os.IsNotExist(err) {
		needsFullInstall = true
	} else if _, err := os.Stat(mirzaBotDir + "/index.php"); os.IsNotExist(err) {
		needsFullInstall = true
	}

	if needsFullInstall {
		if err := s.installMirzaBotNativeStack(ctx, cfg); err != nil {
			return fmt.Errorf("mirzabot install error: %v", err)
		}
	}

	// Always guarantee systemd units exist
	s.EnsureWebUnit()
	s.EnsurePollerUnit()

	st := &TelegramShopStatus{}
	s.fetchBotInfo(cfg.BotToken, st)
	if st.BotUsername == "" {
		st.BotUsername = "ShopBot"
	}

	// Sync config via mirzabot_helper CLI
	if _, err := os.Stat(mirzaHelperPath); err == nil {
		cmdStr := fmt.Sprintf("%s install %s %s %s %s", mirzaHelperPath, cfg.BotToken, cfg.AdminChatID, st.BotUsername, s.formatBotDomain(cfg.Domain))
		out, err := s.execPrivilegedBash(ctx, cmdStr)
		if err != nil {
			return fmt.Errorf("mirza sync error: %s (%v)", string(out), err)
		}
	}

	// Ensure systemd poller and local web service are enabled and actively running
	_, _ = s.execPrivilegedBash(ctx, "systemctl daemon-reload && systemctl enable --now mirzabot-poller.service && systemctl restart mirzabot-poller.service && systemctl enable --now novabot-web.service 2>/dev/null && systemctl restart novabot-web.service 2>/dev/null")

	return nil
}

// Action performs management operations (start, stop, restart, uninstall).
func (s *TelegramShopService) Action(action string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.purgeLegacyBot()
	s.ensureMirzaHelper()
	s.EnsureWebUnit()

	// If Mirza Bot is not yet installed or is uninstalled:
	isUninstalled := false
	if _, err := os.Stat(mirzaConfigPath); os.IsNotExist(err) {
		isUninstalled = true
	} else if cfgData, err := os.ReadFile(mirzaConfigPath); err == nil {
		if strings.Contains(string(cfgData), "{API_KEY}") {
			isUninstalled = true
		}
	}

	if isUninstalled {
		if action == "start" || action == "restart" {
			data, err := os.ReadFile(tgShopConfigPath)
			if err == nil {
				var cfg TelegramShopConfig
				if json.Unmarshal(data, &cfg) == nil && cfg.BotToken != "" && cfg.AdminChatID != "" {
					ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
					defer cancel()
					if err := s.installMirzaBotNativeStack(ctx, &cfg); err != nil {
						return fmt.Errorf("auto-install error: %v", err)
					}
				} else {
					return fmt.Errorf("ربات نوا هنوز نصب نشده است. لطفاً ابتدا توکن و چت‌آیدی را وارد کرده و دکمه نصب را بزنید.")
				}
			} else {
				return fmt.Errorf("ربات نوا هنوز نصب نشده است. لطفاً ابتدا توکن و چت‌آیدی را وارد کرده و دکمه نصب را بزنید.")
			}
		} else {
			return nil
		}
	}

	// Mirza Bot is installed; perform action via helper CLI
	if _, err := os.Stat(mirzaHelperPath); err == nil {
		cmdStr := fmt.Sprintf("%s action %s", mirzaHelperPath, action)
		out, err := s.execPrivilegedBash(context.Background(), cmdStr)
		if err != nil {
			return fmt.Errorf("mirza action error: %s (%v)", string(out), err)
		}
	}

	// Ensure systemd unit matches action
	switch action {
	case "start":
		_, _ = s.execPrivilegedBash(context.Background(), "systemctl enable --now mirzabot-poller.service novabot-web.service && systemctl restart mirzabot-poller.service novabot-web.service")
	case "stop":
		_, _ = s.execPrivilegedBash(context.Background(), "systemctl stop mirzabot-poller.service novabot-web.service")
	case "restart":
		_, _ = s.execPrivilegedBash(context.Background(), "systemctl enable --now mirzabot-poller.service novabot-web.service && systemctl restart mirzabot-poller.service novabot-web.service")
	case "uninstall":
		_ = os.Remove(tgShopConfigPath)
		_, _ = s.execPrivilegedBash(context.Background(), "systemctl stop mirzabot-poller.service 2>/dev/null; systemctl disable mirzabot-poller.service 2>/dev/null; systemctl stop novabot-web.service 2>/dev/null; systemctl disable novabot-web.service 2>/dev/null; systemctl daemon-reload")
	}

	return nil
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
    apt-get install -y -qq php-cli php-fpm php-mysql php-curl php-mbstring php-zip php-gd mariadb-server curl tar unzip || true
elif command -v dnf >/dev/null 2>&1; then
    dnf install -y -q php-cli php-fpm php-mysqlnd php-curl php-mbstring php-zip php-gd mariadb-server curl tar unzip || true
elif command -v yum >/dev/null 2>&1; then
    yum install -y -q php-cli php-fpm php-mysqlnd php-curl php-mbstring php-zip php-gd mariadb-server curl tar unzip || true
fi
systemctl enable --now mariadb 2>/dev/null || systemctl enable --now mysql 2>/dev/null || true
`
	out, err := s.execPrivilegedBash(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to install dependencies: %s (%v)", string(out), err)
	}
	return nil
}
