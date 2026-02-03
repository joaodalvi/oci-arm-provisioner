<div align="center">

```text
   ____  __________    ___    ____  __  ___   ____  ____  ________  __________
  / __ \/ ____/  _/   /   |  / __ \/  |/  /  / __ \/ __ \/ ___/ _ \/ ___/ ___/
 / / / / /    / /    / /| | / /_/ / /|_/ /  / /_/ / /_/ / __/ /_/ / /   \__ \ 
/ /_/ / /____/ /    / ___ |/ _, _/ /  / /  / ____/ _, _/ /_/\__, /___/ ___/ / 
\____/\____/___/   /_/  |_/_/ |_/_/  /_/  /_/   /_/ |_|\__/____//___//____/  
                                                       
🚀 OCI Always Free ARM Instance Automator
```

[![License: GPLv3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/joaodalvi/oci-arm-provisioner)](https://goreportcard.com/report/github.com/joaodalvi/oci-arm-provisioner)
[![Build Status](https://github.com/joaodalvi/oci-arm-provisioner/actions/workflows/release.yml/badge.svg)](https://github.com/joaodalvi/oci-arm-provisioner/actions)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue?logo=docker&logoColor=white)](https://github.com/joaodalvi/oci-arm-provisioner/pkgs/container/oci-arm-provisioner)

**Eliminate "Out of Host Capacity" Errors Forever.**

[Features](#-features) • [Installation](#-installation) • [Configuration](#%EF%B8%8F-configuration) • [Compliance](#-compliance--safety) • [Changelog](CHANGELOG.md)

</div>

---

## 🧐 The Problem
You want the **Oracle Cloud Always Free ARM Instance** (4 OCPUs, 24GB RAM) because it's the best free cloud deal in existence.

But every time you try to create one, you see:
> **"Out of host capacity."**

You are tired of manually refreshing the page at 3 AM hoping for a slot.

## 💡 The Solution
**OCI ARM Provisioner** is a set-and-forget automation tool written in Go. It:
1.  **Watches** for availability in your region (24/7).
2.  **Snipes** the instance the second it becomes available.
3.  **Notifies** you via Discord/Telegram/App so you can celebrate.

It is lightweight (<10MB), safe (respects API limits), and runs on anything (Raspberry Pi, VPS, Docker, Windows).

---

## 🚨 Compliance & Safety
> [!IMPORTANT]
> **READ THIS BEFORE USING**

Oracle has strict Terms of Service regarding the Free Tier.
1.  **One Account Per Person**: Do **NOT** use this tool to manage multiple accounts for yourself. Oracle **will ban you** for multi-accounting.
2.  **Authorized Use Only**: The multi-account feature is designed for managing **authorized tenancies** (e.g., your legitimate personal account + a client/friend's account who gave you keys).
3.  **Rate Limits**: This tool uses a scheduler (default 900s) to essentially "poll" the API. Aggressive polling (e.g., every 1 second) **will get your API keys revoked**. Stick to the defaults.

**Disclaimer**: The author provides this tool for educational and legitimate automation purposes. Use it responsibly.

---

## � Installation

| Platform | Method | Instructions |
| :--- | :--- | :--- |
| **🐳 Docker** | **Compose** | `docker-compose up -d` (Recommended) |
| **🐧 Linux** | **Script** | `curl -L <repo>/install.sh \| bash` (or download release) |
| **🪟 Windows** | **PowerShell** | Download `.zip`, run `.\install.ps1` |
| **🍎 macOS** | **Binary** | Download `.tar.gz`, run binary directly |
| **📦 Arch** | **AUR** | Use provided `PKGBUILD` |

### 🚀 Quick Start (Binary)

1.  **Download** the latest release from [Releases](https://github.com/joaodalvi/oci-arm-provisioner/releases).
2.  **Setup Environment**:
    ```bash
    # Linux/Mac
    ./oci-arm-provisioner --setup
    
    # Windows
    .\oci-arm-provisioner.exe --setup
    ```
    *Follow the wizard to enter your OCI credentials (OCIDs, Keys).*

3.  **Setup Notifications (Optional)**:
    ```bash
    ./oci-arm-provisioner --setup-notifications
    ```
    *Support for Discord, Telegram, Ntfy.sh, Gotify.*

4.  **Run**:
    ```bash
    ./oci-arm-provisioner
    ```

---

## ✨ Features
*   **🏎️ Blazing Fast**: Native Golang binary. No Python/Node dependencies.
*   **🤖 Smart Wizard**: Interactive setup guide generates your `config.yaml` for you.
*   **🔔 Real-Time Alerts**: Get pinged on Discord, Telegram, Slack, Ntfy, or Gotify.
*   **🛡️ Battle Tested**: Handles 500/502/429 errors with exponential backoff.
*   **🐳 Container Ready**: Official Docker image and Compose file included.
*   **🔄 Auto-Discovery**: Automatically scans all Availability Domains (AD-1, AD-2, AD-3) for space.
*   **🔌 Extensible**: Supports multiple accounts (for legitimate separate tenancies).

---

## ⚙️ Configuration
The configuration is stored in `config.yaml`.
**Location:** Current Directory, `~/.config/oci-arm-provisioner/`, or `/etc/oci-arm-provisioner/`.

### Example `config.yaml`
```yaml
accounts:
  personal-account:
    enabled: true
    # Authentication
    user_ocid: "ocid1.user.oc1..."
    tenancy_ocid: "ocid1.tenancy.oc1..."
    fingerprint: "aa:bb:cc..."
    key_file: "~/.oci/oci_api_key.pem"
    region: "us-ashburn-1"
    
    # Instance Specs (Always Free Defaults)
    shape: "VM.Standard.A1.Flex"
    ocpus: 4
    memory_gb: 24
    image_ocid: "ocid1.image.oc1..."
    ssh_public_key: "ssh-rsa AAA..."

notifications:
  enabled: true
  webhook_url: "https://discord.com/api/webhooks/..."
  # or telegram_token / ntfy_topic / gotify_url

scheduler:
  cycle_interval_seconds: 900 # 15 minutes
```

---

## 🛠️ Building from Source
**Requirements**: Go 1.23+

```bash
git clone https://github.com/joaodalvi/oci-arm-provisioner.git
cd oci-arm-provisioner
go mod tidy
go build -ldflags="-s -w" -o oci-arm-provisioner
```

## � License
This project is licensed under the **GPLv3 License**. See [LICENSE](LICENSE) for details.
