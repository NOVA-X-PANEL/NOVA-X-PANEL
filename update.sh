#!/bin/bash
# ============================================================
#  NOVA X PANEL  —  Updater
#  Pulls the latest (or a tagged) build from GitHub and swaps
#  the panel binary, then restarts the service.
#
#  Env (set by the panel self-updater when triggered from the UI):
#    XUI_MAIN_FOLDER         install folder (default /usr/local/nova-x-panel)
#    XUI_SERVICE             systemd unit folder (default /etc/systemd/system)
#    XUI_UPDATE_TAG          release tag, empty = latest, "dev-latest" = rolling
#    XUI_UPDATE_RUN_ID       opaque run id echoed into the status file
#    XUI_UPDATE_STATUS_FILE  path to write the JSON status to
#  Can also be run manually: bash update.sh [tag]
# ============================================================

set -o pipefail

PANEL_NAME="NOVA X PANEL"
REPO="NOVA-X-PANEL/NOVA-X-PANEL"

main_folder="${XUI_MAIN_FOLDER:=/usr/local/nova-x-panel}"
service_folder="${XUI_SERVICE:=/etc/systemd/system}"
service_name="nova-x-panel"
status_file="${XUI_UPDATE_STATUS_FILE:-}"
run_id="${XUI_UPDATE_RUN_ID:-}"
tag="${XUI_UPDATE_TAG:-${1:-}}"

red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; plain='\033[0m'
info() { echo -e "${green}[INFO]${plain} $*"; }
warn() { echo -e "${yellow}[WARN]${plain} $*"; }
err()  { echo -e "${red}[ERR ]${plain} $*"; }

write_status() {
    local state="$1" code="$2"
    [[ -z "$status_file" ]] && return 0
    local now; now="$(date +%s)"
    cat > "$status_file" <<EOF
{"runId":"${run_id}","state":"${state}","exitCode":${code},"finishedAt":${now}}
EOF
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
        *) echo 'amd64' ;;
    esac
}

download_release() {
    local a asset base tmp
    a="$(arch)"
    asset="x-ui-linux-${a}.tar.gz"
    if [[ -n "$tag" ]]; then
        base="https://github.com/${REPO}/releases/download/${tag}/${asset}"
    else
        base="https://github.com/${REPO}/releases/latest/download/${asset}"
    fi
    tmp="$(mktemp -d)"
    info "Downloading ${base}"
    if ! curl -fsSL -o "${tmp}/${asset}" "$base"; then rm -rf "$tmp"; return 1; fi
    if curl -fsSL -o "${tmp}/${asset}.sha256" "${base}.sha256" 2>/dev/null; then
        ( cd "$tmp" && sha256sum -c "${asset}.sha256" >/dev/null 2>&1 ) || {
            err "Checksum mismatch"; rm -rf "$tmp"; return 1;
        }
    fi
    mkdir -p "${tmp}/extract"
    tar -xzf "${tmp}/${asset}" -C "${tmp}/extract"
    systemctl stop "${service_name}" 2>/dev/null || true
    if [[ -d "${tmp}/extract/x-ui" ]]; then
        cp -a "${tmp}/extract/x-ui/." "${main_folder}/"
    else
        cp -a "${tmp}/extract/." "${main_folder}/"
    fi
    rm -rf "$tmp"
    return 0
}

build_from_source() {
    info "Building ${PANEL_NAME} from source..."
    command -v git >/dev/null 2>&1 || { command -v apt-get >/dev/null 2>&1 && apt-get install -y git; }
    command -v go  >/dev/null 2>&1 || {
        command -v apt-get >/dev/null 2>&1 && apt-get install -y golang-go; }
    command -v npm >/dev/null 2>&1 || {
        curl -fsSL https://deb.nodesource.com/setup_22.x | bash - >/dev/null 2>&1
        command -v apt-get >/dev/null 2>&1 && apt-get install -y nodejs; }

    local src="/tmp/nova-x-panel-update"
    rm -rf "$src"
    if [[ -n "$tag" && "$tag" != "dev-latest" ]]; then
        git clone --depth 1 --branch "$tag" "https://github.com/${REPO}.git" "$src" || return 1
    else
        git clone --depth 1 "https://github.com/${REPO}.git" "$src" || return 1
    fi

    ( cd "${src}/frontend" && npm ci --no-audit --no-fund && npm run build ) || { rm -rf "$src"; return 1; }
    mkdir -p "${src}/internal/web/dist"
    ( cd "$src" && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o build/x-ui . ) || { rm -rf "$src"; return 1; }

    systemctl stop "${service_name}" 2>/dev/null || true
    install -m 0755 "${src}/build/x-ui" "${main_folder}/x-ui"
    rm -rf "$src"
    return 0
}

main() {
    [[ $EUID -ne 0 ]] && { err "Please run as root."; write_status failed 1; exit 1; }

    info "Updating ${PANEL_NAME} (tag: ${tag:-latest})"
    write_status pending 0

    if download_release || build_from_source; then
        systemctl daemon-reload
        if systemctl restart "${service_name}"; then
            info "Update complete."
            write_status success 0
            exit 0
        fi
    fi

    err "Update failed."
    write_status failed 1
    exit 1
}

main "$@"
