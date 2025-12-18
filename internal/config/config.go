package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level configuration structure for the OCI ARM Provisioner.
// It maps directly to the YAML configuration file.
type Config struct {
	// Accounts holds the configuration for each OCI tenancy/user to check.
	// The map key is a user-friendly alias (e.g., "personal", "work").
	Accounts map[string]*AccountConfig `yaml:"accounts"`

	// Retry configures the backoff strategy when OCI returns errors (e.g., 500 or 429).
	Retry RetryConfig `yaml:"retry"`

	// Scheduler controls the timing of the provisioning loop.
	Scheduler SchedulerConfig `yaml:"scheduler"`

	// Notifications handles external alerts (e.g., Discord/Slack webhook).
	Notifications NotificationConfig `yaml:"notifications"`

	// Logging configures the output verbosity and storage location.
	Logging LoggingConfig `yaml:"logging"`
}

// AccountConfig defines the OCI credentials and instance specifications for a single account.
type AccountConfig struct {
	// Enabled determines if this account should be processed in the current cycle.
	Enabled bool `yaml:"enabled"`

	// OCI Authentication Details
	UserOCID    string `yaml:"user_ocid"`
	TenancyOCID string `yaml:"tenancy_ocid"`
	Fingerprint string `yaml:"fingerprint"`
	KeyFile     string `yaml:"key_file"` // Path to the RSA private key (PEM). Supports '~'.
	Region      string `yaml:"region"`   // OCI Region code (e.g., "us-ashburn-1").

	// Instance Launch Specifications
	CompartmentOCID    string  `yaml:"compartment_ocid"`
	AvailabilityDomain string  `yaml:"availability_domain"` // Set to "auto" for automatic discovery.
	SubnetOCID         string  `yaml:"subnet_ocid"`
	ImageOCID          string  `yaml:"image_ocid"`
	SSHPublicKey       string  `yaml:"ssh_public_key"` // The Public Key to inject into authorized_keys.
	Shape              string  `yaml:"shape"`          // Recommended: "VM.Standard.A1.Flex"
	OCPUs              float32 `yaml:"ocpus"`          // Max: 4 for Free Tier.
	MemoryGB           float32 `yaml:"memory_gb"`      // Max: 24 for Free Tier.
	BootVolumeSizeGB   int64   `yaml:"boot_volume_size_gb"`
	DisplayName        string  `yaml:"display_name"`
	HostnameLabel      string  `yaml:"hostname_label"`
}

// RetryConfig defines the parameters for the exponential backoff mechanism.
type RetryConfig struct {
	BaseIntervalMinutes int  `yaml:"base_interval_minutes"` // Start waiting this long.
	MaxIntervalMinutes  int  `yaml:"max_interval_minutes"`  // Cap the wait time at this limit.
	ExponentialBackoff  bool `yaml:"exponential_backoff"`   // If true, double wait time on each failure.
}

// SchedulerConfig governs the main execution loop.
type SchedulerConfig struct {
	AccountDelaySeconds  int `yaml:"account_delay_seconds"`  // Pause between accounts to avoid correlation/IP bans.
	CycleIntervalSeconds int `yaml:"cycle_interval_seconds"` // Wait time after checking all accounts before restarting.
}

// NotificationConfig holds settings for alerting the user on success/failure.
// NotificationConfig holds settings for alerting the user on success/failure.
type NotificationConfig struct {
	Enabled        bool   `yaml:"enabled"`
	WebhookURL     string `yaml:"webhook_url"`     // Generic Webhook (Discord/Slack compatible)
	InsistentPing  bool   `yaml:"insistent_ping"`  // If true, adds @everyone or similar to success Msg.
	DigestInterval string `yaml:"digest_interval"` // e.g., "24h", "1h". Empty = disabled.
}

// Deprecated: WebhookConfig is merged into top-level for simplicity, or we keep it if we want multiple providers later.
// For now, flattening it is easier for the user: notifications: { enabled: true, webhook_url: ... }

// LoggingConfig configures the application logs.
type LoggingConfig struct {
	Level  string `yaml:"level"`   // e.g., "INFO", "DEBUG".
	LogDir string `yaml:"log_dir"` // Directory to store log files (e.g., "logs").
}

