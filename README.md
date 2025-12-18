# OCI ARM Provisioner (Go Edition) 🚀

![License: GPLv3](https://img.shields.io/badge/License-GPLv3-blue.svg)
![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)
![Build Status](https://github.com/joaodalvi/oci-arm-provisioner/actions/workflows/release.yml/badge.svg)
![Docker](https://github.com/joaodalvi/oci-arm-provisioner/actions/workflows/docker.yml/badge.svg)

A high-performance, single-binary application to automate the provisioning of **Always Free ARM Instances** (4 OCPUs, 24GB RAM) on Oracle Cloud Infrastructure (OCI).

## ✨ Features
*   **written in Go**: Fast (native binary), lightweight (<10MB), and memory safe.
*   **Multi-Account Support**: Manage multiple OCI tenancies in parallel.
*   **Smart Scheduling**: Configurable delays and cycle intervals to avoid API bans.
*   **Auto-Discovery**: Automatically finds available Availability Domains (ADs).
*   **🔔 Notifications**: Success alerts & daily digests via Discord, Telegram, Ntfy, or Gotify.
*   **Production Ready**: Includes Docker Compose, Systemd units, and Arch PKGBUILD.
*   **Roadmap**: See [ROADMAP.md](ROADMAP.md) for future plans.

## 🔔 Notifications
Get notified instantly when your instance is created!
*   **Success Alerts** (with optional `@everyone` ping)
*   **Daily Digests** (Uptime, cycle counts, health check)

**Supported Platforms:**
*   **Discord / Slack** (Webhook)
*   **Telegram** (Bot) - *Includes auto-discovery of Chat ID!*
*   **Ntfy.sh** (Push) - *Zero setup required*
*   **Gotify** (Self-hosted)

**Setup Wizard:**
Run the app with the setup flag to interactively configure your alerts:
```bash
./oci-arm-provisioner --setup-notifications
```
👉 Full Guide: [docs/NOTIFICATIONS.md](docs/NOTIFICATIONS.md)

## 📂 Project Structure
*   `cmd/` / `internal/`: Go source code
*   `deployments/`: Packaging for Arch Linux & Systemd
*   `configs/`: Example configurations
*   `Makefile`: Unified management commands

## 🚀 Quick Start

### Option 1: Manual Run (Easiest)
1.  **Download** the latest release (or build it).
2.  **Configure**:
    ```bash
    cp config.yaml.example config.yaml
    # Edit config.yaml with your OCI details
    ```
3.  **Run**:
    ```bash
    ./oci-arm-provisioner
    ```

### Option 2: Systemd Service (Linux)
Install as a background service that restarts automatically on boot.
```bash
./install.sh
```
Check logs: `tail -f logs/provisioner.log`

### Option 3: Docker (Compose or Run)

**Using Compose (Recommended):**
```bash
docker-compose up -d
docker-compose logs -f    # View live logs
```

**Using Pre-built Image (Production):**
If pulling from a registry (e.g., GHCR), run:
```bash
docker run -d \
  --name oci-provisioner \
  --restart unless-stopped \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -v $(HOME)/.oci:/root/.oci:ro \
  -v $(pwd)/logs:/app/logs \
  ghcr.io/joaodalvi/oci-arm-provisioner:latest
```

**Viewing Logs:**
*   **Docker Logs**: `docker logs -f oci-provisioner` (Shows pretty console output)
*   **Audit Logs**: `tail -f logs/provisioner.log` (Shows timestamped text file on your host)

## ⚙️ Configuration
The application looks for `config.yaml` in the current directory, `~/.config/oci-arm-provisioner/`, or `/etc/oci-arm-provisioner/`.

**Example `config.yaml`**:
```yaml
accounts:
  my-free-tier:
    enabled: true
    user_ocid: "ocid1.user.oc1..aaaa..."
    tenancy_ocid: "ocid1.tenancy.oc1..aaaa..."
    fingerprint: "xx:xx:xx..."
    key_file: "~/.oci/oci_api_key.pem"
    region: "sa-saopaulo-1"
    compartment_ocid: "ocid1.compartment.oc1..aaaa..."
    availability_domain: "auto" # or specific "AD-1"
    
    # Instance Specs
    shape: "VM.Standard.A1.Flex"
    ocpus: 4
    memory_gb: 24
    image_ocid: "ocid1.image.oc1..."
    ssh_public_key: "ssh-rsa AAAAB3Nza..."

scheduler:
  account_delay_seconds: 30
  cycle_interval_seconds: 60
  loop_forever: true
```

## 🛠️ Building from Source

**Requirements**: Go 1.21+

```bash
git clone https://github.com/joaodalvi/oci-arm-provisioner.git
cd oci-arm-provisioner
go mod tidy
go build -ldflags="-s -w" -o oci-arm-provisioner
```

## 📦 Arch Linux (PKGBUILD)

A `PKGBUILD` is provided for Arch Linux users.

### 1. Build & Install
```bash
makepkg -si
```

### 2. Configure
Copy the example config to your user config directory:
```bash
mkdir -p ~/.config/oci-arm-provisioner
cp /etc/oci-arm-provisioner/config.yaml.example ~/.config/oci-arm-provisioner/config.yaml
# Edit it with your details
nano ~/.config/oci-arm-provisioner/config.yaml
```

### 3. Enable Service
Start it as a **User Service** (no root required):
```bash
systemctl --user enable --now oci-arm-provisioner
```

### 4. View Logs
*   **System Logs**: `journalctl --user -f -u oci-arm-provisioner`
*   **File Logs**: `tail -f ~/logs/provisioner.log` (Created in your home dir)

## 🛡️ Security
*   **Keys**: Store your private keys in `~/.oci/` with `600` permissions. The app will warn you if they are insecure.
*   **Logs**: Console logs are minimal and colored. File logs in `logs/` are timestamped and suitable for auditing.

## 📄 License
MIT License. See [LICENSE](LICENSE) for details.
