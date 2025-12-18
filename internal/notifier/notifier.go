package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/yourusername/oci-arm-provisioner/internal/config"
)

// Notifier handles sending alerts to various platforms (Discord, Telegram, Ntfy).
type Notifier struct {
	Config config.NotificationConfig
	Client *http.Client
}

// New creates a new Notifier instance with the given configuration.
func New(cfg config.NotificationConfig) *Notifier {
	return &Notifier{
		Config: cfg,
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

// --- Payload Structures ---

// Discord
type discordPayload struct {
	Content string         `json:"content,omitempty"`
	Embeds  []discordEmbed `json:"embeds,omitempty"`
}
type discordEmbed struct {
	Title  string  `json:"title"`
	Color  int     `json:"color"`
	Footer *footer `json:"footer,omitempty"`
	Fields []field `json:"fields,omitempty"`
}
type footer struct {
	Text string `json:"text"`
}
type field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

const (
	ColorSuccess = 5763719
	ColorError   = 15548997
	ColorInfo    = 3447003
)

// Telegram
type telegramPayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// Gotify
type gotifyPayload struct {
	Title    string                 `json:"title"`
	Message  string                 `json:"message"`
	Priority int                    `json:"priority"`
	Extras   map[string]interface{} `json:"extras,omitempty"`
}

// --- Helper Methods ---

func (n *Notifier) postJSON(url string, payload interface{}, headers map[string]string) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("req creation failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("api returned status: %d", resp.StatusCode)
	}
	return nil
}

// --- Senders ---

func (n *Notifier) sendWebhook(payload discordPayload) error {
	if n.Config.WebhookURL == "" {
		return nil
	}
	return n.postJSON(n.Config.WebhookURL, payload, nil)
}

func (n *Notifier) sendTelegram(text string) error {
	if n.Config.TelegramToken == "" || n.Config.TelegramChatID == "" {
		return nil
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.Config.TelegramToken)
	payload := telegramPayload{
		ChatID:    n.Config.TelegramChatID,
		Text:      text,
		ParseMode: "HTML",
	}
	return n.postJSON(url, payload, nil)
}

func (n *Notifier) sendNtfy(message, title string, priority int, tags string) error {
	if n.Config.NtfyTopic == "" {
		return nil
	}
	url := fmt.Sprintf("https://ntfy.sh/%s", n.Config.NtfyTopic)
	// Ntfy usually takes raw body, but json is also supported.
	// For raw body we can't use postJSON easily without changing signature.
	// Let's stick to raw body implementation for Ntfy as it's cleaner for simple pushes.
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(message))
	if err != nil {
		return err
	}
	req.Header.Set("Title", title)
	req.Header.Set("Priority", fmt.Sprintf("%d", priority))
	req.Header.Set("Tags", tags)
	req.Header.Set("Markdown", "yes")

	resp, err := n.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("ntfy failed: %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) sendGotify(message, title string, priority int) error {
	if n.Config.GotifyURL == "" || n.Config.GotifyToken == "" {
		return nil
	}
	// Sanitize URL (ensure no trailing slash logic or just rely on user)
	// Assuming well formed URL for now.
	url := fmt.Sprintf("%s/message?token=%s", n.Config.GotifyURL, n.Config.GotifyToken)

	payload := gotifyPayload{
		Title:    title,
		Message:  message,
		Priority: priority,
		Extras: map[string]interface{}{
			"client::display": map[string]string{
				"contentType": "text/markdown",
			},
		},
	}
	return n.postJSON(url, payload, nil)
}

// --- Public API ---

// SendSuccess triggers a "Success" alert to all enabled providers.
// Returns an aggregate error if any provider fails.
func (n *Notifier) SendSuccess(account, instanceID, region string) error {
	var errs []error

	// 1. Discord/Slack Webhook
	if n.Config.WebhookURL != "" {
		content := ""
		if n.Config.InsistentPing {
			content = "@everyone 🚀 Instance Provisioned!"
		}
		embed := discordEmbed{
			Title: "✅ OCI Instance Launched Successfully",
			Color: ColorSuccess,
			Fields: []field{
				{Name: "Account", Value: account, Inline: true},
				{Name: "Region", Value: region, Inline: true},
				{Name: "Instance ID", Value: instanceID, Inline: false},
			},
			Footer: &footer{Text: "OCI ARM Provisioner • " + time.Now().Format("2006-01-02 15:04:05")},
		}
		if err := n.sendWebhook(discordPayload{Content: content, Embeds: []discordEmbed{embed}}); err != nil {
			errs = append(errs, err)
		}
	}

	// 2. Telegram
	if n.Config.TelegramToken != "" {
		msg := fmt.Sprintf("<b>🚀 Instance Launched!</b>\n\n<b>Account:</b> %s\n<b>Region:</b> %s\n<b>Instance ID:</b> <code>%s</code>", account, region, instanceID)
		if n.Config.InsistentPing {
			msg = "🚨 <b>ATTENTION!</b> 🚨\n\n" + msg
		}
		if err := n.sendTelegram(msg); err != nil {
			errs = append(errs, err)
		}
	}

	// 3. Ntfy
	if n.Config.NtfyTopic != "" {
		priority := 4
		if n.Config.InsistentPing {
			priority = 5
		}
		msg := fmt.Sprintf("**Instance Launched!**\n\n**Account:** %s\n**Region:** %s\n**ID:** `%s`", account, region, instanceID)
		if err := n.sendNtfy(msg, "🚀 OCI Provision Success", priority, "tada,rocket"); err != nil {
			errs = append(errs, err)
		}
	}

	// 4. Gotify
	if n.Config.GotifyURL != "" {
		priority := 8
		if n.Config.InsistentPing {
			priority = 10
		}
		msg := fmt.Sprintf("**Instance Launched!**\n\n**Account:** %s\n**Region:** %s\n**ID:** `%s`", account, region, instanceID)
		if err := n.sendGotify(msg, "🚀 OCI Provision Success", priority); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("notification errors: %v", errs)
	}
	return nil
}

// Stats holds metrics for the digest
type Stats struct {
	StartTime      time.Time
	TotalCycles    int
	CapacityErrors int
	OtherErrors    int
	LastSuccess    time.Time
}

// SendDigest triggers a status report alert to all enabled providers.
func (n *Notifier) SendDigest(stats Stats) error {
	uptime := time.Since(stats.StartTime).Round(time.Second)
	var errs []error

	// Discord
	if n.Config.WebhookURL != "" {
		embed := discordEmbed{
			Title: "📊 Daily Execution Digest",
			Color: ColorInfo,
			Fields: []field{
				{Name: "Uptime", Value: uptime.String(), Inline: true},
				{Name: "Total Cycles", Value: fmt.Sprintf("%d", stats.TotalCycles), Inline: true},
				{Name: "Capacity Limits", Value: fmt.Sprintf("%d", stats.CapacityErrors), Inline: true},
				{Name: "Other Errors", Value: fmt.Sprintf("%d", stats.OtherErrors), Inline: true},
			},
			Footer: &footer{Text: "OCI ARM Provisioner"},
		}
		if err := n.sendWebhook(discordPayload{Embeds: []discordEmbed{embed}}); err != nil {
			errs = append(errs, err)
		}
	}

	// Telegram
	if n.Config.TelegramToken != "" {
		msg := fmt.Sprintf("<b>📊 Daily Digest</b>\n\n🕒 <b>Uptime:</b> %s\n🔄 <b>Cycles:</b> %d\n⚠️ <b>Capacity Hits:</b> %d\n❌ <b>Errors:</b> %d",
			uptime.String(), stats.TotalCycles, stats.CapacityErrors, stats.OtherErrors)
		if err := n.sendTelegram(msg); err != nil {
			errs = append(errs, err)
		}
	}

	// Ntfy
	if n.Config.NtfyTopic != "" {
		msg := fmt.Sprintf("**Daily Digest**\n\n🕒 **Uptime:** %s\n🔄 **Cycles:** %d\n⚠️ **Capacity Hits:** %d\n❌ **Errors:** %d",
			uptime.String(), stats.TotalCycles, stats.CapacityErrors, stats.OtherErrors)
		if err := n.sendNtfy(msg, "📊 Status Report", 3, "chart_with_upwards_trend"); err != nil {
			errs = append(errs, err)
		}
	}

	// Gotify
	if n.Config.GotifyURL != "" {
		msg := fmt.Sprintf("**Daily Digest**\n\n🕒 **Uptime:** %s\n🔄 **Cycles:** %d\n⚠️ **Capacity Hits:** %d\n❌ **Errors:** %d",
			uptime.String(), stats.TotalCycles, stats.CapacityErrors, stats.OtherErrors)
		if err := n.sendGotify(msg, "📊 Status Report", 4); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("digest errors: %v", errs)
	}
	return nil
}
