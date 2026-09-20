#!/bin/bash
# ============================================================
#  NOVA X PANEL  —  Installer
#  Independent panel built on top of 3x-ui (GPLv3).
#  Repository: https://github.com/NOVA-X-PANEL/NOVA-X-PANEL
#
#  Usage:
#    bash install.sh              # install latest release (or build from source)
#    bash install.sh v1.2.3       # install a specific release tag
#    bash install.sh dev-latest   # install the rolling dev build
#    XUI_NONINTERACTIVE=1 bash install.sh   # unattended install
# ============================================================

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
blue='\033[0;34m'
plain='\033[0m'

PANEL_NAME="NOVA X PANEL"
REPO="NOVA-X-PANEL/NOVA-X-PANEL"
REPO_URL="https://github.com/${REPO}"

# Branded, fully independent paths (override with env vars if desired).
main_folder="${XUI_MAIN_FOLDER:=/usr/local/nova-x-panel}"
db_folder="${XUI_DB_FOLDER:=/etc/nova-x-panel}"
log_folder="${XUI_LOG_FOLDER:=/var/log/nova-x-panel}"
bin_folder="${main_folder}/bin"

service_name="nova-x-panel"
service_file="/etc/systemd/system/${service_name}.service"
env_file="/etc/default/${service_name}"
result_file="${db_folder}/install-result.env"
cli_path="/usr/bin/nova"

TARGET_TAG="${1:-}"

# ---------------------------------------------------------------------------
# helpers
# ---------------------------------------------------------------------------
info()  { echo -e "${green}[INFO]${plain} $*"; }
warn()  { echo -e "${yellow}[WARN]${plain} $*"; }
err()   { echo -e "${red}[ERR ]${plain} $*"; }

confirm_or_exit() {
    [[ $EUID -ne 0 ]] && { err "Please run this script as root."; exit 1; }
}

# ---------------------------------------------------------------------------
# OS / architecture detection
# ---------------------------------------------------------------------------
detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        release="$ID"
        release_like="${ID_LIKE:-}"
    elif [[ -f /usr/lib/os-release ]]; then
        . /usr/lib/os-release
        release="$ID"
        release_like="${ID_LIKE:-}"
    else
        err "Unsupported OS: cannot read /etc/os-release"; exit 1
    fi
    info "Detected OS: $release"
}

arch() {
    case "$(uname -m)" in
        x86_64 | x64 | amd64) echo 'amd64' ;;
        i*86 | x86)           echo '386' ;;
        armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
        armv7* | armv7 | arm) echo 'armv7' ;;
        armv6* | armv6)       echo 'armv6' ;;
        armv5* | armv5)       echo 'armv5' ;;
        s390x)                echo 's390x' ;;
        *) err "Unsupported CPU architecture: $(uname -m)"; exit 1 ;;
    esac
}

xray_arch() {
    case "$1" in
        amd64) echo '64' ;;
        386)   echo '32' ;;
        arm64) echo 'arm64-v8a' ;;
        armv7) echo 'arm32-v7a' ;;
        armv6) echo 'arm32-v6' ;;
        armv5) echo 'arm32-v5' ;;
        s390x) echo 's390x' ;;
        *)     echo '64' ;;
    esac
}

# ---------------------------------------------------------------------------
# dependency installation
# ---------------------------------------------------------------------------
install_pkgs() {
    local pkgs="curl wget unzip tar tzdata socat ca-certificates jq cron"
    info "Installing dependencies: $pkgs"
    export DEBIAN_FRONTEND=noninteractive
    if command -v apt-get >/dev/null 2>&1; then
        apt-get update -y
        apt-get install -y --no-install-recommends $pkgs || warn "Some packages failed to install"
    elif command -v dnf >/dev/null 2>&1; then
        dnf install -y epel-release >/dev/null 2>&1 || true
        dnf install -y $pkgs || warn "Some packages failed to install"
    elif command -v yum >/dev/null 2>&1; then
        yum install -y epel-release >/dev/null 2>&1 || true
        yum install -y $pkgs || warn "Some packages failed to install"
    elif command -v pacman >/dev/null 2>&1; then
        pacman -Sy --noconfirm $pkgs || warn "Some packages failed to install"
    elif command -v apk >/dev/null 2>&1; then
        apk add --no-cache $pkgs || warn "Some packages failed to install"
    else
        warn "Unknown package manager; please ensure curl, unzip, tar, socat, jq are installed."
    fi
    # Enable cron service if present
    systemctl enable cron >/dev/null 2>&1 || systemctl enable crond >/dev/null 2>&1 || true
}

