# 🗺️ Project Roadmap

This document outlines the future development plans for the **OCI ARM Provisioner**. The primary focus for the next major version is expanding the notification ecosystem to ensure users are instantly alerted when their instances are successfully provisioned.

## 🚀 v0.2.0: Enhanced Notifications

The goal is to move beyond simple generic webhooks and support platform-native integrations.

### 📱 Chat Platforms
- [ ] **Discord**: Native integration using Webhooks (with rich embeds/colors).
- [ ] **Slack**: Support for Slack Incoming Webhooks and App integrations.
- [ ] **Telegram**: Integration with the Telegram Bot API for push notifications.

### 📧 Email Integration
- [ ] **Standard SMTP**: Support for generic email providers (Postfix, SendGrid, etc.).
- [ ] **Gmail / OAuth2**: Secure, modern authentication support for Gmail users (avoiding "Less Secure Apps").
- [ ] **Status Reports**: Optional periodic email summaries (e.g., "Still running... checked 100 times today").

## 🔮 Future Ideas (v0.x.x)

- **Browser Notifications**: Native desktop notifications if running locally.
- **TUI Dashboard**: A terminal user interface to view active threads and logs in real-time.
- **Prometheus Metrics**: Export metrics for Grafana dashboards.

---
*Got a suggestion? Open an issue or PR to add to this roadmap!*
