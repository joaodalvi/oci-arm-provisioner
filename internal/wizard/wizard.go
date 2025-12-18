package wizard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/yourusername/oci-arm-provisioner/internal/config"
	"github.com/yourusername/oci-arm-provisioner/internal/logger"
	"github.com/yourusername/oci-arm-provisioner/internal/notifier"
)

// Run starts the interactive notification setup wizard.
func Run(l *logger.Logger) {
	reader := bufio.NewReader(os.Stdin)

	l.Section("🔔 Notification Setup Wizard")
	fmt.Println("This wizard will help you configure Discord or Slack notifications.")
	fmt.Println()

	// 1. Select Platform
	fmt.Println("Select your platform:")
	fmt.Println("1. Discord / Slack")
	fmt.Println("2. Telegram")
	fmt.Println("3. Ntfy.sh (Zero Setup)")
	fmt.Print("Enter choice (1/2/3): ")
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	var webhookURL, telegramToken, telegramChatID, ntfyTopic, gotifyURL, gotifyToken string

	if choice == "1" {
		// Discord/Slack Flow
		fmt.Println("\n--- Discord/Slack Setup ---")
		fmt.Println("1. Go to Server Settings -> Integrations -> Webhooks (Discord)")
		fmt.Println("   OR Create an Incoming Webhook (Slack)")
		fmt.Println("2. Copy the URL.")
		fmt.Print("👉 Paste the Webhook URL: ")
		webhookURL, _ = reader.ReadString('\n')
		webhookURL = strings.TrimSpace(webhookURL)
	} else if choice == "2" {
		// Telegram Flow
		fmt.Println("\n--- Telegram Setup ---")
		fmt.Println("1. Open Telegram and search for @BotFather")
		fmt.Println("2. Create a new bot with /newbot")
		fmt.Println("3. Copy the HTTP API Token.")
		fmt.Print("👉 Paste Bot Token: ")
		telegramToken, _ = reader.ReadString('\n')
		telegramToken = strings.TrimSpace(telegramToken)

		if telegramToken == "" {
			l.Error("WIZARD", "Token required.")
			return
		}

		fmt.Println("\n⏳ Identifying Chat ID...")
		fmt.Println("👉 Please send a message (e.g. /start) to your bot in Telegram NOW.")

		chatID, err := pollTelegramChatID(telegramToken)
		if err != nil {
			l.Error("WIZARD", fmt.Sprintf("Failed to detect Chat ID: %v", err))
			fmt.Println("You can try again or enter Chat ID manually if you know it.")
			fmt.Print("Enter Chat ID (optional, press enter to skip): ")
			telegramChatID, _ = reader.ReadString('\n')
			telegramChatID = strings.TrimSpace(telegramChatID)
		} else {
			telegramChatID = chatID
			l.Success("WIZARD", fmt.Sprintf("✅ Detected Chat ID: %s", telegramChatID))
		}
	} else if choice == "3" {
		// Ntfy Flow
		fmt.Println("\n--- Ntfy.sh Setup ---")
		fmt.Println("1. Download Ntfy app (Android/iOS) or use web.")
		fmt.Println("2. Pick a UNIQUE topic name (e.g. 'oci-my-secret-topic-99').")
		fmt.Println("3. Subscribe to it in the app.")
		fmt.Print("👉 Enter Topic Name: ")
		ntfyTopic, _ = reader.ReadString('\n')
		ntfyTopic = strings.TrimSpace(ntfyTopic)
	} else if choice == "4" {
		// Gotify Flow
		fmt.Println("\n--- Gotify Setup ---")
		fmt.Println("1. Open your Gotify Web UI.")
		fmt.Println("2. Create a new Application (e.g. 'OCI Bot').")
		fmt.Println("3. Copy the App Token.")
		fmt.Print("👉 Enter Gotify Server URL (e.g. https://gotify.example.com): ")
		gotifyURL, _ = reader.ReadString('\n')
		gotifyURL = strings.TrimSpace(gotifyURL)
		// Basic URL sanitization
		gotifyURL = strings.TrimRight(gotifyURL, "/")
		if !strings.HasPrefix(gotifyURL, "http") {
			fmt.Println("⚠️ URL should start with http:// or https://")
		}

		fmt.Print("👉 Enter App Token: ")
		gotifyToken, _ = reader.ReadString('\n')
		gotifyToken = strings.TrimSpace(gotifyToken)
	} else {
		l.Error("WIZARD", "Invalid choice.")
		return
	}

	// 2. Test Configuration
	fmt.Println("\nTesting connection...")
	testCfg := config.NotificationConfig{
		Enabled:        true,
		WebhookURL:     webhookURL,
		TelegramToken:  telegramToken,
		TelegramChatID: telegramChatID,
		NtfyTopic:      ntfyTopic,
		GotifyURL:      gotifyURL,
		GotifyToken:    gotifyToken,
	}
	n := notifier.New(testCfg)

	err := n.SendSuccess("TEST-ACCOUNT", "test-instance-id", "test-region")
	if err != nil {
		l.Error("WIZARD", fmt.Sprintf("❌ Test failed: %v", err))
		fmt.Print("Save anyway? (y/n): ")
		confirm, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
			return
		}
	} else {
		l.Success("WIZARD", "✅ Test message sent!")
	}

	// 3. Save to Config
	if err := saveConfig(webhookURL, telegramToken, telegramChatID, ntfyTopic, gotifyURL, gotifyToken); err != nil {
		l.Error("WIZARD", fmt.Sprintf("Failed to save config: %v", err))
	} else {
		l.Success("WIZARD", "✅ Config updated successfully!")
	}
}

