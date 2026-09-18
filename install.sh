#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
plain='\033[0m'

[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege" && exit 1

echo -e "${green}Installing NOVA X PANEL...${plain}"

mkdir -p /usr/local/nova-x-panel
cd /usr/local/nova-x-panel

echo -e "${green}Downloading package...${plain}"
curl -L -o release.tar.gz https://github.com/NOVA-X-PANEL/nova-x-panel/archive/refs/heads/main.tar.gz
tar -xzf release.tar.gz --strip-components=1
rm -f release.tar.gz

echo -e "${green}Nova X Panel installed successfully!${plain}"
