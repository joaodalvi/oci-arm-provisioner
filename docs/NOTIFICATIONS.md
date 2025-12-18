# 🔔 Notification System Guide

The **OCI ARM Provisioner** includes a robust notification system to keep you informed about your instance provisioning status.

## Features
*   **🚀 Success Alerts**: Get notified immediately when an instance is successfully launched.
    *   *Optional*: "Insistent Ping" (e.g., `@everyone`) to ensure you wake up.
*   **📊 Daily Digests**: Receive a periodic summary (defaults to every 24h) of:
    *   Uptime & Total Cycles.
    *   Capacity Limit hits (so you know it's trying).
    *   Critical Errors.

---

## 🧙‍♂️ Quick Start: The Setup Wizard
The easiest way to configure notifications is using the built-in interactive wizard. It will guide you through obtaining the necessary keys and testing your connection.

```bash
./oci-arm-provisioner --setup-notifications
```

The wizard supports:
1.  **Discord / Slack** (Webhook)
2.  **Telegram** (Auto-discovery of Chat ID!)
3.  **Ntfy.sh** (No account needed)
4.  **Gotify** (Self-hosted)

---

## 🛠️ Manual Configuration
If you prefer to edit your `config.yaml` manually, here is the reference configuration.

### 1. Discord / Slack
Standard Webhook integration. Use this for free, rich-text alerts.

```yaml
notifications:
  enabled: true
  webhook_url: "https://discord.com/api/webhooks/..."
  insistent_ping: true  # Mentions @everyone on success
  digest_interval: "24h"
```

### 2. Telegram
Native Telegram Bot integration. Send HTML-formatted messages.

1.  Message `@BotFather` to create a new bot and get the **Token**.
2.  Message your bot so it can see you.
3.  Use the Wizard to find your **Chat ID** (or find it via API).

```yaml
notifications:
  enabled: true
  telegram_token: "123456:ABC-DEF..."
  telegram_chat_id: "987654321"
  insistent_ping: true
```

### 3. Ntfy.sh
Privacy-focused push notifications. No account required.

1.  Install the **Ntfy** app (Android/iOS).
2.  Subscribe to a unique topic (e.g., `my-oci-bot-99`).
3.  Configure:

```yaml
notifications:
  enabled: true
  ntfy_topic: "my-oci-bot-99"
  insistent_ping: true # Sends as "High Priority" (flashing/sound)
```

### 4. Gotify
Self-hosted push notification server.

1.  Create an Application in your Gotify Admin UI.
2.  Copy the **App Token**.

```yaml
notifications:
  enabled: true
  gotify_url: "https://gotify.mydomain.com"
  gotify_token: "A1b2C3d4E5"
  insistent_ping: true # Sets Priority to 10
```

---

## ⚙️ Advanced Configuration

| Field | Description | Default |
| :--- | :--- | :--- |
| `enabled` | Master switch to turn notifications on/off. | `false` |
| `insistent_ping` | If `true`, success messages are sent with highest urgency (Discord `@everyone`, Ntfy Priority 5, etc). | `false` |
| `digest_interval` | How often to send the status summary. Set to `""` to disable. | `"24h"` |

## Troubleshooting

**Test Failed?**
*   Check that your Webhook URL was copied correctly (especially minimal changes like trailing slashes).
*   Ensure the machine running the bot has internet access to the notification provider (e.g. `ping discord.com`).

**Telegram Chat ID not found?**
*   You **must** send a message (like `/start`) to your bot first. Bots cannot initiate conversations with users who haven't spoken to them.

---

## 🐳 Docker Users
If you are running the application via Docker, you cannot use the interactive wizard easily.
**Option 1: Edit Config File**
Edit the `config.yaml` file on your host machine (which is mounted into the container).
```bash
nano config.yaml
# Add "webhook_url: ..." manually
```
**Option 2: Environment Variables (Recommended)**
You can inject secrets via environment variables instead of writing them to the file.
```bash
docker run -d \
  -e OCI_NOTIFY_WEBHOOK="https://discord..." \
  -v $(pwd)/config.yaml:/app/config.yaml \
  ...
```

## 🔧 Environment Variables
The following environment variables will **override** settings in `config.yaml`. This is useful for keeping secrets out of your config file.

| Variable | Config Override |
| :--- | :--- |
| `OCI_NOTIFY_WEBHOOK` | `notifications.webhook_url` |
| `OCI_NOTIFY_TELEGRAM_TOKEN` | `notifications.telegram_token` |
| `OCI_NOTIFY_TELEGRAM_CHAT` | `notifications.telegram_chat_id` |
| `OCI_NOTIFY_NTFY_TOPIC` | `notifications.ntfy_topic` |
| `OCI_NOTIFY_GOTIFY_URL` | `notifications.gotify_url` |
| `OCI_NOTIFY_GOTIFY_TOKEN` | `notifications.gotify_token` |
