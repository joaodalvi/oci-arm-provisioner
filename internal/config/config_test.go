package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Validation(t *testing.T) {
	// Setup temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	mockConfig := `
accounts:
  test_account:
    enabled: true
    user_ocid: "ocid.user.1"
    tenancy_ocid: "ocid.tenancy.1"
    fingerprint: "aa:bb:cc"
    key_file: "~/keys/oci.pem"
    region: "us-ashburn-1"
retry:
  base_interval_minutes: 10
`
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
