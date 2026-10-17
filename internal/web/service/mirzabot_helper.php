#!/usr/bin/env php
<?php
// Mirza Bot Helper CLI for NOVA-X-PANEL Integration
declare(strict_types=1);

$botDir = '/var/www/mirzabot';
$configPath = $botDir . '/config.php';
$tableScript = $botDir . '/table.php';
$pollScript = $botDir . '/poll.php';
$pollerService = 'mirzabot-poller.service';

function ensureNovabotWebService(): void {
    $unitPath = '/etc/systemd/system/novabot-web.service';
    if (!is_file($unitPath)) {
        $phpBin = '/usr/bin/php';
        if (!is_executable($phpBin)) {
            $found = trim((string)@shell_exec('command -v php 2>/dev/null'));
            if (!empty($found)) $phpBin = $found;
        }
        $unit = "[Unit]\nDescription=NOVA Bot Web Service (Local)\nAfter=network.target mariadb.service mysql.service\n\n[Service]\nType=simple\nUser=www-data\nGroup=www-data\nWorkingDirectory=/var/www/mirzabot\nEnvironment=\"PHP_CLI_SERVER_WORKERS=8\"\nExecStart={$phpBin} -d memory_limit=256M -d max_execution_time=60 -S 127.0.0.1:9099 -t /var/www/mirzabot\nRestart=always\nRestartSec=2\nStandardOutput=journal\nStandardError=journal\n\n[Install]\nWantedBy=multi-user.target\n";
        @file_put_contents($unitPath, $unit);
        @chmod($unitPath, 0644);
        @shell_exec('systemctl daemon-reload 2>&1');
    }
}

function getDb(): ?PDO {
    global $configPath, $pdo;
    if (!is_file($configPath)) return null;
    try {
        if ($pdo instanceof PDO) {
            return $pdo;
        }
        global $dbhost, $dbname, $usernamedb, $passworddb;
        require_once $configPath;
        if ($pdo instanceof PDO) {
            return $pdo;
        }
        if (!empty($dbhost) && !empty($dbname) && !empty($usernamedb)) {
            $dsn = "mysql:host={$dbhost};dbname={$dbname};charset=utf8mb4";
            $pdo = new PDO($dsn, $usernamedb, $passworddb ?? '', [
                PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
                PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
            ]);
            return $pdo;
        }
        return null;
    } catch (\Throwable $e) {
        error_log("getDb error: " . $e->getMessage());
        return null;
    }
}

$cmd = $argv[1] ?? 'status';

if ($cmd === 'status') {
    if (!is_file($configPath)) {
        echo json_encode(['installed' => false, 'running' => false]);
        exit(0);
    }
    require_once $configPath;
    global $APIKEY, $adminnumber, $usernamebot, $domainhosts;

    $installed = !empty($APIKEY) && $APIKEY !== '{API_KEY}';
    $running = false;
    $users = 0;
    $orders = 0;

    if ($installed) {
        $pollerActive = (trim((string)@shell_exec('systemctl is-active mirzabot-poller.service 2>/dev/null')) === 'active');
        $running = $pollerActive;

        $db = getDb();
        if ($db) {
            try {
                $statusRow = $db->query("SELECT Bot_Status FROM setting LIMIT 1")->fetch(PDO::FETCH_ASSOC);
                if (($statusRow['Bot_Status'] ?? '') === 'botstatusoff') {
                    $running = false;
                }
                $users = (int) $db->query("SELECT COUNT(*) FROM user")->fetchColumn();
                $orders = (int) $db->query("SELECT COUNT(*) FROM invoice WHERE status = 'paid'")->fetchColumn();
            } catch (\Throwable $e) {}
        }
    }

    echo json_encode([
        'installed' => $installed,
        'running' => $running,
        'bot_token' => $installed ? (string)$APIKEY : '',
        'admin_chat_id' => $installed ? (string)$adminnumber : '',
        'bot_username' => $installed ? (string)$usernamebot : '',
        'domain' => (string)($domainhosts ?? ''),
        'registered_users' => $users,
        'active_orders' => $orders,
    ]);
    exit(0);
}

