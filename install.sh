#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
plain='\033[0m'

[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege" && exit 1

echo -e "${green}Installing NOVA X PANEL...${plain}"

xui_folder="/usr/local/x-ui"
mkdir -p ${xui_folder}
cd ${xui_folder}/../

echo -e "${green}Downloading latest release...${plain}"
# Download source or release directly without any API calls
curl -fL -o x-ui.tar.gz https://github.com/NOVA-X-PANEL/nova-x-panel/archive/refs/heads/main.tar.gz
if [[ $? -ne 0 ]]; then
    echo -e "${red}Download failed!${plain}"
    exit 1
fi

tar -xzf x-ui.tar.gz
rm -f x-ui.tar.gz
mv nova-x-panel-main x-ui-src
cd x-ui-src

echo -e "${green}Building x-ui panel...${plain}"
if command -v go &> /dev/null; then
    go build -o x-ui main.go
else
    echo -e "${yellow}Go is not installed. Installing Go...${plain}"
    curl -L -o go.tar.gz https://go.dev/dl/go1.22.2.linux-amd64.tar.gz
    rm -rf /usr/local/go && tar -C /usr/local -xzf go.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    go build -o x-ui main.go
fi

mkdir -p ${xui_folder}
cp x-ui ${xui_folder}/
chmod +x ${xui_folder}/x-ui

# Copy service file
if [ -f "x-ui.service.debian" ]; then
    cp x-ui.service.debian /etc/systemd/system/x-ui.service
    systemctl daemon-reload
    systemctl enable x-ui
    systemctl restart x-ui
fi

echo -e "${green}NOVA X PANEL Installation Completed Successfully!${plain}"
