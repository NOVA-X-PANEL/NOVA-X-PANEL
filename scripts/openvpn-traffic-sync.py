#!/usr/bin/env python3
"""
NOVA-X-PANEL OpenVPN Real-time Traffic Sync Daemon (TCP + UDP)
Polls OpenVPN management sockets (ports 7505 and 7506) periodically and syncs traffic stats to database.
"""
import sys
import os
import time
import socket
import signal

LOG_PATH = "/var/log/openvpn_traffic_sync.log"
MGMT_HOST = "127.0.0.1"
MGMT_PORTS = [7505, 7506]
SYNC_INTERVAL = 30  # seconds

def log_debug(msg):
    try:
        with open(LOG_PATH, "a") as f:
            f.write(f"{time.strftime('%Y-%m-%d %H:%M:%S')} {msg}\n")
    except Exception:
        pass

def get_db_connection():
    db_type = os.environ.get("XUI_DB_TYPE", "")
    dsn = os.environ.get("XUI_DB_DSN", "")
    if os.path.exists("/etc/default/x-ui"):
        try:
            with open("/etc/default/x-ui") as f:
                for line in f:
                    line = line.strip()
                    if line.startswith("XUI_DB_TYPE="):
                        db_type = line.split("=", 1)[1].strip().strip('"\'')
                    elif line.startswith("XUI_DB_DSN="):
                        dsn = line.split("=", 1)[1].strip().strip('"\'')
        except Exception:
            pass

    if db_type == "postgres" and dsn:
        try:
            import psycopg2
            conn = psycopg2.connect(dsn)
            return "postgres", conn
        except Exception as e:
            log_debug(f"psycopg2 connect failed: {e}")

    db_path = "/etc/x-ui/x-ui.db"
    if os.path.exists(db_path):
        try:
            import sqlite3
            conn = sqlite3.connect(db_path, timeout=5.0)
            return "sqlite", conn
        except Exception as e:
            log_debug(f"sqlite3 connect failed: {e}")

    return None, None

def query_management_status(port):
    """Query OpenVPN management socket for current client list with traffic."""
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.settimeout(10)
        s.connect((MGMT_HOST, port))
        
        banner = b""
        start = time.time()
        while time.time() - start < 3:
            try:
                chunk = s.recv(512)
                if not chunk:
                    break
                banner += chunk
                if b">INFO:" in banner:
                    break
            except socket.timeout:
                break
        
        s.sendall(b"status 2\n")
        data = b""
        start = time.time()
        while time.time() - start < 10:
            try:
                chunk = s.recv(4096)
                if not chunk:
                    break
                data += chunk
                if b"END\r\n" in data or b"END\n" in data:
                    break
            except socket.timeout:
                break
        
        try:
            s.sendall(b"quit\n")
        except Exception:
            pass
        s.close()
        return data.decode("utf-8", errors="ignore")
    except Exception as e:
        log_debug(f"MGMT query error on port {port}: {e}")
        return ""

def parse_status(status_text):
    """Parse status output to extract client traffic."""
    clients = {}
    lines = status_text.splitlines()
    for line in lines:
        if line.startswith("CLIENT_LIST,"):
            parts = line.split(",")
            if len(parts) >= 11:
                cn = parts[1].strip()
                uname = parts[9].strip() if len(parts) >= 10 else ""
                bytes_recv = int(parts[5]) if len(parts) >= 6 and parts[5].isdigit() else 0
                bytes_sent = int(parts[6]) if len(parts) >= 7 and parts[6].isdigit() else 0
                real_addr = parts[2].strip() if len(parts) >= 3 else ""
                port = real_addr.split(":")[-1] if ":" in real_addr else ""
                key = uname.lower() if uname else cn.lower()
                if key and key != "server" and key != "undef":
                    if key not in clients:
                        clients[key] = {"rx": 0, "tx": 0, "ports": set()}
                    clients[key]["rx"] += bytes_recv
                    clients[key]["tx"] += bytes_sent
                    if port:
                        clients[key]["ports"].add(port)
    return clients