if ($cmd === 'install') {
    $token = trim($argv[2] ?? '');
    $adminId = trim($argv[3] ?? '');
    $username = trim($argv[4] ?? '');
    $domain = trim($argv[5] ?? '');

    if ($token === '' || $adminId === '') {
        echo json_encode(['success' => false, 'error' => 'token and admin_id are required']);
        exit(1);
    }

    if (!is_file($configPath)) {
        echo json_encode(['success' => false, 'error' => 'config.php not found']);
        exit(1);
    }

    // Update config.php content safely
    $content = file_get_contents($configPath);
    $content = preg_replace('/\\$APIKEY\\s*=\\s*[\'"][^\'"]*[\'"];/', "\$APIKEY = '$token';", $content);
    $content = preg_replace('/\\$adminnumber\\s*=\\s*[\'"][^\'"]*[\'"];/', "\$adminnumber = '$adminId';", $content);
    if ($username !== '') {
        $content = preg_replace('/\\$usernamebot\\s*=\\s*[\'"][^\'"]*[\'"];/', "\$usernamebot = '$username';", $content);
    }
    if ($domain !== '') {
        $cleanDomain = preg_replace('#^https?://#', '', rtrim($domain, '/'));
        $content = preg_replace('/\\$domainhosts\\s*=\\s*[\'"][^\'"]*[\'"];/', "\$domainhosts = '$cleanDomain';", $content);
    }
    file_put_contents($configPath, $content);

    // Update DB
    $db = getDb();
    if ($db) {
        try {
            // Ensure product table has all required columns and safe defaults
            $db->exec("ALTER TABLE product ADD COLUMN IF NOT EXISTS limitip int(11) DEFAULT 0");
            $db->exec("ALTER TABLE product ADD COLUMN IF NOT EXISTS status_product varchar(20) DEFAULT NULL");
            $db->exec("ALTER TABLE product MODIFY one_buy_status varchar(20) DEFAULT '0'");
            $db->exec("ALTER TABLE product MODIFY hide_panel text DEFAULT NULL");
            $db->exec("ALTER TABLE product MODIFY limitip int(11) DEFAULT 0");

            // Check if admin already exists with an existing password hash, otherwise default to admin1234
            $existingAdmin = null;
            try {
                $stmtExist = $db->query("SELECT password FROM admin WHERE username = 'admin' LIMIT 1");
                if ($stmtExist) $existingAdmin = $stmtExist->fetch(PDO::FETCH_ASSOC);
            } catch (\Throwable $e) {}

            $hashedPass = (!empty($existingAdmin['password']) && str_starts_with($existingAdmin['password'], '$2'))
                ? $existingAdmin['password']
                : password_hash('admin1234', PASSWORD_BCRYPT, ['cost' => 12]);

            // Ensure the configured admin exists with administrator role without deleting others
            $stmt = $db->prepare("INSERT INTO admin (id_admin, username, password, rule) VALUES (?, 'admin', ?, 'administrator') ON DUPLICATE KEY UPDATE rule = 'administrator'");
            $stmt->execute([$adminId, $hashedPass]);

            // Ensure admin exists in user table with Active status
            $stmtUser = $db->prepare("INSERT INTO user (id, step, limit_usertest, roll_Status, Processing_value, Processing_value_one, Processing_value_tow, Processing_value_four, User_Status, number, Balance, pagenumber, username, agent, message_count, last_message_time, affiliates, affiliatescount, cardpayment, number_username, namecustom, register, verify, codeInvitation, pricediscount, maxbuyagent, joinchannel, score, status_cron) VALUES (?, 'none', '0', 0, '', '', '', '', 'Active', 'none', '0', '1', 'admin', 'f', '0', '0', '0', '0', '0', '100', 'none', UNIX_TIMESTAMP(), 1, SUBSTRING(MD5(RAND()), 1, 12), '0', '0', '0', '0', '1') ON DUPLICATE KEY UPDATE User_Status = 'Active'");
            $stmtUser->execute([$adminId]);

            $db->exec("UPDATE setting SET Bot_Status = 'botstatuson', roll_Status = 'rolleoff'");
            $db->prepare("UPDATE setting SET id_support = ?")->execute([$adminId]);
            $db->exec("UPDATE setting SET Channel_Report = '' WHERE Channel_Report LIKE '-%'");
        } catch (\Throwable $e) {
            error_log("mirzabot_helper install DB error: " . $e->getMessage());
        }
    }

    // Set Telegram webhook or restart poller
    if (is_file($tableScript)) {
        @shell_exec('php ' . escapeshellarg($tableScript) . ' 2>&1');
    }
    ensureNovabotWebService();
    @shell_exec('systemctl daemon-reload 2>&1');
    @shell_exec('systemctl enable --now mirzabot-poller.service 2>&1');
    @shell_exec('systemctl restart mirzabot-poller.service 2>&1');
    @shell_exec('systemctl enable --now novabot-web.service 2>&1');
    @shell_exec('systemctl restart novabot-web.service 2>&1');

    echo json_encode(['success' => true]);
    exit(0);
}