func pollTelegramChatID(token string) (string, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", token)
	client := http.Client{Timeout: 5 * time.Second}

	// Poll for 30 seconds
	attempts := 6
	for i := 0; i < attempts; i++ {
		resp, err := client.Get(url)
		if err == nil {
			var result struct {
				Ok     bool `json:"ok"`
				Result []struct {
					Message struct {
						Chat struct {
							ID int64 `json:"id"`
						} `json:"chat"`
					} `json:"message"`
				} `json:"result"`
			}
			if json.NewDecoder(resp.Body).Decode(&result) == nil {
				if result.Ok && len(result.Result) > 0 {
					return fmt.Sprintf("%d", result.Result[0].Message.Chat.ID), nil
				}
			}
			resp.Body.Close()
		}
		time.Sleep(3 * time.Second)
		fmt.Print(".")
	}
	return "", fmt.Errorf("timeout waiting for message")
}

// saveConfig updates valid fields in config.yaml
func saveConfig(webhook, tgToken, tgChatID, ntfyTopic, gotifyURL, gotifyToken string) error {
	path := "config.yaml"
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	updatedLines := make([]string, 0, len(lines))

	// Simple key replacement map
	replacements := make(map[string]string)
	if webhook != "" {
		replacements["webhook_url"] = webhook
	}
	if tgToken != "" {
		replacements["telegram_token"] = tgToken
	}
	if tgChatID != "" {
		replacements["telegram_chat_id"] = tgChatID
	}
	if ntfyTopic != "" {
		replacements["ntfy_topic"] = ntfyTopic
	}
	if gotifyURL != "" {
		replacements["gotify_url"] = gotifyURL
	}
	if gotifyToken != "" {
		replacements["gotify_token"] = gotifyToken
	}

	// Allow adding missing keys if logic permits, but regex replacement is safer for existing files.
	// For simplicity, we assume keys exist or we warn.
	// To robustly add keys, we should check if they are missing.

	// Track if we found them
	found := make(map[string]bool)

	for _, line := range lines {
		replaced := false
		for key, val := range replacements {
			re := regexp.MustCompile(fmt.Sprintf(`^\s*%s:.*`, key))
			if re.MatchString(line) {
				prefix := line[:strings.Index(line, key)]
				updatedLines = append(updatedLines, fmt.Sprintf("%s%s: \"%s\"", prefix, key, val))
				found[key] = true
				replaced = true
				break
			}
		}
		if !replaced {
			updatedLines = append(updatedLines, line)
		}
	}

	// Attempt to handle missing keys by appending to "notifications:" block? Too complex with regex.
	// We will just warn if keys were not found.
	for k := range replacements {
		if !found[k] {
			// Fail-safe: Suggest manual addition
			fmt.Printf("⚠️  Key '%s' not found in config.yaml. Please add it manually.\n", k)
		}
	}

	output := strings.Join(updatedLines, "\n")
	info, _ := os.Stat(path)
	return os.WriteFile(path, []byte(output), info.Mode())
}
