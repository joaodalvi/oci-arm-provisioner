package notifier

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yourusername/oci-arm-provisioner/internal/config"
)

type mockTransport struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req)
}

// Since I can't easily change the hardcoded URL in `notifier.go` without editing it,
// I will EDIT `notifier.go` to use a helper for Telegram URL construction or dependency injection?
// ACTUALLY: The easiest way to test hardcoded URLs is to use a Transport that intercepts requests.
// OR: I can modify `notifier.go` to construct URLs dynamically if I want strict testing.

func TestNotifier_AllProviders(t *testing.T) {
	cfg := config.NotificationConfig{
		Enabled:        true,
		InsistentPing:  true,
		WebhookURL:     "http://discord.mock",
		TelegramToken:  "tg-token",
		TelegramChatID: "tg-chat",
		NtfyTopic:      "ntfy-topic",
		GotifyURL:      "http://gotify.mock",
		GotifyToken:    "gotify-token",
	}

	n := New(cfg)

	// Track hits
	hits := make(map[string]bool)

	// Inject Mock Transport
	n.Client.Transport = &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			url := req.URL.String()

			if strings.Contains(url, "discord") {
				hits["discord"] = true
				var p discordPayload
				json.NewDecoder(req.Body).Decode(&p)
				if !strings.Contains(p.Content, "@everyone") {
					t.Error("Discord missing @everyone")
				}
				if len(p.Embeds) == 0 || p.Embeds[0].Color != ColorSuccess {
					t.Error("Discord invalid embed")
				}

			} else if strings.Contains(url, "telegram") {
				hits["telegram"] = true
				if !strings.Contains(url, "tg-token") {
					t.Error("Telegram URL missing token")
				}
				var p telegramPayload
				json.NewDecoder(req.Body).Decode(&p)
				if p.ChatID != "tg-chat" {
					t.Error("Telegram invalid ChatID")
				}
				if !strings.Contains(p.Text, "<b>ATTENTION!</b>") {
					t.Error("Telegram missing urgent text")
				}

			} else if strings.Contains(url, "ntfy") {
				hits["ntfy"] = true
				if !strings.Contains(url, "ntfy-topic") {
					t.Error("Ntfy URL invalid")
				}
				if req.Header.Get("Priority") != "5" {
					t.Error("Ntfy invalid priority for insistent ping")
				}
				if req.Header.Get("Tags") != "tada,rocket" {
					t.Error("Ntfy invalid tags")
				}

			} else if strings.Contains(url, "gotify") {
				hits["gotify"] = true
				if req.URL.Query().Get("token") != "gotify-token" {
					t.Error("Gotify missing token param")
				}
				var p gotifyPayload
				json.NewDecoder(req.Body).Decode(&p)
				if p.Priority != 10 {
					t.Error("Gotify invalid priority")
				}
				if p.Title != "🚀 OCI Provision Success" {
					t.Error("Gotify invalid title")
				}
			}

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString("{}")),
			}, nil
		},
	}

	// Test SendSuccess
	err := n.SendSuccess("test-acct", "inst-1", "region-1")
	if err != nil {
		t.Fatalf("SendSuccess failed: %v", err)
	}

	expected := []string{"discord", "telegram", "ntfy", "gotify"}
	for _, p := range expected {
		if !hits[p] {
			t.Errorf("Provider %s was not called", p)
		}
	}
}

func TestNotifier_SendDigest(t *testing.T) {
	cfg := config.NotificationConfig{
		Enabled:        true,
		WebhookURL:     "http://discord.mock",
		NtfyTopic:      "topic",
		DigestInterval: "24h",
	}
	n := New(cfg)
	hits := make(map[string]bool)

	n.Client.Transport = &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			url := req.URL.String()
			if strings.Contains(url, "discord") {
				hits["discord"] = true
				var p discordPayload
				json.NewDecoder(req.Body).Decode(&p)
				if p.Embeds[0].Title != "📊 Daily Execution Digest" {
					t.Error("Discord digest title mismatch")
				}
			}
			if strings.Contains(url, "ntfy") {
				hits["ntfy"] = true
				if req.Header.Get("Priority") != "3" { // Default for digest
					t.Error("Ntfy digest priority mismatch")
				}
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString("{}"))}, nil
		},
	}

	stats := Stats{
		StartTime:      time.Now().Add(-1 * time.Hour),
		TotalCycles:    100,
		CapacityErrors: 5,
	}
	if err := n.SendDigest(stats); err != nil {
		t.Fatalf("SendDigest failed: %v", err)
	}

	if !hits["discord"] || !hits["ntfy"] {
		t.Error("Digest did not fire for all providers")
	}
}

