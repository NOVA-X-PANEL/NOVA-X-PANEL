#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
DARK SHOP BOT (Mirza Edition) - Complete Automated VPN Sales System for NOVA-X-PANEL
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

    for p in plans:
        if "price" not in p and "price_tomans" in p:
            p["price"] = p["price_tomans"]
        if "days" not in p and "period_days" in p:
            p["days"] = p["period_days"]
        if "price" not in p:
            p["price"] = 50000
        if "days" not in p:
            p["days"] = 30
        if "traffic_gb" not in p:
            p["traffic_gb"] = 50.0
    return plans

def get_plan_by_id(plan_id):
    plans = load_plans()
    for p in plans:
        if p.get("id") == plan_id:
            return p
    return None

def get_user_state(user_id):
    conn = get_db()
    c = conn.cursor()
    c.execute("SELECT state, data FROM user_state WHERE user_id = ?", (user_id,))
    row = c.fetchone()
    conn.close()
    if row:
        data = {}
        if row["data"]:
            try:
                data = json.loads(row["data"])
            except Exception:
                pass
        return {"state": row["state"], "data": data}
    return {"state": "IDLE", "data": {}}

def set_user_state(user_id, state, data=None):
    conn = get_db()
    c = conn.cursor()
    data_str = json.dumps(data, ensure_ascii=False) if data else "{}"
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

def clear_user_state(user_id):
    set_user_state(user_id, "IDLE", {})

def update_user_profile(user_id, username, first_name):
    conn = get_db()
    c = conn.cursor()
    now = int(time.time())
    c.execute("""
    INSERT INTO users (user_id, username, first_name, created_at, last_seen)
    VALUES (?, ?, ?, ?, ?)
    ON CONFLICT(user_id) DO UPDATE SET
        username = excluded.username,
        first_name = excluded.first_name,
        last_seen = excluded.last_seen
    """, (user_id, username or "", first_name or "", now, now))
    conn.commit()
    conn.close()

def get_user(user_id):
    conn = get_db()
    c = conn.cursor()
    c.execute("SELECT * FROM users WHERE user_id = ?", (user_id,))
    row = c.fetchone()
    conn.close()
    return dict(row) if row else None

def change_user_balance(user_id, amount_delta):
    conn = get_db()
    c = conn.cursor()
    c.execute("UPDATE users SET balance = MAX(0, balance + ?) WHERE user_id = ?", (amount_delta, user_id))
    conn.commit()
    c.execute("SELECT balance FROM users WHERE user_id = ?", (user_id,))
    row = c.fetchone()
    conn.close()
    return row["balance"] if row else 0

def generate_qr_bytes(text_data):
    if qrcode is not None:
        try:
            qr = qrcode.QRCode(
                version=1,
                error_correction=qrcode.constants.ERROR_CORRECT_L,
                box_size=8,
                border=3,
            )
            qr.add_data(text_data)
            qr.make(fit=True)
            img = qr.make_image(fill_color="black", back_color="white")
            bio = io.BytesIO()
            img.save(bio, format="PNG")
            bio.seek(0)
            return bio.getvalue()
        except Exception as e:
            logger.warning(f"Local QR generation error: {e}")
            
    # Zero-dependency HTTP fallback via public QR API
    try:
        import urllib.request
        import urllib.parse
        encoded = urllib.parse.quote(text_data)
        qr_url = f"https://api.qrserver.com/v1/create-qr-code/?size=300x300&data={encoded}"
        req = urllib.request.Request(qr_url, headers={"User-Agent": "Mozilla/5.0"})
        with urllib.request.urlopen(req, timeout=10) as resp:
            return resp.read()
    except Exception as e:
        logger.warning(f"Remote QR fetch fallback error: {e}")
        return None


def get_bot_token():
    cfg = load_config()
    return cfg.get("bot_token", "")

def get_admin_chat_id():
    cfg = load_config()
    admin_id = cfg.get("admin_chat_id", 0)
    try:
        return int(admin_id)
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
            logger.error(f"Telegram API request {method} exception: {e}")
            return None

    # 2. Pure Python standard library urllib fallback (zero external dependencies)
    try:
        import urllib.request
        import urllib.error
        
        if files:
            boundary = '----WebKitFormBoundary' + uuid.uuid4().hex
            body = bytearray()
            if payload:
                for k, v in payload.items():
                    body.extend(f'--{boundary}\r\nContent-Disposition: form-data; name="{k}"\r\n\r\n{v}\r\n'.encode('utf-8'))
            for field_name, file_tuple in files.items():
                filename, file_bytes, mime_type = file_tuple
                body.extend(f'--{boundary}\r\nContent-Disposition: form-data; name="{field_name}"; filename="{filename}"\r\nContent-Type: {mime_type}\r\n\r\n'.encode('utf-8'))
                body.extend(file_bytes)
                body.extend(b'\r\n')
            body.extend(f'--{boundary}--\r\n'.encode('utf-8'))
            headers = {'Content-Type': f'multipart/form-data; boundary={boundary}'}
            req = urllib.request.Request(url, data=bytes(body), headers=headers, method='POST')
        else:
            data_bytes = json.dumps(payload or {}).encode('utf-8')
            headers = {'Content-Type': 'application/json'}
            req = urllib.request.Request(url, data=data_bytes, headers=headers, method='POST')
            
        with urllib.request.urlopen(req, timeout=25) as resp:
            data = json.loads(resp.read().decode('utf-8'))
            if not data.get("ok"):
                logger.warning(f"Telegram API {method} error: {data.get('description')}")
            return data
    except urllib.error.HTTPError as e:
        err_body = e.read().decode('utf-8', errors='ignore')
        logger.warning(f"Telegram API {method} HTTP {e.code} error: {err_body}")
        try:
            return json.loads(err_body)
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
        if status in ["creator", "administrator", "member", "restricted"]:
            return True
        return False
    # If bot cannot check channel (e.g. not admin in channel), gracefully allow user
    return True


def get_inbound_and_sub_info():
    if not os.path.exists(XUI_DB_PATH):
        return None, 2096, "/sub/", "", ""
    conn = sqlite3.connect(XUI_DB_PATH, timeout=10.0)
    c = conn.cursor()
    c.execute("SELECT id, port, protocol, settings, stream_settings FROM inbounds WHERE enable = 1 ORDER BY id ASC")
    rows = c.fetchall()
    
    sub_port = 2096
    sub_path = "/sub/"
    sub_domain = ""
    sub_uri = ""
    c.execute("SELECT key, value FROM settings WHERE key IN ('subPort', 'subPath', 'subDomain', 'subURI')")
    for k, v in c.fetchall():
        if k == 'subPort' and v:
            try:
                sub_port = int(v)
            except Exception:
                pass
        elif k == 'subPath' and v:
            sub_path = v
        elif k == 'subDomain' and v:
            sub_domain = v.strip()
        elif k == 'subURI' and v:
            sub_uri = v.strip()
    conn.close()

    inbound = None
    for r in rows:
        if r[2] == 'vless':
            inbound = r
            break
    if not inbound and rows:
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
    
    # Restart x-ui to load changes
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
        "total_gb": traffic_gb,
        "days": expiry_days
    }

def renew_xray_client(client_email, add_gb, add_days):
    if not os.path.exists(XUI_DB_PATH):
        return False
    
    conn = sqlite3.connect(XUI_DB_PATH, timeout=20.0)
    c = conn.cursor()
    
    # Find inbound
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