def sync_traffic(db, backend, clients, now_ms):
    """Sync traffic deltas to database."""
    if not clients:
        return

    for name, data in clients.items():
        try:
            row = db.execute(
                "SELECT up, down FROM open_vpn_users WHERE LOWER(name) = LOWER(?);",
                "SELECT up, down FROM open_vpn_users WHERE LOWER(name) = LOWER(?);",
                (name,)
            ).fetchone()

            if not row:
                continue

            db_up, db_down = row
            current_total_rx = int(db_up or 0)
            current_total_tx = int(db_down or 0)

            session_row = db.execute(
                "SELECT last_rx, last_tx FROM open_vpn_sessions WHERE LOWER(name) = LOWER(?);",
                "SELECT last_rx, last_tx FROM open_vpn_sessions WHERE LOWER(name) = LOWER(?);",
                (name,)
            ).fetchone()

            last_rx = int(session_row[0]) if session_row and session_row[0] else 0
            last_tx = int(session_row[1]) if session_row and session_row[1] else 0

            live_rx = data["rx"]
            live_tx = data["tx"]
            delta_rx = max(0, live_rx - last_rx)
            delta_tx = max(0, live_tx - last_tx)

            if delta_rx > 0 or delta_tx > 0:
                new_up = current_total_rx + delta_rx
                new_down = current_total_tx + delta_tx

                db.execute(
                    "UPDATE open_vpn_users SET up = ?, down = ?, updated_at = ? WHERE LOWER(name) = LOWER(?);",
                    "UPDATE open_vpn_users SET up = ?, down = ?, updated_at = ? WHERE LOWER(name) = LOWER(?);",
                    (new_up, new_down, now_ms, name)
                )

                db.execute(
                    "UPDATE open_vpn_sessions SET last_rx = ?, last_tx = ?, updated_at = ? WHERE LOWER(name) = LOWER(?);",
                    "UPDATE open_vpn_sessions SET last_rx = ?, last_tx = ?, updated_at = ? WHERE LOWER(name) = LOWER(?);",
                    (live_rx, live_tx, now_ms, name)
                )

                log_debug(f"SYNC: {name} delta_rx={delta_rx} delta_tx={delta_tx} total_up={new_up} total_down={new_down}")

        except Exception as e:
            log_debug(f"SYNC ERROR for {name}: {e}")

def main():
    log_debug("Starting OpenVPN Traffic Sync Daemon (TCP + UDP)")
    
    db_backend, db_conn = get_db_connection()
    if not db_conn:
        log_debug("FATAL: No database connection")
        sys.exit(1)

    class DB:
        def __init__(self, conn, backend):
            self.conn = conn
            self.backend = backend
        def execute(self, sql_lite, sql_pg, params=()):
            cur = self.conn.cursor()
            try:
                if self.backend == "postgres":
                    cur.execute(sql_pg, params)
                else:
                    cur.execute(sql_lite, params)
                self.conn.commit()
                return cur
            finally:
                cur.close()
        def close(self):
            try:
                self.conn.close()
            except Exception:
                pass

    db = DB(db_conn, db_backend)

    try:
        if db_backend == "sqlite":
            db.execute("", "ALTER TABLE open_vpn_sessions ADD COLUMN last_rx INTEGER DEFAULT 0;")
            db.execute("", "ALTER TABLE open_vpn_sessions ADD COLUMN last_tx INTEGER DEFAULT 0;")
    except Exception:
        pass

    running = True
    def signal_handler(sig, frame):
        nonlocal running
        running = False
    signal.signal(signal.SIGTERM, signal_handler)
    signal.signal(signal.SIGINT, signal_handler)

    while running:
        now_ms = int(time.time() * 1000)
        combined_clients = {}
        for port in MGMT_PORTS:
            status = query_management_status(port)
            if status:
                clients = parse_status(status)
                for k, v in clients.items():
                    if k not in combined_clients:
                        combined_clients[k] = {"rx": 0, "tx": 0, "ports": set()}
                    combined_clients[k]["rx"] += v["rx"]
                    combined_clients[k]["tx"] += v["tx"]
        
        if combined_clients:
            sync_traffic(db, db_backend, combined_clients, now_ms)
        
        for _ in range(SYNC_INTERVAL):
            if not running:
                break
            time.sleep(1)

    db.close()
    log_debug("Traffic Sync Daemon stopped")

if __name__ == "__main__":
    main()
