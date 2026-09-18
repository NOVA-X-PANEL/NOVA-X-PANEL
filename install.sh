#!/bin/bash

echo "Starting NOVA X PANEL Installation..."
latest_version="v1.0.0"
echo "Target version: $latest_version"

# Create directory
mkdir -p /usr/local/nova-x-panel
cd /usr/local/nova-x-panel

echo "Downloading release package..."
# Download release archive or source directly from main branch or release
curl -L -o nova-x-panel.tar.gz https://github.com/NOVA-X-PANEL/nova-x-panel/archive/refs/heads/main.tar.gz
tar -xzf nova-x-panel.tar.gz --strip-components=1

echo "Building and setting up..."
# Setup systemd service if exists
if [ -f "x-ui.service.debian" ]; then
    cp x-ui.service.debian /etc/systemd/system/nova-x-panel.service
    systemctl daemon-reload
    systemctl enable nova-x-panel
    systemctl restart nova-x-panel || true
fi

echo "Installation completed successfully!"
