#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain}Please run this script with root privilege." && exit 1

xui_folder="/usr/local/x-ui"

echo -e "${green}=== Installing NOVA X PANEL ===${plain}"

# Install dependencies
if command -v apt-get &>/dev/null; then
    apt-get update && apt-get install -y curl wget tar socat certbot
elif command -v dnf &>/dev/null; then
    dnf install -y curl wget tar socat certbot
elif command -v yum &>/dev/null; then
    yum install -y curl wget tar socat certbot
fi

mkdir -p ${xui_folder}
cd ${xui_folder}

echo -e "${green}Downloading Nova X Panel release package...${plain}"
curl -L -o x-ui-linux-amd64.tar.gz https://github.com/NOVA-X-PANEL/nova-x-panel/archive/refs/heads/main.tar.gz
if [ $? -ne 0 ]; then
    echo -e "${red}Download failed! Please check your network connection.${plain}"
    exit 1
fi

tar -xzf x-ui-linux-amd64.tar.gz --strip-components=1
rm -f x-ui-linux-amd64.tar.gz

chmod +x x-ui
chmod +x x-ui.sh

if [ -f "x-ui.service.debian" ]; then
    cp x-ui.service.debian /etc/systemd/system/x-ui.service
elif [ -f "deploy/x-ui.service" ]; then
    cp deploy/x-ui.service /etc/systemd/system/x-ui.service
fi

systemctl daemon-reload
systemctl enable x-ui
systemctl restart x-ui

echo -e "${green}=== NOVA X PANEL Installed Successfully! ===${plain}"