if ($cmd === 'action') {
    $action = trim($argv[2] ?? '');
    $db = getDb();

    if ($action === 'start') {
        if ($db) {
            try {
                $db->exec("UPDATE setting SET Bot_Status = 'botstatuson'");
            } catch (\Throwable $e) {}
        }
        ensureNovabotWebService();
        @shell_exec('systemctl daemon-reload 2>&1');
        @shell_exec('systemctl enable --now mirzabot-poller.service 2>&1');
        @shell_exec('systemctl restart mirzabot-poller.service 2>&1');
        @shell_exec('systemctl enable --now novabot-web.service 2>&1');
        @shell_exec('systemctl restart novabot-web.service 2>&1');
        echo json_encode(['success' => true]);
        exit(0);
    }

    if ($action === 'stop') {
        if ($db) $db->exec("UPDATE setting SET Bot_Status = 'botstatusoff'");
        @shell_exec('systemctl stop mirzabot-poller.service 2>&1');
        @shell_exec('systemctl stop novabot-web.service 2>&1');
        echo json_encode(['success' => true]);
        exit(0);
    }

    if ($action === 'restart') {
        if (is_file($tableScript)) @shell_exec('php ' . escapeshellarg($tableScript) . ' 2>&1');
        ensureNovabotWebService();
        @shell_exec('systemctl daemon-reload 2>&1');
        @shell_exec('systemctl enable --now mirzabot-poller.service 2>&1');
        @shell_exec('systemctl restart mirzabot-poller.service 2>&1');
        @shell_exec('systemctl enable --now novabot-web.service 2>&1');
        @shell_exec('systemctl restart novabot-web.service 2>&1');
        echo json_encode(['success' => true]);
        exit(0);
    }

    if ($action === 'uninstall') {
        @shell_exec('systemctl stop mirzabot-poller.service 2>&1');
        @shell_exec('systemctl disable mirzabot-poller.service 2>&1');
        @shell_exec('systemctl stop novabot-web.service 2>&1');
        @shell_exec('systemctl disable novabot-web.service 2>&1');
        require_once $configPath;
        global $APIKEY;
        if (!empty($APIKEY) && $APIKEY !== '{API_KEY}') {
            @file_get_contents("https://api.telegram.org/bot{$APIKEY}/deleteWebhook?drop_pending_updates=true");
        }
        $content = file_get_contents($configPath);
        $content = preg_replace('/\\$APIKEY\\s*=\\s*[\'"][^\'"]*[\'"];/', "\$APIKEY = '{API_KEY}';", $content);
        $content = preg_replace('/\\$adminnumber\\s*=\\s*[\'"][^\'"]*[\'"];/', "\$adminnumber = '{admin_number}';", $content);
        file_put_contents($configPath, $content);
        if ($db) $db->exec("UPDATE setting SET Bot_Status = 'botstatusoff'");
        echo json_encode(['success' => true]);
        exit(0);
    }

    echo json_encode(['success' => false, 'error' => 'unknown action']);
    exit(1);
}
