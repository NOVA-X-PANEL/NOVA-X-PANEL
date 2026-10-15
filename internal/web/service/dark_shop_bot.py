#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
DARK SHOP BOT (Mirza Bot 100% Native Edition)
Automated VPN Sales, Invoicing, Receipt Verification, and Client Management System for NOVA-X-PANEL
"""

import os
import sys
import time
import datetime
import json
import uuid
import random
import string
import logging
import sqlite3
import threading
import io
import urllib.request
import urllib.parse
import urllib.error

try:
    import requests
except ImportError:
    requests = None

try:
    import qrcode
except ImportError:
    qrcode = None

CONFIG_PATH = "/etc/dark-shop/config.json"
PLANS_PATH = "/etc/dark-shop/plans.json"
SHOP_DB_PATH = "/etc/dark-shop/shop.db"
XUI_DB_PATH = "/etc/x-ui/x-ui.db"

logging.basicConfig(
    format="%(asctime)s [%(levelname)s] %(message)s",
    level=logging.INFO
)
logger = logging.getLogger("dark_shop_mirza")

# -------------------------------------------------------------
# Jalali (Shamsi) Date Converter
# -------------------------------------------------------------
def gregorian_to_jalali(gy, gm, gd):
    g_d_m = [0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334]
    if gy > 1600:
        jy = 979
        gy -= 1600
    else:
        jy = 0
        gy -= 621
    gy2 = gy if gm > 2 else gy - 1
    days = 365 * gy + (gy2 + 3) // 4 - (gy2 + 99) // 100 + (gy2 + 399) // 400 - 80 + gd + g_d_m[gm - 1]
    jy += 33 * (days // 12053)
    days %= 12053
    jy += 4 * (days // 1461)
    days %= 1461
    if days > 365:
        jy += (days - 1) // 365
        days = (days - 1) % 365
    if days < 186:
        jm = 1 + days // 31
        jd = 1 + days % 31
    else:
        jm = 7 + (days - 186) // 30
        jd = 1 + (days - 186) % 30
    return jy, jm, jd

def format_timestamp_jalali(ts):
    if not ts or ts <= 0:
        ts = int(time.time())
    dt = datetime.datetime.fromtimestamp(ts)
    jy, jm, jd = gregorian_to_jalali(dt.year, dt.month, dt.day)
    date_str = f"{jy}/{jm:02d}/{jd:02d}"
    time_str = dt.strftime("%H:%M:%S")
    return date_str, time_str

# -------------------------------------------------------------
# Database Setup & Migrations
# -------------------------------------------------------------
def get_db():
    conn = sqlite3.connect(SHOP_DB_PATH, timeout=30.0)
    conn.row_factory = sqlite3.Row
    return conn

def init_db():
    os.makedirs(os.path.dirname(SHOP_DB_PATH), exist_ok=True)
    conn = get_db()
    c = conn.cursor()
    c.execute("""
    CREATE TABLE IF NOT EXISTS users (
        user_id INTEGER PRIMARY KEY,
        username TEXT,
        first_name TEXT,
        balance INTEGER DEFAULT 0,
        claimed_membership_gift INTEGER DEFAULT 0,
        created_at INTEGER,
        last_seen INTEGER,
        is_banned INTEGER DEFAULT 0
    )
    """)
    c.execute("""
    CREATE TABLE IF NOT EXISTS referrals (
        user_id INTEGER PRIMARY KEY,
        referrer_id INTEGER,
        created_at INTEGER
    )
    """)
    c.execute("""
    CREATE TABLE IF NOT EXISTS referral_rewards (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER,
        referrer_id INTEGER,
        amount INTEGER,
        order_id INTEGER,
        created_at INTEGER
    )
    """)
    c.execute("""
    CREATE TABLE IF NOT EXISTS discount_codes (
        code TEXT PRIMARY KEY,
        percent INTEGER,
        max_uses INTEGER,
        used_count INTEGER DEFAULT 0,
        expire_at INTEGER
    )
    """)
    c.execute("""
    CREATE TABLE IF NOT EXISTS trials (
        user_id INTEGER PRIMARY KEY,
        username TEXT,
        sub_id TEXT,
        created_at INTEGER
    )
    """)
    c.execute("""
    CREATE TABLE IF NOT EXISTS orders (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER,
        username TEXT,
        order_type TEXT DEFAULT 'PLAN',
        plan_id INTEGER,
        plan_title TEXT,
        price_tomans INTEGER,
        discount_code TEXT,
        final_price INTEGER,
        target_email TEXT,
        receipt_file_id TEXT,
        status TEXT,
        sub_id TEXT,
        created_at INTEGER
    )
    """)
    c.execute("""
    CREATE TABLE IF NOT EXISTS user_state (
        user_id INTEGER PRIMARY KEY,
        state TEXT,
        data TEXT,
        updated_at INTEGER
    )
    """)
    c.execute("""
    CREATE TABLE IF NOT EXISTS settings (
        key TEXT PRIMARY KEY,
        value TEXT
    )
    """)
    c.execute("""
    CREATE TABLE IF NOT EXISTS notifications_sent (
        client_email TEXT PRIMARY KEY,
        notified_type TEXT,
        sent_at INTEGER
    )
    """)
    c.execute("""
    CREATE TABLE IF NOT EXISTS wheel_luck (
        user_id INTEGER PRIMARY KEY,
        last_spun_at INTEGER
    )
    """)
    # Try adding missing columns if table already existed
    try:
        c.execute("ALTER TABLE users ADD COLUMN claimed_membership_gift INTEGER DEFAULT 0")
    except Exception:
        pass

    # Set default settings
    c.execute("INSERT OR IGNORE INTO settings (key, value) VALUES ('force_join_channel', '')")
    c.execute("INSERT OR IGNORE INTO settings (key, value) VALUES ('force_join_enabled', '0')")
    c.execute("INSERT OR IGNORE INTO settings (key, value) VALUES ('referral_percent', '10')")
    c.execute("INSERT OR IGNORE INTO settings (key, value) VALUES ('support_username', '')")
    c.execute("INSERT OR IGNORE INTO settings (key, value) VALUES ('trial_gb', '1')")
    c.execute("INSERT OR IGNORE INTO settings (key, value) VALUES ('trial_days', '1')")
    conn.commit()
    conn.close()

def get_setting(key, default=""):
    try:
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT value FROM settings WHERE key = ?", (key,))
        row = c.fetchone()
        conn.close()
        return row["value"] if row else default
    except Exception:
        return default

def set_setting(key, value):
    conn = get_db()
    c = conn.cursor()
    c.execute("INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", (key, str(value)))
    conn.commit()
    conn.close()

# -------------------------------------------------------------
# Configuration & Plan Management
# -------------------------------------------------------------
def load_config():
    if not os.path.exists(CONFIG_PATH):
        return {}
    with open(CONFIG_PATH, "r", encoding="utf-8") as f:
        return json.load(f)

DEFAULT_PLANS = [
    {"id": 1, "title": "پلن برنزی (۳۰ گیگ - ۱ ماهه)", "traffic_gb": 30.0, "days": 30, "price": 45000, "enabled": True},
    {"id": 2, "title": "پلن نقره‌ای (۵۰ گیگ - ۱ ماهه)", "traffic_gb": 50.0, "days": 30, "price": 75000, "enabled": True},
    {"id": 3, "title": "پلن طلایی (۱۰۰ گیگ - ۲ ماهه)", "traffic_gb": 100.0, "days": 60, "price": 140000, "enabled": True},
    {"id": 4, "title": "پلن الماس (۲۰۰ گیگ - ۳ ماهه)", "traffic_gb": 200.0, "days": 90, "price": 260000, "enabled": True},
]

def load_plans():
    plans = []
    if os.path.exists(PLANS_PATH):
        try:
            with open(PLANS_PATH, "r", encoding="utf-8") as f:
                plans = json.load(f)
        except Exception:
            plans = []
    if not plans:
        plans = [dict(p) for p in DEFAULT_PLANS]
        try:
            os.makedirs(os.path.dirname(PLANS_PATH), exist_ok=True)
            with open(PLANS_PATH, "w", encoding="utf-8") as f:
                json.dump(plans, f, ensure_ascii=False, indent=2)
        except Exception:
            pass
    # Normalize plan keys
    normalized = []
    for p in plans:
        traffic = float(p.get("traffic_gb", p.get("traffic", 30)))
        days = int(p.get("days", p.get("period_days", 30)))
        price = int(p.get("price", p.get("price_tomans", 50000)))
        enabled = p.get("enabled", True)
        if enabled:
            normalized.append({
                "id": p.get("id", len(normalized) + 1),
                "title": p.get("title", f"پلن {traffic} گیگ"),
                "traffic_gb": traffic,
                "days": days,
                "price": price,
                "enabled": True
            })
    return normalized

def save_plans(plans):
    os.makedirs(os.path.dirname(PLANS_PATH), exist_ok=True)
    with open(PLANS_PATH, "w", encoding="utf-8") as f:
        json.dump(plans, f, ensure_ascii=False, indent=2)

def get_plan_by_id(plan_id):
    for p in load_plans():
        if p["id"] == plan_id:
            return p
    return None

# -------------------------------------------------------------
# User Profile & State Operations
# -------------------------------------------------------------
def get_user(user_id):
    conn = get_db()
    c = conn.cursor()
    c.execute("SELECT * FROM users WHERE user_id = ?", (user_id,))
    row = c.fetchone()
    conn.close()
    return row

def update_user_profile(user_id, username, first_name):
    conn = get_db()
    c = conn.cursor()
    now = int(time.time())
    c.execute("""
    INSERT INTO users (user_id, username, first_name, balance, created_at, last_seen, is_banned)
    VALUES (?, ?, ?, 0, ?, ?, 0)
    ON CONFLICT(user_id) DO UPDATE SET
        username = excluded.username,
        first_name = excluded.first_name,
        last_seen = excluded.last_seen
    """, (user_id, username, first_name, now, now))
    conn.commit()
    conn.close()

def change_user_balance(user_id, amount_tomans):
    conn = get_db()
    c = conn.cursor()
    c.execute("UPDATE users SET balance = balance + ? WHERE user_id = ?", (amount_tomans, user_id))
    c.execute("SELECT balance FROM users WHERE user_id = ?", (user_id,))
    row = c.fetchone()
    conn.commit()
    conn.close()
    return row["balance"] if row else 0

def set_user_state(user_id, state, data=None):
    conn = get_db()
    c = conn.cursor()
    data_str = json.dumps(data) if data is not None else "{}"
    c.execute("""
    INSERT INTO user_state (user_id, state, data, updated_at)
    VALUES (?, ?, ?, ?)
    ON CONFLICT(user_id) DO UPDATE SET
        state = excluded.state,
        data = excluded.data,
        updated_at = excluded.updated_at
    """, (user_id, state, data_str, int(time.time())))
    conn.commit()
    conn.close()

def get_user_state(user_id):
    conn = get_db()
    c = conn.cursor()
    c.execute("SELECT state, data FROM user_state WHERE user_id = ?", (user_id,))
    row = c.fetchone()
    conn.close()
    if not row:
        return {"state": "", "data": {}}
    try:
        data = json.loads(row["data"])
    except Exception:
        data = {}
    return {"state": row["state"], "data": data}

def clear_user_state(user_id):
    conn = get_db()
    c = conn.cursor()
    c.execute("DELETE FROM user_state WHERE user_id = ?", (user_id,))
    conn.commit()
    conn.close()

def apply_referral_reward(buyer_id, buyer_username, order_id, paid_tomans):
    conn = get_db()
    c = conn.cursor()
    c.execute("SELECT referrer_id FROM referrals WHERE user_id = ?", (buyer_id,))
    row = c.fetchone()
    if not row:
        conn.close()
        return
    referrer_id = row["referrer_id"]
    pct = int(get_setting("referral_percent", "10"))
    if pct <= 0:
        conn.close()
        return
    reward = int(paid_tomans * (pct / 100.0))
    if reward > 0:
        c.execute("""
        INSERT INTO referral_rewards (user_id, referrer_id, amount, order_id, created_at)
        VALUES (?, ?, ?, ?, ?)
        """, (buyer_id, referrer_id, reward, order_id, int(time.time())))
        c.execute("UPDATE users SET balance = balance + ? WHERE user_id = ?", (reward, referrer_id))
        conn.commit()
        # Notify Referrer
        uname_show = f"@{buyer_username}" if buyer_username else f"کاربر {buyer_id}"
        msg = (
            f"🎉 <b>پاداش زیرمجموعه‌گیری واریز شد!</b>\n\n"
            f"زیرمجموعه شما ({uname_show}) خریدی به مبلغ <b>{paid_tomans:,} تومان</b> انجام داد.\n"
            f"مبلغ <b>{reward:,} تومان</b> ({pct}٪) به کیف پول شما واریز گردید! 💰"
        )
        send_message(referrer_id, msg)
    conn.close()

# -------------------------------------------------------------
# Telegram Bot API Helpers
# -------------------------------------------------------------
def get_bot_token():
    cfg = load_config()
    return cfg.get("bot_token", "").strip()

def get_admin_chat_id():
    cfg = load_config()
    raw = str(cfg.get("admin_chat_id", "0")).strip()
    try:
        return int(raw)
    except Exception:
        return 0

def tg_request(method, payload=None, files=None):
    token = get_bot_token()
    if not token:
        return None
    url = f"https://api.telegram.org/bot{token}/{method}"
    
    # 1. Use requests if installed
    if requests is not None:
        try:
            if files:
                res = requests.post(url, data=payload or {}, files=files, timeout=25)
            else:
                res = requests.post(url, json=payload or {}, timeout=25)
            data = res.json()
            if not data.get("ok"):
                logger.warning(f"Telegram API {method} error: {data.get('description')}")
            return data
        except Exception as e:
            logger.error(f"requests error in tg_request ({method}): {e}")
            return None

    # 2. Fallback to standard library urllib
    try:
        headers = {}
        if files:
            boundary = "----DarkShopBoundary" + str(uuid.uuid4()).replace("-", "")
            headers["Content-Type"] = f"multipart/form-data; boundary={boundary}"
            body = bytearray()
            for k, v in (payload or {}).items():
                body.extend(f"--{boundary}\r\nContent-Disposition: form-data; name=\"{k}\"\r\n\r\n{v}\r\n".encode("utf-8"))
            for field_name, (fname, fcontent, ctype) in files.items():
                body.extend(f"--{boundary}\r\nContent-Disposition: form-data; name=\"{field_name}\"; filename=\"{fname}\"\r\nContent-Type: {ctype}\r\n\r\n".encode("utf-8"))
                body.extend(fcontent)
                body.extend(b"\r\n")
            body.extend(f"--{boundary}--\r\n".encode("utf-8"))
            req = urllib.request.Request(url, data=bytes(body), headers=headers, method="POST")
        else:
            headers["Content-Type"] = "application/json; charset=utf-8"
            req_data = json.dumps(payload or {}).encode("utf-8")
            req = urllib.request.Request(url, data=req_data, headers=headers, method="POST")

        with urllib.request.urlopen(req, timeout=25) as response:
            res_body = response.read().decode("utf-8")
            return json.loads(res_body)
    except urllib.error.HTTPError as e:
        err_msg = e.read().decode("utf-8")
        logger.warning(f"Telegram API HTTP error ({method}): {err_msg}")
        try:
            return json.loads(err_msg)
        except Exception:
            return None
    except Exception as e:
        logger.error(f"Network error in tg_request ({method}): {e}")
        return None

def send_message(chat_id, text, reply_markup=None, parse_mode="HTML"):
    payload = {"chat_id": chat_id, "text": text, "parse_mode": parse_mode}
    if reply_markup:
        payload["reply_markup"] = reply_markup
    return tg_request("sendMessage", payload)

def send_photo(chat_id, photo, caption=None, reply_markup=None, parse_mode="HTML"):
    payload = {"chat_id": chat_id, "parse_mode": parse_mode}
    if caption:
        payload["caption"] = caption
    if reply_markup:
        payload["reply_markup"] = json.dumps(reply_markup) if isinstance(reply_markup, dict) else reply_markup
    if isinstance(photo, bytes):
        files = {"photo": ("qrcode.png", photo, "image/png")}
        return tg_request("sendPhoto", payload, files=files)
    else:
        payload["photo"] = photo
        return tg_request("sendPhoto", payload)

def send_dice(chat_id, emoji="🎲"):
    payload = {"chat_id": chat_id, "emoji": emoji}
    return tg_request("sendDice", payload)

def edit_message_text(chat_id, message_id, text, reply_markup=None, parse_mode="HTML"):
    payload = {"chat_id": chat_id, "message_id": message_id, "text": text, "parse_mode": parse_mode}
    if reply_markup:
        payload["reply_markup"] = reply_markup
    return tg_request("editMessageText", payload)

def answer_callback_query(callback_query_id, text=None, show_alert=False):
    payload = {"callback_query_id": callback_query_id}
    if text:
        payload["text"] = text
        payload["show_alert"] = show_alert
    return tg_request("answerCallbackQuery", payload)

def forward_message(chat_id, from_chat_id, message_id):
    payload = {"chat_id": chat_id, "from_chat_id": from_chat_id, "message_id": message_id}
    return tg_request("forwardMessage", payload)

def check_channel_membership(user_id):
    force_enabled = get_setting("force_join_enabled", "0")
    if force_enabled != "1":
        return True
    channel = get_setting("force_join_channel", "")
    if not channel:
        return True
    res = tg_request("getChatMember", {"chat_id": channel, "user_id": user_id})
    if res and res.get("ok"):
        status = res.get("result", {}).get("status", "")
        return status in ["creator", "administrator", "member", "restricted"]
    return False

def generate_qr_bytes(text):
    if qrcode is not None:
        try:
            qr = qrcode.QRCode(box_size=10, border=2)
            qr.add_data(text)
            qr.make(fit=True)
            img = qr.make_image(fill_color="black", back_color="white")
            buf = io.BytesIO()
            img.save(buf, format="PNG")
            return buf.getvalue()
        except Exception:
            pass
    # Remote fallback QR service
    try:
        enc = urllib.parse.quote(text)
        url = f"https://api.qrserver.com/v1/create-qr-code/?size=350x350&data={enc}"
        req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
        with urllib.request.urlopen(req, timeout=10) as resp:
            return resp.read()
    except Exception:
        return None

# -------------------------------------------------------------
# Xray & 3X-UI Integration
# -------------------------------------------------------------
def get_inbound_and_sub_info():
    if not os.path.exists(XUI_DB_PATH):
        return None, 2096, "/sub/", "", ""
    conn = sqlite3.connect(XUI_DB_PATH, timeout=10.0)
    c = conn.cursor()
    c.execute("SELECT id, port, protocol, settings, stream_settings, remark FROM inbounds WHERE enable = 1")
    rows = c.fetchall()
    c.execute("SELECT key, value FROM settings WHERE key IN ('subPort', 'subPath', 'subDomain', 'subURI')")
    s_rows = dict(c.fetchall())
    conn.close()

    sub_port = int(s_rows.get("subPort", 2096) or 2096)
    sub_path = s_rows.get("subPath", "/sub/") or "/sub/"
    sub_domain = s_rows.get("subDomain", "") or ""
    sub_uri = s_rows.get("subURI", "") or ""

    inbound = None
    for r in rows:
        if r[2] == "vless":
            inbound = r
            break
    if not inbound and len(rows) > 0:
        inbound = rows[0]
    return inbound, sub_port, sub_path, sub_domain, sub_uri

def add_xray_client(user_id, username, traffic_gb, expiry_days, is_trial=False):
    if not os.path.exists(XUI_DB_PATH):
        logger.error("x-ui.db not found!")
        return None
    
    inbound, sub_port, sub_path, sub_domain, sub_uri = get_inbound_and_sub_info()
    if not inbound:
        logger.error("No active inbound found in x-ui.db!")
        return None
    
    inbound_id = inbound[0]
    inbound_port = inbound[1]
    inbound_settings = json.loads(inbound[3])
    
    client_uuid = str(uuid.uuid4())
    rand_suffix = "".join(random.choices(string.ascii_lowercase + string.digits, k=6))
    prefix = "trial" if is_trial else "usr"
    email = f"nova_{prefix}_{user_id}_{rand_suffix}"
    sub_id = "".join(random.choices(string.ascii_lowercase + string.digits, k=16))
    
    total_bytes = int(traffic_gb * 1024 * 1024 * 1024)
    now_ms = int(time.time() * 1000)
    expiry_time_ms = now_ms + int(expiry_days * 86400 * 1000)
    
    new_client = {
        "id": client_uuid,
        "email": email,
        "flow": "",
        "limitIp": 0,
        "totalGB": total_bytes,
        "expiryTime": expiry_time_ms,
        "enable": True,
        "tgId": str(user_id),
        "subId": sub_id,
        "reset": 0
    }
    
    if "clients" not in inbound_settings:
        inbound_settings["clients"] = []
    inbound_settings["clients"].append(new_client)
    
    conn = sqlite3.connect(XUI_DB_PATH, timeout=20.0)
    c = conn.cursor()
    c.execute("UPDATE inbounds SET settings = ? WHERE id = ?", (json.dumps(inbound_settings), inbound_id))
    
    c.execute("""
    INSERT INTO clients (inbound_id, enable, email, up, down, expiry_time, total_gb, reset, created_at, updated_at)
    VALUES (?, 1, ?, 0, 0, ?, ?, 0, ?, ?)
    """, (inbound_id, email, expiry_time_ms, total_bytes, int(time.time()), int(time.time())))
    
    c.execute("""
    INSERT INTO client_traffics (inbound_id, enable, email, up, down, expiry_time, total, reset)
    VALUES (?, 1, ?, 0, 0, ?, ?, 0)
    """, (inbound_id, email, expiry_time_ms, total_bytes))
    
    conn.commit()
    conn.close()
    
    # Restart x-ui to apply client changes cleanly
    os.system("systemctl restart x-ui")
    
    sub_clean_path = sub_path.strip("/")
    if sub_uri and sub_uri.startswith("http"):
        sub_url = f"{sub_uri.rstrip('/')}/{sub_id}"
    elif sub_domain:
        sub_url = f"https://{sub_domain}:{sub_port}/{sub_clean_path}/{sub_id}"
    else:
        sub_url = f"http://127.0.0.1:{sub_port}/{sub_clean_path}/{sub_id}"

    host = sub_domain or "127.0.0.1"
    vless_link = f"vless://{client_uuid}@{host}:{inbound_port}?type=ws&security=tls&path=%2Fnova-ws&sni={host}#NOVA_{email}"
    
    return {
        "uuid": client_uuid,
        "email": email,
        "sub_id": sub_id,
        "sub_url": sub_url,
        "vless_link": vless_link,
        "client_email": email
    }

def renew_xray_client(client_email, add_gb, add_days):
    if not os.path.exists(XUI_DB_PATH):
        return False
    
    conn = sqlite3.connect(XUI_DB_PATH, timeout=20.0)
    c = conn.cursor()
    c.execute("SELECT id, settings FROM inbounds WHERE enable = 1")
    rows = c.fetchall()
    
    found = False
    now_ms = int(time.time() * 1000)
    add_bytes = int(add_gb * 1024 * 1024 * 1024)
    add_ms = int(add_days * 86400 * 1000)
    
    new_expiry = now_ms + add_ms
    new_total = add_bytes
    
    for r in rows:
        inbound_id = r[0]
        settings = json.loads(r[1])
        clients = settings.get("clients", [])
        for cl in clients:
            if cl.get("email") == client_email:
                cur_exp = cl.get("expiryTime", 0)
                if cur_exp > now_ms:
                    cl["expiryTime"] = cur_exp + add_ms
                else:
                    cl["expiryTime"] = now_ms + add_ms
                new_expiry = cl["expiryTime"]
                
                cl["totalGB"] = cl.get("totalGB", 0) + add_bytes
                new_total = cl["totalGB"]
                cl["enable"] = True
                found = True
                break
        if found:
            c.execute("UPDATE inbounds SET settings = ? WHERE id = ?", (json.dumps(settings), inbound_id))
            break
            
    if not found:
        conn.close()
        return False
        
    c.execute("UPDATE clients SET enable = 1, expiry_time = ?, total_gb = ? WHERE email = ?", (new_expiry, new_total, client_email))
    c.execute("UPDATE client_traffics SET enable = 1, expiry_time = ?, total = ?, up = 0, down = 0 WHERE email = ?", (new_expiry, new_total, client_email))
    
    conn.commit()
    conn.close()
    
    os.system("systemctl restart x-ui")
    return True

def get_user_xray_clients(user_id):
    if not os.path.exists(XUI_DB_PATH):
        return []
    conn = sqlite3.connect(XUI_DB_PATH, timeout=10.0)
    c = conn.cursor()
    c.execute("SELECT id, settings FROM inbounds")
    inbound_rows = c.fetchall()
    
    _, sub_port, sub_path, sub_domain, sub_uri = get_inbound_and_sub_info()
    sub_clean_path = sub_path.strip("/")
    
    c.execute("SELECT email, up, down, total, expiry_time, enable FROM client_traffics")
    traffic_map = {}
    for r in c.fetchall():
        traffic_map[r[0]] = {
            "up": r[1],
            "down": r[2],
            "total": r[3],
            "expiry_time": r[4],
            "enable": bool(r[5])
        }
    conn.close()
    
    user_clients = []
    str_uid = str(user_id)
    for ir in inbound_rows:
        try:
            st = json.loads(ir[1])
            for cl in st.get("clients", []):
                if str(cl.get("tgId", "")) == str_uid:
                    email = cl.get("email", "")
                    tr = traffic_map.get(email, {})
                    used_bytes = tr.get("up", 0) + tr.get("down", 0)
                    total_bytes = tr.get("total", cl.get("totalGB", 0))
                    expiry_ms = tr.get("expiry_time", cl.get("expiryTime", 0))
                    
                    sub_id = cl.get("subId", "")
                    if sub_id:
                        if sub_uri and sub_uri.startswith("http"):
                            sub_url = f"{sub_uri.rstrip('/')}/{sub_id}"
                        elif sub_domain:
                            sub_url = f"https://{sub_domain}:{sub_port}/{sub_clean_path}/{sub_id}"
                        else:
                            sub_url = f"http://127.0.0.1:{sub_port}/{sub_clean_path}/{sub_id}"
                    else:
                        sub_url = ""
                    
                    user_clients.append({
                        "email": email,
                        "uuid": cl.get("id"),
                        "sub_url": sub_url,
                        "used_gb": round(used_bytes / (1024**3), 2),
                        "total_gb": round(total_bytes / (1024**3), 2),
                        "expiry_time_ms": expiry_ms,
                        "enable": tr.get("enable", cl.get("enable", True))
                    })
        except Exception:
            pass
    return user_clients

# -------------------------------------------------------------
# Keyboards & Menus (100% Mirza Bot UX)
# -------------------------------------------------------------
def get_main_keyboard(user_id):
    admin_id = get_admin_chat_id()
    kb = [
        [{"text": "🔐 خرید اشتراک"}, {"text": "♻️ تمدید سرویس"}],
        [{"text": "🔑 اکانت تست"}, {"text": "🎲 گردونه شانس"}],
        [{"text": "🛍 سرویس های من"}, {"text": "🏦 کیف پول + شارژ"}],
        [{"text": "👥 زیر مجموعه گیری"}, {"text": "💵 تعرفه اشتراک ها"}],
        [{"text": "☎️ پشتیبانی"}, {"text": "📚 آموزش"}]
    ]
    if user_id == admin_id and admin_id != 0:
        kb.append([{"text": "⚙️ پنل مدیریت"}])
    return {"keyboard": kb, "resize_keyboard": True}

def get_cancel_keyboard():
    return {
        "keyboard": [[{"text": "🔙 انصراف و بازگشت به منو"}]],
        "resize_keyboard": True
    }

def get_admin_inline_keyboard():
    return {
        "inline_keyboard": [
            [{"text": "📊 آمار و گزارش لحظه‌ای", "callback_data": "adm_stats"}, {"text": "👥 مدیریت کاربران", "callback_data": "adm_user_manage"}],
            [{"text": "🛍 لیست پلن‌ها", "callback_data": "adm_plans"}, {"text": "💳 تنظیمات شماره کارت", "callback_data": "adm_card"}],
            [{"text": "📢 ارسال پیام همگانی", "callback_data": "adm_bcast"}, {"text": "🔄 فوروارد همگانی", "callback_data": "adm_fwd"}],
            [{"text": "🏷 ساخت کد تخفیف", "callback_data": "adm_add_disc"}, {"text": "🔒 قفل جوین اجباری کانال", "callback_data": "adm_force_join"}],
            [{"text": "⚙️ تنظیمات اکانت تست", "callback_data": "adm_trial_cfg"}, {"text": "💰 تنظیم پورسانت معرف", "callback_data": "adm_ref_cfg"}],
            [{"text": "🚪 خروج از مدیریت", "callback_data": "adm_exit"}]
        ]
    }

# -------------------------------------------------------------
# Background Notifier (Quota & Expiry Alerts)
# -------------------------------------------------------------
def background_notifier():
    while True:
        time.sleep(300)
        try:
            if not os.path.exists(XUI_DB_PATH):
                continue
            x_conn = sqlite3.connect(XUI_DB_PATH, timeout=10.0)
            x_c = x_conn.cursor()
            x_c.execute("SELECT id, settings FROM inbounds WHERE enable = 1")
            inbounds = x_c.fetchall()
            x_c.execute("SELECT email, up, down, total, expiry_time FROM client_traffics WHERE enable = 1")
            traffics = {r[0]: {"used": r[1]+r[2], "total": r[3], "exp": r[4]} for r in x_c.fetchall()}
            x_conn.close()

            s_conn = get_db()
            s_c = s_conn.cursor()

            now_ms = int(time.time() * 1000)
            for ib in inbounds:
                try:
                    st = json.loads(ib[1])
                    for cl in st.get("clients", []):
                        tg_id = cl.get("tgId")
                        if not tg_id or not tg_id.isdigit():
                            continue
                        uid = int(tg_id)
                        email = cl.get("email")
                        tr = traffics.get(email)
                        if not tr:
                            continue

                        used = tr["used"]
                        total = tr["total"]
                        exp = tr["exp"]

                        # Check 90% volume threshold
                        if total > 0 and (used / total) >= 0.90:
                            s_c.execute("SELECT notified_type FROM notifications_sent WHERE client_email = ?", (email,))
                            row = s_c.fetchone()
                            if not row or row["notified_type"] != "VOLUME_90":
                                rem_mb = max(0, int((total - used) / (1024 * 1024)))
                                alert_txt = (
                                    f"⚠️ <b>هشدار اتمام حجم اشتراک!</b>\n\n"
                                    f"کاربر گرامی، بیش از ۹۰٪ از حجم سرویس شما مصرف شده است.\n"
                                    f"سرویس: <code>{email}</code>\n"
                                    f"حجم باقیمانده: <b>{rem_mb} مگابایت</b>\n\n"
                                    f"جهت جلوگیری از قطع ارتباط، لطفاً از منوی «♻️ تمدید سرویس» نسبت به تمدید اقدام فرمایید."
                                )
                                send_message(uid, alert_txt)
                                s_c.execute("""
                                INSERT INTO notifications_sent (client_email, notified_type, sent_at)
                                VALUES (?, 'VOLUME_90', ?)
                                ON CONFLICT(client_email) DO UPDATE SET notified_type = excluded.notified_type, sent_at = excluded.sent_at
                                """, (email, int(time.time())))
                                s_conn.commit()

                        # Check 24 hours expiry threshold
                        if exp > 0 and (0 < (exp - now_ms) <= 86400 * 1000):
                            s_c.execute("SELECT sent_at FROM notifications_sent WHERE client_email = ? AND notified_type = 'EXPIRING'", (email,))
                            if not s_c.fetchone():
                                exp_str = datetime.datetime.fromtimestamp(exp/1000).strftime('%Y-%m-%d %H:%M')
                                alert_txt = (
                                    f"⏳ <b>هشدار انقضای زمان سرویس!</b>\n\n"
                                    f"کاربر گرامی، کمتر از ۲۴ ساعت تا پایان اعتبار اشتراک شما باقیمانده است.\n"
                                    f"سرویس: <code>{email}</code>\n"
                                    f"تاریخ انقضا: <code>{exp_str}</code>\n\n"
                                    f"جهت تداوم اتصال، از دکمه «♻️ تمدید سرویس» استفاده فرمایید."
                                )
                                send_message(uid, alert_txt)
                                s_c.execute("""
                                INSERT INTO notifications_sent (client_email, notified_type, sent_at)
                                VALUES (?, 'EXPIRING', ?)
                                ON CONFLICT(client_email) DO UPDATE SET sent_at = excluded.sent_at
                                """, (email, int(time.time())))
                                s_conn.commit()
                except Exception:
                    pass
            s_conn.close()
        except Exception as e:
            logger.error(f"Notifier thread error: {e}")

# -------------------------------------------------------------
# Telegram Callback Query Handler (Exact Mirza Bot Behavior)
# -------------------------------------------------------------
def handle_callback_query(cq):
    cq_id = cq["id"]
    from_user = cq["from"]
    user_id = from_user["id"]
    data = cq.get("data", "")
    message = cq.get("message")
    msg_id = message["message_id"] if message else None
    
    admin_id = get_admin_chat_id()
    
    if data == "check_join":
        if check_channel_membership(user_id):
            answer_callback_query(cq_id, "عضویت شما تایید شد! ✅")
            if msg_id:
                edit_message_text(user_id, msg_id, "✅ عضویت شما تایید گردید. به ربات خوش آمدید!", get_main_keyboard(user_id))
        else:
            channel = get_setting("force_join_channel", "")
            answer_callback_query(cq_id, f"هنوز در کانال {channel} عضو نشده‌اید!", show_alert=True)
        return
        
    if data == "menu_home":
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, "به منوی اصلی بازگشتید. جهت ادامه از دکمه‌های زیر استفاده فرمایید:")
        return

    if data == "adm_exit":
        if user_id == admin_id and msg_id:
            edit_message_text(user_id, msg_id, "از پنل مدیریت خارج شدید.")
        answer_callback_query(cq_id)
        return

    # Admin Stats
    if data == "adm_stats":
        if user_id != admin_id:
            answer_callback_query(cq_id, "دسترسی غیرمجاز", show_alert=True)
            return
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT COUNT(*) FROM users")
        total_users = c.fetchone()[0]
        c.execute("SELECT COUNT(*) FROM orders WHERE status = 'APPROVED'")
        approved_orders = c.fetchone()[0]
        c.execute("SELECT SUM(final_price) FROM orders WHERE status = 'APPROVED'")
        rev = c.fetchone()[0] or 0
        c.execute("SELECT COUNT(*) FROM trials")
        total_trials = c.fetchone()[0]
        c.execute("SELECT SUM(balance) FROM users")
        total_wallets = c.fetchone()[0] or 0
        conn.close()
        
        stat_text = (
            f"📊 <b>گزارش آماری ربات فروشگاه (NOVA Mirza Edition)</b>\n\n"
            f"👥 <b>تعداد کل کاربران:</b> {total_users:,} نفر\n"
            f"🎁 <b>تست‌های صادر شده:</b> {total_trials:,} عدد\n"
            f"🛒 <b>سفارشات موفق:</b> {approved_orders:,} عدد\n"
            f"💰 <b>مجموع فروش نقدی:</b> {rev:,} تومان\n"
            f"💳 <b>موجودی در گردش کیف پول‌ها:</b> {total_wallets:,} تومان\n\n"
            f"⚙️ <b>وضعیت اتصال پنل:</b> متصل و پایدار 🟢"
        )
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, stat_text, get_admin_inline_keyboard())
        return

    if data == "adm_bcast":
        if user_id != admin_id:
            return
        set_user_state(user_id, "WAIT_BCAST")
        answer_callback_query(cq_id)
        send_message(user_id, "📢 <b>ارسال پیام همگانی</b>\n\nلطفاً متن پیام مورد نظر را ارسال کنید:", get_cancel_keyboard())
        return

    if data == "adm_fwd":
        if user_id != admin_id:
            return
        set_user_state(user_id, "WAIT_FWD")
        answer_callback_query(cq_id)
        send_message(user_id, "🔄 <b>فوروارد همگانی</b>\n\nلطفاً پیامی که می‌خواهید برای همه فوروارد شود را به ربات فوروارد نمایید:", get_cancel_keyboard())
        return

    if data == "adm_user_manage":
        if user_id != admin_id:
            return
        set_user_state(user_id, "WAIT_USER_SEARCH")
        answer_callback_query(cq_id)
        send_message(user_id, "👥 <b>مدیریت کاربر</b>\n\nلطفاً شناسه عددی (User ID) یا آیدی کاربر را ارسال کنید:", get_cancel_keyboard())
        return

    if data == "adm_card":
        if user_id != admin_id:
            return
        cfg = load_config()
        cur_card = cfg.get("card_number", "تنظیم نشده")
        cur_holder = cfg.get("card_holder", "تنظیم نشده")
        set_user_state(user_id, "WAIT_SET_CARD")
        answer_callback_query(cq_id)
        msg_t = (
            f"💳 <b>تنظیمات کارت بانکی</b>\n\n"
            f"شماره کارت فعلی: <code>{cur_card}</code>\n"
            f"به نام: <b>{cur_holder}</b>\n\n"
            f"جهت تغییر، شماره کارت ۱۶ رقمی جدید را ارسال کنید:"
        )
        send_message(user_id, msg_t, get_cancel_keyboard())
        return

    if data == "adm_plans":
        if user_id != admin_id:
            return
        plans = load_plans()
        ikb_rows = []
        for p in plans:
            ikb_rows.append([{"text": f"❌ حذف {p['title']}", "callback_data": f"adm_delplan_{p['id']}"}])
        ikb_rows.append([{"text": "➕ افزودن پلن جدید", "callback_data": "adm_addplan"}])
        ikb_rows.append([{"text": "🔙 بازگشت به مدیریت", "callback_data": "adm_stats"}])
        txt = "🛍 <b>مدیریت پلن‌های فروشگاه:</b>\nجهت حذف روی هر پلن کلیک کنید یا دکمه افزودن را بزنید:"
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, txt, {"inline_keyboard": ikb_rows})
        return

    if data.startswith("adm_delplan_"):
        if user_id != admin_id:
            return
        pid = int(data.split("_")[2])
        plans = [p for p in load_plans() if p["id"] != pid]
        save_plans(plans)
        answer_callback_query(cq_id, "پلن با موفقیت حذف شد.")
        # Reload
        ikb_rows = []
        for p in plans:
            ikb_rows.append([{"text": f"❌ حذف {p['title']}", "callback_data": f"adm_delplan_{p['id']}"}])
        ikb_rows.append([{"text": "➕ افزودن پلن جدید", "callback_data": "adm_addplan"}])
        ikb_rows.append([{"text": "🔙 بازگشت به مدیریت", "callback_data": "adm_stats"}])
        if msg_id:
            edit_message_text(user_id, msg_id, "🛍 <b>مدیریت پلن‌های فروشگاه:</b>", {"inline_keyboard": ikb_rows})
        return

    if data == "adm_addplan":
        if user_id != admin_id:
            return
        set_user_state(user_id, "WAIT_NEW_PLAN")
        answer_callback_query(cq_id)
        hint = (
            "➕ <b>افزودن پلن جدید</b>\n\n"
            "لطفاً اطلاعات پلن را به فرمت زیر در یک خط ارسال فرمایید:\n"
            "<code>عنوان|حجم به گیگ|مدت به روز|قیمت به تومان</code>\n\n"
            "مثال:\n"
            "<code>پلن نقره‌ای|50|30|75000</code>"
        )
        send_message(user_id, hint, get_cancel_keyboard())
        return

    if data == "adm_add_disc":
        if user_id != admin_id:
            return
        set_user_state(user_id, "WAIT_ADD_DISC")
        answer_callback_query(cq_id)
        send_message(user_id, "🏷 <b>ساخت کد تخفیف</b>\n\nکد و درصد تخفیف را به فرمت زیر ارسال کنید:\n<code>کد|درصد|تعداد استفاده</code>\n\nمثال:\n<code>NOVA20|20|50</code>", get_cancel_keyboard())
        return

    if data == "adm_force_join":
        if user_id != admin_id:
            return
        cur_ch = get_setting("force_join_channel", "تنظیم نشده")
        cur_en = get_setting("force_join_enabled", "0")
        en_str = "فعال 🟢" if cur_en == "1" else "غیرفعال 🔴"
        ikb = {
            "inline_keyboard": [
                [{"text": "تغییر وضعیت فعال/غیرفعال", "callback_data": "adm_toggle_force"}],
                [{"text": "تغییر آیدی کانال", "callback_data": "adm_set_channel"}],
                [{"text": "🔙 بازگشت", "callback_data": "adm_stats"}]
            ]
        }
        answer_callback_query(cq_id)
        txt = f"🔒 <b>تنظیمات قفل کانال اجباری</b>\n\nکانال فعلی: <code>{cur_ch}</code>\nوضعیت: <b>{en_str}</b>"
        if msg_id:
            edit_message_text(user_id, msg_id, txt, ikb)
        return

    if data == "adm_toggle_force":
        if user_id != admin_id:
            return
        cur_en = get_setting("force_join_enabled", "0")
        new_en = "0" if cur_en == "1" else "1"
        set_setting("force_join_enabled", new_en)
        answer_callback_query(cq_id, f"وضعیت قفل تغییر یافت به: {'فعال' if new_en=='1' else 'غیرفعال'}")
        return

    if data == "adm_set_channel":
        if user_id != admin_id:
            return
        set_user_state(user_id, "WAIT_SET_CHANNEL")
        answer_callback_query(cq_id)
        send_message(user_id, "لطفاً آیدی کانال را به همراه @ ارسال کنید (ربات باید در کانال ادمین باشد):\nمثال: @MyChannel", get_cancel_keyboard())
        return

    if data == "adm_trial_cfg":
        if user_id != admin_id:
            return
        t_gb = get_setting("trial_gb", "1")
        t_d = get_setting("trial_days", "1")
        set_user_state(user_id, "WAIT_SET_TRIAL")
        answer_callback_query(cq_id)
        send_message(user_id, f"🔑 <b>تنظیمات اکانت تست</b>\n\nحجم فعلی: {t_gb} GB\nمدت فعلی: {t_d} روز\n\nلطفاً مقدار جدید را به فرمت <code>حجم|روز</code> ارسال فرمایید (مثال: <code>1|1</code>):", get_cancel_keyboard())
        return

    if data == "adm_ref_cfg":
        if user_id != admin_id:
            return
        pct = get_setting("referral_percent", "10")
        set_user_state(user_id, "WAIT_SET_REF_PCT")
        answer_callback_query(cq_id)
        send_message(user_id, f"💰 <b>تنظیم درصد پورسانت معرف</b>\n\nدرصد فعلی: {pct}٪\n\nلطفاً درصد جدید را به صورت عدد لاتین بفرستید (مثال: 15):", get_cancel_keyboard())
        return

    # Admin User Actions
    if data.startswith("adm_user_ban_"):
        if user_id != admin_id:
            return
        target_uid = int(data.split("_")[3])
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT is_banned FROM users WHERE user_id = ?", (target_uid,))
        row = c.fetchone()
        cur_ban = row["is_banned"] if row else 0
        new_ban = 0 if cur_ban else 1
        c.execute("UPDATE users SET is_banned = ? WHERE user_id = ?", (new_ban, target_uid))
        conn.commit()
        conn.close()
        st_text = "مسدود شد ⛔️" if new_ban else "رفع مسدودی شد 🟢"
        answer_callback_query(cq_id, f"کاربر {target_uid} {st_text}")
        return

    if data.startswith("adm_user_add_"):
        if user_id != admin_id:
            return
        target_uid = int(data.split("_")[3])
        set_user_state(user_id, "WAIT_USER_ADD_BAL", {"target_uid": target_uid})
        answer_callback_query(cq_id)
        send_message(user_id, f"مبلغ مورد نظر برای شارژ کیف پول کاربر <code>{target_uid}</code> را به تومان بفرستید:", get_cancel_keyboard())
        return

    if data.startswith("adm_user_sub_"):
        if user_id != admin_id:
            return
        target_uid = int(data.split("_")[3])
        set_user_state(user_id, "WAIT_USER_SUB_BAL", {"target_uid": target_uid})
        answer_callback_query(cq_id)
        send_message(user_id, f"مبلغ مورد نظر برای کسر از کیف پول کاربر <code>{target_uid}</code> را به تومان بفرستید:", get_cancel_keyboard())
        return

    # Order Approvals & Rejections (Admin)
    if data.startswith("reject_"):
        if user_id != admin_id:
            return
        order_id = int(data.split("_")[1])
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT * FROM orders WHERE id = ?", (order_id,))
        order = c.fetchone()
        if not order:
            answer_callback_query(cq_id, "سفارش یافت نشد.")
            conn.close()
            return
        c.execute("UPDATE orders SET status = 'REJECTED' WHERE id = ?", (order_id,))
        conn.commit()
        conn.close()
        answer_callback_query(cq_id, "فیش واریزی رد شد.")
        send_message(order["user_id"], f"❌ <b>فیش واریزی شما تایید نشد!</b>\n\nسفارش: #{order_id}\nدر صورت بروز اشتباه یا نیاز به پیگیری، با پشتیبانی در ارتباط باشید.")
        if msg_id:
            edit_message_text(admin_id, msg_id, f"❌ فیش سفارش #{order_id} توسط شما رد شد.")
        return

    if data.startswith("approve_"):
        if user_id != admin_id:
            return
        order_id = int(data.split("_")[1])
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT * FROM orders WHERE id = ?", (order_id,))
        order = c.fetchone()
        if not order or order["status"] != "PENDING":
            answer_callback_query(cq_id, "این سفارش قبلاً پردازش شده است.")
            conn.close()
            return
            
        buyer_id = order["user_id"]
        buyer_name = order["username"] or str(buyer_id)
        order_type = order["order_type"]
        final_price = order["final_price"]
        target_email = order["target_email"]
        
        c.execute("UPDATE orders SET status = 'APPROVED' WHERE id = ?", (order_id,))
        conn.commit()
        conn.close()
        
        answer_callback_query(cq_id, "سفارش با موفقیت تایید گردید! 🚀")
        apply_referral_reward(buyer_id, buyer_name, order_id, final_price)
        
        # 1. Handle Wallet Top-up Order
        if order_type == "WALLET_TOPUP":
            new_bal = change_user_balance(buyer_id, final_price)
            send_message(buyer_id, f"🎉 <b>فیش واریزی شما تایید شد!</b>\n\nمبلغ <b>{final_price:,} تومان</b> به کیف پول حساب شما اضافه گردید.\n💵 موجودی فعلی: <b>{new_bal:,} تومان</b>", get_main_keyboard(buyer_id))
            if msg_id:
                edit_message_text(admin_id, msg_id, f"✅ فیش شارژ کیف پول #{order_id} به مبلغ {final_price:,} تومان تایید شد.")
            return

        # 2. Handle Renewal Order
        if order_type == "RENEW":
            plan = get_plan_by_id(order["plan_id"])
            if plan:
                ok = renew_xray_client(target_email, plan["traffic_gb"], plan["days"])
                if ok:
                    send_message(buyer_id, f"✅ <b>سرویس شما با موفقیت تمدید شد!</b>\n\nسرویس: <code>{target_email}</code>\nافزوده شد: <b>{plan['traffic_gb']} گیگابایت</b> و <b>{plan['days']} روز</b>", get_main_keyboard(buyer_id))
                    if msg_id:
                        edit_message_text(admin_id, msg_id, f"✅ سفارش تمدید #{order_id} با موفقیت تایید و اعمال شد.")
                else:
                    send_message(admin_id, f"⚠️ خطایی در تمدید خودکار کلاینت {target_email} رخ داد.")
            return

        # 3. Handle Regular Plan Purchase Order
        plan = get_plan_by_id(order["plan_id"])
        if not plan:
            send_message(admin_id, "پلن مربوطه یافت نشد!")
            return
            
        res = add_xray_client(buyer_id, buyer_name, plan["traffic_gb"], plan["days"])
        if not res:
            send_message(admin_id, f"❌ خطا در ایجاد کلاینت Xray برای سفارش #{order_id}!")
            send_message(buyer_id, "خطایی در ساخت خودکار کانفیگ رخ داده است. لطفاً به پشتیبانی پیام دهید.")
            return
            
        conn = get_db()
        c = conn.cursor()
        c.execute("UPDATE orders SET sub_id = ? WHERE id = ?", (res["sub_id"], order_id))
        conn.commit()
        conn.close()

        # Mirza Bot Format for created service
        conf_msg = (
            f"✅ سرویس با موفقیت ایجاد شد\n\n"
            f"👤 نام کاربری سرویس : <code>{res['client_email']}</code>\n"
            f"🌿 نام سرویس: {plan['title']}\n"
            f"‏🇺🇳 لوکیشن: آلمان 🇩🇪\n"
            f"⏳ مدت زمان: {plan['days']} روز\n"
            f"🗜 حجم سرویس: {plan['traffic_gb']} گیگابایت\n\n"
            f"لینک اتصال:\n"
            f"<code>{res['sub_url']}</code>\n\n"
            f"⚡️ کانفیگ تک‌خطی:\n"
            f"<code>{res['vless_link']}</code>\n\n"
            f"🧑‍🦯 شما میتوانید شیوه اتصال را با فشردن دکمه زیر و انتخاب سیستم عامل خود را دریافت کنید"
        )
        ikb = {
            "inline_keyboard": [
                [{"text": "📚 مشاهده آموزش استفاده", "callback_data": "help_menu"}]
            ]
        }
        send_message(buyer_id, conf_msg, ikb)
        qr_bytes = generate_qr_bytes(res["sub_url"])
        if qr_bytes:
            send_photo(buyer_id, qr_bytes, caption="📸 اسکن بارکد QR جهت اتصال مستقیم")
        
        if msg_id:
            edit_message_text(admin_id, msg_id, f"✅ سفارش #{order_id} تایید و تحویل کاربر گردید.")
        return

    # User Plan Selection & Pre-Invoice (Mirza Format)
    if data.startswith("plan_"):
        plan_id = int(data.split("_")[1])
        plan = get_plan_by_id(plan_id)
        if not plan:
            answer_callback_query(cq_id, "پلن نامعتبر است.")
            return
            
        u = get_user(user_id)
        bal = u["balance"] if u else 0
        price = plan["price"]
        uname_display = f"@{from_user.get('username')}" if from_user.get("username") else str(user_id)
        
        # Exact Mirza Bot Pre-Invoice
        txt = (
            f"📇 <b>پیش فاکتور شما:</b>\n"
            f"👤 <b>نام کاربری:</b> <code>{uname_display}</code>\n"
            f"🔐 <b>نام سرویس:</b> {plan['title']}\n"
            f"📆 <b>مدت اعتبار:</b> {plan['days']} روز\n"
            f"💶 <b>قیمت:</b> {price:,} تومان\n"
            f"👥 <b>حجم اکانت:</b> {plan['traffic_gb']} گیگ\n"
            f"💵 <b>موجودی کیف پول شما :</b> {bal:,} تومان\n\n"
            f"💰 <b>سفارش شما آماده پرداخت است.</b>"
        )
        ikb_rows = []
        if bal >= price:
            ikb_rows.append([{"text": "⚡️ پرداخت آنی از کیف پول", "callback_data": f"buy_wallet_{plan_id}"}])
        ikb_rows.append([{"text": "💳 پرداخت کارت به کارت", "callback_data": f"buy_card_{plan_id}"}])
        ikb_rows.append([{"text": "🎁 اعمال کد تخفیف", "callback_data": f"apply_disc_{plan_id}"}])
        ikb_rows.append([{"text": "🔙 بازگشت به لیست پلن‌ها", "callback_data": "list_plans"}])
        
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, txt, {"inline_keyboard": ikb_rows})
        return

    if data == "list_plans":
        plans = load_plans()
        ikb_rows = []
        for p in plans:
            ikb_rows.append([{"text": f"🛒 {p['title']} — {p['price']:,} تومان", "callback_data": f"plan_{p['id']}"}])
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, "🛍️ لطفاً سرویسی که می‌خواهید خریداری کنید را انتخاب کنید!", {"inline_keyboard": ikb_rows})
        return

    # Wallet Payment
    if data.startswith("buy_wallet_"):
        plan_id = int(data.split("_")[2])
        plan = get_plan_by_id(plan_id)
        if not plan:
            return
        price = plan["price"]
        u = get_user(user_id)
        bal = u["balance"] if u else 0
        if bal < price:
            answer_callback_query(cq_id, "موجودی کیف پول شما کافی نیست!", show_alert=True)
            return
            
        change_user_balance(user_id, -price)
        res = add_xray_client(user_id, from_user.get("username", ""), plan["traffic_gb"], plan["days"])
        if not res:
            change_user_balance(user_id, price) # refund
            answer_callback_query(cq_id, "خطا در برقراری ارتباط با هسته سرور!", show_alert=True)
            return
            
        conn = get_db()
        c = conn.cursor()
        c.execute("""
        INSERT INTO orders (user_id, username, order_type, plan_id, plan_title, price_tomans, final_price, status, sub_id, created_at)
        VALUES (?, ?, 'PLAN', ?, ?, ?, ?, 'APPROVED', ?, ?)
        """, (user_id, from_user.get("username", ""), plan_id, plan["title"], price, price, res["sub_id"], int(time.time())))
        order_id = c.lastrowid
        conn.commit()
        conn.close()
        
        answer_callback_query(cq_id, "خرید با موفقیت انجام شد! 🚀")
        apply_referral_reward(user_id, from_user.get("username", ""), order_id, price)
        
        # Exact Mirza Bot Format
        user_msg = (
            f"✅ سرویس با موفقیت ایجاد شد\n\n"
            f"👤 نام کاربری سرویس : <code>{res['client_email']}</code>\n"
            f"🌿 نام سرویس: {plan['title']}\n"
            f"‏🇺🇳 لوکیشن: آلمان 🇩🇪\n"
            f"⏳ مدت زمان: {plan['days']} روز\n"
            f"🗜 حجم سرویس: {plan['traffic_gb']} گیگابایت\n\n"
            f"لینک اتصال:\n"
            f"<code>{res['sub_url']}</code>\n\n"
            f"⚡️ کانفیگ تک‌خطی:\n"
            f"<code>{res['vless_link']}</code>\n\n"
            f"🧑‍🦯 شما میتوانید شیوه اتصال را با فشردن دکمه زیر و انتخاب سیستم عامل خود را دریافت کنید"
        )
        ikb = {
            "inline_keyboard": [
                [{"text": "📚 مشاهده آموزش استفاده", "callback_data": "help_menu"}]
            ]
        }
        if msg_id:
            edit_message_text(user_id, msg_id, user_msg, ikb)
        else:
            send_message(user_id, user_msg, ikb)
        qr_bytes = generate_qr_bytes(res["sub_url"])
        if qr_bytes:
            send_photo(user_id, qr_bytes, caption="📸 اسکن بارکد QR جهت اتصال مستقیم")
        
        # Notify admin of instant wallet sale
        admin_notice = f"⚡️ <b>فروش آنی از کیف پول</b>\n\n👤 کاربر: @{from_user.get('username', '')} (<code>{user_id}</code>)\n📦 پلن: {plan['title']}\n💵 مبلغ: {price:,} تومان"
        send_message(admin_id, admin_notice)
        return

    # Card Payment Step 1: Instruction and Bank Info (Exact Mirza Format)
    if data.startswith("buy_card_"):
        plan_id = int(data.split("_")[2])
        plan = get_plan_by_id(plan_id)
        if not plan:
            return
        cfg = load_config()
        card_num = cfg.get("card_number", "۶۰۳۷-۹۹۷۴-XXXX-XXXX")
        card_holder = cfg.get("card_holder", "DARK VVPN")
        price = plan["price"]
        
        txt = (
            f"برای خرید سرویس، مبلغ <code>{price:,}</code> تومان را به شماره‌ی حساب زیر واریز کنید 👇🏻\n\n"
            f"====================\n"
            f"<code>{card_num}</code>\n"
            f"{card_holder}\n"
            f"====================\n\n"
            f"❌ این تراکنش به مدت یک ساعت اعتبار دارد پس از آن امکان پرداخت این تراکنش امکان ندارد.\n"
            f"‼️ مبلغ باید همان مبلغی که در بالا ذکر شده واریز نمایید.\n"
            f"‼️ امکان برداشت وجه از کیف پول نیست.\n"
            f"‼️ مسئولیت واریز اشتباهی با شماست.\n"
            f"🔝 بعد از پرداخت دکمه پرداخت کردم را زده سپس تصویر رسید را ارسال نمایید\n"
            f"💵 بعد از تایید پرداختتون توسط ادمین، سرویس شما به صورت خودکار صادر خواهد شد."
        )
        ikb = {
            "inline_keyboard": [
                [{"text": "✅ پرداخت کردم | ارسال رسید.", "callback_data": f"sendreceipt_plan_{plan_id}"}],
                [{"text": "🔙 بازگشت به پیش‌فاکتور", "callback_data": f"plan_{plan_id}"}]
            ]
        }
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, txt, ikb)
        else:
            send_message(user_id, txt, ikb)
        return

    # Card Payment Step 2: Prompt for Receipt Photo
    if data.startswith("sendreceipt_plan_"):
        plan_id = int(data.split("_")[2])
        plan = get_plan_by_id(plan_id)
        if not plan:
            return
        set_user_state(user_id, "WAIT_RECEIPT_PLAN", {"plan_id": plan_id, "price": plan["price"]})
        answer_callback_query(cq_id)
        send_message(user_id, "📌 لطفاً تصویر رسید واریزی (عکس فیش بانکی) خود را ارسال نمایید:", get_cancel_keyboard())
        return

    if data.startswith("apply_disc_"):
        plan_id = int(data.split("_")[2])
        set_user_state(user_id, "WAIT_DISCOUNT_CODE", {"plan_id": plan_id})
        answer_callback_query(cq_id)
        send_message(user_id, "🏷 لطفاً کد تخفیف خود را ارسال کنید:", get_cancel_keyboard())
        return

    # Topup Menu
    if data == "topup_menu":
        answer_callback_query(cq_id)
        amounts = [50000, 100000, 200000, 500000]
        ikb_rows = []
        ikb_rows.append([{"text": f"💳 {amounts[0]:,} تومان", "callback_data": f"topup_{amounts[0]}"}, {"text": f"💳 {amounts[1]:,} تومان", "callback_data": f"topup_{amounts[1]}"}])
        ikb_rows.append([{"text": f"💳 {amounts[2]:,} تومان", "callback_data": f"topup_{amounts[2]}"}, {"text": f"💳 {amounts[3]:,} تومان", "callback_data": f"topup_{amounts[3]}"}])
        ikb_rows.append([{"text": "✏️ مبلغ دلخواه", "callback_data": "topup_custom"}])
        ikb_rows.append([{"text": "🔙 بازگشت به کیف پول", "callback_data": "menu_wallet"}])
        txt = "💰 <b>مبلغ مورد نظر جهت افزایش موجودی را انتخاب فرمایید:</b>"
        if msg_id:
            edit_message_text(user_id, msg_id, txt, {"inline_keyboard": ikb_rows})
        else:
            send_message(user_id, txt, {"inline_keyboard": ikb_rows})
        return

    if data == "topup_custom":
        set_user_state(user_id, "WAIT_CUSTOM_TOPUP")
        answer_callback_query(cq_id)
        send_message(user_id, "✏️ لطفاً مبلغ مورد نظر جهت شارژ کیف پول را به تومان و به صورت لاتین ارسال فرمایید (مثال: 50000):", get_cancel_keyboard())
        return

    if data.startswith("topup_"):
        amt = int(data.split("_")[1])
        cfg = load_config()
        card_num = cfg.get("card_number", "۶۰۳۷-۹۹۷۴-XXXX-XXXX")
        card_holder = cfg.get("card_holder", "DARK VVPN")
        txt = (
            f"برای افزایش موجودی، مبلغ <code>{amt:,}</code> تومان را به شماره‌ی حساب زیر واریز کنید 👇🏻\n\n"
            f"====================\n"
            f"<code>{card_num}</code>\n"
            f"{card_holder}\n"
            f"====================\n\n"
            f"❌ این تراکنش به مدت یک ساعت اعتبار دارد پس از آن امکان پرداخت این تراکنش امکان ندارد.\n"
            f"‼️ مبلغ باید همان مبلغی که در بالا ذکر شده واریز نمایید.\n"
            f"‼️ امکان برداشت وجه از کیف پول نیست.\n"
            f"‼️ مسئولیت واریز اشتباهی با شماست.\n"
            f"🔝 بعد از پرداخت دکمه پرداخت کردم را زده سپس تصویر رسید را ارسال نمایید\n"
            f"💵 بعد از تایید پرداختتون توسط ادمین کیف پول شما شارژ خواهد شد"
        )
        ikb = {
            "inline_keyboard": [
                [{"text": "✅ پرداخت کردم | ارسال رسید.", "callback_data": f"sendreceipt_topup_{amt}"}],
                [{"text": "🔙 بازگشت به منوی قبل", "callback_data": "topup_menu"}]
            ]
        }
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, txt, ikb)
        else:
            send_message(user_id, txt, ikb)
        return

    if data.startswith("sendreceipt_topup_"):
        amt = int(data.split("_")[2])
        set_user_state(user_id, "WAIT_RECEIPT_WALLET", {"amount": amt})
        answer_callback_query(cq_id)
        send_message(user_id, "📌 لطفاً تصویر رسید واریزی (عکس فیش بانکی) خود را ارسال نمایید:", get_cancel_keyboard())
        return

    # Renew Services
    if data.startswith("renew_"):
        email = data[6:]
        plans = load_plans()
        ikb_rows = []
        for p in plans:
            ikb_rows.append([{"text": f"🔄 {p['title']} — {p['price']:,} تومان", "callback_data": f"dorenew_{p['id']}_{email}"}])
        ikb_rows.append([{"text": "🔙 بازگشت", "callback_data": "my_services"}])
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, f"سرویس انتخابی برای تمدید: <code>{email}</code>\nپلن تمدید مورد نظر را انتخاب فرمایید:", {"inline_keyboard": ikb_rows})
        return

    if data.startswith("dorenew_"):
        parts = data.split("_")
        plan_id = int(parts[1])
        email = "_".join(parts[2:])
        plan = get_plan_by_id(plan_id)
        if not plan:
            return
        u = get_user(user_id)
        bal = u["balance"] if u else 0
        price = plan["price"]
        
        txt = (
            f"📇 <b>پیش فاکتور تمدید سرویس:</b>\n"
            f"👤 <b>نام کاربری:</b> <code>{email}</code>\n"
            f"🔐 <b>پلن تمدید:</b> {plan['title']}\n"
            f"📆 <b>مدت افزایش:</b> {plan['days']} روز\n"
            f"👥 <b>حجم افزایش:</b> {plan['traffic_gb']} گیگ\n"
            f"💶 <b>قیمت:</b> {price:,} تومان\n"
            f"💵 <b>موجودی کیف پول:</b> {bal:,} تومان\n\n"
            f"روش پرداخت را انتخاب نمایید:"
        )
        ikb_rows = []
        if bal >= price:
            ikb_rows.append([{"text": "⚡️ پرداخت آنی از کیف پول", "callback_data": f"doreneww_{plan_id}_{email}"}])
        ikb_rows.append([{"text": "💳 پرداخت کارت به کارت", "callback_data": f"dorenewc_{plan_id}_{email}"}])
        ikb_rows.append([{"text": "🔙 بازگشت به لیست پلن‌ها", "callback_data": f"renew_{email}"}])
        
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, txt, {"inline_keyboard": ikb_rows})
        return

    if data.startswith("doreneww_"):
        parts = data.split("_")
        plan_id = int(parts[1])
        email = "_".join(parts[2:])
        plan = get_plan_by_id(plan_id)
        if not plan:
            return
        price = plan["price"]
        u = get_user(user_id)
        bal = u["balance"] if u else 0
        if bal < price:
            answer_callback_query(cq_id, "موجودی کیف پول کافی نیست!", show_alert=True)
            return
        change_user_balance(user_id, -price)
        ok = renew_xray_client(email, plan["traffic_gb"], plan["days"])
        if not ok:
            change_user_balance(user_id, price)
            answer_callback_query(cq_id, "خطا در تمدید کلاینت!", show_alert=True)
            return
            
        conn = get_db()
        c = conn.cursor()
        c.execute("""
        INSERT INTO orders (user_id, username, order_type, plan_id, plan_title, price_tomans, final_price, target_email, status, created_at)
        VALUES (?, ?, 'RENEW', ?, ?, ?, ?, ?, 'APPROVED', ?)
        """, (user_id, from_user.get("username", ""), plan_id, plan["title"], price, price, email, int(time.time())))
        order_id = c.lastrowid
        conn.commit()
        conn.close()
        
        apply_referral_reward(user_id, from_user.get("username", ""), order_id, price)
        answer_callback_query(cq_id, "اشتراک با موفقیت تمدید شد! 🚀")
        if msg_id:
            edit_message_text(user_id, msg_id, f"✅ <b>سرویس شما با موفقیت تمدید شد!</b>\n\nسرویس: <code>{email}</code>\nافزوده شد: <b>{plan['traffic_gb']} گیگابایت</b> و <b>{plan['days']} روز</b>\nمبلغ کسر شده: {price:,} تومان")
        return

    if data.startswith("dorenewc_"):
        parts = data.split("_")
        plan_id = int(parts[1])
        email = "_".join(parts[2:])
        plan = get_plan_by_id(plan_id)
        if not plan:
            return
        cfg = load_config()
        card_num = cfg.get("card_number", "۶۰۳۷-۹۹۷۴-XXXX-XXXX")
        card_holder = cfg.get("card_holder", "DARK VVPN")
        price = plan["price"]
        txt = (
            f"برای تمدید سرویس، مبلغ <code>{price:,}</code> تومان را به شماره‌ی حساب زیر واریز کنید 👇🏻\n\n"
            f"====================\n"
            f"<code>{card_num}</code>\n"
            f"{card_holder}\n"
            f"====================\n\n"
            f"❌ این تراکنش به مدت یک ساعت اعتبار دارد پس از آن امکان پرداخت این تراکنش امکان ندارد.\n"
            f"‼️ مبلغ باید همان مبلغی که در بالا ذکر شده واریز نمایید.\n"
            f"🔝 بعد از پرداخت دکمه پرداخت کردم را زده سپس تصویر رسید را ارسال نمایید\n"
            f"💵 بعد از تایید پرداختتون توسط ادمین، سرویس شما تمدید خواهد شد."
        )
        ikb = {
            "inline_keyboard": [
                [{"text": "✅ پرداخت کردم | ارسال رسید.", "callback_data": f"sendreceipt_renew_{plan_id}_{email}"}],
                [{"text": "🔙 بازگشت به پیش‌فاکتور", "callback_data": f"dorenew_{plan_id}_{email}"}]
            ]
        }
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, txt, ikb)
        else:
            send_message(user_id, txt, ikb)
        return

    if data.startswith("sendreceipt_renew_"):
        parts = data.split("_")
        plan_id = int(parts[2])
        email = "_".join(parts[3:])
        plan = get_plan_by_id(plan_id)
        if not plan:
            return
        set_user_state(user_id, "WAIT_RECEIPT_RENEW", {"plan_id": plan_id, "email": email, "price": plan["price"]})
        answer_callback_query(cq_id)
        send_message(user_id, "📌 لطفاً تصویر رسید واریزی (عکس فیش بانکی) خود را ارسال نمایید:", get_cancel_keyboard())
        return

    # My Services Menu
    if data == "my_services":
        clients = get_user_xray_clients(user_id)
        if not clients:
            answer_callback_query(cq_id, "سرویس فعالی ندارید.")
            if msg_id:
                edit_message_text(user_id, msg_id, "⛔️شما هیچ سرویس فعالی ندارید")
            return
        txt = (
            "🛍 <b>اشتراک های خریداری شده توسط شما</b>\n\n"
            "⚠️برای مشاهده اطلاعات و مدیریت روی نام کاربری کلیک کنید:"
        )
        ikb_rows = []
        for cl in clients:
            rem_gb = max(0.0, cl["total_gb"] - cl["used_gb"])
            ikb_rows.append([{"text": f"👤 {cl['email']} ({rem_gb:.1f} GB)", "callback_data": f"srvinfo_{cl['email']}"}])
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, txt, {"inline_keyboard": ikb_rows})
        return

    if data.startswith("srvinfo_"):
        email = data[8:]
        clients = get_user_xray_clients(user_id)
        target = next((c for c in clients if c["email"] == email), None)
        if not target:
            answer_callback_query(cq_id, "سرویس یافت نشد!", show_alert=True)
            return
            
        rem_gb = max(0.0, target["total_gb"] - target["used_gb"])
        st_lbl = "فعال 🟢" if target["enable"] else "غیرفعال 🔴"
        exp_date = datetime.datetime.fromtimestamp(target["expiry_time_ms"]/1000).strftime('%Y-%m-%d %H:%M') if target["expiry_time_ms"] > 0 else "نامحدود"
        
        info_txt = (
            f"🗂 <b>اطلاعات سرویس:</b>\n\n"
            f"👤 <b>نام کاربری:</b> <code>{target['email']}</code>\n"
            f"📊 <b>حجم کل:</b> {target['total_gb']} گیگابایت\n"
            f"🔋 <b>مصرف شده:</b> {target['used_gb']} گیگابایت (باقیمانده: <b>{rem_gb:.2f} گیگابایت</b>)\n"
            f"⏳ <b>تاریخ انقضا:</b> <code>{exp_date}</code>\n"
            f"🚦 <b>وضعیت:</b> {st_lbl}\n\n"
            f"🔗 <b>لینک اتصال / سابسکریپشن:</b>\n"
            f"<code>{target['sub_url']}</code>"
        )
        ikb = {
            "inline_keyboard": [
                [
                    {"text": "♻️ تمدید سرویس", "callback_data": f"renew_{target['email']}"},
                    {"text": "📸 دریافت بارکد QR", "callback_data": f"qr_{target['email']}"}
                ],
                [{"text": "🔙 بازگشت به لیست سرویس‌ها", "callback_data": "my_services"}]
            ]
        }
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, info_txt, ikb)
        else:
            send_message(user_id, info_txt, ikb)
        return

    if data.startswith("qr_"):
        email = data[3:]
        clients = get_user_xray_clients(user_id)
        target = next((c for c in clients if c["email"] == email), None)
        if not target or not target["sub_url"]:
            answer_callback_query(cq_id, "لینک سابسکریپشن موجود نیست.")
            return
        qr_bytes = generate_qr_bytes(target["sub_url"])
        answer_callback_query(cq_id)
        if qr_bytes:
            send_photo(user_id, qr_bytes, caption=f"📸 بارکد QR سرویس <code>{email}</code>")
        return

    # Claim Membership Gift
    if data == "claim_member_gift":
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT claimed_membership_gift FROM users WHERE user_id = ?", (user_id,))
        row = c.fetchone()
        claimed = row["claimed_membership_gift"] if row and "claimed_membership_gift" in row.keys() else 0
        if claimed:
            answer_callback_query(cq_id, "شما قبلاً هدیه عضویت را دریافت کرده‌اید.", show_alert=True)
            conn.close()
            return
            
        gift_amt = 5000
        c.execute("UPDATE users SET balance = balance + ?, claimed_membership_gift = 1 WHERE user_id = ?", (gift_amt, user_id))
        conn.commit()
        conn.close()
        answer_callback_query(cq_id, f"🎉 مبلغ {gift_amt:,} تومان هدیه عضویت به کیف پول شما اضافه شد!", show_alert=True)
        send_message(user_id, f"🎉 <b>هدیه عضویت فعال شد!</b>\n\nمبلغ <b>{gift_amt:,} تومان</b> به کیف پول حساب شما واریز گردید.", get_main_keyboard(user_id))
        return

    # Gift Code Prompt
    if data == "gift_code_prompt":
        set_user_state(user_id, "WAIT_GIFT_CODE")
        answer_callback_query(cq_id)
        send_message(user_id, "🎁 لطفاً کد هدیه یا کد تخفیف خود را ارسال فرمایید:", get_cancel_keyboard())
        return

    # Tutorials (Help)
    if data == "help_menu":
        txt = "📚 <b>آموزش اتصال و دانلود نرم‌افزارها</b>\n\nسیستم عامل مورد نظر خود را جهت دریافت آموزش انتخاب نمایید:"
        ikb = {
            "inline_keyboard": [
                [{"text": "🤖 اندروید (Android)", "callback_data": "help_android"}, {"text": "🍏 آیفون (iOS)", "callback_data": "help_ios"}],
                [{"text": "💻 ویندوز (Windows)", "callback_data": "help_windows"}],
                [{"text": "🔙 بازگشت به منوی اصلی", "callback_data": "menu_home"}]
            ]
        }
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, txt, ikb)
        else:
            send_message(user_id, txt, ikb)
        return

    if data == "help_android":
        txt = (
            "🤖 <b>آموزش اتصال در اندروید (Android):</b>\n\n"
            "۱. نرم‌افزار <b>v2rayNG</b> را از لینک زیر دانلود و نصب کنید:\n"
            "🔗 <a href=\"https://play.google.com/store/apps/details?id=com.v2ray.ang\">دانلود از گوگل پلی</a>\n"
            "🔗 <a href=\"https://github.com/2dust/v2rayNG/releases\">دانلود مستقیم از گیت‌هاب</a>\n\n"
            "۲. لینک اشتراک (سابسکریپشن) دریافتی از ربات را کپی کنید.\n"
            "۳. وارد برنامه v2rayNG شده و از منوی سه خط بالا، گزینه <b>Subscription group setting</b> را انتخاب کنید.\n"
            "۴. علامت + بالا را بزنید و در بخش URL لینک خود را Paste کنید و تیک بالای صفحه را بزنید.\n"
            "۵. به صفحه اصلی برنامه برگردید و از منوی سه نقطه بالا، گزینه <b>Update subscription</b> را لمس کنید تا سرورها بارگذاری شوند.\n"
            "۶. یک سرور را انتخاب کرده و دکمه اتصال دایره‌ای پایین را بزنید. 🟢"
        )
        ikb = {"inline_keyboard": [[{"text": "🔙 بازگشت به بخش آموزش", "callback_data": "help_menu"}]]}
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, txt, ikb)
        return

    if data == "help_ios":
        txt = (
            "🍏 <b>آموزش اتصال در آیفون و آیپد (iOS):</b>\n\n"
            "۱. نرم‌افزار <b>Streisand</b> یا <b>V2Box</b> را از اپ استور نصب کنید:\n"
            "🔗 <a href=\"https://apps.apple.com/app/streisand/id6450534064\">دانلود نرم‌افزار Streisand</a>\n"
            "🔗 <a href=\"https://apps.apple.com/app/v2box-v2ray-client/id6446814043\">دانلود نرم‌افزار V2Box</a>\n\n"
            "۲. لینک سابسکریپشن را کپی کنید.\n"
            "۳. در برنامه Streisand، علامت + بالای صفحه را بزنید و گزینه <b>Add Subscription from Clipboard</b> را انتخاب نمایید.\n"
            "۴. سرورها به‌طور خودکار اضافه می‌شوند؛ یکی را انتخاب کرده و متصل شوید. 🟢"
        )
        ikb = {"inline_keyboard": [[{"text": "🔙 بازگشت به بخش آموزش", "callback_data": "help_menu"}]]}
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, txt, ikb)
        return

    if data == "help_windows":
        txt = (
            "💻 <b>آموزش اتصال در کامپیوتر و ویندوز (Windows):</b>\n\n"
            "۱. نرم‌افزار محبوب <b>v2rayN</b> یا <b>NekoBox</b> را دانلود و استخراج کنید:\n"
            "🔗 <a href=\"https://github.com/2dust/v2rayN/releases\">دانلود نرم‌افزار v2rayN</a>\n\n"
            "۲. لینک سابسکریپشن را کپی کنید.\n"
            "۳. وارد برنامه v2rayN شده و از منوی بالا <b>Subscription Group</b> -> <b>Subscription Group Setting</b> را باز کنید.\n"
            "۴. دکمه Add را زده، در فیلد URL لینک را وارد کرده و Confirm کنید.\n"
            "۵. سپس کلید ترکیبی Ctrl + U را بزنید تا سرورها لود شوند.\n"
            "۶. سرور را انتخاب کرده، اینتر بزنید و حالت سیستم‌پراکسی (System Proxy) را در پایین روی Set System Proxy بگذارید. 🟢"
        )
        ikb = {"inline_keyboard": [[{"text": "🔙 بازگشت به بخش آموزش", "callback_data": "help_menu"}]]}
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, txt, ikb)
        return

    # FAQ View (Mirza Bot 100% Native)
    if data == "faq_view":
        faq_text = (
            "💡 <b>سوالات متداول ⁉️</b>\n\n"
            "1️⃣ <b>فیلترشکن شما آیپی ثابته؟ میتونم برای صرافی های ارز دیجیتال استفاده کنم؟</b>\n"
            "✅ به دلیل وضعیت نت و محدودیت های کشور سرویس ما مناسب ترید نیست و فقط لوکیشن‌ ثابته.\n\n"
            "2️⃣ <b>اگه قبل از منقضی شدن اکانت، تمدیدش کنم روزهای باقی مانده می سوزد؟</b>\n"
            "✅ خیر، روزهای باقیمونده اکانت موقع تمدید حساب میشن و به اکانت اضافه می‌شوند.\n\n"
            "3️⃣ <b>اگه به یک اکانت بیشتر از حد مجاز متصل شیم چه اتفاقی میافته؟</b>\n"
            "✅ در این صورت حجم سرویس شما زودتر تمام خواهد شد.\n\n"
            "4️⃣ <b>فیلترشکن شما از چه نوعیه؟</b>\n"
            "✅ فیلترشکن های ما v2ray است و پروتکل‌های مختلفی رو ساپورت میکنیم تا حتی در زمان اختلال بدون مشکل متصل شوید.\n\n"
            "5️⃣ <b>فیلترشکن از کدوم کشور است؟</b>\n"
            "✅ سرورهای ما از کشورهای آلمان، فنلاند و فرانسه با پینگ بسیار پایین هستند.\n\n"
            "6️⃣ <b>چطور باید از این فیلترشکن استفاده کنم؟</b>\n"
            "✅ برای آموزش استفاده از برنامه، روی دکمه «📚 آموزش» بزنید.\n\n"
            "7️⃣ <b>فیلترشکن وصل نمیشه، چیکار کنم؟</b>\n"
            "✅ به همراه یک عکس از پیغام خطایی که میگیرید به پشتیبانی مراجعه کنید.\n\n"
            "💡 در صورتی که جواب سوالتون رو نگرفتید میتونید به پشتیبانی پیام دهید."
        )
        ikb = {"inline_keyboard": [[{"text": "🔙 بازگشت به پشتیبانی", "callback_data": "support_menu"}]]}
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, faq_text, ikb)
        return

    if data == "support_menu":
        answer_callback_query(cq_id)
        sup = get_setting("support_username", "")
        cfg = load_config()
        if not sup:
            sup = cfg.get("support_username", "")
        sup_clean = sup.lstrip("@") if sup else ""
        sup_msg = (
            "☎️ <b>پشتیبانی:</b>\n\n"
            "در صورتی که سوال، ابهام یا مشکلی در سرویس دارید، می‌توانید با پشتیبانی ما در ارتباط باشید:\n"
        )
        if sup_clean:
            sup_msg += f"👉 @{sup_clean}\n\n"
        sup_msg += "💡 همچنین می‌توانید قبل از ارتباط، سوالات متداول را مطالعه فرمایید یا پیام خود را همین‌جا ارسال کنید:"
        ikb = {
            "inline_keyboard": [
                [{"text": "❓ سوالات متداول", "callback_data": "faq_view"}],
                [{"text": "🎟 ارسال پیام به پشتیبانی", "callback_data": "send_ticket"}],
                [{"text": "🔙 بازگشت به منوی اصلی", "callback_data": "menu_home"}]
            ]
        }
        if msg_id:
            edit_message_text(user_id, msg_id, sup_msg, ikb)
        return

    if data == "send_ticket":
        set_user_state(user_id, "WAIT_SUPPORT_MSG")
        answer_callback_query(cq_id)
        send_message(user_id, "📌 لطفاً پیام خود را ارسال فرمایید تا برای مدیریت ارسال شود:", get_cancel_keyboard())
        return

    if data.startswith("reply_user_"):
        target_uid = int(data.split("_")[2])
        set_user_state(user_id, "WAIT_ADMIN_REPLY", {"target_uid": target_uid})
        answer_callback_query(cq_id)
        send_message(user_id, f"✍️ لطفاً متن پاسخ به کاربر <code>{target_uid}</code> را ارسال فرمایید:", get_cancel_keyboard())
        return

# -------------------------------------------------------------
# Main Message Handler (Mirza Bot 100% Native)
# -------------------------------------------------------------
def handle_message(msg):
    from_user = msg.get("from", {})
    user_id = from_user.get("id")
    if not user_id:
        return
        
    username = from_user.get("username", "")
    first_name = from_user.get("first_name", "")
    text = msg.get("text", "")
    photo = msg.get("photo")
    admin_id = get_admin_chat_id()
    
    update_user_profile(user_id, username, first_name)
    user_obj = get_user(user_id)
    if user_obj and user_obj.get("is_banned"):
        send_message(user_id, "⛔️ حساب کاربری شما توسط مدیریت مسدود شده است.")
        return
        
    state_info = get_user_state(user_id)
    cur_state = state_info["state"]
    sdata = state_info["data"]
    
    # Cancel handler
    if text in ["/cancel", "🔙 انصراف و بازگشت به منو", "🔙 انصراف"]:
        clear_user_state(user_id)
        send_message(user_id, "عملیات لغو شد. به منوی اصلی بازگشتید.", get_main_keyboard(user_id))
        return

    # Check Channel Membership before continuing
    if not check_channel_membership(user_id) and user_id != admin_id:
        ch = get_setting("force_join_channel", "")
        if ch:
            ikb = {
                "inline_keyboard": [
                    [{"text": "📢 عضویت در کانال رسمی", "url": f"https://t.me/{ch.lstrip('@')}"}],
                    [{"text": "✅ بررسی مجدد عضویت", "callback_data": "check_join"}]
                ]
            }
            send_message(user_id, f"⚠️ <b>کاربر گرامی!</b>\nبرای استفاده از امکانات ربات، ابتدا باید در کانال ما عضو شوید:\n{ch}", ikb)
            return

    # Process /start and referral code
    if text.startswith("/start"):
        clear_user_state(user_id)
        parts = text.split()
        if len(parts) > 1:
            raw_ref = parts[1]
            if raw_ref.startswith("ref_"):
                raw_ref = raw_ref[4:]
            try:
                ref_id = int(raw_ref)
                if ref_id != user_id:
                    conn = get_db()
                    c = conn.cursor()
                    c.execute("INSERT OR IGNORE INTO referrals (user_id, referrer_id, created_at) VALUES (?, ?, ?)", (user_id, ref_id, int(time.time())))
                    conn.commit()
                    conn.close()
            except Exception:
                pass
                
        welcome_txt = (
            f"سلام <b>{first_name or 'کاربر'}</b> عزیز به ربات خوش آمدید 🌹\n\n"
            f"🚀 ارائه‌دهنده پرسرعت‌ترین و پایدارترین سرورهای V2Ray و Sing-box\n"
            f"🛡 اتصال تضمینی و ضد فیلتر بر روی تمامی اپراتورها\n"
            f"⚡️ تحویل آنی و خودکار اشتراک پس از پرداخت\n\n"
            f"💡 جهت شروع، از منوی زیر گزینه مورد نظر را انتخاب فرمایید:"
        )
        send_message(user_id, welcome_txt, get_main_keyboard(user_id))
        return

    # Admin Panel command
    if text in ["/admin", "/panel", "⚙️ پنل مدیریت", "⚙️ پنل مدیریت ربات", "⚙️ مدیریت"]:
        if user_id != admin_id:
            send_message(user_id, "دسترسی غیرمجاز!")
            return
        clear_user_state(user_id)
        send_message(user_id, "⚙️ <b>به پنل مدیریت ربات خوش آمدید:</b>\nجهت مدیریت، بخش مورد نظر را انتخاب نمایید:", get_admin_inline_keyboard())
        return

    # Handle State: WAIT_BCAST
    if cur_state == "WAIT_BCAST" and user_id == admin_id:
        clear_user_state(user_id)
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT user_id FROM users")
        all_u = [r[0] for r in c.fetchall()]
        conn.close()
        send_message(user_id, f"🚀 ارسال همگانی به {len(all_u)} کاربر آغاز شد...", get_main_keyboard(user_id))
        succ = 0
        for uid in all_u:
            res = send_message(uid, text)
            if res and res.get("ok"):
                succ += 1
            time.sleep(0.04)
        send_message(user_id, f"✅ پیام همگانی با موفقیت برای {succ} نفر ارسال گردید.")
        return

    # Handle State: WAIT_FWD
    if cur_state == "WAIT_FWD" and user_id == admin_id:
        clear_user_state(user_id)
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT user_id FROM users")
        all_u = [r[0] for r in c.fetchall()]
        conn.close()
        send_message(user_id, f"🚀 فوروارد همگانی به {len(all_u)} کاربر آغاز شد...", get_main_keyboard(user_id))
        succ = 0
        for uid in all_u:
            res = forward_message(uid, msg["chat"]["id"], msg["message_id"])
            if res and res.get("ok"):
                succ += 1
            time.sleep(0.04)
        send_message(user_id, f"✅ فوروارد همگانی با موفقیت برای {succ} نفر انجام شد.")
        return

    # Handle State: WAIT_USER_SEARCH
    if cur_state == "WAIT_USER_SEARCH" and user_id == admin_id:
        clear_user_state(user_id)
        target_uid = None
        try:
            target_uid = int(text.strip())
        except Exception:
            pass
        conn = get_db()
        c = conn.cursor()
        if target_uid:
            c.execute("SELECT * FROM users WHERE user_id = ?", (target_uid,))
        else:
            uname = text.strip().lstrip("@")
            c.execute("SELECT * FROM users WHERE username = ?", (uname,))
        u_info = c.fetchone()
        conn.close()
        
        if not u_info:
            send_message(user_id, "کاربر یافت نشد!", get_main_keyboard(user_id))
            return
            
        t_id = u_info["user_id"]
        t_uname = u_info["username"]
        t_name = u_info["first_name"]
        t_bal = u_info["balance"]
        
        clients = get_user_xray_clients(t_id)
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT is_banned FROM users WHERE user_id = ?", (t_id,))
        ub_row = c.fetchone()
        conn.close()
        is_b = ub_row["is_banned"] if ub_row else 0
        b_lbl = "مسدود ⛔️" if is_b else "فعال 🟢"
        
        u_txt = (
            f"👤 <b>اطلاعات کاربر:</b>\n\n"
            f"▫️ شناسه عددی: <code>{t_id}</code>\n"
            f"▫️ یوزرنیم: @{t_uname}\n"
            f"▫️ نام: {t_name}\n"
            f"▫️ وضعیت حساب: <b>{b_lbl}</b>\n"
            f"💰 موجودی کیف پول: <b>{t_bal:,} تومان</b>\n"
            f"🌐 تعداد سرویس‌های فعال: {len(clients)}\n"
        )
        ikb_user = {
            "inline_keyboard": [
                [
                    {"text": "➕ افزایش موجودی", "callback_data": f"adm_user_add_{t_id}"},
                    {"text": "➖ کسر موجودی", "callback_data": f"adm_user_sub_{t_id}"}
                ],
                [
                    {"text": "مسدود / رفع مسدودی", "callback_data": f"adm_user_ban_{t_id}"}
                ],
                [
                    {"text": "🔙 بازگشت به پنل مدیریت", "callback_data": "adm_stats"}
                ]
            ]
        }
        send_message(user_id, u_txt, ikb_user)
        return

    # Handle State: WAIT_USER_ADD_BAL
    if cur_state == "WAIT_USER_ADD_BAL" and user_id == admin_id:
        clear_user_state(user_id)
        target_uid = sdata.get("target_uid")
        try:
            amt = int(text.strip().replace(",", ""))
            new_bal = change_user_balance(target_uid, amt)
            send_message(user_id, f"✅ مبلغ {amt:,} تومان به کیف پول کاربر <code>{target_uid}</code> اضافه شد.\nموجودی جدید: <b>{new_bal:,} تومان</b>", get_main_keyboard(user_id))
            send_message(target_uid, f"🎉 <b>کیف پول شما توسط مدیریت شارژ شد!</b>\n\n💰 مبلغ: <b>{amt:,} تومان</b>\n💵 موجودی فعلی: <b>{new_bal:,} تومان</b>", get_main_keyboard(target_uid))
        except Exception as e:
            send_message(user_id, f"❌ خطا: {e}", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_USER_SUB_BAL
    if cur_state == "WAIT_USER_SUB_BAL" and user_id == admin_id:
        clear_user_state(user_id)
        target_uid = sdata.get("target_uid")
        try:
            amt = int(text.strip().replace(",", ""))
            new_bal = change_user_balance(target_uid, -amt)
            send_message(user_id, f"✅ مبلغ {amt:,} تومان از کیف پول کاربر <code>{target_uid}</code> کسر شد.\nموجودی جدید: <b>{new_bal:,} تومان</b>", get_main_keyboard(user_id))
        except Exception as e:
            send_message(user_id, f"❌ خطا: {e}", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_CUSTOM_TOPUP
    if cur_state == "WAIT_CUSTOM_TOPUP":
        try:
            amt = int(text.strip().replace(",", ""))
            if amt < 10000:
                send_message(user_id, "حداقل مبلغ شارژ ۱۰,۰۰۰ تومان است. لطفاً مبلغ دیگری ارسال فرمایید:")
                return
            cfg = load_config()
            card_num = cfg.get("card_number", "۶۰۳۷-۹۹۷۴-XXXX-XXXX")
            card_holder = cfg.get("card_holder", "DARK VVPN")
            txt = (
                f"برای افزایش موجودی، مبلغ <code>{amt:,}</code> تومان را به شماره‌ی حساب زیر واریز کنید 👇🏻\n\n"
                f"====================\n"
                f"<code>{card_num}</code>\n"
                f"{card_holder}\n"
                f"====================\n\n"
                f"❌ این تراکنش به مدت یک ساعت اعتبار دارد پس از آن امکان پرداخت این تراکنش امکان ندارد.\n"
                f"‼️ مبلغ باید همان مبلغی که در بالا ذکر شده واریز نمایید.\n"
                f"‼️ امکان برداشت وجه از کیف پول نیست.\n"
                f"‼️ مسئولیت واریز اشتباهی با شماست.\n"
                f"🔝 بعد از پرداخت دکمه پرداخت کردم را زده سپس تصویر رسید را ارسال نمایید\n"
                f"💵 بعد از تایید پرداختتون توسط ادمین کیف پول شما شارژ خواهد شد"
            )
            ikb = {
                "inline_keyboard": [
                    [{"text": "✅ پرداخت کردم | ارسال رسید.", "callback_data": f"sendreceipt_topup_{amt}"}],
                    [{"text": "🔙 بازگشت به منوی قبل", "callback_data": "topup_menu"}]
                ]
            }
            clear_user_state(user_id)
            send_message(user_id, txt, ikb)
        except Exception:
            send_message(user_id, "لطفاً مبلغ را به عدد انگلیسی ارسال فرمایید (مثلاً 50000):")
        return

    # Handle State: WAIT_NEW_PLAN
    if cur_state == "WAIT_NEW_PLAN" and user_id == admin_id:
        clear_user_state(user_id)
        parts = text.strip().split("|")
        if len(parts) != 4:
            send_message(user_id, "فرمت نامعتبر است! عملیات لغو شد.", get_main_keyboard(user_id))
            return
        try:
            title = parts[0].strip()
            traffic_gb = float(parts[1].strip())
            days = int(parts[2].strip())
            price = int(parts[3].strip())
            plans = load_plans()
            new_id = max([p["id"] for p in plans], default=0) + 1
            plans.append({"id": new_id, "title": title, "traffic_gb": traffic_gb, "days": days, "price": price, "enabled": True})
            save_plans(plans)
            send_message(user_id, f"✅ پلن جدید <b>{title}</b> با موفقیت اضافه شد.", get_main_keyboard(user_id))
        except Exception as e:
            send_message(user_id, f"خطا در ایجاد پلن: {e}", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_SET_CARD
    if cur_state == "WAIT_SET_CARD" and user_id == admin_id:
        card_num = text.strip()
        set_user_state(user_id, "WAIT_SET_HOLDER", {"card_number": card_num})
        send_message(user_id, "اکنون نام صاحب حساب (دارنده کارت) را ارسال فرمایید:", get_cancel_keyboard())
        return

    if cur_state == "WAIT_SET_HOLDER" and user_id == admin_id:
        clear_user_state(user_id)
        card_holder = text.strip()
        card_num = sdata.get("card_number")
        cfg = load_config()
        cfg["card_number"] = card_num
        cfg["card_holder"] = card_holder
        with open(CONFIG_PATH, "w", encoding="utf-8") as f:
            json.dump(cfg, f, ensure_ascii=False, indent=2)
        send_message(user_id, f"✅ اطلاعات کارت با موفقیت به‌روزرسانی شد:\n\nکارت: <code>{card_num}</code>\nبه نام: <b>{card_holder}</b>", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_ADD_DISC
    if cur_state == "WAIT_ADD_DISC" and user_id == admin_id:
        clear_user_state(user_id)
        parts = text.strip().split("|")
        if len(parts) != 3:
            send_message(user_id, "فرمت نامعتبر است! عملیات لغو شد.", get_main_keyboard(user_id))
            return
        try:
            code = parts[0].strip().upper()
            pct = int(parts[1].strip())
            max_u = int(parts[2].strip())
            conn = get_db()
            c = conn.cursor()
            c.execute("INSERT OR REPLACE INTO discount_codes (code, percent, max_uses, used_count) VALUES (?, ?, ?, 0)", (code, pct, max_u))
            conn.commit()
            conn.close()
            send_message(user_id, f"✅ کد تخفیف <code>{code}</code> با {pct}٪ تخفیف ایجاد شد.", get_main_keyboard(user_id))
        except Exception as e:
            send_message(user_id, f"خطا: {e}", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_SET_CHANNEL
    if cur_state == "WAIT_SET_CHANNEL" and user_id == admin_id:
        clear_user_state(user_id)
        ch = text.strip()
        if not ch.startswith("@"):
            ch = "@" + ch
        set_setting("force_join_channel", ch)
        set_setting("force_join_enabled", "1")
        send_message(user_id, f"✅ کانال <code>{ch}</code> با موفقیت تنظیم و قفل اجباری فعال گردید.", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_SET_TRIAL
    if cur_state == "WAIT_SET_TRIAL" and user_id == admin_id:
        clear_user_state(user_id)
        parts = text.strip().split("|")
        if len(parts) == 2:
            try:
                set_setting("trial_gb", str(float(parts[0].strip())))
                set_setting("trial_days", str(float(parts[1].strip())))
                send_message(user_id, f"✅ تنظیمات اکانت تست به‌روزرسانی شد: {parts[0]} گیگابایت / {parts[1]} روز", get_main_keyboard(user_id))
                return
            except Exception:
                pass
        send_message(user_id, "فرمت نامعتبر است.", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_SET_REF_PCT
    if cur_state == "WAIT_SET_REF_PCT" and user_id == admin_id:
        clear_user_state(user_id)
        try:
            pct = int(text.strip())
            set_setting("referral_percent", str(pct))
            send_message(user_id, f"✅ پورسانت زیرمجموعه‌گیری به {pct}٪ تنظیم شد.", get_main_keyboard(user_id))
        except Exception:
            send_message(user_id, "لطفاً عدد لاتین ارسال فرمایید.", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_SUPPORT_MSG
    if cur_state == "WAIT_SUPPORT_MSG":
        clear_user_state(user_id)
        send_message(user_id, "✅ پیام شما با موفقیت ارسال شد و پس از بررسی به شما پاسخ داده خواهد شد.", get_main_keyboard(user_id))
        uname = f"@{username}" if username else str(user_id)
        admin_t = (
            f"📩 <b>پیام جدید از پشتیبانی:</b>\n\n"
            f"👤 کاربر: {uname} (<code>{user_id}</code>)\n"
            f"📝 متن پیام:\n{text}"
        )
        ikb_reply = {
            "inline_keyboard": [
                [{"text": "✍️ پاسخ به کاربر", "callback_data": f"reply_user_{user_id}"}]
            ]
        }
        send_message(admin_id, admin_t, ikb_reply)
        return

    # Handle State: WAIT_ADMIN_REPLY
    if cur_state == "WAIT_ADMIN_REPLY" and user_id == admin_id:
        clear_user_state(user_id)
        target_uid = sdata.get("target_uid")
        ans_text = (
            f"📩 <b>یک پیام از سمت مدیریت برای شما ارسال گردید:</b>\n\n"
            f"{text}"
        )
        send_message(target_uid, ans_text, get_main_keyboard(target_uid))
        send_message(user_id, f"✅ پاسخ با موفقیت برای کاربر <code>{target_uid}</code> ارسال شد.", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_GIFT_CODE
    if cur_state == "WAIT_GIFT_CODE":
        code_input = text.strip().upper()
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT * FROM discount_codes WHERE code = ?", (code_input,))
        disc = c.fetchone()
        if not disc or disc["used_count"] >= disc["max_uses"]:
            conn.close()
            send_message(user_id, "❌ کد هدیه نامعتبر است یا ظرفیت استفاده از آن به پایان رسیده است.", get_main_keyboard(user_id))
            clear_user_state(user_id)
            return
            
        c.execute("UPDATE discount_codes SET used_count = used_count + 1 WHERE code = ?", (code_input,))
        conn.commit()
        conn.close()
        clear_user_state(user_id)
        
        reward = disc["percent"] * 1000
        new_bal = change_user_balance(user_id, reward)
        send_message(user_id, f"🎉 <b>کد هدیه با موفقیت فعال شد!</b>\n\nمبلغ <b>{reward:,} تومان</b> به کیف پول شما اضافه گردید.\nموجودی فعلی: <b>{new_bal:,} تومان</b>", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_DISCOUNT_CODE
    if cur_state == "WAIT_DISCOUNT_CODE":
        code_input = text.strip().upper()
        plan_id = sdata.get("plan_id")
        plan = get_plan_by_id(plan_id)
        if not plan:
            clear_user_state(user_id)
            send_message(user_id, "پلن نامعتبر است.", get_main_keyboard(user_id))
            return
            
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT * FROM discount_codes WHERE code = ?", (code_input,))
        disc = c.fetchone()
        conn.close()
        
        if not disc or disc["used_count"] >= disc["max_uses"]:
            send_message(user_id, "❌ کد تخفیف نامعتبر یا منقضی شده است. بدون تخفیف ادامه می‌دهیم:", get_main_keyboard(user_id))
            clear_user_state(user_id)
            # Resend normal plan invoice
            send_message(user_id, f"مجدداً روی پلن کلیک کنید:")
            return
            
        pct = disc["percent"]
        orig_price = plan["price"]
        discounted_price = int(orig_price * ((100 - pct) / 100.0))
        clear_user_state(user_id)
        
        u = get_user(user_id)
        bal = u["balance"] if u else 0
        uname_display = f"@{username}" if username else str(user_id)
        
        txt = (
            f"📇 <b>پیش فاکتور شما (با کد تخفیف {code_input}):</b>\n"
            f"👤 <b>نام کاربری:</b> <code>{uname_display}</code>\n"
            f"🔐 <b>نام سرویس:</b> {plan['title']}\n"
            f"📆 <b>مدت اعتبار:</b> {plan['days']} روز\n"
            f"👥 <b>حجم اکانت:</b> {plan['traffic_gb']} گیگ\n"
            f"💶 <b>قیمت اصلی:</b> <s>{orig_price:,}</s> تومان\n"
            f"🎉 <b>قیمت با {pct}٪ تخفیف:</b> <b>{discounted_price:,} تومان</b>\n"
            f"💵 <b>موجودی کیف پول شما :</b> {bal:,} تومان\n\n"
            f"💰 <b>سفارش شما آماده پرداخت است.</b>"
        )
        ikb_rows = []
        if bal >= discounted_price:
            ikb_rows.append([{"text": "⚡️ پرداخت آنی از کیف پول", "callback_data": f"buy_wallet_{plan_id}"}])
        ikb_rows.append([{"text": "💳 پرداخت کارت به کارت", "callback_data": f"buy_card_{plan_id}"}])
        ikb_rows.append([{"text": "🔙 بازگشت به لیست پلن‌ها", "callback_data": "list_plans"}])
        send_message(user_id, txt, {"inline_keyboard": ikb_rows})
        return

    # Handle Photo Upload for Receipts (WAIT_RECEIPT_PLAN / WAIT_RECEIPT_WALLET / WAIT_RECEIPT_RENEW)
    if photo and cur_state in ["WAIT_RECEIPT_PLAN", "WAIT_RECEIPT_WALLET", "WAIT_RECEIPT_RENEW"]:
        file_id = photo[-1]["file_id"]
        clear_user_state(user_id)

        order_type = "PLAN"
        plan_id = sdata.get("plan_id", 0)
        price = sdata.get("price", 0)
        target_email = ""
        plan_title = "خرید اشتراک"

        if cur_state == "WAIT_RECEIPT_WALLET":
            order_type = "WALLET_TOPUP"
            price = sdata.get("amount", 0)
            plan_title = f"افزایش موجودی کیف پول ({price:,} تومان)"
        elif cur_state == "WAIT_RECEIPT_RENEW":
            order_type = "RENEW"
            target_email = sdata.get("email", "")
            plan = get_plan_by_id(plan_id)
            plan_title = f"تمدید سرویس {target_email} ({plan['title'] if plan else ''})"
        else:
            plan = get_plan_by_id(plan_id)
            if plan:
                plan_title = plan["title"]

        conn = get_db()
        c = conn.cursor()
        c.execute("""
        INSERT INTO orders (user_id, username, order_type, plan_id, plan_title, price_tomans, final_price, target_email, receipt_file_id, status, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'PENDING', ?)
        """, (user_id, username, order_type, plan_id, plan_title, price, price, target_email, file_id, int(time.time())))
        order_id = c.lastrowid
        conn.commit()
        conn.close()

        # Send acknowledgment to user
        ack_txt = (
            f"✅ <b>فیش واریزی شما با موفقیت ثبت شد!</b>\n\n"
            f"🧾 <b>شماره پیگیری سفارش:</b> #{order_id}\n"
            f"💰 <b>مبلغ:</b> {price:,} تومان\n\n"
            f"فیش شما برای ادمین ارسال گردید. به محض تایید، کانفیگ شما به صورت خودکار صادر و ارسال خواهد شد."
        )
        send_message(user_id, ack_txt, get_main_keyboard(user_id))

        # Forward Receipt to Admin with Inline Approval Buttons
        admin_cap = (
            f"🔔 <b>فیش واریزی جدید دریافت شد!</b>\n\n"
            f"🧾 <b>سفارش:</b> #{order_id}\n"
            f"👤 <b>کاربر:</b> @{username} (<code>{user_id}</code>)\n"
            f"📦 <b>نوع سفارش:</b> {plan_title}\n"
            f"💵 <b>مبلغ:</b> <b>{price:,} تومان</b>\n"
        )
        if target_email:
            admin_cap += f"🎯 <b>کلاینت هدف:</b> <code>{target_email}</code>\n"
            
        ikb_admin = {
            "inline_keyboard": [
                [
                    {"text": "✅ تایید و صدور آنی", "callback_data": f"approve_{order_id}"},
                    {"text": "❌ رد فیش", "callback_data": f"reject_{order_id}"}
                ]
            ]
        }
        send_photo(admin_id, file_id, caption=admin_cap, reply_markup=ikb_admin)
        return

    # User Keyboards (Mirza Bot 100% Native UX)
    if text in ["🔐 خرید اشتراک", "🛒 خرید اشتراک", "/buy"]:
        clear_user_state(user_id)
        plans = load_plans()
        ikb_rows = []
        for p in plans:
            ikb_rows.append([{"text": f"🛒 {p['title']} — {p['price']:,} تومان", "callback_data": f"plan_{p['id']}"}])
        send_message(user_id, "🛍️ لطفاً سرویسی که می‌خواهید خریداری کنید را انتخاب کنید!", {"inline_keyboard": ikb_rows})
        return

    if text in ["🔑 اکانت تست", "🎁 اکانت تست", "🎁 تست رایگان ۲۴ ساعته", "/test"]:
        clear_user_state(user_id)
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT * FROM trials WHERE user_id = ?", (user_id,))
        already = c.fetchone()
        conn.close()
        if already:
            send_message(user_id, "⚠️ محدودیت ساخت اشتراک تست شما به پایان رسید.", get_main_keyboard(user_id))
            return

        trial_gb = float(get_setting("trial_gb", "1"))
        trial_days = float(get_setting("trial_days", "1"))
        hours_str = f"{int(trial_days * 24)}"
        vol_str = f"{int(trial_gb * 1024)}" if trial_gb < 1 else f"{int(trial_gb * 1024)}"

        send_message(user_id, "♻️ در حال ساختن سرویس شما...")
        res = add_xray_client(user_id, username, traffic_gb=trial_gb, expiry_days=trial_days, is_trial=True)
        if not res:
            send_message(user_id, "❌ خطایی در ساخت اشتراک رخ داده است برای رفع مشکل با پشتیبانی در ارتباط باشد.", get_main_keyboard(user_id))
            return

        conn = get_db()
        c = conn.cursor()
        c.execute("INSERT OR REPLACE INTO trials (user_id, username, sub_id, created_at) VALUES (?, ?, ?, ?)", (user_id, username, res["sub_id"], int(time.time())))
        conn.commit()
        conn.close()

        t_msg = (
            f"✅ سرویس با موفقیت ایجاد شد\n\n"
            f"👤 نام کاربری سرویس : <code>{res['client_email']}</code>\n"
            f"🌿 نام سرویس: تست رایگان\n"
            f"‏🇺🇳 لوکیشن: آلمان 🇩🇪\n"
            f"⏳ مدت زمان: {hours_str}  ساعت\n"
            f"🗜 حجم سرویس:  {vol_str} مگابایت\n\n"
            f"لینک اتصال:\n"
            f"<code>{res['sub_url']}</code>\n\n"
            f"⚡️ کانفیگ تک‌خطی:\n"
            f"<code>{res['vless_link']}</code>\n\n"
            f"🧑‍🦯 شما میتوانید شیوه اتصال را با فشردن دکمه زیر و انتخاب سیستم عامل خود را دریافت کنید"
        )
        ikb = {
            "inline_keyboard": [
                [{"text": "📚 مشاهده آموزش استفاده", "callback_data": "help_menu"}]
            ]
        }
        send_message(user_id, t_msg, ikb)
        qr_bytes = generate_qr_bytes(res["sub_url"])
        if qr_bytes:
            send_photo(user_id, qr_bytes, caption="📸 اسکن بارکد QR جهت اتصال مستقیم")
        return

    if text in ["🛍 سرویس های من", "📊 سرویس‌های من", "/services"]:
        clear_user_state(user_id)
        clients = get_user_xray_clients(user_id)
        if not clients:
            send_message(user_id, "⛔️شما هیچ سرویس فعالی ندارید", get_main_keyboard(user_id))
            return
            
        txt = (
            "🛍 <b>اشتراک های خریداری شده توسط شما</b>\n\n"
            "⚠️برای مشاهده اطلاعات و مدیریت روی نام کاربری کلیک کنید:"
        )
        ikb_rows = []
        for cl in clients:
            rem_gb = max(0.0, cl["total_gb"] - cl["used_gb"])
            ikb_rows.append([{"text": f"👤 {cl['email']} ({rem_gb:.1f} GB)", "callback_data": f"srvinfo_{cl['email']}"}])
            
        send_message(user_id, txt, {"inline_keyboard": ikb_rows})
        return

    if text in ["♻️ تمدید سرویس", "🔄 تمدید اشتراک", "🔄 تمدید سرویس", "/renew", "/extend"]:
        clear_user_state(user_id)
        clients = get_user_xray_clients(user_id)
        if not clients:
            send_message(user_id, "⛔️شما هیچ سرویس فعالی برای تمدید ندارید", get_main_keyboard(user_id))
            return
        ikb_rows = []
        for cl in clients:
            ikb_rows.append([{"text": f"🔄 {cl['email']}", "callback_data": f"renew_{cl['email']}"}])
        send_message(user_id, "سرویسی که قصد تمدید آن را دارید انتخاب کنید:", {"inline_keyboard": ikb_rows})
        return

    if text in ["🏦 کیف پول + شارژ", "💰 کیف پول من", "/wallet"]:
        clear_user_state(user_id)
        u = get_user(user_id)
        bal = u["balance"] if u else 0
        
        # Shamsi dates
        reg_ts = u["created_at"] if (u and "created_at" in u.keys() and u["created_at"]) else int(time.time())
        reg_date, reg_time = format_timestamp_jalali(reg_ts)
        now_date, now_time = format_timestamp_jalali(int(time.time()))
        
        # Count user orders & referrals
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT COUNT(*) FROM orders WHERE user_id = ? AND status = 'APPROVED'", (user_id,))
        orders_count = c.fetchone()[0]
        c.execute("SELECT COUNT(*) FROM referrals WHERE referrer_id = ?", (user_id,))
        ref_count = c.fetchone()[0]
        conn.close()
        
        w_txt = (
            f"🗂 <b>اطلاعات حساب کاربری شما :</b>\n\n\n"
            f"🪪 شناسه کاربری: <code>{user_id}</code>\n"
            f"👤 نام: <code>{first_name or 'کاربر'}</code>\n"
            f"👥 کد معرف شما : <code>{user_id}</code>\n"
            f"📱 شماره تماس : ثبت نشده\n"
            f"⌚️زمان ثبت نام : {reg_date} {reg_time}\n"
            f"💰 موجودی: <b>{bal:,} تومان</b>\n"
            f"🛒 تعداد سرویس های خریداری شده : {orders_count} عدد\n"
            f"📑 تعداد فاکتور های پرداخت شده : {orders_count} عدد\n"
            f"🤝 تعداد زیر مجموعه های شما : {ref_count} نفر\n"
            f"🔖 گروه کاربری : کاربر عادی\n\n"
            f"📆 {now_date} → ⏰ {now_time}"
        )
        ikb = {
            "inline_keyboard": [
                [{"text": "💰 افزایش موجودی", "callback_data": "topup_menu"}, {"text": "🎁 کد هدیه", "callback_data": "gift_code_prompt"}],
                [{"text": "🔙 بازگشت به منوی اصلی", "callback_data": "menu_home"}]
            ]
        }
        send_message(user_id, w_txt, ikb)
        return

    if text in ["👥 زیر مجموعه گیری", "👥 زیرمجموعه‌گیری", "/affiliates"]:
        clear_user_state(user_id)
        bot_info = tg_request("getMe")
        bot_uname = bot_info.get("result", {}).get("username", "bot") if bot_info else "bot"
        ref_link = f"https://t.me/{bot_uname}?start={user_id}"
        
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT COUNT(*) FROM referrals WHERE referrer_id = ?", (user_id,))
        ref_count = c.fetchone()[0]
        c.execute("SELECT COUNT(*) FROM orders WHERE user_id = ? AND status = 'APPROVED'", (user_id,))
        orders_count = c.fetchone()[0]
        c.execute("SELECT SUM(amount) FROM referral_rewards WHERE referrer_id = ?", (user_id,))
        rew_total = c.fetchone()[0] or 0
        conn.close()
        
        pct = get_setting("referral_percent", "10")
        ref_msg = (
            f"<b>💼 زیرمجموعه‌گیری و هدیه خوش‌آمد</b>\n\n"
            f"با دعوت دوستان از طریق <b>لینک اختصاصی</b>، بدون پرداخت حتی ۱ ریال کیف پولت شارژ میشه و از خدمات ربات استفاده می‌کنی!\n\n"
            f"<b>💸 پورسانت خرید:</b>\n"
            f"• {pct} درصد از مبلغ خرید زیرمجموعه به شما تعلق می‌گیره\n\n"
            f"<b>📊 آمار شما:</b>\n"
            f"• 👥 زیرمجموعه‌ها: {ref_count} نفر\n"
            f"• 🛒 خریدها: {orders_count} عدد\n"
            f"• 💵 مجموع پاداش: {rew_total:,} تومان\n\n"
            f"🔗 لینک ریفرال جهت احراز زیر مجموعه :\n"
            f"<code>{ref_link}</code>\n\n"
            f"<b>📢 دعوت کن، هدیه بگیر، رشد کن!</b>"
        )
        ikb = {
            "inline_keyboard": [
                [{"text": "🎁 دریافت هدیه عضویت", "callback_data": "claim_member_gift"}],
                [{"text": "🔙 بازگشت به منوی اصلی", "callback_data": "menu_home"}]
            ]
        }
        send_message(user_id, ref_msg, ikb)
        return

    if text in ["💵 تعرفه اشتراک ها", "💵 تعرفه‌ها", "/tariffs"]:
        clear_user_state(user_id)
        plans = load_plans()
        txt = "💵 <b>تعرفه اشتراک‌های پرسرعت:</b>\n\n"
        for i, p in enumerate(plans, 1):
            txt += f"<b>{i}️⃣ {p['title']}</b>\n"
            txt += f"▫️ حجم ترافیک: <b>{p['traffic_gb']} گیگابایت</b>\n"
            txt += f"▫️ مدت اعتبار: <b>{p['days']} روز</b>\n"
            txt += f"▫️ قیمت: <b>{p['price']:,} تومان</b>\n\n"
        txt += (
            "⚡️ سرورهای پرسرعت با آپ‌تایم ۹۹.۹٪\n"
            "🛡 بدون قطعی و سازگار با تمامی اپراتورها\n"
            "🚀 تحویل آنی پس از پرداخت\n\n"
            "💡 برای سفارش هر پلن، روی دکمه «🔐 خرید اشتراک» کلیک فرمایید."
        )
        ikb = {"inline_keyboard": [[{"text": "🔐 خرید اشتراک", "callback_data": "list_plans"}]]}
        send_message(user_id, txt, ikb)
        return

    if text in ["🎲 گردونه شانس", "/wheel", "/luck"]:
        clear_user_state(user_id)
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT last_spun_at FROM wheel_luck WHERE user_id = ?", (user_id,))
        row = c.fetchone()
        now = int(time.time())
        if row and (now - row["last_spun_at"]) < 86400:
            conn.close()
            send_message(user_id, "❌ شما امروز شرکت کردید فردا مجددا  شانس خود را امتحان کنید", get_main_keyboard(user_id))
            return
            
        c.execute("INSERT OR REPLACE INTO wheel_luck (user_id, last_spun_at) VALUES (?, ?)", (user_id, now))
        conn.commit()
        conn.close()
        
        # Native Telegram animated dice!
        dice_res = send_dice(user_id, "🎲")
        dice_val = 1
        if dice_res and dice_res.get("ok"):
            dice_val = dice_res.get("result", {}).get("dice", {}).get("value", 1)
            
        time.sleep(2.5) # Wait for Telegram dice animation
        
        if dice_val >= 4:
            prize_amount = 10000
            change_user_balance(user_id, prize_amount)
            msg = f"🤩 تبریک شما برنده شدید . مبلغ {prize_amount:,} تومان به حساب کاربری شما اضافه گردید."
        else:
            msg = "🥲متاسفانه شما برنده نشدید روز دیگر مجددا تست کنید"
            
        send_message(user_id, msg, get_main_keyboard(user_id))
        return

    if text in ["📚 آموزش", "📱 دانلود برنامه‌ها", "/help"]:
        clear_user_state(user_id)
        txt = "📚 <b>آموزش اتصال و دانلود نرم‌افزارها</b>\n\nسیستم عامل مورد نظر خود را جهت دریافت آموزش انتخاب نمایید:"
        ikb = {
            "inline_keyboard": [
                [{"text": "🤖 اندروید (Android)", "callback_data": "help_android"}, {"text": "🍏 آیفون (iOS)", "callback_data": "help_ios"}],
                [{"text": "💻 ویندوز (Windows)", "callback_data": "help_windows"}],
                [{"text": "🔙 بازگشت به منوی اصلی", "callback_data": "menu_home"}]
            ]
        }
        send_message(user_id, txt, ikb)
        return

    if text in ["☎️ پشتیبانی", "📞 پشتیبانی", "/support"]:
        clear_user_state(user_id)
        sup = get_setting("support_username", "")
        cfg = load_config()
        if not sup:
            sup = cfg.get("support_username", "")
        sup_clean = sup.lstrip("@") if sup else ""
        
        sup_msg = (
            "☎️ <b>پشتیبانی:</b>\n\n"
            "در صورتی که سوال، ابهام یا مشکلی در اتصال دارید، می‌توانید با پشتیبانی ما در ارتباط باشید:\n"
        )
        if sup_clean:
            sup_msg += f"👉 @{sup_clean}\n\n"
            
        sup_msg += "💡 همچنین می‌توانید قبل از ارتباط، سوالات متداول را مطالعه فرمایید یا پیام خود را همین‌جا ارسال کنید:"
        ikb = {
            "inline_keyboard": [
                [{"text": "❓ سوالات متداول", "callback_data": "faq_view"}],
                [{"text": "🎟 ارسال پیام به پشتیبانی", "callback_data": "send_ticket"}],
                [{"text": "🔙 بازگشت به منوی اصلی", "callback_data": "menu_home"}]
            ]
        }
        send_message(user_id, sup_msg, ikb)
        return

    # Fallback response
    send_message(user_id, "لطفاً از دکمه‌های زیر جهت ادامه استفاده فرمایید:", get_main_keyboard(user_id))

# -------------------------------------------------------------
# Main Long-Polling Loop
# -------------------------------------------------------------
def main():
    logger.info("Starting DARK SHOP BOT (Mirza Bot 100% Native Edition)...")
    init_db()
    
    # Start auto notifier thread
    t = threading.Thread(target=background_notifier, daemon=True)
    t.start()
    
    token = get_bot_token()
    if not token:
        logger.error("No bot token set in config!")
        sys.exit(1)

    # Purge any stale webhook from previous sessions
    try:
        del_res = tg_request("deleteWebhook", {"drop_pending_updates": False})
        logger.info(f"Purged webhook status: {del_res}")
    except Exception as e:
        logger.warning(f"Error purging webhook: {e}")

    # Check connection
    me = tg_request("getMe")
    if not me or not me.get("ok"):
        logger.error(f"Cannot connect to Telegram Bot API with token: {token}")
        sys.exit(1)
        
    bot_user = me["result"]["username"]
    logger.info(f"Bot connected successfully as @{bot_user}")

    last_update_id = 0
    while True:
        try:
            params = {"offset": last_update_id + 1, "timeout": 20}
            updates = tg_request("getUpdates", params)
            if updates and updates.get("ok"):
                for upd in updates.get("result", []):
                    last_update_id = max(last_update_id, upd["update_id"])
                    if "message" in upd:
                        handle_message(upd["message"])
                    elif "callback_query" in upd:
                        handle_callback_query(upd["callback_query"])
            time.sleep(0.5)
        except Exception as e:
            logger.error(f"Error in bot poll loop: {e}")
            time.sleep(3)

if __name__ == "__main__":
    main()