def get_main_keyboard(user_id):
    admin_id = get_admin_chat_id()
    kb = [
        [{"text": "🔐 خرید اشتراک"}, {"text": "♻️ تمدید سرویس"}],
        [{"text": "🔑 اکانت تست"}, {"text": "🎲 گردونه شانس"}],
        [{"text": "🛍 سرویس های من"}, {"text": "🏦 کیف پول + شارژ"}],
        [{"text": "👥 زیر مجموعه گیری"}, {"text": "💵 تعرفه اشتراک ها"}],
        [{"text": "☎️ پشتیبانی"}, {"text": "📚 آموزش"}]
    ]
    if user_id == admin_id:
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
            [{"text": "🏷 مدیریت کدهای تخفیف", "callback_data": "adm_discounts"}, {"text": "🔒 قفل جوین اجباری", "callback_data": "adm_force_join"}],
            [{"text": "🔑 تنظیمات اکانت تست", "callback_data": "adm_trial_settings"}],
            [{"text": "🔙 خروج از پنل مدیریت", "callback_data": "adm_exit"}]
        ]
    }

def apply_referral_reward(buyer_user_id, buyer_name, order_id, amount_paid):
    try:
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT referrer_id FROM referrals WHERE user_id = ?", (buyer_user_id,))
        row = c.fetchone()
        if not row:
            conn.close()
            return
        referrer_id = row["referrer_id"]
        percent = int(get_setting("referral_percent", "10"))
        reward = int(amount_paid * percent / 100)
        if reward <= 0:
            conn.close()
            return
            
        c.execute("""
        INSERT INTO referral_rewards (user_id, referrer_id, amount, order_id, created_at)
        VALUES (?, ?, ?, ?, ?)
        """, (buyer_user_id, referrer_id, reward, order_id, int(time.time())))
        conn.commit()
        conn.close()
        
        # Credit wallet
        new_bal = change_user_balance(referrer_id, reward)
        
        # Notify referrer
        msg = (
            f"🎉 <b>پاداش زیرمجموعه‌گیری واریز شد!</b>\n\n"
            f"👤 کاربر دعوت‌شده توسط شما (<code>{buyer_name or buyer_user_id}</code>) یک خرید انجام داد.\n"
            f"💰 <b>مبلغ هدیه:</b> {reward:,} تومان ({percent}٪)\n"
            f"💳 <b>موجودی جدید کیف پول شما:</b> {new_bal:,} تومان\n\n"
            f"ممنون از همراهی شما! ✨"
        )
        send_message(referrer_id, msg)
    except Exception as e:
        logger.error(f"Error in apply_referral_reward: {e}")