# ---------------------------------------------------------------------------
# fetch latest release tag from GitHub (empty when none)
# ---------------------------------------------------------------------------
latest_tag() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
        | jq -r '.tag_name // empty' 2>/dev/null
}

# ---------------------------------------------------------------------------
# install from a published release asset, returns 0 on success
# ---------------------------------------------------------------------------
install_from_release() {
    local tag="$1" a="$2"
    local asset="x-ui-linux-${a}.tar.gz"
    local url base tmp
    if [[ -n "$tag" ]]; then
        base="https://github.com/${REPO}/releases/download/${tag}/${asset}"
    else
        base="https://github.com/${REPO}/releases/latest/download/${asset}"
    fi
    tmp="$(mktemp -d)"
    info "Downloading ${base}"
    if ! curl -fsSL -o "${tmp}/${asset}" "$base"; then
        rm -rf "$tmp"; return 1
    fi
    # verify sha256 when available
    if curl -fsSL -o "${tmp}/${asset}.sha256" "${base}.sha256" 2>/dev/null; then
        ( cd "$tmp" && sha256sum -c "${asset}.sha256" >/dev/null 2>&1 ) \
            && info "Checksum verified." \
            || { err "Checksum mismatch — aborting."; rm -rf "$tmp"; return 1; }
    fi
    mkdir -p "${tmp}/extract"
    tar -xzf "${tmp}/${asset}" -C "${tmp}/extract"
    # Keep any custom files already present in bin/ (e.g. user-supplied *.dat).
    if [[ -d "${bin_folder}" ]]; then
        cp -a "${bin_folder}" "${tmp}/bin_backup"
    fi
    mkdir -p "${main_folder}"
    if [[ -d "${tmp}/extract/x-ui" ]]; then
        cp -a "${tmp}/extract/x-ui/." "${main_folder}/"
    else
        cp -a "${tmp}/extract/." "${main_folder}/"
    fi
    if [[ -d "${tmp}/bin_backup" ]]; then
        mkdir -p "${bin_folder}"
        local b
        for b_path in "${tmp}/bin_backup"/*; do
            [[ -e "$b_path" ]] || continue
            b="$(basename "$b_path")"
            [[ -e "${bin_folder}/${b}" ]] || cp -a "$b_path" "${bin_folder}/${b}"
        done
    fi
    rm -rf "$tmp"
    return 0
}

# ---------------------------------------------------------------------------
# build from source (fallback when no release exists)
# ---------------------------------------------------------------------------
build_from_source() {
    info "No prebuilt release available. Building ${PANEL_NAME} from source..."
    install_git
    install_go
    install_node

    local src="/tmp/nova-x-panel-src"
    rm -rf "$src"
    if [[ -n "$TARGET_TAG" ]]; then
        git clone --depth 1 --branch "$TARGET_TAG" "https://github.com/${REPO}.git" "$src" 2>/dev/null \
            || git clone --depth 1 "https://github.com/${REPO}.git" "$src"
    else
        git clone --depth 1 "https://github.com/${REPO}.git" "$src"
    fi

    info "Building frontend bundle..."
    ( cd "${src}/frontend" && npm ci --no-audit --no-fund && npm run build ) \
        || { err "Frontend build failed."; exit 1; }

    info "Building panel binary..."
    mkdir -p "${src}/internal/web/dist"
    ( cd "$src" && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o "build/x-ui" . ) \
        || { err "Go build failed."; exit 1; }

    mkdir -p "${main_folder}"
    install -m 0755 "${src}/build/x-ui" "${main_folder}/x-ui"
    rm -rf "$src"
    return 0
}

install_git()  { command -v git  >/dev/null 2>&1 || pkg_install "git";  }
install_go()   { command -v go   >/dev/null 2>&1 || { pkg_install "golang-go" || install_go_tarball; }; }
install_node() {
    if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
        if command -v apt-get >/dev/null 2>&1; then
            curl -fsSL https://deb.nodesource.com/setup_22.x | bash - >/dev/null 2>&1
            apt-get install -y nodejs >/dev/null 2>&1
        else
            pkg_install "nodejs npm"
        fi
    fi
}

pkg_install() {
    if command -v apt-get >/dev/null 2>&1; then apt-get install -y $1
    elif command -v dnf >/dev/null 2>&1; then dnf install -y $1
    elif command -v yum >/dev/null 2>&1; then yum install -y $1
    elif command -v pacman >/dev/null 2>&1; then pacman -S --noconfirm $1
    elif command -v apk >/dev/null 2>&1; then apk add --no-cache $1
    else return 1; fi
}

install_go_tarball() {
    local ver="1.23.4" a
    a="$(arch)"
    info "Installing Go ${ver} from tarball..."
    curl -fsSL "https://go.dev/dl/go${ver}.linux-${a}.tar.gz" -o /tmp/go.tar.gz || { err "Go download failed"; exit 1; }
    rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tar.gz && rm -f /tmp/go.tar.gz
    export PATH="$PATH:/usr/local/go/bin"
    echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
}

# ---------------------------------------------------------------------------
# Xray-core
# ---------------------------------------------------------------------------
ensure_xray() {
    local a xa tmp url
    a="$(arch)"; xa="$(xray_arch "$a")"
    if [[ -x "${bin_folder}/xray-linux-${a}" ]]; then
        info "Xray-core already present."
        return 0
    fi
    mkdir -p "${bin_folder}"
    tmp="$(mktemp -d)"
    info "Downloading Xray-core for linux-${xa}..."
    url="https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-${xa}.zip"
    if curl -fsSL -o "${tmp}/xray.zip" "$url"; then
        unzip -o "${tmp}/xray.zip" -d "$tmp" >/dev/null 2>&1
        install -m 0755 "${tmp}/xray" "${bin_folder}/xray-linux-${a}"
        info "Xray-core installed."
    else
        warn "Could not download Xray-core automatically. Place the binary at ${bin_folder}/xray-linux-${a}"
    fi
    rm -rf "$tmp"
}

# ---------------------------------------------------------------------------
# service + env + CLI
# ---------------------------------------------------------------------------
write_env_file() {
    mkdir -p "${main_folder}" "${db_folder}" "${log_folder}"
    cat > "${env_file}" <<EOF
# Environment for ${PANEL_NAME} (managed by install.sh)
XUI_MAIN_FOLDER=${main_folder}
XUI_BIN_FOLDER=${bin_folder}
XUI_DB_FOLDER=${db_folder}
XUI_LOG_FOLDER=${log_folder}
XRAY_VMESS_AEAD_FORCED=false
EOF
    chmod 640 "${env_file}"
}

write_service() {
    cat > "${service_file}" <<EOF
[Unit]
Description=${PANEL_NAME} Service
After=network.target
Wants=network.target
StartLimitIntervalSec=180
StartLimitBurst=10

[Service]
EnvironmentFile=-${env_file}
Environment="XRAY_VMESS_AEAD_FORCED=false"
Type=simple
WorkingDirectory=${main_folder}/
ExecStart=${main_folder}/x-ui
ExecReload=/bin/kill -USR1 \$MAINPID
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable "${service_name}" >/dev/null 2>&1
}

install_cli() {
    if [[ -f "$(dirname "$0")/nova-panel.sh" ]]; then
        install -m 0755 "$(dirname "$0")/nova-panel.sh" "${main_folder}/nova-panel.sh"
    fi
    if [[ -f "${main_folder}/nova-panel.sh" ]]; then
        ln -sf "${main_folder}/nova-panel.sh" "${cli_path}"
    fi
}

# ---------------------------------------------------------------------------
# credentials
# ---------------------------------------------------------------------------
random_string() { head -c 16 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c "${1:-12}"; }

set_credentials() {
    local user pass port base
    user="${XUI_USERNAME:-$(random_string 8)}"
    pass="${XUI_PASSWORD:-$(random_string 12)}"
    port="${XUI_PANEL_PORT:-$(shuf -i 2000-65000 -n 1 2>/dev/null || echo 2053)}"
    base="${XUI_WEB_BASE_PATH:-}"
    "${main_folder}/x-ui" setting -username "$user" -password "$pass" >/dev/null 2>&1
    "${main_folder}/x-ui" setting -port "$port" >/dev/null 2>&1
    if [[ -n "$base" ]]; then
        "${main_folder}/x-ui" setting -webBasePath "$base" >/dev/null 2>&1
    fi
    export NOVA_USER="$user" NOVA_PASS="$pass" NOVA_PORT="$port"
    {
        echo "XUI_USERNAME=$user"
        echo "XUI_PASSWORD=$pass"
        echo "XUI_PANEL_PORT=$port"
        echo "XUI_WEB_BASE_PATH=$base"
    } > "${result_file}"
    chmod 600 "${result_file}"
}

print_summary() {
    local ip port user_display pass_display
    ip="$(curl -fsSL --max-time 5 https://api.ipify.org 2>/dev/null || echo '<server-ip>')"
    port="$("${main_folder}/x-ui" setting -show 2>/dev/null | grep -i port | head -1 | grep -oE '[0-9]+' | head -1)"
    user_display="${NOVA_USER:-see ${result_file}}"
    pass_display="${NOVA_PASS:-see ${result_file}}"
    echo
    echo -e "${green}==================== ${PANEL_NAME} installed ====================${plain}"
    echo -e " Panel URL   : ${blue}http://${ip}:${port:-2053}${plain}"
    echo -e " Username    : ${blue}${user_display}${plain}"
    echo -e " Password    : ${blue}${pass_display}${plain}"
    echo -e " Config file : ${result_file}"
    echo -e " Manage with : ${blue}nova${plain}"
    echo -e "${green}================================================================${plain}"
}

# ---------------------------------------------------------------------------
# main
# ---------------------------------------------------------------------------
main() {
    confirm_or_exit
    detect_os
    local a; a="$(arch)"
    echo -e "${blue}${PANEL_NAME} installer${plain}  (arch: ${a})"

    install_pkgs

    mkdir -p "${main_folder}" "${db_folder}" "${log_folder}" "${bin_folder}"

    local tag="$TARGET_TAG"
    if [[ -z "$tag" ]]; then
        tag="$(latest_tag)"
        [[ -n "$tag" ]] && info "Latest release: $tag"
    fi

    # A panel already running out of ${main_folder} holds its own binary open;
    # replacing it in place fails with ETXTBSY ("Text file busy"). Stop it first
    # and let the restart below bring the new build up. Skipped on a fresh box.
    if systemctl is-active --quiet "${service_name}" 2>/dev/null; then
        info "Stopping the running ${PANEL_NAME} service before updating files..."
        systemctl stop "${service_name}" || true
    fi

    if ! install_from_release "$tag" "$a"; then
        warn "Release install unavailable — falling back to source build."
        build_from_source
    fi

    chmod +x "${main_folder}/x-ui" 2>/dev/null || true
    ensure_xray
    write_env_file
    write_service
    install_cli
    set_credentials

    systemctl restart "${service_name}"
    sleep 2

    if systemctl is-active --quiet "${service_name}"; then
        info "${PANEL_NAME} service is running."
    else
        warn "Service did not start. Check: journalctl -u ${service_name} -e"
    fi

    print_summary
}

main "$@"
