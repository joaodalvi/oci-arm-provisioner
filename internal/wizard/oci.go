package wizard

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/yourusername/oci-arm-provisioner/internal/logger"
)

// RunOCI starts the interactive OCI configuration wizard.
func RunOCI(l *logger.Logger) {
	reader := bufio.NewReader(os.Stdin)
	l.Section("☁️  OCI Setup Wizard")
	fmt.Println("This wizard will help you create your 'config.yaml' for Oracle Cloud.")
	fmt.Println("You will need your OCI Console open.")
	fmt.Println()

	// 1. Profile Name
	fmt.Print("👉 Name this account profile (default 'default'): ")
	profileName, _ := reader.ReadString('\n')
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		profileName = "default"
	}

	// 2. Credentials
	fmt.Println("\n--- Credentials ---")
	fmt.Println("Find these in OCI Console -> Profile -> Tenancy / User Settings.")

	fmt.Print("👉 User OCID (ocid1.user...): ")
	userOCID, _ := reader.ReadString('\n')
	userOCID = strings.TrimSpace(userOCID)

	fmt.Print("👉 Tenancy OCID (ocid1.tenancy...): ")
	tenancyOCID, _ := reader.ReadString('\n')
	tenancyOCID = strings.TrimSpace(tenancyOCID)

	fmt.Print("👉 API Key Fingerprint (xx:xx:xx...): ")
	fingerprint, _ := reader.ReadString('\n')
	fingerprint = strings.TrimSpace(fingerprint)

	fmt.Print("👉 Region (e.g. us-ashburn-1, sa-saopaulo-1): ")
	region, _ := reader.ReadString('\n')
	region = strings.TrimSpace(region)

	// 3. Key File
	fmt.Println("\n--- API Key ---")
	fmt.Println("Path to your private key file (PEM).")
	fmt.Print("👉 Path (default '~/.oci/oci_api_key.pem'): ")
	keyPath, _ := reader.ReadString('\n')
	keyPath = strings.TrimSpace(keyPath)
	if keyPath == "" {
		keyPath = "~/.oci/oci_api_key.pem"
	}

	// Validate Key Path (simple check)
	expandedPath := keyPath
	if strings.HasPrefix(keyPath, "~/") {
		home, _ := os.UserHomeDir()
		expandedPath = filepath.Join(home, keyPath[2:])
	}
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		l.Error("WIZARD", fmt.Sprintf("⚠️  Warning: Key file not found at %s", expandedPath))
		fmt.Println("You can continue, but ensure the file exists before running the provisioner.")
	}

	// 4. Compartment
	fmt.Println("\n--- Compartment ---")
	fmt.Println("Press ENTER to use your Tenancy OCID (Root Compartment).")
	fmt.Print("👉 Compartment OCID: ")
	compartmentOCID, _ := reader.ReadString('\n')
	compartmentOCID = strings.TrimSpace(compartmentOCID)
	if compartmentOCID == "" {
		compartmentOCID = tenancyOCID
	}

	// 5. Resources (Always Free Defaults)
	fmt.Println("\n--- Instance Config ---")
	fmt.Print("👉 Use 'Always Free' ARM defaults (4 OCPU, 24GB RAM)? (Y/n): ")
	useDefaults, _ := reader.ReadString('\n')
	useDefaults = strings.TrimSpace(useDefaults)

	var shape string
	var ocpus, memory float32
	// var bootVol int

	if strings.ToLower(useDefaults) == "n" {
		// Custom
		fmt.Print("👉 Shape (e.g. VM.Standard.A1.Flex): ")
		shape, _ = reader.ReadString('\n')
		shape = strings.TrimSpace(shape)

		fmt.Print("👉 OCPUs (1-4): ")
		fmt.Scanf("%f", &ocpus)

		fmt.Print("👉 Memory (GB): ")
		fmt.Scanf("%f", &memory)
		reader.ReadString('\n') // clear buffer
	} else {
		shape = "VM.Standard.A1.Flex" // Updated to correct Shape Name
		ocpus = 4
		memory = 24
	}

	// SSH Key
	fmt.Println("\n--- SSH Access ---")
	fmt.Println("Paste your public key (starts with ssh-rsa ...).")
	fmt.Print("👉 SSH Public Key: ")
	sshKey, _ := reader.ReadString('\n')
	sshKey = strings.TrimSpace(sshKey)

	// 6. Generate Config
	err := saveOCIConfig("config.yaml", profileName, userOCID, tenancyOCID, fingerprint, keyPath, region, compartmentOCID, shape, ocpus, memory, sshKey)
	if err != nil {
		l.Error("WIZARD", fmt.Sprintf("Failed to save config: %v", err))
		return
	}
	l.Success("WIZARD", "✅ config.yaml created successfully!")

	// 7. Chain Notification Wizard
	fmt.Println("\n--- Notifications ---")
	fmt.Print("👉 Do you want to configure alerts (Discord/Telegram/etc) now? (y/N): ")
	wantNotes, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(wantNotes)) == "y" {
		RunNotifications(l)
	} else {
		fmt.Println("\nConfiguration complete! You can set up alerts later with '--setup-notifications'.")
		fmt.Println("Run './oci-arm-provisioner' to start!")
	}
}

const configTemplate = `accounts:
  {{.ProfileName}}:
    enabled: true
    user_ocid: "{{.UserOCID}}"
    tenancy_ocid: "{{.TenancyOCID}}"
    fingerprint: "{{.Fingerprint}}"
    key_file: "{{.KeyPath}}"
    region: "{{.Region}}"
    compartment_ocid: "{{.CompartmentOCID}}"
    availability_domain: "auto"
    shape: "{{.Shape}}"
    ocpus: {{.OCPUs}}
    memory_gb: {{.Memory}}
    image_ocid: "ocid1.image.oc1.iad.aaaa..." # TODO: UPDATE THIS ID FROM ORACLE DOCS FOR YOUR REGION
    ssh_public_key: "{{.SSHKey}}"
    boot_volume_size_gb: 50
    display_name: "arm-instance-1"
    hostname_label: "arm-1"

scheduler:
  account_delay_seconds: 30
  cycle_interval_seconds: 60

retry:
  base_interval_minutes: 15
  max_interval_minutes: 120
  exponential_backoff: true

logging:
  level: "INFO"
  log_dir: "logs"

notifications:
  enabled: false
`

type configData struct {
	ProfileName     string
	UserOCID        string
	TenancyOCID     string
	Fingerprint     string
	KeyPath         string
	Region          string
	CompartmentOCID string
	Shape           string
	OCPUs           float32
	Memory          float32
	SSHKey          string
}

func saveOCIConfig(path, profile, user, tenancy, finger, key, region, compartment, shape string, ocpus, memory float32, ssh string) error {
	if _, err := os.Stat(path); err == nil {
		// File exists
		// For now, we backup and overwrite, OR we could warn.
		// "Pro" move is to backup.
		os.Rename(path, path+".bak")
		fmt.Printf("⚠️  Existing %s moved to %s.bak\n", path, path)
	}

	t, err := template.New("config").Parse(configTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	data := configData{
		ProfileName:     profile,
		UserOCID:        user,
		TenancyOCID:     tenancy,
		Fingerprint:     finger,
		KeyPath:         key,
		Region:          region,
		CompartmentOCID: compartment,
		Shape:           shape,
		OCPUs:           ocpus,
		Memory:          memory,
		SSHKey:          ssh,
	}

	return t.Execute(f, data)
}
