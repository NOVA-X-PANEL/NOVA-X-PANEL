#!/usr/bin/env php
<?php
// Mirza Bot Helper CLI for NOVA-X-PANEL Integration
declare(strict_types=1);

$botDir = '/var/www/mirzabot';
$configPath = $botDir . '/config.php';
$tableScript = $botDir . '/table.php';
$pollScript = $botDir . '/poll.php';
$pollerService = 'mirzabot-poller.service';

function getDb(): ?PDO {
    global $configPath;
    if (!is_file($configPath)) return null;
    try {
        require_once $configPath;
        global $pdo;
        return $pdo instanceof PDO ? $pdo : null;
    } catch (\Throwable $e) {
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
    file_put_contents($configPath, $content);

    // Update DB
    $db = getDb();
    if ($db) {
        try {
            $stmt = $db->prepare("UPDATE admin SET id_admin = ? WHERE rule = 'administrator' LIMIT 1");
            $stmt->execute([$adminId]);
            $db->exec("UPDATE setting SET Bot_Status = 'botstatuson'");
        } catch (\Throwable $e) {}
    }

    // Set Telegram webhook or restart poller
    if (is_file($tableScript)) {
        @shell_exec('php ' . escapeshellarg($tableScript) . ' 2>&1');
    }
    @shell_exec('systemctl enable --now mirzabot-poller.service 2>&1');
    @shell_exec('systemctl restart mirzabot-poller.service 2>&1');

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
        @shell_exec('systemctl enable --now mirzabot-poller.service 2>&1');
        @shell_exec('systemctl restart mirzabot-poller.service 2>&1');
        echo json_encode(['success' => true]);
        exit(0);
    }

    if ($action === 'stop') {
        if ($db) $db->exec("UPDATE setting SET Bot_Status = 'botstatusoff'");
        @shell_exec('systemctl stop mirzabot-poller.service 2>&1');
        echo json_encode(['success' => true]);
        exit(0);
    }

    if ($action === 'restart') {
        if (is_file($tableScript)) @shell_exec('php ' . escapeshellarg($tableScript) . ' 2>&1');
        @shell_exec('systemctl restart mirzabot-poller.service 2>&1');
        echo json_encode(['success' => true]);
        exit(0);
    }

    if ($action === 'uninstall') {
        @shell_exec('systemctl stop mirzabot-poller.service 2>&1');
        @shell_exec('systemctl disable mirzabot-poller.service 2>&1');
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
