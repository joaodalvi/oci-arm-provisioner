#!/bin/bash
set -e

echo "🚀 Installing OCI ARM Provisioner..."

# 1. Build
echo "📦 Building binary..."
go build -ldflags="-s -w" -o oci-arm-provisioner
chmod +x oci-arm-provisioner

# 2. Install Binary
echo "📂 Installing to /usr/local/bin/"
sudo mv oci-arm-provisioner /usr/local/bin/

# 3. Create Config Dir
echo "⚙️  Creating config directory /etc/oci-arm-provisioner/"
sudo mkdir -p /etc/oci-arm-provisioner
if [ ! -f /etc/oci-arm-provisioner/config.yaml ]; then
    sudo cp config.yaml.example /etc/oci-arm-provisioner/config.yaml.example
    echo "⚠️  Please configure /etc/oci-arm-provisioner/config.yaml used by the service (or ~/.config/... for user service)"
fi

# 4. Install Service (User or System?)
# README suggests user service. But install.sh usually implies system-wide or user local?
# Let's support User Service as per README.

echo "🔧 Installing Systemd User Service..."
mkdir -p ~/.config/systemd/user/
cp deployments/systemd/oci-arm-provisioner.service ~/.config/systemd/user/
cp deployments/systemd/oci-arm-provisioner.timer ~/.config/systemd/user/

# Reload
systemctl --user daemon-reload

echo "✅ Installation Complete!"
echo "👉 Run 'systemctl --user enable --now oci-arm-provisioner' to start."
echo "👉 Logs: 'journalctl --user -f -u oci-arm-provisioner'"