def background_notifier():
    while True:
        try:
            time.sleep(3600) # every hour
            if not os.path.exists(XUI_DB_PATH):
                continue
            conn = sqlite3.connect(XUI_DB_PATH, timeout=10.0)
            c = conn.cursor()
            c.execute("SELECT id, settings FROM inbounds WHERE enable = 1")
            inbounds = c.fetchall()
            
            c.execute("SELECT email, up, down, total, expiry_time, enable FROM client_traffics WHERE enable = 1")
            traffics = {r[0]: {"used": r[1]+r[2], "total": r[3], "exp": r[4]} for r in c.fetchall()}
            conn.close()
            
            s_conn = get_db()
            sc = s_conn.cursor()
            now_ms = int(time.time() * 1000)
            
            for ib in inbounds:
                try:
                    st = json.loads(ib[1])
                    for cl in st.get("clients", []):
                        tg_id = cl.get("tgId")
                        if not tg_id or str(tg_id) in ["", "0"]:
                            continue
                        email = cl.get("email", "")
                        tr = traffics.get(email)
                        if not tr:
                            continue
                        
                        rem_bytes = tr["total"] - tr["used"]
                        rem_gb = rem_bytes / (1024**3)
                        rem_days = (tr["exp"] - now_ms) / (86400 * 1000) if tr["exp"] > 0 else 999
                        
                        # Check conditions
                        if rem_gb <= 1.0 or rem_days <= 2.0:
                            sc.execute("SELECT sent_at FROM notifications_sent WHERE client_email = ?", (email,))
                            already = sc.fetchone()
                            # Notify at most once per 48 hours
                            if not already or (int(time.time()) - already["sent_at"] > 172800):
                                warn_msg = (
                                    f"⚠️ <b>هشدار انقضای اشتراک VPN</b>\n\n"
                                    f"مشترک گرامی، سرویس شما (<code>{email}</code>) به پایان نزدیک است:\n"
                                    f"📊 حجم باقیمانده: <b>{rem_gb:.2f} گیگابایت</b>\n"
                                    f"⏳ اعتبار زمانی: <b>{max(0, int(rem_days))} روز</b>\n\n"
                                    f"برای جلوگیری از قطع اتصال، می‌توانید از بخش «🔄 تمدید اشتراک» در ربات، سرویس خود را بدون تغییر لینک تمدید فرمایید."
                                )
                                send_message(int(tg_id), warn_msg)
                                sc.execute("""
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
                edit_message_text(user_id, msg_id, "✅ عضویت شما تایید گردید. به ربات DARK SHOP خوش آمدید!", get_main_keyboard(user_id))
        else:
            channel = get_setting("force_join_channel", "")
            answer_callback_query(cq_id, f"هنوز در کانال {channel} عضو نشده‌اید!", show_alert=True)
        return
        
    if data == "adm_exit":
        if user_id == admin_id and msg_id:
            edit_message_text(user_id, msg_id, "از پنل مدیریت خارج شدید.")
        answer_callback_query(cq_id)
        return

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
            f"📊 <b>گزارش آماری ربات فروشگاه (NOVA Edition)</b>\n\n"
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
        send_message(user_id, "📢 <b>ارسال پیام همگانی</b>\n\nلطفاً متن پیام مورد نظر را ارسال کنید (برای لغو /cancel را ارسال کنید):", get_cancel_keyboard())
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
        set_user_state(user_id, "WAIT_NEW_CARD")
        answer_callback_query(cq_id)
        send_message(user_id, f"💳 <b>تنظیمات شماره کارت بانکی</b>\n\nشماره کارت فعلی: <code>{cur_card}</code>\nنام صاحب حساب: <code>{cur_holder}</code>\n\nجهت تغییر، اطلاعات جدید را به صورت زیر بفرستید:\n<code>شماره‌کارت*نام‌صاحب‌حساب</code>\nمثال:\n<code>6037997412345678*علی علوی</code>", get_cancel_keyboard())
        return

    if data == "adm_discounts":
        if user_id != admin_id:
            return
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT code, percent, max_uses, used_count FROM discount_codes ORDER BY code ASC")
        rows = c.fetchall()
        conn.close()
        
        txt = "🏷 <b>لیست کدهای تخفیف فعال:</b>\n\n"
        if not rows:
            txt += "هیچ کد تخفیفی ثبت نشده است.\n"
        else:
            for r in rows:
                txt += f"▫️ کد: <code>{r['code']}</code> | {r['percent']}٪ | مصرف: {r['used_count']}/{r['max_uses']}\n"
        txt += "\nبرای افزودن کد جدید دکمه زیر را لمس کنید:"
        ikb = {
            "inline_keyboard": [
                [{"text": "➕ افزودن کد تخفیف جدید", "callback_data": "adm_add_code"}],
                [{"text": "🔙 بازگشت به پنل مدیریت", "callback_data": "adm_stats"}]
            ]
        }
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, txt, ikb)
        return

    if data == "adm_add_code":
        if user_id != admin_id:
            return
        set_user_state(user_id, "WAIT_NEW_CODE")
        answer_callback_query(cq_id)
        send_message(user_id, "🏷 <b>افزودن کد تخفیف</b>\n\nاطلاعات را با فرمت زیر بفرستید:\n<code>کد*درصد*حداکثر_تعداد</code>\nمثال:\n<code>DARK20*20*100</code> (کد DARK20 با ۲۰ درصد تخفیف برای ۱۰۰ نفر)", get_cancel_keyboard())
        return

    if data == "adm_force_join":
        if user_id != admin_id:
            return
        cur_status = get_setting("force_join_enabled", "0")
        new_status = "0" if cur_status == "1" else "1"
        set_setting("force_join_enabled", new_status)
        channel = get_setting("force_join_channel", "")
        status_lbl = "فعال ✅" if new_status == "1" else "غیرفعال ❌"
        answer_callback_query(cq_id, f"قفل جوین اجباری {status_lbl} شد.")
        if msg_id:
            ikb = {
                "inline_keyboard": [
                    [{"text": f"تغییر وضعیت (اکنون: {status_lbl})", "callback_data": "adm_force_join"}],
                    [{"text": "🔙 بازگشت", "callback_data": "adm_stats"}]
                ]
            }
            edit_message_text(user_id, msg_id, f"📢 <b>قفل جوین اجباری کانال</b>\n\nکانال هدف: <code>{channel}</code>\nوضعیت فعلی: <b>{status_lbl}</b>", ikb)
        return

    if data == "adm_plans":
        if user_id != admin_id:
            return
        plans = load_plans()
        txt = "🛍 <b>لیست پلن‌های فروشگاه (Mirza Edition):</b>\n\n"
        ikb_rows = []
        for p in plans:
            txt += f"▫️ <b>#{p.get('id')} - {p.get('title')}</b>\n"
            txt += f"   حجم: {p.get('traffic_gb')} GB | زمان: {p.get('days')} روز | قیمت: {p.get('price'):,} تومان\n\n"
            ikb_rows.append([{"text": f"❌ حذف #{p.get('id')} {p.get('title')}", "callback_data": f"adm_delplan_{p.get('id')}"}])
        ikb_rows.append([{"text": "➕ افزودن پلن جدید", "callback_data": "adm_add_plan"}])
        ikb_rows.append([{"text": "🔙 بازگشت به پنل مدیریت", "callback_data": "adm_stats"}])
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, txt, {"inline_keyboard": ikb_rows})
        return

    if data == "adm_add_plan":
        if user_id != admin_id:
            return
        set_user_state(user_id, "WAIT_NEW_PLAN")
        answer_callback_query(cq_id)
        txt = (
            "🛍 <b>افزودن پلن جدید</b>\n\n"
            "مشخصات را با فرمت زیر ارسال کنید:\n"
            "<code>عنوان*حجم_گیگ*مدت_روز*قیمت_تومان</code>\n\n"
            "مثال:\n"
            "<code>پلن نامحدود*150*30*180000</code>"
        )
        send_message(user_id, txt, get_cancel_keyboard())
        return

    if data.startswith("adm_delplan_"):
        if user_id != admin_id:
            return
        pid = int(data.split("_")[2])
        plans = load_plans()
        plans = [p for p in plans if p.get("id") != pid]
        try:
            with open(PLANS_PATH, "w", encoding="utf-8") as f:
                json.dump(plans, f, ensure_ascii=False, indent=2)
        except Exception:
            pass
        answer_callback_query(cq_id, "پلن با موفقیت حذف شد! ✅")
        if msg_id:
            edit_message_text(user_id, msg_id, "✅ پلن با موفقیت حذف گردید.", get_admin_inline_keyboard())
        return

    if data == "adm_trial_settings":
        if user_id != admin_id:
            return
        t_gb = get_setting("trial_gb", "1")
        t_days = get_setting("trial_days", "1")
        set_user_state(user_id, "WAIT_TRIAL_SETTINGS")
        answer_callback_query(cq_id)
        txt = (
            f"🔑 <b>تنظیمات اکانت تست (Mirza Edition):</b>\n\n"
            f"▫️ حجم تست فعلی: <b>{t_gb} گیگابایت</b>\n"
            f"▫️ مدت اعتبار فعلی: <b>{t_days} روز</b>\n\n"
            f"جهت تغییر، مقادیر جدید را با فرمت زیر بفرستید:\n"
            f"<code>حجم_گیگابایت*مدت_روز</code>\n\n"
            f"مثال:\n"
            f"<code>2*1</code> (یعنی ۲ گیگابایت برای ۱ روز)"
        )
        send_message(user_id, txt, get_cancel_keyboard())
        return

    if data.startswith("adm_user_add_"):
        if user_id != admin_id:
            return
        target_uid = int(data.split("_")[3])
        set_user_state(user_id, "WAIT_USER_ADD_BAL", {"target_uid": target_uid})
        answer_callback_query(cq_id)
        send_message(user_id, f"💰 لطفاً مبلغ مورد نظر جهت **افزایش موجودی** کاربر <code>{target_uid}</code> را به تومان وارد کنید:\n(مثال: 50000)", get_cancel_keyboard())
        return

    if data.startswith("adm_user_sub_"):
        if user_id != admin_id:
            return
        target_uid = int(data.split("_")[3])
        set_user_state(user_id, "WAIT_USER_SUB_BAL", {"target_uid": target_uid})
        answer_callback_query(cq_id)
        send_message(user_id, f"➖ لطفاً مبلغ مورد نظر جهت **کسر از موجودی** کاربر <code>{target_uid}</code> را به تومان وارد کنید:\n(مثال: 20000)", get_cancel_keyboard())
        return

    if data.startswith("adm_user_ban_"):
        if user_id != admin_id:
            return
        target_uid = int(data.split("_")[3])
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT is_banned FROM users WHERE user_id = ?", (target_uid,))
        r = c.fetchone()
        cur_ban = r["is_banned"] if r else 0
        new_ban = 0 if cur_ban else 1
        c.execute("UPDATE users SET is_banned = ? WHERE user_id = ?", (new_ban, target_uid))
        conn.commit()
        conn.close()
        st_txt = "مسدود (Ban) شد ⛔️" if new_ban else "آزاد (Unban) شد ✅"
        answer_callback_query(cq_id, f"وضعیت کاربر به {st_txt} تغییر یافت.", show_alert=True)
        return

    if data.startswith("qr_"):
        email = data[3:]
        clients = get_user_xray_clients(user_id)
        target = None
        for cl in clients:
            if cl.get("email") == email:
                target = cl
                break
        if target and target.get("sub_url"):
            answer_callback_query(cq_id)
            qr_bytes = generate_qr_bytes(target["sub_url"])
            send_photo(user_id, qr_bytes, caption=f"📸 <b>بارکد اتصال سرویس:</b> <code>{email}</code>\n🔗 <code>{target['sub_url']}</code>")
        else:
            answer_callback_query(cq_id, "لینک سابسکریپشن یافت نشد!", show_alert=True)
        return

    # Handle Approval / Rejection by Admin
    if data.startswith("approve_") or data.startswith("reject_"):
        if user_id != admin_id:
            answer_callback_query(cq_id, "فقط ادمین می‌تواند فیش را تایید کند!", show_alert=True)
            return
        parts = data.split("_")
        action = parts[0]
        order_id = int(parts[1])
        
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT * FROM orders WHERE id = ?", (order_id,))
        order = c.fetchone()
        
        if not order:
            answer_callback_query(cq_id, "سفارش یافت نشد!", show_alert=True)
            conn.close()
            return
            
        if order["status"] != "PENDING":
            answer_callback_query(cq_id, f"این سفارش قبلاً تعیین وضعیت شده است ({order['status']}).", show_alert=True)
            conn.close()
            return
            
        buyer_id = order["user_id"]
        buyer_name = order["username"]
        order_type = order["order_type"]
        final_price = order["final_price"]
        
        if action == "reject":
            c.execute("UPDATE orders SET status = 'REJECTED' WHERE id = ?", (order_id,))
            conn.commit()
            conn.close()
            answer_callback_query(cq_id, "فیش رد شد.")
            send_message(buyer_id, f"❌ متأسفانه فیش واریزی سفارش شماره #{order_id} توسط ادمین رد شد.\nاگر اشتباهی رخ داده با پشتیبانی در ارتباط باشید.")
            if msg_id:
                edit_message_text(admin_id, msg_id, f"❌ فیش سفارش #{order_id} توسط شما رد شد.")
            return
            
        # Action is APPROVE
        if order_type == "WALLET":
            c.execute("UPDATE orders SET status = 'APPROVED' WHERE id = ?", (order_id,))
            conn.commit()
            conn.close()
            new_bal = change_user_balance(buyer_id, final_price)
            answer_callback_query(cq_id, "کیف پول شارژ شد! ✅")
            send_message(buyer_id, f"✅ <b>کیف پول شما با موفقیت شارژ شد!</b>\n\n💰 مبلغ شارژ: {final_price:,} تومان\n💳 موجودی فعلی: <b>{new_bal:,} تومان</b>", get_main_keyboard(buyer_id))
            if msg_id:
                edit_message_text(admin_id, msg_id, f"✅ فیش شارژ کیف پول سفارش #{order_id} تایید شد و حساب کاربر شارژ گردید.")
            return
            
        elif order_type == "RENEW":
            target_email = order["target_email"]
            plan = get_plan_by_id(order["plan_id"])
            if not plan:
                conn.close()
                return
            ok = renew_xray_client(target_email, plan["traffic_gb"], plan["days"])
            c.execute("UPDATE orders SET status = 'APPROVED' WHERE id = ?", (order_id,))
            conn.commit()
            conn.close()
            
            if ok:
                answer_callback_query(cq_id, "سرویس تمدید شد! ✅")
                send_message(buyer_id, f"🎉 <b>اشتراک شما با موفقیت تمدید شد!</b>\n\n📦 پلن تمدید: <b>{plan['title']}</b>\n📊 حجم اضافه شده: {plan['traffic_gb']} گیگابایت\n⏳ زمان اضافه شده: {plan['days']} روز\n\nلینک و کانفیگ قبلی شما مجدداً فعال گردید.", get_main_keyboard(buyer_id))
                apply_referral_reward(buyer_id, buyer_name, order_id, final_price)
            else:
                answer_callback_query(cq_id, "خطا در تمدید کلاینت!", show_alert=True)
            if msg_id:
                edit_message_text(admin_id, msg_id, f"✅ فیش تمدید سفارش #{order_id} تایید شد.")
            return
            
        else: # Regular PLAN purchase
            plan = get_plan_by_id(order["plan_id"])
            if not plan:
                conn.close()
                return
            res = add_xray_client(buyer_id, buyer_name, plan["traffic_gb"], plan["days"])
            if not res:
                answer_callback_query(cq_id, "خطا در ساخت کلاینت در هسته پنل!", show_alert=True)
                conn.close()
                return
                
            c.execute("UPDATE orders SET status = 'APPROVED', sub_id = ? WHERE id = ?", (res["sub_id"], order_id))
            conn.commit()
            conn.close()
            
            answer_callback_query(cq_id, "سفارش تایید و کانفیگ صادر شد! ✅")
            apply_referral_reward(buyer_id, buyer_name, order_id, final_price)
            
            # Send to Buyer
            user_msg = (
                f"🎉 <b>خرید شما با موفقیت تایید شد!</b>\n\n"
                f"📦 <b>پلن:</b> {plan['title']}\n"
                f"📊 <b>حجم:</b> {plan['traffic_gb']} گیگابایت\n"
                f"⏳ <b>مدت زمان:</b> {plan['days']} روز\n\n"
                f"🔗 <b>لینک سابسکریپشن هوشمند (پیشنهادی):</b>\n<code>{res['sub_url']}</code>\n\n"
                f"⚡️ <b>کانفیگ تک‌خطی VLESS:</b>\n<code>{res['vless_link']}</code>\n\n"
                f"💡 <i>بارکد QR اختصاصی نیز در پیام زیر برای اتصال سریع ارسال شد.</i>"
            )
            send_message(buyer_id, user_msg, get_main_keyboard(buyer_id))
            
            qr_bytes = generate_qr_bytes(res["sub_url"])
            send_photo(buyer_id, qr_bytes, caption="📸 اسکن QR Code در نرم‌افزارهای v2rayNG / Sing-box / Streisand")
            
            if msg_id:
                edit_message_text(admin_id, msg_id, f"✅ سفارش #{order_id} تایید و تحویل کاربر گردید.")
            return

    # User Plan Selection
    if data.startswith("plan_"):
        plan_id = int(data.split("_")[1])
        plan = get_plan_by_id(plan_id)
        if not plan:
            answer_callback_query(cq_id, "پلن نامعتبر است.")
            return
            
        u = get_user(user_id)
        bal = u["balance"] if u else 0
        price = plan["price"]
        
        txt = (
            f"📦 <b>جزئیات پلن انتخابی:</b>\n\n"
            f"🔹 <b>عنوان:</b> {plan['title']}\n"
            f"🔹 <b>حجم ترافیک:</b> {plan['traffic_gb']} گیگابایت\n"
            f"🔹 <b>مدت اعتبار:</b> {plan['days']} روز\n"
            f"💵 <b>قیمت:</b> {price:,} تومان\n\n"
            f"💰 <b>موجودی کیف پول شما:</b> {bal:,} تومان\n\n"
            f"روش پرداخت مورد نظر را انتخاب نمایید:"
        )
        ikb_rows = []
        if bal >= price:
            ikb_rows.append([{"text": "⚡️ خرید آنی از کیف پول (تحویل فوری)", "callback_data": f"buy_wallet_{plan_id}"}])
        ikb_rows.append([{"text": "💳 پرداخت کارت‌به‌کارت (ارسال فیش)", "callback_data": f"buy_card_{plan_id}"}])
        ikb_rows.append([{"text": "🏷 اعمال کد تخفیف", "callback_data": f"apply_disc_{plan_id}"}])
        ikb_rows.append([{"text": "🔙 بازگشت به پلن‌ها", "callback_data": "list_plans"}])
        
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
            edit_message_text(user_id, msg_id, "🌟 <b>پلن مورد نظر خود را انتخاب کنید:</b>", {"inline_keyboard": ikb_rows})
        return

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
        
        user_msg = (
            f"🎉 <b>خرید شما با موفقیت از کیف پول پرداخت شد!</b>\n\n"
            f"📦 <b>پلن:</b> {plan['title']}\n"
            f"📊 <b>حجم:</b> {plan['traffic_gb']} گیگابایت\n"
            f"⏳ <b>اعتبار:</b> {plan['days']} روز\n"
            f"💰 <b>مبلغ کسر شده:</b> {price:,} تومان\n\n"
            f"🔗 <b>لینک سابسکریپشن هوشمند:</b>\n<code>{res['sub_url']}</code>\n\n"
            f"⚡️ <b>کانفیگ تک‌خطی:</b>\n<code>{res['vless_link']}</code>"
        )
        if msg_id:
            edit_message_text(user_id, msg_id, user_msg)
        qr_bytes = generate_qr_bytes(res["sub_url"])
        send_photo(user_id, qr_bytes, caption="📸 اسکن بارکد QR جهت اتصال مستقیم")
        
        # Notify admin of instant wallet sale
        admin_notice = f"⚡️ <b>فروش آنی از کیف پول</b>\n\n👤 کاربر: @{from_user.get('username', '')} (<code>{user_id}</code>)\n📦 پلن: {plan['title']}\n💵 مبلغ: {price:,} تومان"
        send_message(admin_id, admin_notice)
        return

    if data.startswith("buy_card_"):
        plan_id = int(data.split("_")[2])
        plan = get_plan_by_id(plan_id)
        if not plan:
            return
        cfg = load_config()
        card_num = cfg.get("card_number", "۶۰۳۷-۹۹۷۴-XXXX-XXXX")
        card_holder = cfg.get("card_holder", "DARK VVPN")
        
        set_user_state(user_id, "WAIT_RECEIPT_PLAN", {"plan_id": plan_id, "price": plan["price"]})
        answer_callback_query(cq_id)
        
        txt = (
            f"💳 <b>اطلاعات پرداخت کارت به کارت:</b>\n\n"
            f"📦 <b>پلن انتخابی:</b> {plan['title']}\n"
            f"💵 <b>مبلغ دقیق واریزی:</b> <code>{plan['price']:,}</code> تومان\n\n"
            f"💳 <b>شماره کارت:</b>\n<code>{card_num}</code>\n"
            f"👤 <b>به نام:</b> <b>{card_holder}</b>\n\n"
            f"📸 پس از واریز، لطفاً <b>عکس فیش واریزی</b> را همین‌جا ارسال نمایید:"
        )
        send_message(user_id, txt, get_cancel_keyboard())
        return

    if data.startswith("apply_disc_"):
        plan_id = int(data.split("_")[2])
        set_user_state(user_id, "WAIT_DISCOUNT_CODE", {"plan_id": plan_id})
        answer_callback_query(cq_id)
        send_message(user_id, "🏷 لطفاً کد تخفیف خود را ارسال کنید:", get_cancel_keyboard())
        return
        
    if data == "topup_menu":
        answer_callback_query(cq_id)
        amounts = [50000, 100000, 200000, 500000]
        ikb_rows = []
        for a in amounts:
            ikb_rows.append([{"text": f"💳 شارژ {a:,} تومان", "callback_data": f"topup_{a}"}])
        ikb_rows.append([{"text": "🔙 بازگشت", "callback_data": "adm_exit"}])
        txt = "💰 <b>مبلغ شارژ کیف پول را انتخاب کنید:</b>\nپس از انتخاب، شماره کارت جهت واریز نمایش داده می‌شود."
        if msg_id:
            edit_message_text(user_id, msg_id, txt, {"inline_keyboard": ikb_rows})
        else:
            send_message(user_id, txt, {"inline_keyboard": ikb_rows})
        return

    if data.startswith("topup_"):
        amt = int(data.split("_")[1])
        cfg = load_config()
        card_num = cfg.get("card_number", "۶۰۳۷-۹۹۷۴-XXXX-XXXX")
        card_holder = cfg.get("card_holder", "DARK VVPN")
        set_user_state(user_id, "WAIT_RECEIPT_WALLET", {"amount": amt})
        answer_callback_query(cq_id)
        txt = (
            f"💳 <b>شارژ کیف پول به مبلغ {amt:,} تومان:</b>\n\n"
            f"💳 <b>شماره کارت:</b> <code>{card_num}</code>\n"
            f"👤 <b>به نام:</b> <b>{card_holder}</b>\n\n"
            f"لطفاً پس از واریز، <b>عکس فیش بانکی</b> را ارسال کنید:"
        )
        send_message(user_id, txt, get_cancel_keyboard())
        return

    if data.startswith("renew_"):
        email = data[6:]
        plans = load_plans()
        ikb_rows = []
        for p in plans:
            ikb_rows.append([{"text": f"🔄 {p['title']} ({p['traffic_gb']}GB - {p['price']:,}T)", "callback_data": f"dorenew_{p['id']}_{email}"}])
        ikb_rows.append([{"text": "🔙 بازگشت", "callback_data": "adm_exit"}])
        answer_callback_query(cq_id)
        if msg_id:
            edit_message_text(user_id, msg_id, f"سرویس انتخابی برای تمدید: <code>{email}</code>\nپلن تمدید مورد نظر را انتخاب فرمایید:", {"inline_keyboard": ikb_rows})
        else:
            send_message(user_id, f"سرویس انتخابی برای تمدید: <code>{email}</code>\nپلن تمدید مورد نظر را انتخاب فرمایید:", {"inline_keyboard": ikb_rows})
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
        
        ikb_rows = []
        if bal >= price:
            ikb_rows.append([{"text": "⚡️ تمدید آنی از کیف پول", "callback_data": f"doreneww_{plan_id}_{email}"}])
        ikb_rows.append([{"text": "💳 پرداخت کارت‌به‌کارت", "callback_data": f"dorenewc_{plan_id}_{email}"}])
        
        txt = f"تمدید سرویس <code>{email}</code> با پلن <b>{plan['title']}</b>\nمبلغ: {price:,} تومان\nروش پرداخت را انتخاب نمایید:"
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
            edit_message_text(user_id, msg_id, f"🎉 <b>اشتراک شما تمدید گردید!</b>\n\nسرویس: <code>{email}</code>\nافزوده شد: <b>{plan['traffic_gb']} گیگابایت</b> و <b>{plan['days']} روز</b>\nمبلغ کسر شده از کیف پول: {price:,} تومان")
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
        set_user_state(user_id, "WAIT_RECEIPT_RENEW", {"plan_id": plan_id, "email": email, "price": plan["price"]})
        answer_callback_query(cq_id)
        txt = (
            f"💳 <b>تمدید سرویس با کارت به کارت:</b>\n\n"
            f"سرویس: <code>{email}</code>\n"
            f"مبلغ: <code>{plan['price']:,}</code> تومان\n\n"
            f"شماره کارت: <code>{card_num}</code>\n"
            f"به نام: <b>{card_holder}</b>\n\n"
            f"لطفاً عکس فیش واریزی را ارسال کنید:"
        )
        send_message(user_id, txt, get_cancel_keyboard())
        return


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
        if len(parts) > 1 and parts[1].startswith("ref_"):
            try:
                ref_id = int(parts[1][4:])
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
            f"🛡 اتصال تضمینی و ضد فیلتر بر روی تمامی اپراتورها (همراه‌اول، ایرانسل، مخابرات، رایتل و وای‌فای)\n"
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
        send_message(user_id, "⚙️ <b>به پنل مدیریت ربات (Mirza Edition) خوش آمدید:</b>\nجهت مدیریت، بخش مورد نظر را انتخاب نمایید:", get_admin_inline_keyboard())
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
            time.sleep(0.04) # rate limit friendly
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
            set_user_state(user_id, "WAIT_RECEIPT_WALLET", {"amount": amt})
            txt = (
                f"💳 <b>شارژ کیف پول به مبلغ {amt:,} تومان:</b>\n\n"
                f"💳 <b>شماره کارت:</b> <code>{card_num}</code>\n"
                f"👤 <b>به نام:</b> <b>{card_holder}</b>\n\n"
                f"لطفاً پس از واریز، <b>عکس فیش بانکی</b> را ارسال فرمایید:"
            )
            send_message(user_id, txt, get_cancel_keyboard())
        except Exception:
            send_message(user_id, "لطفاً مبلغ را به عدد انگلیسی ارسال فرمایید (مثلاً 50000):")
        return

    # Handle State: WAIT_NEW_PLAN
    if cur_state == "WAIT_NEW_PLAN" and user_id == admin_id:
        clear_user_state(user_id)
        parts = text.split("*")
        if len(parts) >= 4:
            try:
                title = parts[0].strip()
                gb = float(parts[1].strip())
                days = int(parts[2].strip())
                price = int(parts[3].strip().replace(",", ""))
                plans = load_plans()
                new_id = max([p.get("id", 0) for p in plans] or [0]) + 1
                plans.append({
                    "id": new_id,
                    "title": title,
                    "traffic_gb": gb,
                    "days": days,
                    "price": price,
                    "enabled": True,
                    "inbound_id": 1
                })
                with open(PLANS_PATH, "w", encoding="utf-8") as f:
                    json.dump(plans, f, ensure_ascii=False, indent=2)
                send_message(user_id, f"✅ پلن <b>{title}</b> ({gb}GB - {days} روز - {price:,} تومان) با موفقیت ذخیره شد.", get_main_keyboard(user_id))
            except Exception as e:
                send_message(user_id, f"❌ خطا در مقادیر: {e}", get_main_keyboard(user_id))
        else:
            send_message(user_id, "❌ فرمت ورودی نامعتبر بود. تغییری ایجاد نشد.", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_TRIAL_SETTINGS
    if cur_state == "WAIT_TRIAL_SETTINGS" and user_id == admin_id:
        clear_user_state(user_id)
        parts = text.split("*")
        if len(parts) >= 2:
            try:
                gb = str(float(parts[0].strip()))
                days = str(int(parts[1].strip()))
                set_setting("trial_gb", gb)
                set_setting("trial_days", days)
                send_message(user_id, f"✅ تنظیمات اکانت تست به‌روزرسانی شد:\nحجم: <b>{gb} گیگابایت</b>\nمدت اعتبار: <b>{days} روز</b>", get_main_keyboard(user_id))
            except Exception as e:
                send_message(user_id, f"❌ خطا در مقادیر: {e}", get_main_keyboard(user_id))
        else:
            send_message(user_id, "❌ فرمت نامعتبر بود.", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_NEW_CARD
    if cur_state == "WAIT_NEW_CARD" and user_id == admin_id:
        clear_user_state(user_id)
        if "*" in text:
            parts = text.split("*", 1)
            c_num = parts[0].strip()
            c_hold = parts[1].strip()
            cfg = load_config()
            cfg["card_number"] = c_num
            cfg["card_holder"] = c_hold
            with open(CONFIG_PATH, "w", encoding="utf-8") as f:
                json.dump(cfg, f, ensure_ascii=False, indent=2)
            send_message(user_id, f"✅ اطلاعات کارت به‌روزرسانی شد:\nشماره: <code>{c_num}</code>\nنام: <b>{c_hold}</b>", get_main_keyboard(user_id))
        else:
            send_message(user_id, "فرمت نامعتبر بود. تغییری ایجاد نشد.", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_NEW_CODE
    if cur_state == "WAIT_NEW_CODE" and user_id == admin_id:
        clear_user_state(user_id)
        parts = text.split("*")
        if len(parts) >= 3:
            code = parts[0].strip().upper()
            pct = int(parts[1].strip())
            mx = int(parts[2].strip())
            conn = get_db()
            c = conn.cursor()
            c.execute("INSERT OR REPLACE INTO discount_codes (code, percent, max_uses, used_count) VALUES (?, ?, ?, 0)", (code, pct, mx))
            conn.commit()
            conn.close()
            send_message(user_id, f"✅ کد تخفیف <code>{code}</code> با {pct}٪ تخفیف برای {mx} نفر با موفقیت ثبت شد.", get_main_keyboard(user_id))
        else:
            send_message(user_id, "فرمت نامعتبر بود.", get_main_keyboard(user_id))
        return

    # Handle State: WAIT_DISCOUNT_CODE
    if cur_state == "WAIT_DISCOUNT_CODE":
        plan_id = sdata.get("plan_id")
        plan = get_plan_by_id(plan_id)
        code_input = text.strip().upper()
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT * FROM discount_codes WHERE code = ?", (code_input,))
        disc = c.fetchone()
        conn.close()
        
        if not disc or disc["used_count"] >= disc["max_uses"]:
            send_message(user_id, "❌ کد تخفیف نامعتبر یا منقضی شده است. لطفاً کد دیگری ارسال کنید یا /cancel را بزنید:")
            return
            
        pct = disc["percent"]
        orig_price = plan["price"]
        final_price = int(orig_price * (100 - pct) / 100)
        clear_user_state(user_id)
        
        u = get_user(user_id)
        bal = u["balance"] if u else 0
        
        txt = (
            f"🎉 <b>کد تخفیف {pct}٪ با موفقیت اعمال شد!</b>\n\n"
            f"📦 <b>پلن:</b> {plan['title']}\n"
            f"💵 قیمت اصلی: <s>{orig_price:,}</s> تومان\n"
            f"🔥 <b>قیمت با تخفیف:</b> <b>{final_price:,} تومان</b>\n\n"
            f"💰 موجودی کیف پول شما: {bal:,} تومان\n\n"
            f"روش پرداخت را انتخاب نمایید:"
        )
        ikb_rows = []
        if bal >= final_price:
            ikb_rows.append([{"text": "⚡️ خرید آنی از کیف پول", "callback_data": f"buy_wallet_{plan_id}"}])
        ikb_rows.append([{"text": "💳 پرداخت کارت‌به‌کارت", "callback_data": f"buy_card_{plan_id}"}])
        send_message(user_id, txt, {"inline_keyboard": ikb_rows})
        return

    # Handle Photo Receipts
    if photo and cur_state in ["WAIT_RECEIPT_PLAN", "WAIT_RECEIPT_WALLET", "WAIT_RECEIPT_RENEW"]:
        file_id = photo[-1]["file_id"]
        clear_user_state(user_id)
        
        order_type = "PLAN"
        plan_id = sdata.get("plan_id")
        plan_title = ""
        price = sdata.get("price", 0)
        target_email = sdata.get("email", "")
        
        if cur_state == "WAIT_RECEIPT_WALLET":
            order_type = "WALLET"
            plan_title = "شارژ کیف پول"
            price = sdata.get("amount", 0)
        elif cur_state == "WAIT_RECEIPT_RENEW":
            order_type = "RENEW"
            plan = get_plan_by_id(plan_id)
            plan_title = f"تمدید: {plan['title']}" if plan else "تمدید"
        else:
            plan = get_plan_by_id(plan_id)
            plan_title = plan["title"] if plan else ""

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

    # User Keyboards (Mirza Edition)
    if text in ["🔐 خرید اشتراک", "🛒 خرید اشتراک", "/buy"]:
        clear_user_state(user_id)
        plans = load_plans()
        ikb_rows = []
        for p in plans:
            ikb_rows.append([{"text": f"🛒 {p['title']} — {p['price']:,} تومان", "callback_data": f"plan_{p['id']}"}])
        send_message(user_id, "🌟 <b>پلن مورد نظر خود را برای خرید انتخاب فرمایید:</b>", {"inline_keyboard": ikb_rows})
        return

    if text in ["🔑 اکانت تست", "🎁 تست رایگان ۲۴ ساعته", "🎁 اکانت تست", "🎁 دریافت کانفیگ رایگان", "/test"]:
        clear_user_state(user_id)
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT * FROM trials WHERE user_id = ?", (user_id,))
        already = c.fetchone()
        conn.close()
        if already:
            t_warn = (
                "⚠️ <b>کاربر گرامی؛</b>\n\n"
                "شما قبلاً یک‌بار اکانت تست رایگان دریافت کرده‌اید.\n"
                "جهت خرید اشتراک جدید، از منوی «🔐 خرید اشتراک» استفاده فرمایید."
            )
            send_message(user_id, t_warn, get_main_keyboard(user_id))
            return

        trial_gb = float(get_setting("trial_gb", "1"))
        trial_days = float(get_setting("trial_days", "1"))
        days_str = f"{int(trial_days * 24)} ساعت" if trial_days < 1 else f"{int(trial_days)} روز"

        send_message(user_id, "⏳ در حال ساخت اکانت تست اختصاصی برای شما...")
        res = add_xray_client(user_id, username, traffic_gb=trial_gb, expiry_days=trial_days, is_trial=True)
        if not res:
            send_message(user_id, "❌ متأسفانه در حال حاضر امکان صدور اکانت تست وجود ندارد. لطفاً دقایقی دیگر تلاش فرمایید.", get_main_keyboard(user_id))
            return

        conn = get_db()
        c = conn.cursor()
        c.execute("INSERT OR REPLACE INTO trials (user_id, username, sub_id, created_at) VALUES (?, ?, ?, ?)", (user_id, username, res["sub_id"], int(time.time())))
        conn.commit()
        conn.close()

        t_msg = (
            f"✅ <b>سرویس با موفقیت ایجاد شد</b>\n\n"
            f"👤 نام کاربری سرویس : <code>{res['client_email']}</code>\n"
            f"🌿 نام سرویس: اکانت تست رایگان\n"
            f"⏳ مدت زمان: {days_str}\n"
            f"🗜 حجم سرویس: {trial_gb} گیگابایت\n\n"
            f"<b>لینک اتصال:</b>\n"
            f"<code>{res['sub_url']}</code>\n\n"
            f"⚡️ <b>کانفیگ تک‌خطی VLESS:</b>\n"
            f"<code>{res['vless_link']}</code>\n\n"
            f"🧑‍🦯 شما می‌توانید شیوه اتصال را با فشردن دکمه «📚 آموزش» دریافت کنید یا بارکد QR زیر را اسکن نمایید."
        )
        send_message(user_id, t_msg, get_main_keyboard(user_id))
        qr_bytes = generate_qr_bytes(res["sub_url"])
        send_photo(user_id, qr_bytes, caption="📸 اسکن بارکد QR جهت اتصال مستقیم")
        return

    if text in ["🛍 سرویس های من", "📊 سرویس‌های من", "/services"]:
        clear_user_state(user_id)
        clients = get_user_xray_clients(user_id)
        if not clients:
            send_message(user_id, "شما در حال حاضر هیچ سرویس فعالی ندارید. می‌توانید با دکمه «🔐 خرید اشتراک» یا «🔑 اکانت تست» سرویس دریافت کنید.", get_main_keyboard(user_id))
            return
            
        send_message(user_id, f"🛍 <b>لیست سرویس‌های فعال شما ({len(clients)} سرویس):</b>")
        for cl in clients:
            rem_gb = max(0, cl["total_gb"] - cl["used_gb"])
            st_lbl = "فعال 🟢" if cl["enable"] else "غیرفعال 🔴"
            exp_date = datetime.datetime.fromtimestamp(cl["expiry_time_ms"]/1000).strftime('%Y-%m-%d %H:%M') if cl["expiry_time_ms"] > 0 else "نامحدود"
            c_txt = (
                f"👤 <b>نام کاربری:</b> <code>{cl['email']}</code>\n"
                f"📊 مصرف: <b>{cl['used_gb']} GB</b> از <b>{cl['total_gb']} GB</b> (باقیمانده: {rem_gb:.2f} GB)\n"
                f"⏳ انقضا: <code>{exp_date}</code>\n"
                f"وضعیت: {st_lbl}\n\n"
                f"🔗 <b>لینک سابسکریپشن:</b>\n<code>{cl['sub_url']}</code>"
            )
            ikb = {
                "inline_keyboard": [
                    [
                        {"text": "♻️ تمدید سرویس", "callback_data": f"renew_{cl['email']}"},
                        {"text": "📸 دریافت بارکد QR", "callback_data": f"qr_{cl['email']}"}
                    ]
                ]
            }
            send_message(user_id, c_txt, ikb)
        return

    if text in ["♻️ تمدید سرویس", "🔄 تمدید اشتراک", "🔄 تمدید سرویس", "/renew", "/extend"]:
        clear_user_state(user_id)
        clients = get_user_xray_clients(user_id)
        if not clients:
            send_message(user_id, "شما اشتراکی برای تمدید ندارید. لطفاً ابتدا از منوی «🔐 خرید اشتراک» خرید فرمایید.", get_main_keyboard(user_id))
            return
        ikb_rows = []
        for cl in clients:
            ikb_rows.append([{"text": f"🔄 تمدید سرویس {cl['email']}", "callback_data": f"renew_{cl['email']}"}])
        send_message(user_id, "سرویسی که قصد تمدید آن را دارید انتخاب کنید:", {"inline_keyboard": ikb_rows})
        return

    if text in ["🏦 کیف پول + شارژ", "💰 کیف پول من", "/wallet"]:
        clear_user_state(user_id)
        u = get_user(user_id)
        bal = u["balance"] if u else 0
        w_txt = (
            f"🏦 <b>کیف پول حساب کاربری شما</b>\n\n"
            f"💵 موجودی فعلی: <b>{bal:,} تومان</b>\n\n"
            f"با شارژ کیف پول، می‌توانید تمام پلن‌ها و تمدیدها را به صورت **آنی و بدون معطلی برای بررسی فیش** خریداری فرمایید."
        )
        ikb = {
            "inline_keyboard": [
                [{"text": "💰 افزایش موجودی", "callback_data": "topup_menu"}],
                [{"text": "🔐 خرید اشتراک از موجودی", "callback_data": "list_plans"}]
            ]
        }
        send_message(user_id, w_txt, ikb)
        return

    if text in ["👥 زیر مجموعه گیری", "👥 زیرمجموعه‌گیری", "/affiliates"]:
        clear_user_state(user_id)
        bot_info = tg_request("getMe")
        bot_uname = bot_info.get("result", {}).get("username", "nova_panel_bot") if bot_info else "bot"
        ref_link = f"https://t.me/{bot_uname}?start=ref_{user_id}"
        
        conn = get_db()
        c = conn.cursor()
        c.execute("SELECT COUNT(*) FROM referrals WHERE referrer_id = ?", (user_id,))
        ref_count = c.fetchone()[0]
        c.execute("SELECT SUM(amount) FROM referral_rewards WHERE referrer_id = ?", (user_id,))
        rew_total = c.fetchone()[0] or 0
        conn.close()
        
        pct = get_setting("referral_percent", "10")
        ref_msg = (
            f"👥 <b>سیستم همکاری در فروش و زیرمجموعه‌گیری</b>\n\n"
            f"با دعوت از دوستانتان به ربات، <b>{pct}٪ از هر خرید آن‌ها</b> برای همیشه به کیف پول شما واریز می‌شود!\n\n"
            f"🔗 <b>لینک اختصاصی دعوت شما:</b>\n<code>{ref_link}</code>\n\n"
            f"📊 <b>آمار شما:</b>\n"
            f"▫️ تعداد افراد دعوت‌شده: <b>{ref_count} نفر</b>\n"
            f"▫️ مجموع پاداش دریافتی: <b>{rew_total:,} تومان</b>\n\n"
            f"<i>لینک بالا را برای دوستانتان فوروارد کنید.</i>"
        )
        send_message(user_id, ref_msg, get_main_keyboard(user_id))
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
            "🛡 بدون قطعی و سازگار با اینترنت‌های همراه و ثابت\n"
            "🚀 تحویل آنی پس از پرداخت\n\n"
            "برای خرید هر یک از پلن‌ها، دکمه «🔐 خرید اشتراک» را لمس کنید."
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
            rem_hrs = int((86400 - (now - row["last_spun_at"])) / 3600) + 1
            conn.close()
            send_message(user_id, f"⏳ <b>شما امروز گردونه شانس را چرخانده‌اید!</b>\n\nلطفاً حدود <b>{rem_hrs} ساعت دیگر</b> مجدداً شانس خود را امتحان کنید. 🎲", get_main_keyboard(user_id))
            return
            
        c.execute("INSERT OR REPLACE INTO wheel_luck (user_id, last_spun_at) VALUES (?, ?)", (user_id, now))
        conn.commit()
        conn.close()
        
        prizes = [
            {"type": "wallet", "amount": 5000, "text": "💰 ۵,۰۰۰ تومان شارژ هدیه کیف پول"},
            {"type": "wallet", "amount": 10000, "text": "💰 ۱۰,۰۰۰ تومان شارژ هدیه کیف پول"},
            {"type": "wallet", "amount": 20000, "text": "🔥 ۲۰,۰۰۰ تومان شارژ هدیه ویژه کیف پول"},
            {"type": "discount", "percent": 15, "text": "🎁 کد تخفیف ۱۵ درصدی"},
            {"type": "discount", "percent": 25, "text": "🎁 کد تخفیف ۲۵ درصدی طلایی"},
        ]
        chosen = random.choice(prizes)
        if chosen["type"] == "wallet":
            new_bal = change_user_balance(user_id, chosen["amount"])
            msg = (
                f"🎉 <b>تبریک! گردونه شانس چرخید و برنده شدید:</b>\n\n"
                f"{chosen['text']}\n\n"
                f"💵 موجودی فعلی کیف پول شما: <b>{new_bal:,} تومان</b>"
            )
        else:
            code = f"LUCK-{random.randint(1000, 9999)}"
            conn = get_db()
            c = conn.cursor()
            c.execute("INSERT OR REPLACE INTO discount_codes (code, percent, max_uses, used_count) VALUES (?, ?, 1, 0)", (code, chosen["percent"]))
            conn.commit()
            conn.close()
            msg = (
                f"🎉 <b>تبریک! گردونه شانس چرخید و برنده شدید:</b>\n\n"
                f"{chosen['text']}\n\n"
                f"🏷 کد تخفیف اختصاصی شما:\n<code>{code}</code>\n\n"
                f"<i>این کد هنگام خرید اشتراک قابل استفاده است.</i>"
            )
        send_message(user_id, msg, get_main_keyboard(user_id))
        return

    if text in ["📚 آموزش", "📱 دانلود برنامه‌ها", "/help"]:
        clear_user_state(user_id)
        dl_msg = (
            f"📚 <b>آموزش اتصال و دانلود نرم‌افزارها:</b>\n\n"
            f"🤖 <b>اندروید:</b>\n"
            f"۱. برنامه <b>v2rayNG</b> یا <b>Sing-box</b> را نصب کنید.\n"
            f"۲. لینک سابسکریپشن را کپی کرده و در برنامه وارد نمایید.\n"
            f"• <a href=\"https://github.com/2dust/v2rayNG/releases\">دانلود v2rayNG از گیت‌هاب</a>\n"
            f"• <a href=\"https://play.google.com/store/apps/details?id=com.v2ray.ang\">دانلود v2rayNG از گوگل‌پلی</a>\n\n"
            f"🍏 <b>آیفون و آیپد (iOS):</b>\n"
            f"۱. برنامه <b>Streisand</b> یا <b>V2Box</b> را نصب کنید.\n"
            f"۲. علامت + را زده و لینک اشتراک را اضافه کنید.\n"
            f"• <a href=\"https://apps.apple.com/app/streisand/id6450534064\">دانلود Streisand (اپ استور)</a>\n"
            f"• <a href=\"https://apps.apple.com/app/v2box-v2ray-client/id6446814043\">دانلود V2Box (اپ استور)</a>\n\n"
            f"💻 <b>ویندوز (PC):</b>\n"
            f"• <a href=\"https://github.com/2dust/v2rayN/releases\">دانلود v2rayN از گیت‌هاب</a>"
        )
        send_message(user_id, dl_msg, get_main_keyboard(user_id))
        return

    if text in ["☎️ پشتیبانی", "📞 پشتیبانی", "/support"]:
        clear_user_state(user_id)
        sup = get_setting("support_username", "")
        cfg = load_config()
        if not sup:
            sup = cfg.get("support_username", "")
        if sup:
            sup_clean = sup.lstrip("@")
            sup_msg = (
                f"☎️ <b>پشتیبانی:</b>\n\n"
                f"جهت پیگیری سفارشات، سوالات پیش از خرید یا گزارش اختلال، با پشتیبانی در ارتباط باشید:\n"
                f"👉 @{sup_clean}"
            )
        else:
            sup_msg = "☎️ <b>پشتیبانی:</b>\n\nجهت ارتباط با پشتیبانی، پیام خود را همین‌جا ارسال فرمایید تا به دست مدیریت برسد."
        send_message(user_id, sup_msg, get_main_keyboard(user_id))
        return

def main():
    logger.info("Starting DARK SHOP BOT (Mirza Edition)...")
    init_db()
    
    # Start auto notifier thread
    t = threading.Thread(target=background_notifier, daemon=True)
    t.start()
    
    token = get_bot_token()
    if not token:
        logger.error("No bot token set in config!")
        sys.exit(1)

    # Purge any stale webhook from previous bot sessions so getUpdates works cleanly
    try:
        del_res = tg_request("deleteWebhook", {"drop_pending_updates": False})
        logger.info(f"Purged webhook status: {del_res}")
    except Exception as e:
        logger.warning(f"Error purging webhook: {e}")

    # Check connection and verify bot identity
    me = tg_request("getMe")
    if me and me.get("ok"):
        bot_info = me.get("result", {})
        logger.info(f"Bot connected successfully as @{bot_info.get('username')} (Name: {bot_info.get('first_name')}, ID: {bot_info.get('id')})")
    else:
        logger.error(f"Failed to verify bot token with Telegram: {me}")
        
    last_update_id = 0
    logger.info("Bot polling loop running successfully.")
    while True:
        try:
            res = tg_request("getUpdates", {"offset": last_update_id + 1, "timeout": 20})
            if res and res.get("ok"):
                for upd in res.get("result", []):
                    last_update_id = upd["update_id"]
                    try:
                        if "message" in upd:
                            handle_message(upd["message"])
                        elif "callback_query" in upd:
                            handle_callback_query(upd["callback_query"])
                    except Exception as handler_err:
                        logger.error(f"Exception handling update {last_update_id}: {handler_err}")
            time.sleep(0.5)
        except Exception as e:
            logger.error(f"Polling loop exception: {e}")
            time.sleep(2)

if __name__ == "__main__":
    main()

