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

func (n *Notifier) send(payload discordPayload) error {
	if !n.Config.Enabled || n.Config.WebhookURL == "" {
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
		return fmt.Errorf("webhook failed with status: %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) SendSuccess(account, instanceID, region string) error {
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

	return n.send(discordPayload{
		Content: content,
		Embeds:  []discordEmbed{embed},
	})
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

	embed := discordEmbed{
		Title: "📊 Daily Execution Digest",
		Color: ColorInfo,
		Fields: []field{
			{Name: "Uptime", Value: uptime.String(), Inline: true},
			{Name: "Total Cycles", Value: fmt.Sprintf("%d", stats.TotalCycles), Inline: true},
			{Name: "Capacity Limits", Value: fmt.Sprintf("%d", stats.CapacityErrors), Inline: true},
			{Name: "Other Errors", Value: fmt.Sprintf("%d", stats.OtherErrors), Inline: true},
		},
		Footer: &footer{Text: "OCI ARM Provisioner • Check logs for details"},
	}

	return n.send(discordPayload{
		Embeds: []discordEmbed{embed},
	})
}
