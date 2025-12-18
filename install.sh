#!/bin/bash
set -e


# Function: Uninstall
uninstall() {
    echo "🗑️  Uninstalling OCI ARM Provisioner..."
    systemctl --user disable --now oci-arm-provisioner 2>/dev/null || true
    rm -f ~/.config/systemd/user/oci-arm-provisioner.service
    rm -f ~/.config/systemd/user/oci-arm-provisioner.timer
    systemctl --user daemon-reload
    
    sudo rm -f /usr/local/bin/oci-arm-provisioner
    
    echo "❓ Remove configuration? (/etc/oci-arm-provisioner/)"
    read -p "   (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        sudo rm -rf /etc/oci-arm-provisioner
        echo "   Deleted /etc/oci-arm-provisioner"
    else
        echo "   Kept /etc/oci-arm-provisioner"
    fi
    
    echo "✅ Identify & clean complete."
}

# Function: Install
install() {
    echo "🚀 Installing OCI ARM Provisioner..."

    # 1. Build
    if ! command -v go &> /dev/null; then
        echo "❌ Error: 'go' is not installed."
        exit 1
    fi
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
        if [ -f config.yaml.example ]; then
             sudo cp config.yaml.example /etc/oci-arm-provisioner/config.yaml.example
        fi
        echo "⚠️  Please configure /etc/oci-arm-provisioner/config.yaml (or ~/.config/oci-arm-provisioner/config.yaml)"
    fi

    # 4. Install Service
    echo "🔧 Installing Systemd User Service..."
    mkdir -p ~/.config/systemd/user/
    cp deployments/systemd/oci-arm-provisioner.service ~/.config/systemd/user/
    cp deployments/systemd/oci-arm-provisioner.timer ~/.config/systemd/user/

    # Reload
    systemctl --user daemon-reload

    echo "✅ Installation Complete!"
    echo "👉 Run 'systemctl --user enable --now oci-arm-provisioner' to start."
}

# Main Logic
case "$1" in
    uninstall)
        uninstall
        ;;
    *)
        install
        ;;
esac
