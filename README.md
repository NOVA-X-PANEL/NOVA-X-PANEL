<div align="center">

**English** · [فارسی](README.fa_IR.md) · [العربية](README.ar_EG.md) · [中文](README.zh_CN.md) · [Español](README.es_ES.md) · [Русский](README.ru_RU.md) · [Türkçe](README.tr_TR.md)

<br/>

<img src="./media/nova-banner.svg" alt="NOVA X PANEL — advanced web control panel for Xray-core" width="100%" />

<br/>
<br/>

**An advanced, open-source web control panel for [Xray-core](https://github.com/XTLS/Xray-core).**
Deploy, configure and monitor VLESS, VMess, Trojan, Shadowsocks, WireGuard, AmneziaWG, TUIC v5, Hysteria2 and MTProto — from a single VPS to multi-node fleets.

<br/>

[![Release](https://img.shields.io/github/v/release/NOVA-X-PANEL/NOVA-X-PANEL?style=flat-square&label=release&color=8b5cf6)](https://github.com/NOVA-X-PANEL/NOVA-X-PANEL/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/NOVA-X-PANEL/NOVA-X-PANEL/release.yml?style=flat-square&label=build)](https://github.com/NOVA-X-PANEL/NOVA-X-PANEL/actions)
[![Downloads](https://img.shields.io/github/downloads/NOVA-X-PANEL/NOVA-X-PANEL/total?style=flat-square&label=downloads&color=22d3ee)](https://github.com/NOVA-X-PANEL/NOVA-X-PANEL/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/NOVA-X-PANEL/NOVA-X-PANEL?style=flat-square&label=go&color=4f7cff)](go.mod)
[![React](https://img.shields.io/badge/React-19-4f7cff?style=flat-square&logo=react&logoColor=white)](frontend)
[![License](https://img.shields.io/badge/license-GPL--3.0-8b5cf6?style=flat-square)](LICENSE)
[![PRs](https://img.shields.io/badge/PRs-welcome-22d3ee?style=flat-square)](#-contributing)
[![Stars](https://img.shields.io/github/stars/NOVA-X-PANEL/NOVA-X-PANEL?style=flat-square&label=stars&color=8b5cf6)](https://github.com/NOVA-X-PANEL/NOVA-X-PANEL/stargazers)

<br/>

[**Quick start**](#en-quick-start) &nbsp;·&nbsp;
[**Features**](#en-features) &nbsp;·&nbsp;
[**Screenshots**](#en-screenshots) &nbsp;·&nbsp;
[**Protocols**](#en-protocols) &nbsp;·&nbsp;
[**Documentation**](https://docs.sanaei.dev) &nbsp;·&nbsp;
[**Configuration**](#en-config) &nbsp;·&nbsp;
[**Contributing**](#-contributing)

</div>

---

> [!IMPORTANT]
> This project is intended for **personal use only**. Please do not use it for illegal purposes or in a production environment.

## ✨ Why NOVA X PANEL

**NOVA X PANEL** is an **independent** panel maintained in its own repository, built on top of the open-source [3x-ui](https://github.com/MHSanaei/3x-ui) project (which itself descends from the original X-UI). It keeps the broad protocol support, the improved stability, the per-client traffic accounting and the quality-of-life features of its upstream, then adds its own **branding, packaging and a bespoke "Nova Glass" interface**.

- 🚀 **One panel, every protocol** — thirteen inbound types, modern transports and REALITY/XTLS out of the box.
- 🧩 **Built for scale** — manage and clone inbounds across many servers from a single panel.
- 📊 **Accurate accounting** — per-inbound, per-client and per-outbound traffic with live online status.
- 🎨 **A panel that looks the part** — a custom violet→blue glass theme, 13 UI languages, dark and ultra-dark modes.
- 🔒 **Security-minded** — scoped API tokens, RBAC admin roles, 2FA, HWID limits and Fail2ban integration.
- 📦 **Painless to run** — a single installer for 7 Linux architectures plus Windows, SQLite by default, PostgreSQL when you need it.

<a name="en-quick-start"></a>

## 🚀 Quick start

```bash
bash <(curl -Ls https://raw.githubusercontent.com/NOVA-X-PANEL/NOVA-X-PANEL/main/install.sh)
```

Install a specific release (e.g. `v1.6.0`):

```bash
bash <(curl -Ls https://raw.githubusercontent.com/NOVA-X-PANEL/NOVA-X-PANEL/main/install.sh) v1.6.0
```

Install the rolling **dev** build (latest per-commit pre-release from `main` — not a stable release):

```bash
bash <(curl -Ls https://raw.githubusercontent.com/NOVA-X-PANEL/NOVA-X-PANEL/main/install.sh) dev-latest
```

During installation a random username, password and access path are generated. Afterwards run **`nova`** (or `nova-x-panel`) to open the management menu, where you can start/stop the service, view or reset your credentials, manage SSL certificates and more.

Every release asset ships with a `.sha256` sum next to it; both `install.sh` and the updater verify the archive against it and abort on a mismatch.

### Unattended install

The installer also runs **non-interactively** for cloud-init. Set `XUI_NONINTERACTIVE=1` (or pipe with no TTY) and it installs end-to-end with zero prompts, generating random credentials and writing them to `/etc/nova-x-panel/install-result.env`. See [`deploy/`](deploy/) for:

- [Cloud-init user-data](deploy/cloud-init/) — unattended install on any cloud (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [Hetzner Cloud notes](deploy/marketplace/hetzner/) — cloud-init deployment on Hetzner

<a name="en-screenshots"></a>

## 🖼 Screenshots

<details>
<summary><b>Click to expand the gallery</b></summary>

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

<a name="en-features"></a>

## 🧰 Features

<table>
<tr>
<td width="50%" valign="top">

**Connectivity**
- **Multi-protocol inbounds** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, AmneziaWG, TUIC v5, Hysteria2, MTProto, HTTP, SOCKS (Mixed), Dokodemo-door / Tunnel and TUN.
- **Modern transports & security** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade and XHTTP; secured with TLS, XTLS and REALITY.
- **Fallbacks** — serve multiple protocols on one port (e.g. VLESS and Trojan on 443).
- **AmneziaWG built-in** — DPI-resistant WireGuard on a userspace network stack; no kernel module, DKMS or extra packages.
- **TUIC v5 sidecar** — high-performance QUIC proxy with native UDP relay metering, 0-RTT handshakes and BBR.
- **MTProto proxies** — per-client FakeTLS secrets, ad-tags and quotas applied live without dropping connections.

</td>
<td width="50%" valign="top">

**Operations**
- **Per-client management** — traffic quotas, expiry dates, IP limits with trusted-address exemptions, HWID device limits, scheduled renewal cycles, live online status, share links, QR codes and subscriptions.
- **Traffic statistics** — detailed stats per inbound, per client and per outbound, with reset controls.
- **Multi-node support** — manage and scale many servers from one panel, including cloning inbounds to other nodes.
- **Outbound & routing** — WARP, NordVPN, PIA, custom routing rules, load balancers with fallback chaining and proxy chaining; bundled geosite/geoip categories browsable from the rule editor.
- **Built-in subscription server** — raw, JSON and Clash output auto-selected by User-Agent, plus [custom page templates](docs/custom-subscription-templates.md).
- **Telegram & Discord bots** — remote monitoring and management.

</td>
</tr>
<tr>
<td width="50%" valign="top">

**Security & access**
- **Admin roles (RBAC)** — granular per-resource permissions and per-admin client scoping.
- **Scoped API tokens** — optionally expiring, with an in-panel API reference.
- **Fail2ban integration** — enforces per-client IP limits.
- **HWID device limits** — cap how many devices a client may use.

</td>
<td width="50%" valign="top">

**Platform**
- **Flexible storage** — SQLite (default) or PostgreSQL.
- **13 UI languages** with dark, light and ultra-dark themes.
- **Installable (PWA)** — pin the panel to your desktop or phone home screen.
- **RESTful API** — full OpenAPI reference generated from the Go sources.

</td>
</tr>
</table>

<a name="en-protocols"></a>

## 🌐 Supported protocols

| Category | Supported |
| --- | --- |
| **Inbounds** | VLESS · VMess · Trojan · Shadowsocks · WireGuard · AmneziaWG · TUIC v5 · Hysteria2 · MTProto · HTTP · SOCKS (Mixed) · Dokodemo-door / Tunnel · TUN |
| **Transports** | TCP (Raw) · mKCP · WebSocket · gRPC · HTTPUpgrade · XHTTP |
| **Security** | TLS · XTLS · REALITY · none |
| **Subscriptions** | Raw · JSON · Clash (auto-selected by User-Agent) |
| **Clients** | Xray-core · mihomo · sing-box · mtg-multi |

## 💻 Supported platforms

**Operating systems:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine, and Windows.

**Architectures:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`

<a name="en-config"></a>

## ⚙ Configuration

### Database

NOVA X PANEL supports two backends, chosen during the install:

- **SQLite** (default) — a single file at `/etc/nova-x-panel/x-ui.db`. Zero setup, ideal for small and medium deployments.
- **PostgreSQL** — recommended for high client counts or multi-node setups. The installer can install PostgreSQL locally for you, or accept a DSN to an existing server.

At runtime the backend is selected via environment variables (the installer writes these to `/etc/default/x-ui` for you):

```env
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

<details>
<summary><b>Migrating an existing SQLite install to PostgreSQL</b></summary>

<br/>

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# then set XUI_DB_TYPE and XUI_DB_DSN in /etc/default/x-ui and restart:
systemctl restart x-ui
```

The source SQLite file is left untouched; remove it manually once you have verified the new backend.

</details>

### Docker

The default `docker compose up -d` keeps using SQLite. To run with the bundled PostgreSQL service, uncomment the two `XUI_DB_*` env lines in `docker-compose.yml` and start with the profile:

```bash
docker compose --profile postgres up -d
```

The image bundles Fail2ban (enabled by default) to enforce per-client **IP limits**. Fail2ban bans offenders with `iptables`, which requires the `NET_ADMIN` capability. `docker-compose.yml` already grants it via `cap_add`; if you start the container with `docker run` instead, add the capabilities yourself, otherwise bans are logged but never applied:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

### Environment variables

| Variable | Description | Default |
| --- | --- | --- |
| `XUI_DB_TYPE` | Database backend: `sqlite` or `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL connection string (when `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | Directory for the SQLite database file | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | Maximum open connections (PostgreSQL pool) | — |
| `XUI_DB_MAX_IDLE_CONNS` | Maximum idle connections (PostgreSQL pool) | — |
| `XUI_INIT_WEB_BASE_PATH` | The initial URI path for the web panel | `/` |
| `XUI_ENABLE_FAIL2BAN` | Enable Fail2ban-based IP-limit enforcement | `true` |
| `XUI_LOG_LEVEL` | Log verbosity (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_DEBUG` | Enable debug mode | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Enable the tunnel health monitor (probes a URL and restarts xray after repeated failures; a restart drops all clients) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | Proxy the probe is sent through; point it at a local xray inbound so the probe tests the tunnel (e.g. `socks5://127.0.0.1:1080`). Empty means the probe only checks host connectivity | — |
| `XUI_TUNNEL_HEALTH_URL` | URL probed for tunnel health | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | Interval between probes | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | Per-probe timeout | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | Consecutive failures before a restart is triggered | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | Minimum delay between consecutive restarts | `5m` |
| `NODE_TOKEN_ENCRYPTION` | Encryption at rest for node API tokens: `off`, `migration`, or `required` (note: no `XUI_` prefix) | `off` |
| `XUI_NODE_TOKEN_KEY_FILE` | JSON keyring (mode `0600`) holding the active key id and its base64 32-byte keys | `/etc/x-ui/node_token_key.json` |
| `XUI_NODE_TOKEN_KEY` | A single base64 32-byte key, used only when the key file cannot be loaded | — |

The complete list is on the [environment variables reference](https://docs.sanaei.dev/docs/reference/env-vars).

### Panel management

```bash
nova              # open the interactive management menu
nova start        # start the panel
nova stop         # stop the panel
nova restart      # restart the panel
nova status       # service status
nova settings     # show / change panel settings (port, path, credentials)
nova update       # update to the latest release
```

### 🛠 Under the hood

| Layer | Stack |
| --- | --- |
| Backend | Go 1.27 · Gin · GORM · a managed `Xray-core` child process |
| Frontend | React 19 · Ant Design 6 · Vite 8 · TypeScript |
| Storage | SQLite (CGo) or PostgreSQL |
| Sidecars | `mtg-multi` (MTProto) · `tuic-server` (TUIC v5) |

## 🌍 Supported languages

The panel UI is available in **13 languages**:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## 📚 Documentation & API

Full documentation — installation, configuration, operations and the complete API reference — lives at **[docs.sanaei.dev](https://docs.sanaei.dev)**. The OpenAPI document is also served by the panel itself at `/panel/api/openapi.json`.

## 🤝 Contributing

Contributions are welcome. Please read the [Contributing Guide](CONTRIBUTING.md) before opening an issue or a pull request. Security issues should follow [SECURITY.md](SECURITY.md).

## 🙏 Acknowledgements

NOVA X PANEL builds on the work of others:

- [3x-ui](https://github.com/MHSanaei/3x-ui) and the original X-UI project — the upstream this panel is derived from.
- [Xray-core](https://github.com/XTLS/Xray-core) — the proxy engine.
- [alireza0](https://github.com/alireza0/) — a special thanks.
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (**GPL-3.0**) — routing rules with built-in Iranian domains, focused on security and ad-blocking.
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (**GPL-3.0**) — automatically updated routing rules for blocked domains and addresses in Russia.

### Community tools

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (**MIT**) — manage inbounds, clients, panel settings and Xray configuration as code with Terraform / OpenTofu.
- [3X-UI Manager](https://github.com/yukh975/3X-UI-Manager) (**MIT**) — native Android client: dashboard, inbounds, clients with QR sharing, nodes and multi-panel management. Available on F-Droid.

## ⭐ Support the project

If NOVA X PANEL is useful to you, please give it a **star** — it helps others find the project.

<p align="center">
  <a href="https://github.com/NOVA-X-PANEL/NOVA-X-PANEL/stargazers">
    <img src="./media/nova-logo.png" alt="NOVA X PANEL" width="88" />
  </a>
</p>

## 📈 Star history

<a href="https://www.star-history.com/?repos=NOVA-X-PANEL%2FNOVA-X-PANEL&type=date&legend=top-left">
 <picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=NOVA-X-PANEL/NOVA-X-PANEL&type=date&theme=dark&legend=top-left" />
  <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=NOVA-X-PANEL/NOVA-X-PANEL&type=date&legend=top-left" />
  <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=NOVA-X-PANEL/NOVA-X-PANEL&type=date&legend=top-left" />
 </picture>
</a>

## 📄 License

Released under the [GNU General Public License v3.0](LICENSE).

<div align="center">
<br/>
<sub>Built with ❤️ for the Xray community — <b>NOVA X PANEL</b></sub>
</div>