// LoadConfig attempts to locate and parse the YAML configuration file.
// Prioritizes 'path' argument -> OCI_ARM_CONFIG env var -> standard file locations.
// Returns the parsed Config struct, the path of the loaded file, or an error.
func LoadConfig(path string) (*Config, string, error) {
	loadPath := path
	if loadPath == "" {
		loadPath = findConfig()
	}

	if loadPath == "" {
		return nil, "", fmt.Errorf("config.yaml not found in standard locations")
	}

	// Convert to absolute path for clarity in logs.
	if abs, err := filepath.Abs(loadPath); err == nil {
		loadPath = abs
	}

	data, err := os.ReadFile(loadPath)
	if err != nil {
		return nil, loadPath, fmt.Errorf("error reading config: %w", err)
	}

	var cfg Config
	// Apply sensible default values before parsing.
	cfg.Scheduler.AccountDelaySeconds = 450
	cfg.Scheduler.CycleIntervalSeconds = 900
	cfg.Retry.BaseIntervalMinutes = 15
	cfg.Retry.MaxIntervalMinutes = 120
	cfg.Logging.LogDir = "logs"

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, loadPath, fmt.Errorf("error parsing yaml: %w", err)
	}

	// Post-Process Paths & Validation
	for name, acc := range cfg.Accounts {
		if !acc.Enabled {
			continue
		}

		// 1. Required String Fields
		if acc.UserOCID == "" || acc.TenancyOCID == "" || acc.Fingerprint == "" || acc.Region == "" {
			return nil, loadPath, fmt.Errorf("account '%s': missing required OCID, Fingerprint, or Region", name)
		}

		// 2. Key File Path & Existence
		if strings.HasPrefix(acc.KeyFile, "~") {
			usr, _ := user.Current()
			if usr != nil {
				acc.KeyFile = filepath.Join(usr.HomeDir, acc.KeyFile[2:])
			}
		}
		if abs, err := filepath.Abs(acc.KeyFile); err == nil {
			acc.KeyFile = abs
		}
		if _, err := os.Stat(acc.KeyFile); os.IsNotExist(err) {
			return nil, loadPath, fmt.Errorf("account '%s': key file not found at %s", name, acc.KeyFile)
		}

		// 3. Resource Constraints (Sanity Checks)
		if acc.OCPUs <= 0 {
			return nil, loadPath, fmt.Errorf("account '%s': ocpus must be positive (got %f)", name, acc.OCPUs)
		}
		if acc.MemoryGB <= 0 {
			return nil, loadPath, fmt.Errorf("account '%s': memory_gb must be positive (got %f)", name, acc.MemoryGB)
		}
		if acc.BootVolumeSizeGB < 50 {
			// OCI often requires 50GB min for many images, alerting the user is helpful.
			return nil, loadPath, fmt.Errorf("account '%s': boot_volume_size_gb must be at least 50 (got %d)", name, acc.BootVolumeSizeGB)
		}
	}

	// Security/Stability
	const MinCycleInterval = 10
	if cfg.Scheduler.CycleIntervalSeconds < MinCycleInterval {
		cfg.Scheduler.CycleIntervalSeconds = MinCycleInterval
	}
	if cfg.Scheduler.AccountDelaySeconds < 0 {
		cfg.Scheduler.AccountDelaySeconds = 0
	}

	return &cfg, loadPath, nil
}

// findConfig searches for 'config.yaml' in an ordered list of standard locations.
func findConfig() string {
	// 1. Environment Variable
	if env := os.Getenv("OCI_ARM_CONFIG"); env != "" {
		return env
	}
	// 2. Current Working Directory
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	// 3. User Config Directory (~/.config/oci-arm-provisioner/)
	usr, err := user.Current()
	if err == nil {
		p := filepath.Join(usr.HomeDir, ".config", "oci-arm-provisioner", "config.yaml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// 4. System Config Directory
	if _, err := os.Stat("/etc/oci-arm-provisioner/config.yaml"); err == nil {
		return "/etc/oci-arm-provisioner/config.yaml"
	}
	return ""
}