// --- SendSuccessVerified Tests ---

// mockVerifiedDetails implements VerifiedInstanceDetails interface for testing
type mockVerifiedDetails struct {
	instanceID string
	publicIP   string
	ocpus      float32
	memoryGB   float32
	state      string
	region     string
}

func (m *mockVerifiedDetails) GetInstanceID() string { return m.instanceID }
func (m *mockVerifiedDetails) GetPublicIP() string   { return m.publicIP }
func (m *mockVerifiedDetails) GetOCPUs() float32     { return m.ocpus }
func (m *mockVerifiedDetails) GetMemoryGB() float32  { return m.memoryGB }
func (m *mockVerifiedDetails) GetState() string      { return m.state }
func (m *mockVerifiedDetails) GetRegion() string     { return m.region }

func TestSendSuccessVerified_AllProviders(t *testing.T) {
	cfg := config.NotificationConfig{
		Enabled:        true,
		InsistentPing:  true,
		WebhookURL:     "http://discord.mock",
		TelegramToken:  "tg-token",
		TelegramChatID: "tg-chat",
		NtfyTopic:      "ntfy-topic",
		GotifyURL:      "http://gotify.mock",
		GotifyToken:    "gotify-token",
	}

	n := New(cfg)
	hits := make(map[string]bool)

	n.Client.Transport = &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			url := req.URL.String()

			if strings.Contains(url, "discord") {
				hits["discord"] = true
				var p discordPayload
				json.NewDecoder(req.Body).Decode(&p)
				// Verify verified message format
				if len(p.Embeds) > 0 {
					embed := p.Embeds[0]
					if !strings.Contains(embed.Title, "Verified") {
						t.Error("Discord missing 'Verified' in title")
					}
					// Check for Public IP field
					hasIPField := false
					for _, f := range embed.Fields {
						if f.Name == "Public IP" {
							hasIPField = true
						}
					}
					if !hasIPField {
						t.Error("Discord missing Public IP field")
					}
				}
			} else if strings.Contains(url, "telegram") {
				hits["telegram"] = true
				var p telegramPayload
				json.NewDecoder(req.Body).Decode(&p)
				if !strings.Contains(p.Text, "Verified") {
					t.Error("Telegram missing 'Verified' in text")
				}
				if !strings.Contains(p.Text, "203.0.113.1") {
					t.Error("Telegram missing public IP")
				}
			} else if strings.Contains(url, "ntfy") {
				hits["ntfy"] = true
			} else if strings.Contains(url, "gotify") {
				hits["gotify"] = true
			}

			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString("{}"))}, nil
		},
	}

	details := &mockVerifiedDetails{
		instanceID: "ocid1.instance.test",
		publicIP:   "203.0.113.1",
		ocpus:      4,
		memoryGB:   24,
		state:      "RUNNING",
		region:     "us-ashburn-1",
	}

	if err := n.SendSuccessVerified("test-account", details); err != nil {
		t.Fatalf("SendSuccessVerified failed: %v", err)
	}

	expected := []string{"discord", "telegram", "ntfy", "gotify"}
	for _, p := range expected {
		if !hits[p] {
			t.Errorf("Provider %s was not called", p)
		}
	}
}

func TestSendSuccessVerified_NilDetails(t *testing.T) {
	cfg := config.NotificationConfig{
		Enabled:    true,
		WebhookURL: "http://test.mock",
	}

	n := New(cfg)

	err := n.SendSuccessVerified("test", nil)
	if err == nil {
		t.Error("expected error for nil details")
	}
}
