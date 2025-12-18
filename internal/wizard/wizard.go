package wizard

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

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
	fmt.Println("1. Discord")
	fmt.Println("2. Slack")
	fmt.Print("Enter choice (1/2): ")
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	var instructions string
	switch choice {
	case "1":
		instructions = `
--- Discord Setup Instructions ---
1. Go to your Server Settings -> Integrations -> Webhooks.
2. Click "New Webhook".
3. Select the channel where you want notifications.
4. Click "Copy Webhook URL".
`
	case "2":
		instructions = `
--- Slack Setup Instructions ---
1. Create an Incoming Webhook for your workspace.
2. Select the channel.
3. Copy the "Webhook URL" (starts with https://hooks.slack.com/...)
`
	default:
		l.Error("WIZARD", "Invalid choice. Exiting.")
		return
	}

	fmt.Println(instructions)
	fmt.Print("👉 Paste the Webhook URL here: ")
	url, _ := reader.ReadString('\n')
	url = strings.TrimSpace(url)

	if url == "" {
		l.Error("WIZARD", "Empty URL provided. Exiting.")
		return
	}

	// 2. Test the URL
	fmt.Println("\nTesting connection...")
	testCfg := config.NotificationConfig{
		Enabled:    true,
		WebhookURL: url,
	}
	n := notifier.New(testCfg)

	// Create a dummy success message for testing
	err := n.SendSuccess("TEST-ACCOUNT", "test-instance-id", "test-region")
	if err != nil {
		l.Error("WIZARD", fmt.Sprintf("❌ Failed to send test message: %v", err))
		fmt.Print("Do you want to save anyway? (y/n): ")
		confirm, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
			return
		}
	} else {
		l.Success("WIZARD", "✅ Test message sent successfully!")
	}

	// 3. Save to Config
	if err := saveConfig(url); err != nil {
		l.Error("WIZARD", fmt.Sprintf("Failed to update config.yaml: %v", err))
		fmt.Println("Please manually update 'webhook_url' in your config.yaml.")
	} else {
		l.Success("WIZARD", "✅ config.yaml updated successfully!")
		fmt.Println("Run the app again without flags to start.")
	}
}

// saveConfig attempts to update the webhook_url in config.yaml preserving comments.
func saveConfig(url string) error {
	// Locate config file
	path := "config.yaml" // Default preference
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try finding it if not in CWD? user might be running from elsewhere
		// For simplicity, we assume running from project root for wizard.
		return fmt.Errorf("config.yaml not found in current directory")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	stats, err := os.Stat(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	updatedLines := make([]string, 0, len(lines))

	// Regex to find "webhook_url:" key (ignoring comments)
	re := regexp.MustCompile(`^\s*webhook_url:.*`)
	// Regex to find "enabled:" under notifications (tricky without parsing structure)
	// We will just replace webhook_url and tell user to ensure enabled is true if we can't easily find context.
	// Actually, let's just replace webhook_url.

	found := false
	for _, line := range lines {
		if re.MatchString(line) {
			// Preserve indentation
			prefix := line[:strings.Index(line, "webhook_url")]
			updatedLines = append(updatedLines, fmt.Sprintf("%swebhook_url: \"%s\"", prefix, url))
			found = true
		} else {
			updatedLines = append(updatedLines, line)
		}
	}

	if !found {
		// If not found, it might be commented out or missing.
		// Append to end is risky.
		return fmt.Errorf("could not find 'webhook_url' key in config.yaml. Please add it manually.")
	}

	output := strings.Join(updatedLines, "\n")
	return os.WriteFile(path, []byte(output), stats.Mode())
}
