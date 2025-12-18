package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/yourusername/oci-arm-provisioner/internal/config"
)

type Notifier struct {
	Config config.NotificationConfig
	Client *http.Client
}

func New(cfg config.NotificationConfig) *Notifier {
	return &Notifier{
		Config: cfg,
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Discord Payload Structure
type discordPayload struct {
	Content string         `json:"content,omitempty"`
	Embeds  []discordEmbed `json:"embeds,omitempty"`
}

type discordEmbed struct {
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Color       int     `json:"color"` // Decimal color code
	Footer      *footer `json:"footer,omitempty"`
	Fields      []field `json:"fields,omitempty"`
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
	ColorSuccess = 5763719  // Green
	ColorError   = 15548997 // Red
	ColorInfo    = 3447003  // Blue
)

// Telegram Payload
type telegramPayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func (n *Notifier) sendWebhook(payload discordPayload) error {
	if n.Config.WebhookURL == "" {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", n.Config.WebhookURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook failed: %d", resp.StatusCode)
	}
	return nil
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
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram failed: %d", resp.StatusCode)
	}
	return nil
}

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

	if len(errs) > 0 {
		return fmt.Errorf("digest errors: %v", errs)
	}
	return nil
}
