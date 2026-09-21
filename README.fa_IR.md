<div align="center" dir="rtl">

[English](README.md) · **فارسی** · [العربية](README.ar_EG.md) · [中文](README.zh_CN.md) · [Español](README.es_ES.md) · [Русский](README.ru_RU.md) · [Türkçe](README.tr_TR.md)

<br/>

<img src="./media/nova-banner.svg" alt="NOVA X PANEL — پنل کنترل وب پیشرفته برای Xray-core" width="100%" />

<br/>
<br/>

**یک پنل کنترل وب پیشرفته و متن‌باز برای [Xray-core](https://github.com/XTLS/Xray-core).**
استقرار، پیکربندی و نظارت بر VLESS، VMess، Trojan، Shadowsocks، WireGuard، AmneziaWG، TUIC v5، Hysteria2 و MTProto — از یک VPS تکی تا ناوگان‌های چندنودی.

<br/>

[![Release](https://img.shields.io/github/v/release/NOVA-X-PANEL/NOVA-X-PANEL?style=flat-square&label=release&color=8b5cf6)](https://github.com/NOVA-X-PANEL/NOVA-X-PANEL/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/NOVA-X-PANEL/NOVA-X-PANEL/release.yml?style=flat-square&label=build)](https://github.com/NOVA-X-PANEL/NOVA-X-PANEL/actions)
[![Downloads](https://img.shields.io/github/downloads/NOVA-X-PANEL/NOVA-X-PANEL/total?style=flat-square&label=downloads&color=22d3ee)](https://github.com/NOVA-X-PANEL/NOVA-X-PANEL/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/NOVA-X-PANEL/NOVA-X-PANEL?style=flat-square&label=go&color=4f7cff)](go.mod)
[![React](https://img.shields.io/badge/React-19-4f7cff?style=flat-square&logo=react&logoColor=white)](frontend)
[![License](https://img.shields.io/badge/license-GPL--3.0-8b5cf6?style=flat-square)](LICENSE)

<br/>

[**شروع سریع**](#fa-quick-start) &nbsp;·&nbsp;
[**ویژگی‌ها**](#fa-features) &nbsp;·&nbsp;
[**اسکرین‌شات‌ها**](#fa-screenshots) &nbsp;·&nbsp;
[**پروتکل‌ها**](#fa-protocols) &nbsp;·&nbsp;
[**مستندات**](https://docs.sanaei.dev/fa) &nbsp;·&nbsp;
[**پیکربندی**](#fa-config)

</div>

---

> [!IMPORTANT]
> این پروژه فقط برای **استفاده‌ی شخصی** در نظر گرفته شده است. لطفاً از آن برای اهداف غیرقانونی یا در محیط تولید (production) استفاده نکنید.

## ✨ چرا NOVA X PANEL

**NOVA X PANEL** یک پنل **مستقل** است که در مخزن اختصاصی خودش نگهداری می‌شود و بر پایه‌ی پروژه‌ی متن‌باز [3x-ui](https://github.com/MHSanaei/3x-ui) (که خودش از X-UI اصلی منشعب شده) ساخته شده است. این پنل پشتیبانی گسترده از پروتکل‌ها، پایداری بهتر، حسابداری ترافیک به‌ازای هر کلاینت و قابلیت‌های رفاهی نسخه‌ی بالادست را حفظ کرده و **برند، بسته‌بندی و رابط کاربری اختصاصی «Nova Glass»** خودش را به آن اضافه می‌کند.

- 🚀 **یک پنل، همه‌ی پروتکل‌ها** — سیزده نوع اینباند، ترنسپورت‌های مدرن و REALITY/XTLS از همان ابتدا.
- 🧩 **ساخته‌شده برای مقیاس** — مدیریت و کلون‌کردن اینباندها روی چندین سرور از یک پنل واحد.
- 📊 **حسابداری دقیق** — ترافیک به‌ازای هر اینباند، هر کلاینت و هر اوتباند، همراه با وضعیت آنلاینِ زنده.
- 🎨 **پنلی که خوش‌ساخت است** — تم شیشه‌ای اختصاصی بنفش→آبی، ۱۳ زبان رابط کاربری و حالت‌های تیره و اولترا-تیره.
- 🔒 **با نگاه به امنیت** — توکن‌های API محدودشده، نقش‌های ادمین (RBAC)، ورود دو مرحله‌ای، محدودیت دستگاه (HWID) و یکپارچگی با Fail2ban.
- 📦 **راه‌اندازی بی‌دردسر** — یک نصب‌کننده برای ۷ معماری لینوکس به‌همراه ویندوز، SQLite به‌صورت پیش‌فرض و PostgreSQL در صورت نیاز.

<a name="fa-quick-start"></a>

## 🚀 شروع سریع

```bash
bash <(curl -Ls https://raw.githubusercontent.com/NOVA-X-PANEL/NOVA-X-PANEL/main/install.sh)
```

نصب یک نسخه‌ی مشخص (مثلاً `v1.10.1`):

```bash
bash <(curl -Ls https://raw.githubusercontent.com/NOVA-X-PANEL/NOVA-X-PANEL/main/install.sh) v1.10.1
```

نصب نسخه‌ی غلتانِ **dev** (آخرین پیش‌انتشار به‌ازای هر کامیت از شاخه‌ی `main` — نه یک انتشار پایدار):

```bash
bash <(curl -Ls https://raw.githubusercontent.com/NOVA-X-PANEL/NOVA-X-PANEL/main/install.sh) dev-latest
```

در حین نصب، یک نام کاربری، رمز عبور و مسیر دسترسی تصادفی تولید می‌شود. پس از نصب، دستور **`nova`** (یا `nova-x-panel`) را اجرا کنید تا منوی مدیریت باز شود؛ در آنجا می‌توانید سرویس را شروع/متوقف کنید، اطلاعات ورود خود را ببینید یا بازنشانی کنید، گواهی‌های SSL را مدیریت کنید و کارهای دیگری انجام دهید.

هر فایل انتشار به‌همراه یک جمع کنترلی `.sha256` در کنارش منتشر می‌شود. هم `install.sh` و هم به‌روزرسان، آرشیو را در برابر آن جمع کنترلی بررسی می‌کنند و در صورت عدم تطابق متوقف می‌شوند.

### نصب بدون نظارت

‏نصب‌کننده به‌صورت **غیرتعاملی** نیز برای cloud-init اجرا می‌شود.
‏`XUI_NONINTERACTIVE=1` را تنظیم کنید (یا بدون TTY از طریق pipe اجرا کنید) تا نصب به‌صورت سرتاسری و بدون هیچ پرسشی انجام شود، اطلاعات ورود تصادفی تولید کرده و آن‌ها را در `/etc/nova-x-panel/install-result.env` می‌نویسد. برای موارد زیر به [`deploy/`](deploy/) مراجعه کنید:

- [user-data مربوط به Cloud-init](deploy/cloud-init/) — نصب بدون نظارت روی هر ابری (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [یادداشت‌های Hetzner Cloud](deploy/marketplace/hetzner/) — استقرار مبتنی بر cloud-init روی Hetzner

<a name="fa-screenshots"></a>

## 🖼 اسکرین‌شات‌ها

<details>
<summary><b>برای دیدن گالری کلیک کنید</b></summary>

<br/>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
    <img alt="Overview" src="./media/01-overview-light.png">
  </picture>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/02-inbounds-dark.png">
    <img alt="Inbounds" src="./media/02-inbounds-light.png">
  </picture>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/03-client-dark.png">
    <img alt="Clients" src="./media/03-client-light.png">
  </picture>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/05-nodes-dark.png">
    <img alt="Nodes" src="./media/05-nodes-light.png">
  </picture>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/06-settings-dark.png">
    <img alt="Settings" src="./media/06-settings-light.png">
  </picture>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/08-api-docs-dark.png">
    <img alt="API docs" src="./media/08-api-docs-light.png">
  </picture>
</p>

</details>

<a name="fa-features"></a>

## 🧰 ویژگی‌ها

<table dir="rtl">
<tr>
<td width="50%" valign="top">

**اتصال**
- **اینباندهای چندپروتکلی** — VLESS، VMess، Trojan، Shadowsocks، WireGuard، AmneziaWG، TUIC v5، Hysteria2، MTProto، HTTP، SOCKS (Mixed)، Dokodemo-door / Tunnel و TUN.
- **ترنسپورت‌ها و امنیت مدرن** — TCP (Raw)، mKCP، WebSocket، gRPC، HTTPUpgrade و XHTTP؛ ایمن‌شده با TLS، XTLS و REALITY.
- **فال‌بک (Fallback)** — ارائه‌ی چند پروتکل روی یک پورت واحد (مثلاً VLESS و Trojan روی پورت 443).
- **‏AmneziaWG داخلی** — نسخه‌ی مقاوم در برابر DPI از WireGuard روی یک پشته‌ی شبکه‌ی فضای کاربر؛ بدون ماژول کرنل، DKMS یا بسته‌ی اضافی.
- **‏TUIC v5** — پراکسی QUIC با کارایی بالا، اندازه‌گیری بومی ترافیک رله UDP، دست‌دادن‌های 0-RTT و BBR.
- **پراکسی‌های MTProto** — سکرت‌های FakeTLS، ad-tag و سهمیه‌ها به‌ازای هر کلاینت که به‌صورت زنده و بدون قطع اتصال‌های موجود اعمال می‌شوند.

</td>
<td width="50%" valign="top">

**بهره‌برداری**
- **مدیریت به‌ازای هر کلاینت** — سهمیه‌ی ترافیک، تاریخ انقضا، محدودیت IP با امکان استثنا کردن آدرس‌های مورد اعتماد، محدودیت دستگاه (HWID)، چرخه‌های تمدید زمان‌بندی‌شده، وضعیت آنلاینِ زنده، لینک‌های اشتراک‌گذاری، کدهای QR و سابسکریپشن‌ها.
- **آمار ترافیک** — آمار تفصیلی به‌ازای هر اینباند، هر کلاینت و هر اوتباند، همراه با کنترل بازنشانی.
- **پشتیبانی از چند نود** — مدیریت و مقیاس‌دهی روی چندین سرور از یک پنل واحد، از جمله کلون‌کردن اینباندها روی نودهای دیگر.
- **اوتباند و مسیریابی** — WARP، NordVPN، PIA، قوانین مسیریابی سفارشی، متعادل‌کننده‌های بار با فال‌بک و زنجیره‌کردن پراکسی اوتباند؛ دسته‌بندی‌های geosite و geoip همراه‌شده مستقیماً از ویرایشگر قوانین قابل مرور هستند.
- **سرور سابسکریپشن داخلی** — خروجی raw، JSON و Clash که بر پایه‌ی User-Agent کلاینت به‌صورت خودکار انتخاب می‌شود، به‌همراه [قالب‌های صفحه‌ی سفارشی](docs/custom-subscription-templates.md).
- **ربات‌های تلگرام و دیسکورد** — نظارت و مدیریت از راه دور.

</td>
</tr>
<tr>
<td width="50%" valign="top">

**امنیت و دسترسی**
- **نقش‌های ادمین (RBAC)** — مجوزهای دقیق به‌ازای هر منبع و محدوده‌ی کلاینتی به‌ازای هر ادمین.
- **توکن‌های API محدودشده** — با انقضای اختیاری، به‌همراه مرجع API درون‌پنل.
- **یکپارچگی با Fail2ban** — اعمال محدودیت IP به‌ازای هر کلاینت.
- **محدودیت دستگاه (HWID)** — تعیین سقف تعداد دستگاه‌های مجاز هر کلاینت.

</td>
<td width="50%" valign="top">

**پلتفرم**
- **ذخیره‌سازی منعطف** — SQLite (پیش‌فرض) یا PostgreSQL.
- **۱۳ زبان رابط کاربری** با تم‌های تیره، روشن و اولترا-تیره.
- **پنل قابل نصب (PWA)** — پنل را به دسکتاپ یا صفحه‌ی اصلی گوشی خود سنجاق کنید.
- **‏RESTful API** — مرجع کامل OpenAPI که از سورس‌های Go تولید می‌شود.

</td>
</tr>
</table>

<a name="fa-protocols"></a>

## 🌐 پروتکل‌های پشتیبانی‌شده

| دسته | موارد پشتیبانی‌شده |
| --- | --- |
| **اینباندها** | VLESS · VMess · Trojan · Shadowsocks · WireGuard · AmneziaWG · TUIC v5 · Hysteria2 · MTProto · HTTP · SOCKS (Mixed) · Dokodemo-door / Tunnel · TUN |
| **ترنسپورت‌ها** | TCP (Raw) · mKCP · WebSocket · gRPC · HTTPUpgrade · XHTTP |
| **امنیت** | TLS · XTLS · REALITY · none |
| **سابسکریپشن‌ها** | ‏Raw · JSON · Clash (انتخاب خودکار بر پایه‌ی User-Agent) |
| **کلاینت‌ها** | Xray-core · mihomo · sing-box · mtg-multi |

## 💻 پلتفرم‌های پشتیبانی‌شده

**سیستم‌عامل‌ها:** Ubuntu، Debian، Armbian، Fedora، CentOS، RHEL، AlmaLinux، Rocky Linux، Oracle Linux، Amazon Linux، Virtuozzo، Arch، Manjaro، Parch، openSUSE (Tumbleweed / Leap)، Alpine و Windows.

**معماری‌ها:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`

<a name="fa-config"></a>

## ⚙ پیکربندی

### پایگاه‌داده

‏NOVA X PANEL از دو بک‌اند پشتیبانی می‌کند که در حین نصب انتخاب می‌شوند:

- **SQLite** (پیش‌فرض) — یک فایل واحد در مسیر `/etc/nova-x-panel/x-ui.db`. بدون نیاز به تنظیمات، ایده‌آل برای استقرارهای کوچک و متوسط.
- **PostgreSQL** — برای تعداد کلاینت بالا یا راه‌اندازی‌های چندنودی توصیه می‌شود. نصب‌کننده می‌تواند PostgreSQL را به‌صورت محلی برایتان نصب کند، یا یک DSN به یک سرور موجود را بپذیرد.

در زمان اجرا، بک‌اند از طریق متغیرهای محیطی انتخاب می‌شود (نصب‌کننده این موارد را برای شما در `/etc/default/x-ui` می‌نویسد):

```env
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

<details>
<summary><b>انتقال یک نصب موجود SQLite به PostgreSQL</b></summary>

<br/>

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# سپس XUI_DB_TYPE و XUI_DB_DSN را در /etc/default/x-ui تنظیم کرده و ری‌استارت کنید:
systemctl restart x-ui
```

فایل اصلی SQLite دست‌نخورده باقی می‌ماند؛ پس از اطمینان از صحت بک‌اند جدید، آن را به‌صورت دستی حذف کنید.

</details>

### Docker

دستور پیش‌فرض `docker compose up -d` همچنان از SQLite استفاده می‌کند. برای اجرا با سرویس PostgreSQL همراه، دو خط متغیر محیطی `XUI_DB_*` را در `docker-compose.yml` از حالت کامنت خارج کنید و با پروفایل زیر اجرا کنید:

```bash
docker compose --profile postgres up -d
```

این ایمیج، Fail2ban را (که به‌صورت پیش‌فرض فعال است) برای اعمال **محدودیت‌های IP** به‌ازای هر کلاینت همراه دارد. ‏Fail2ban متخلفان را با `iptables` مسدود می‌کند که به مجوز `NET_ADMIN` نیاز دارد. فایل `docker-compose.yml` این مجوز را از قبل از طریق `cap_add` می‌دهد؛ اگر به‌جای آن کانتینر را با `docker run` اجرا می‌کنید، خودتان مجوزها را اضافه کنید، در غیر این صورت مسدودسازی‌ها فقط ثبت می‌شوند اما هرگز اعمال نمی‌شوند:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

### متغیرهای محیطی

| متغیر | توضیحات | پیش‌فرض |
| --- | --- | --- |
| `XUI_DB_TYPE` | بک‌اند پایگاه‌داده: `sqlite` یا `postgres` | `sqlite` |
| `XUI_DB_DSN` | رشته‌ی اتصال PostgreSQL (وقتی `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | پوشه‌ی فایل پایگاه‌داده‌ی SQLite | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | حداکثر اتصالات باز (استخر PostgreSQL) | — |
| `XUI_DB_MAX_IDLE_CONNS` | حداکثر اتصالات بی‌کار (استخر PostgreSQL) | — |
| `XUI_INIT_WEB_BASE_PATH` | مسیر URI اولیه برای پنل وب | `/` |
| `XUI_ENABLE_FAIL2BAN` | فعال‌سازی اعمال محدودیت IP مبتنی بر Fail2ban | `true` |
| `XUI_LOG_LEVEL` | سطح گزارش‌گیری (`debug`، `info`، `warning`، `error`) | `info` |
| `XUI_DEBUG` | فعال‌سازی حالت دیباگ | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | فعال‌سازی پایشگر سلامت تونل (یک URL را پروب می‌کند و پس از خطاهای مکرر، xray را ری‌استارت می‌کند؛ یک ری‌استارت همه‌ی کلاینت‌ها را قطع می‌کند) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | پراکسی‌ای که پروب از طریق آن ارسال می‌شود؛ آن را به یک اینباند محلی xray اشاره دهید تا پروب خودِ تونل را آزمایش کند (مثلاً `socks5://127.0.0.1:1080`). خالی بودن یعنی پروب فقط اتصال به هاست را بررسی می‌کند | — |
| `XUI_TUNNEL_HEALTH_URL` | ‏URL ای که برای سلامت تونل پروب می‌شود | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | فاصله‌ی زمانی بین پروب‌ها | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | مهلت زمانی هر پروب | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | تعداد خطاهای متوالی پیش از آن‌که یک ری‌استارت فعال شود | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | حداقل تأخیر بین ری‌استارت‌های متوالی | `5m` |
| `NODE_TOKEN_ENCRYPTION` | رمزگذاری توکن‌های API نود در حالت سکون: `off`، `migration` یا `required` (بدون پیشوند `XUI_`) | `off` |
| `XUI_NODE_TOKEN_KEY_FILE` | حلقه‌کلید JSON (با دسترسی `0600`) شامل شناسه‌ی کلید فعال و کلیدهای ۳۲ بایتی base64 | `/etc/x-ui/node_token_key.json` |
| `XUI_NODE_TOKEN_KEY` | یک کلید ۳۲ بایتی base64 که تنها در صورت بارگذاری‌نشدن فایل کلید استفاده می‌شود | — |

فهرست کامل در [مرجع متغیرهای محیطی](https://docs.sanaei.dev/fa/docs/reference/env-vars) موجود است.

### مدیریت پنل

```bash
nova              # باز کردن منوی مدیریت تعاملی
nova start        # شروع پنل
nova stop         # توقف پنل
nova restart      # ری‌استارت پنل
nova status       # وضعیت سرویس
nova settings     # نمایش / تغییر تنظیمات پنل (پورت، مسیر، اطلاعات ورود)
nova update       # به‌روزرسانی به آخرین انتشار
```

### 🛠 زیر پوست پنل

| لایه | فناوری |
| --- | --- |
| بک‌اند | Go 1.27 · Gin · GORM · یک پروسه‌ی فرزند مدیریت‌شده‌ی `Xray-core` |
| فرانت‌اند | React 19 · Ant Design 6 · Vite 8 · TypeScript |
| ذخیره‌سازی | SQLite (CGo) یا PostgreSQL |
| سایدکارها | `mtg-multi` (‏MTProto) · `tuic-server` (‏TUIC v5) |

## 🌍 زبان‌های پشتیبانی‌شده

رابط کاربری پنل به **۱۳ زبان** در دسترس است:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## 📚 مستندات و API

مستندات کامل — نصب، پیکربندی، بهره‌برداری و مرجع کامل API — در **[docs.sanaei.dev](https://docs.sanaei.dev/fa)** در دسترس است. سند OpenAPI همچنین توسط خود پنل در مسیر `/panel/api/openapi.json` سرو می‌شود.

## 🤝 مشارکت

از مشارکت‌ها استقبال می‌شود. لطفاً پیش از باز کردن issue یا pull request، [راهنمای مشارکت](CONTRIBUTING.md) را مطالعه کنید. مسائل امنیتی باید مطابق [SECURITY.md](SECURITY.md) گزارش شوند.

## 🙏 قدردانی

‏NOVA X PANEL بر تلاش دیگران بنا شده است:

- [3x-ui](https://github.com/MHSanaei/3x-ui) و پروژه‌ی اصلی X-UI — بالادستی که این پنل از آن مشتق شده است.
- [Xray-core](https://github.com/XTLS/Xray-core) — موتور پراکسی.
- [alireza0](https://github.com/alireza0/) — تشکر ویژه.
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (**GPL-3.0**) — قوانین مسیریابی با دامنه‌های ایرانی داخلی، با تمرکز بر امنیت و مسدودسازی تبلیغات.
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (**GPL-3.0**) — قوانین مسیریابی به‌روزشده‌ی خودکار برای دامنه‌ها و آدرس‌های مسدودشده در روسیه.

### ابزارهای جامعه

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (**MIT**) — مدیریت اینباندها، کلاینت‌ها، تنظیمات پنل و پیکربندی Xray به‌صورت کد (Configuration as Code) با Terraform / OpenTofu.
- [3X-UI Manager](https://github.com/yukh975/3X-UI-Manager) (**MIT**) — کلاینت بومی اندروید: داشبورد، اینباندها، کلاینت‌ها با اشتراک‌گذاری QR، نودها و مدیریت چند پنل. در دسترس روی F-Droid.

## ⭐ حمایت از پروژه

اگر NOVA X PANEL برایتان مفید است، لطفاً یک **ستاره** به آن بدهید — این کار به دیگران کمک می‌کند پروژه را پیدا کنند.

<p align="center">
  <a href="https://github.com/NOVA-X-PANEL/NOVA-X-PANEL/stargazers">
    <img src="./media/nova-logo.png" alt="NOVA X PANEL" width="88" />
  </a>
</p>

## 📈 تاریخچه‌ی ستاره‌ها

<a href="https://www.star-history.com/?repos=NOVA-X-PANEL%2FNOVA-X-PANEL&type=date&legend=top-left">
 <picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=NOVA-X-PANEL/NOVA-X-PANEL&type=date&theme=dark&legend=top-left" />
  <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=NOVA-X-PANEL/NOVA-X-PANEL&type=date&legend=top-left" />
  <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=NOVA-X-PANEL/NOVA-X-PANEL&type=date&legend=top-left" />
 </picture>
</a>

## 📄 مجوز

منتشرشده تحت [مجوز عمومی همگانی گنو نسخه‌ی ۳](LICENSE).

<div align="center">
<br/>
<sub>ساخته‌شده با ❤️ برای جامعه‌ی Xray — <b>NOVA X PANEL</b></sub>
</div>
