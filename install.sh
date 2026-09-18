#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
plain='\033[0m'

[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege" && exit 1

echo -e "${green}Installing NOVA X PANEL...${plain}"

xui_folder="/usr/local/x-ui"
mkdir -p ${xui_folder}
cd ${xui_folder}/../

echo -e "${green}Downloading source repository package...${plain}"
curl -L -o x-ui.tar.gz https://github.com/NOVA-X-PANEL/nova-x-panel/archive/refs/heads/main.tar.gz
tar -xzf x-ui.tar.gz
rm -f x-ui.tar.gz
rm -rf ${xui_folder}/*
cp -r nova-x-panel-main/* ${xui_folder}/
rm -rf nova-x-panel-main

cd ${xui_folder}
chmod +x x-ui.sh

if [ -f "x-ui.service.debian" ]; then
    cp x-ui.service.debian /etc/systemd/system/x-ui.service
    systemctl daemon-reload
    systemctl enable x-ui
    systemctl restart x-ui
fi

echo -e "${green}NOVA X PANEL Installed Successfully!${plain}"
