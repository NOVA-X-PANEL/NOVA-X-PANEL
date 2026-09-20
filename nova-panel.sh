#!/bin/bash
# ============================================================
#  NOVA X PANEL  —  Management menu
#  Installed as /usr/bin/nova  (symlink to this script)
# ============================================================

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
blue='\033[0;34m'
plain='\033[0m'

PANEL_NAME="NOVA X PANEL"
REPO="NOVA-X-PANEL/NOVA-X-PANEL"

main_folder="${XUI_MAIN_FOLDER:=/usr/local/nova-x-panel}"
env_file="/etc/default/nova-x-panel"
service_name="nova-x-panel"
BIN="${main_folder}/x-ui"

# load branded env so CLI subcommands hit the same database as the service
if [[ -f "$env_file" ]]; then
    set -a
    # shellcheck disable=SC1090
    . "$env_file"
    set +a
fi

[[ $EUID -ne 0 ]] && echo -e "${red}Please run as root.${plain}" && exit 1

show_status() {
    if systemctl is-active --quiet "${service_name}"; then
        echo -e " ${service_name}: ${green}running${plain}"
    else
        echo -e " ${service_name}: ${red}stopped${plain}"
    fi
}

show_settings() {
    "${BIN}" setting -show 2>/dev/null
    local port path
    port="$("${BIN}" setting -show 2>/dev/null | grep -i port | grep -oE '[0-9]+' | head -1)"
    path="$("${BIN}" setting -show 2>/dev/null | grep -i 'base path' | awk '{print $NF}')"
    [[ -n "$port" ]] && echo -e " Panel URL: ${blue}http://<server-ip>:${port}${path}${plain}"
}

menu() {
    clear
    echo -e "${blue}========================================${plain}"
    echo -e "        ${PANEL_NAME} management"
    echo -e "${blue}========================================${plain}"
    show_status
    echo    "----------------------------------------"
    echo    " 1) Start"
    echo    " 2) Stop"
    echo    " 3) Restart"
    echo    " 4) Status"
    echo    " 5) Show settings / login info"
    echo    " 6) Reset username & password"
    echo    " 7) Change port"
    echo    " 8) Update (rebuild from GitHub)"
    echo    " 9) Service logs"
    echo    " 0) Exit"
    echo    "----------------------------------------"
    read -r -p "Choose [0-9]: " choice
    case "$choice" in
        1) systemctl start   "${service_name}" && echo "started" ;;
        2) systemctl stop    "${service_name}" && echo "stopped" ;;
        3) systemctl restart "${service_name}" && echo "restarted" ;;
        4) systemctl status  "${service_name}" --no-pager ;;
        5) show_settings ;;
        6)
            local u p
            u="$(head -c 16 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 8)"
            p="$(head -c 16 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 12)"
            "${BIN}" setting -username "$u" -password "$p"
            echo -e " New username: ${green}${u}${plain}"
            echo -e " New password: ${green}${p}${plain}"
            systemctl restart "${service_name}"
            ;;
        7)
            read -r -p "New port: " newport
            [[ "$newport" =~ ^[0-9]+$ ]] || { echo "invalid port"; return; }
            "${BIN}" setting -port "$newport" && systemctl restart "${service_name}"
            echo "port set to $newport"
            ;;
        8)
            echo "Rebuilding from https://github.com/${REPO} ..."
            if [[ -x "${main_folder}/update.sh" ]]; then
                bash "${main_folder}/update.sh"
            else
                bash <(curl -Ls "https://raw.githubusercontent.com/${REPO}/main/update.sh")
            fi
            ;;
        9) journalctl -u "${service_name}" -e --no-pager ;;
        0) exit 0 ;;
        *) echo "invalid choice" ;;
    esac
    echo
    read -r -p "Press Enter to return to the menu..." _
}

# non-interactive passthrough: `nova <subcommand ...>`
if [[ $# -gt 0 ]]; then
    case "$1" in
        start)   systemctl start "${service_name}" ;;
        stop)    systemctl stop "${service_name}" ;;
        restart) systemctl restart "${service_name}" ;;
        status)  systemctl status "${service_name}" --no-pager ;;
        log|logs) journalctl -u "${service_name}" -e --no-pager ;;
        update)  bash <(curl -Ls "https://raw.githubusercontent.com/${REPO}/main/update.sh") ;;
        show)    show_settings ;;
        version) "${BIN}" -v ;;
        *)       "${BIN}" "$@" ;;
    esac
    exit $?
fi

while true; do
    menu
done
