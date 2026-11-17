#!/usr/bin/env python3
"""
NOVA-X-PANEL OpenVPN Real-time Traffic Sync Daemon
Polls OpenVPN management socket every 30 seconds,
parses `status 2` output, and updates `open_vpn_users.up`/`down`
in SQLite. Works with single TCP instance on port 7505.
"""
import sqlite3
import socket
import time
import re
import os
import signal
import sys

DB_PATH = os.getenv("X_UI_DB", "/etc/x-ui/x-ui.db")
MGMT_HOST = "127.0.0.1"
MGMT_PORT = 7505
POLL_INTERVAL = 30
SOCKET_TIMEOUT = 5

running = True

def signal_handler(signum, frame):
    global running
    print(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] Traffic Sync Daemon stopped")
    running = False

signal.signal(signal.SIGTERM, signal_handler)
signal.signal(signal.SIGINT, signal_handler)

def query_mgmt(cmd: str) -> str:
    """Send a command to OpenVPN management interface and return full response."""
    try:
        with socket.create_connection((MGMT_HOST, MGMT_PORT), timeout=SOCKET_TIMEOUT) as sock:
            sock.settimeout(SOCKET_TIMEOUT)
            sock.sendall((cmd + "\n").encode())
            data = b""
            while True:
                chunk = sock.recv(4096)
                if not chunk:
                    break
                data += chunk
                if b"END" in chunk:
                    break
            return data.decode(errors="ignore")
    except Exception as e:
        raise RuntimeError(f"MGMT query error: {e}")

def parse_status_v2(resp: str):
    """Parse `status 2` response into list of dicts per connected client."""
    clients = []
    in_client_list = False
    for line in resp.splitlines():
        line = line.strip()
        if not line:
            continue
        if line.startswith("CLIENT_LIST"):
            in_client_list = True
            continue
        if line.startswith("ROUTING_TABLE"):
            break
        if in_client_list:
            parts = line.split(",")
            if len(parts) >= 6:
                name = parts[1]
                # parts[2]=real_address, [3]=virtual_address, [4]=virtual_ipv6, [5]=bytes_received, [6]=bytes_sent
                try:
                    bytes_recv = int(parts[5]) if len(parts) > 5 else 0
                    bytes_sent = int(parts[6]) if len(parts) > 6 else 0
                except ValueError:
                    continue
                if name and name != "UNDEF":
                    clients.append({
                        "name": name,
                        "bytes_recv": bytes_recv,
                        "bytes_sent": bytes_sent,
                    })
    return clients

def sync_once():
    try:
        resp = query_mgmt("status 2")
    except Exception as e:
        print(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] {e}")
        return

    if not resp or "END" not in resp:
        print(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] Invalid status response")
        return

    clients = parse_status_v2(resp)
    if not clients:
        print(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] No clients to sync")
        return

    try:
        conn = sqlite3.connect(DB_PATH, timeout=10)
        conn.execute("PRAGMA journal_mode=WAL;")
        cur = conn.cursor()
        for c in clients:
            cur.execute("""
                UPDATE open_vpn_users
                SET up = up + ?,
                    down = down + ?
                WHERE name = ?
            """, (c["bytes_sent"], c["bytes_recv"], c["name"]))
            if cur.rowcount == 0:
                print(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] User not found in DB: {c['name']}")
        conn.commit()
        conn.close()
        print(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] Synced {len(clients)} client(s)")
    except Exception as e:
        print(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] DB error: {e}")

def main():
    print(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] Starting OpenVPN Traffic Sync Daemon (TCP port {MGMT_PORT})")
    while running:
        sync_once()
        time.sleep(POLL_INTERVAL)

if __name__ == "__main__":
    main()
