#!/bin/bash
set -e

# Define dist location
DIST_DIR="$(pwd)/dist"

echo "🧪 Starting Package Verification..."

# 1. Verify Debian/Ubuntu (.deb)
echo "---------------------------------------------------"
echo "📦 Testing DEB package on Debian Stable..."
DEB_FILE=$(find "$DIST_DIR" -name "*_linux_amd64.deb" | head -n 1)

if [ -z "$DEB_FILE" ]; then
    echo "❌ No .deb file found in $DIST_DIR"
else
    echo "   Found: $(basename "$DEB_FILE")"
    docker run --rm -v "$DIST_DIR:/dist" debian:stable bash -c "
        apt-get update -qq && \
        dpkg -i /dist/$(basename "$DEB_FILE") && \
        echo '✅ Package Installed Successfully' && \
        ls -l /usr/bin/oci-arm-provisioner && \
        ls -l /etc/oci-arm-provisioner/config.yaml.example && \
        ls -l /usr/lib/systemd/user/oci-arm-provisioner.service && \
        oci-arm-provisioner --help > /dev/null && \
        echo '✅ Binary Execution Verified'
    "
fi

# 2. Verify RHEL/AlmaLinux (.rpm)
echo "---------------------------------------------------"
echo "📦 Testing RPM package on AlmaLinux 9..."
RPM_FILE=$(find "$DIST_DIR" -name "*_linux_amd64.rpm" | head -n 1)

if [ -z "$RPM_FILE" ]; then
    echo "❌ No .rpm file found in $DIST_DIR"
else
    echo "   Found: $(basename "$RPM_FILE")"
    docker run --rm -v "$DIST_DIR:/dist" almalinux:9 bash -c "
        rpm -ivh /dist/$(basename "$RPM_FILE") && \
        echo '✅ Package Installed Successfully' && \
        ls -l /usr/bin/oci-arm-provisioner && \
        ls -l /etc/oci-arm-provisioner/config.yaml.example && \
        ls -l /usr/lib/systemd/user/oci-arm-provisioner.service && \
        oci-arm-provisioner --help > /dev/null && \
        echo '✅ Binary Execution Verified'
    "
fi

echo "---------------------------------------------------"
echo "🎉 Verification Complete!"
