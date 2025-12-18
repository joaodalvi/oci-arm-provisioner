package wizard

import (
	"os"
	"strings"
	"testing"
)

func TestSaveOCIConfig(t *testing.T) {
	tmpFile := "config_test_gen.yaml"
	defer os.Remove(tmpFile)
	defer os.Remove(tmpFile + ".bak")

	// 1. Create Config
	err := saveOCIConfig(
		tmpFile,
		"test-profile",
		"ocid1.user.test",
		"ocid1.tenancy.test",
		"xx:xx:xx",
		"/tmp/key.pem",
		"us-sanjose-1",
		"ocid1.compartment.test",
		"VM.Standard.A1.Flex",
		4,
		24,
		"ssh-rsa AAAA...",
	)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// 2. Read Config
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read generated config: %v", err)
	}
	s := string(content)

	// 3. Assert Content
	checks := []struct {
		name string
		want string
	}{
		{"Profile", "test-profile:"},
		{"UserOCID", `user_ocid: "ocid1.user.test"`},
		{"TenancyOCID", `tenancy_ocid: "ocid1.tenancy.test"`},
		{"Region", `region: "us-sanjose-1"`},
		{"Shape", `shape: "VM.Standard.A1.Flex"`},
		{"OCPUs", "ocpus: 4"},
		{"Memory", "memory_gb: 24"},
		{"SSHKey", `ssh_public_key: "ssh-rsa AAAA..."`},
	}

	for _, c := range checks {
		if !strings.Contains(s, c.want) {
			t.Errorf("Config missing %s. Want %s", c.name, c.want)
		}
	}

	// 4. Test Backup Logic
	// Write again to trigger backup
	err = saveOCIConfig(tmpFile, "p2", "u2", "t2", "f2", "k2", "r2", "c2", "s2", 1, 1, "ssh2")
	if err != nil {
		t.Fatalf("Failed to overwrite config: %v", err)
	}

	if _, err := os.Stat(tmpFile + ".bak"); os.IsNotExist(err) {
		t.Error("Backup file was not created on overwrite")
	}
}
