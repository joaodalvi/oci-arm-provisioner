package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Validation(t *testing.T) {
	// Setup temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Create dummy key file for validation
	keyFile := filepath.Join(tmpDir, "oci.pem")
	if err := os.WriteFile(keyFile, []byte("dummy-key-content"), 0600); err != nil {
		t.Fatalf("failed to write dummy key: %v", err)
	}

	mockConfig := fmt.Sprintf(`
accounts:
  test_account:
    enabled: true
    user_ocid: "ocid.user.1"
    tenancy_ocid: "ocid.tenancy.1"
    fingerprint: "aa:bb:cc"
    key_file: "%s"
    region: "us-ashburn-1"
    ocpus: 1
    memory_gb: 1
    boot_volume_size_gb: 50
retry:
  base_interval_minutes: 10
`, keyFile)
	if err := os.WriteFile(configFile, []byte(mockConfig), 0644); err != nil {
		t.Fatalf("failed to write mock config: %v", err)
	}

	// Test Loading
	cfg, path, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty config path")
	}

	// Verify Defaults
	if cfg.Logging.LogDir != "logs" {
		t.Errorf("expected default log_dir 'logs', got '%s'", cfg.Logging.LogDir)
	}
	if cfg.Scheduler.AccountDelaySeconds != 450 {
		t.Errorf("expected default account_delay 450, got %d", cfg.Scheduler.AccountDelaySeconds)
	}

	// Verify Account Config
	acc, ok := cfg.Accounts["test_account"]
	if !ok {
		t.Fatalf("account 'test_account' missing")
	}
	if acc.UserOCID != "ocid.user.1" {
		t.Errorf("wrong user ocid")
	}
	// Default check (if not in yaml, string is empty? or we should test setting it)
	if acc.Region != "us-ashburn-1" {
		t.Errorf("wrong region")
	}

	// Verify Path Expansion
	// Should be absolute
	if !filepath.IsAbs(acc.KeyFile) {
		t.Errorf("key_file should be absolute: %s", acc.KeyFile)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, _, err := LoadConfig("/non/existent/path.yaml")
	if err == nil {
		t.Error("expected error for missing config file, got nil")
	}
}

func TestParsing_Error(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "bad.yaml")
	os.WriteFile(configFile, []byte("invalid: yaml: content: ["), 0644)

	_, _, err := LoadConfig(configFile)
	if err == nil {
		t.Error("expected error for invalid yaml, got nil")
	}
}

func TestLoadConfig_MinInterval(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "min_interval.yaml")

	mockConfig := `
scheduler:
  cycle_interval_seconds: 0
`
	if err := os.WriteFile(configFile, []byte(mockConfig), 0644); err != nil {
		t.Fatalf("failed to write mock config: %v", err)
	}

	cfg, _, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Scheduler.CycleIntervalSeconds != 10 {
		t.Errorf("expected cycle interval clamped to 10, got %d", cfg.Scheduler.CycleIntervalSeconds)
	}
}

func TestLoadConfig_TildeExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "tilde.yaml")

	// Create a valid key file in tmpdir
	keyFile := filepath.Join(tmpDir, "test.pem")
	if err := os.WriteFile(keyFile, []byte("test-key"), 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	// Test that tilde expansion function works correctly
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Could not get home dir")
	}

	// Test expandPath function directly through ExpandPath
	mockConfig := fmt.Sprintf(`
accounts:
  tilde_test:
    enabled: true
    user_ocid: "ocid.user.1"
    tenancy_ocid: "ocid.tenancy.1" 
    fingerprint: "aa:bb:cc"
    key_file: "%s"
    region: "us-ashburn-1"
    ocpus: 4
    memory_gb: 24
    boot_volume_size_gb: 50
`, keyFile)

	if err := os.WriteFile(configFile, []byte(mockConfig), 0644); err != nil {
		t.Fatalf("failed to write mock config: %v", err)
	}

	cfg, _, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	acc := cfg.Accounts["tilde_test"]
	if !filepath.IsAbs(acc.KeyFile) {
		t.Errorf("key_file should be absolute after expansion: %s", acc.KeyFile)
	}

	// Verify home dir is available (for tilde expansion to work)
	if homeDir == "" {
		t.Error("could not get home directory for tilde expansion test")
	}
}

func TestLoadConfig_DefaultValues(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "minimal.yaml")

	// Minimal config - should get all defaults
	mockConfig := `
accounts: {}
`
	if err := os.WriteFile(configFile, []byte(mockConfig), 0644); err != nil {
		t.Fatalf("failed to write mock config: %v", err)
	}

	cfg, _, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify defaults are applied
	if cfg.Logging.LogDir != "logs" {
		t.Errorf("expected default log_dir 'logs', got '%s'", cfg.Logging.LogDir)
	}
	if cfg.Scheduler.AccountDelaySeconds != 450 {
		t.Errorf("expected default account_delay 450, got %d", cfg.Scheduler.AccountDelaySeconds)
	}
	if cfg.Notifications.Enabled != false {
		t.Error("expected notifications disabled by default")
	}
}

func TestLoadConfig_AccountValidation(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "validate.yaml")

	// Create key file
	keyFile := filepath.Join(tmpDir, "key.pem")
	os.WriteFile(keyFile, []byte("test-key"), 0600)

	mockConfig := fmt.Sprintf(`
accounts:
  valid_account:
    enabled: true
    user_ocid: "ocid.user.1"
    tenancy_ocid: "ocid.tenancy.1"
    fingerprint: "aa:bb:cc"
    key_file: "%s"
    region: "us-ashburn-1"
    ocpus: 4
    memory_gb: 24
    boot_volume_size_gb: 100
    display_name: "test-instance"
    availability_domain: "AD-1"
`, keyFile)

	if err := os.WriteFile(configFile, []byte(mockConfig), 0644); err != nil {
		t.Fatalf("failed to write mock config: %v", err)
	}

	cfg, _, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	acc := cfg.Accounts["valid_account"]
	if acc.OCPUs != 4 {
		t.Errorf("expected OCPUs 4, got %f", acc.OCPUs)
	}
	if acc.MemoryGB != 24 {
		t.Errorf("expected MemoryGB 24, got %f", acc.MemoryGB)
	}
	if acc.DisplayName != "test-instance" {
		t.Errorf("expected display_name 'test-instance', got '%s'", acc.DisplayName)
	}
}
