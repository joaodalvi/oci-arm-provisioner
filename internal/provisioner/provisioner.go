package provisioner

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/yourusername/oci-arm-provisioner/internal/config"
	"github.com/yourusername/oci-arm-provisioner/internal/logger"
	"github.com/yourusername/oci-arm-provisioner/internal/notifier"
)

// ComputeClientOps defines the interface for OCI Compute operations, enabling testing/mocking.
type ComputeClientOps interface {
	LaunchInstance(ctx context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error)
	ListInstances(ctx context.Context, request core.ListInstancesRequest) (core.ListInstancesResponse, error)
	GetInstance(ctx context.Context, request core.GetInstanceRequest) (core.GetInstanceResponse, error)
	ListVnicAttachments(ctx context.Context, request core.ListVnicAttachmentsRequest) (core.ListVnicAttachmentsResponse, error)
}

// VirtualNetworkClientOps defines the interface for OCI Virtual Network operations.
type VirtualNetworkClientOps interface {
	GetVnic(ctx context.Context, request core.GetVnicRequest) (core.GetVnicResponse, error)
}

// IdentityClientOps defines the interface for OCI Identity operations.
type IdentityClientOps interface {
	ListAvailabilityDomains(ctx context.Context, request identity.ListAvailabilityDomainsRequest) (identity.ListAvailabilityDomainsResponse, error)
}

// SimpleConfigProvider is a wrapper around OCI's RawConfigurationProvider to support
// in-memory RSA keys loaded from files that might not use standard paths.
type SimpleConfigProvider struct {
	common.ConfigurationProvider
	Key *rsa.PrivateKey
}

// PrivateRSAKey returns the loaded private key, satisfying the OCI ConfigurationProvider interface.
func (p *SimpleConfigProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	return p.Key, nil
}

// Provisioner is the main manager that orchestrates provisioning across multiple accounts.
// It holds the workers (one per account) and global configuration.
type Provisioner struct {
	Config      *config.Config
	Logger      *logger.Logger
	Notifier    *notifier.Notifier
	Tracker     *notifier.Tracker
	Workers     []*AccountWorker // List of initialized workers for enabled accounts.
	Provisioned map[string]bool  // Tracks accounts that have successfully provisioned.
}

// New initializes the Provisioner manager.
// It iterates through the enabled accounts in the configuration and creates an AccountWorker for each.
func New(cfg *config.Config, log *logger.Logger, tracker *notifier.Tracker) *Provisioner {
	n := notifier.New(cfg.Notifications)

	p := &Provisioner{
		Config:      cfg,
		Logger:      log,
		Notifier:    n,
		Tracker:     tracker,
		Workers:     make([]*AccountWorker, 0),
		Provisioned: make(map[string]bool),
	}

	// Initialize workers for all enabled accounts
	for name, accConfig := range cfg.Accounts {
		if accConfig.Enabled {
			worker := &AccountWorker{
				AccountName: name,
				Config:      accConfig,
				Logger:      log,
				Notifier:    n,
				Tracker:     tracker,
			}
			p.Workers = append(p.Workers, worker)
		}
	}

	return p
}

// RunCycle executes one provisioning pass for all enabled accounts.
// It respects the configured delay between accounts to avoid IP correlation/rate-limiting.
func (p *Provisioner) RunCycle(ctx context.Context) {
	p.Tracker.IncCycle()
	for i, worker := range p.Workers {
		// Check for cancellation before starting work on an account
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Skip accounts that are already provisioned
		if p.Provisioned[worker.AccountName] {
			p.Logger.Info(worker.AccountName, "✅ Already provisioned - skipping")
			continue
		}

		// Execute provision logic for the worker
		success, _, err := worker.Provision(ctx)
		if err != nil {
			p.Logger.Error(worker.AccountName, fmt.Sprintf("Cycle failed: %v", err))
		}

		// Mark as provisioned on success
		if success {
			p.Provisioned[worker.AccountName] = true
		}

		// Sleep between accounts (but not after the last one)
		if i < len(p.Workers)-1 {
			if p.Config.Scheduler.AccountDelaySeconds > 0 {
				delay := time.Duration(p.Config.Scheduler.AccountDelaySeconds) * time.Second
				p.Logger.Info("SCHEDULER", fmt.Sprintf("Waiting %ds before next account...", p.Config.Scheduler.AccountDelaySeconds))

				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
					// Continue
				}
			}
		}
	}
}

// AccountWorker handles the provisioning logic for a single OCI account.
type AccountWorker struct {
	AccountName          string
	Config               *config.AccountConfig
	Logger               *logger.Logger
	Notifier             *notifier.Notifier
	Tracker              *notifier.Tracker
	ComputeClient        ComputeClientOps
	IdentityClient       IdentityClientOps
	VirtualNetworkClient VirtualNetworkClientOps
}

// getProvider loads the OCI credentials and creates a ConfigurationProvider.
// It performs security checks on the key file permissions and size.
func (w *AccountWorker) getProvider() (common.ConfigurationProvider, error) {
	// 1. Safety Checks: Verify key file existence and size.
	info, err := os.Stat(w.Config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("key file not found: %s", w.Config.KeyFile)
	}

	const MaxKeySize = 16 * 1024
	if info.Size() > MaxKeySize {
		return nil, fmt.Errorf("key file too large (%d bytes), max is %d", info.Size(), MaxKeySize)
	}

	// Permission warning
	mode := info.Mode()
	if mode&0077 != 0 {
		w.Logger.Warn(w.AccountName, fmt.Sprintf("Key file '%s' has permissive permissions (%o). It should be 400 or 600.", w.Config.KeyFile, mode))
	}

	// 2. Read Key File
	content, err := os.ReadFile(w.Config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// 3. Decode PEM
	block, _ := pem.Decode(content)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from %s", w.Config.KeyFile)
	}

	// 4. Parse RSA Key (Try PKCS1 first, then PKCS8)
	var key *rsa.PrivateKey
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = k
	} else {
		if k8, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
			if rsaKey, ok := k8.(*rsa.PrivateKey); ok {
				key = rsaKey
			}
		}
	}

	if key == nil {
		return nil, fmt.Errorf("failed to parse private key from %s (ensure RSA PEM)", w.Config.KeyFile)
	}

	// Create OCI Provider
	baseProvider := common.NewRawConfigurationProvider(
		w.Config.TenancyOCID,
		w.Config.UserOCID,
		w.Config.Region,
		w.Config.Fingerprint,
		"",  // Passphrase (not supported in simple config)
		nil, // private key loaded manually below
	)

	return &SimpleConfigProvider{
		ConfigurationProvider: baseProvider,
		Key:                   key,
	}, nil
}

// initClients initializes the OCI Compute, Identity, and VirtualNetwork clients if they haven't been already.
func (w *AccountWorker) initClients() error {
	if w.ComputeClient != nil && w.IdentityClient != nil && w.VirtualNetworkClient != nil {
		return nil
	}

	provider, err := w.getProvider()
	if err != nil {
		return err
	}

	if w.ComputeClient == nil {
		client, err := core.NewComputeClientWithConfigurationProvider(provider)
		if err != nil {
			return fmt.Errorf("failed to create compute client: %w", err)
		}
		w.ComputeClient = &client
	}

	if w.IdentityClient == nil {
		client, err := identity.NewIdentityClientWithConfigurationProvider(provider)
		if err != nil {
			return fmt.Errorf("failed to create identity client: %w", err)
		}
		w.IdentityClient = &client
	}

	if w.VirtualNetworkClient == nil {
		client, err := core.NewVirtualNetworkClientWithConfigurationProvider(provider)
		if err != nil {
			return fmt.Errorf("failed to create virtual network client: %w", err)
		}
		w.VirtualNetworkClient = &client
	}

	return nil
}

// Provision attempts to create the configured instance.
// It checks for existing instances, resolves the AD, and handles OCI errors/retries.
// Returns: (success, retryable, error)
func (w *AccountWorker) Provision(parentCtx context.Context) (bool, bool, error) {
	// Add timeout to prevent hanging on network issues
	ctx, cancel := context.WithTimeout(parentCtx, 60*time.Second)
	defer cancel()

	if err := w.initClients(); err != nil {
		return false, false, err
	}

	w.Logger.Info(w.AccountName, "Checking for existing instances...")
	existing, err := w.checkExisting(ctx)
	if err != nil {
		return false, false, err
	}
	if existing {
		w.Logger.Info(w.AccountName, "Instance already exists. Stopping.")
		return true, false, nil
	}

	// Auto-Detect Availability Domain if set to "auto"
	ad := w.Config.AvailabilityDomain
	if ad == "auto" {
		req := identity.ListAvailabilityDomainsRequest{
			CompartmentId: common.String(w.Config.TenancyOCID),
		}
		resp, err := w.IdentityClient.ListAvailabilityDomains(ctx, req)
		if err != nil {
			return false, false, fmt.Errorf("failed to list ADs: %w", err)
		}
		if len(resp.Items) == 0 {
			return false, false, fmt.Errorf("no ADs found")
		}
		// Typically pick the first one, or round-robin could be implemented here.
		ad = *resp.Items[0].Name
		w.Logger.Info(w.AccountName, fmt.Sprintf("Auto-selected AD: %s", ad))
	}

	w.Logger.Info(w.AccountName, fmt.Sprintf("Launching instance '%s'...", w.Config.DisplayName))

	// Construct Launch Request
	req := core.LaunchInstanceRequest{
		LaunchInstanceDetails: core.LaunchInstanceDetails{
			AvailabilityDomain: common.String(ad),
			CompartmentId:      common.String(w.Config.CompartmentOCID),
			DisplayName:        common.String(w.Config.DisplayName),
			Shape:              common.String(w.Config.Shape),
			ShapeConfig: &core.LaunchInstanceShapeConfigDetails{
				Ocpus:       common.Float32(w.Config.OCPUs),
				MemoryInGBs: common.Float32(w.Config.MemoryGB),
			},
			SourceDetails: core.InstanceSourceViaImageDetails{
				ImageId:             common.String(w.Config.ImageOCID),
				BootVolumeSizeInGBs: common.Int64(w.Config.BootVolumeSizeGB),
			},
			CreateVnicDetails: &core.CreateVnicDetails{
				SubnetId:       common.String(w.Config.SubnetOCID),
				AssignPublicIp: common.Bool(true),
				HostnameLabel:  common.String(w.Config.HostnameLabel),
			},
			Metadata: map[string]string{
				"ssh_authorized_keys": w.Config.SSHPublicKey,
			},
		},
	}

	// API Call
	resp, err := w.ComputeClient.LaunchInstance(ctx, req)
	if err != nil {
		if serviceErr, ok := common.IsServiceError(err); ok {
			code := serviceErr.GetHTTPStatusCode()
			msg := strings.ToLower(serviceErr.GetMessage())

			w.Logger.Warn(w.AccountName, fmt.Sprintf("OCI Error %d: %s", code, serviceErr.GetMessage()))

			// Handle Capacity/Limit errors gracefully (Retryable)
			if code == 500 || strings.Contains(msg, "capacity") || strings.Contains(msg, "limit") {
				w.Logger.Warn(w.AccountName, "Capacity/Limit error. Will retry.")
				w.Tracker.IncCapacity()
				return false, true, nil
			}
			// Handle Rate Limiting (Retryable)
			if code == 429 {
				w.Logger.Warn(w.AccountName, "Rate limited. Will retry.")
				w.Tracker.IncError()
				return false, true, nil
			}
		}
		// Non-retryable error
		w.Tracker.IncError()
		return false, false, err
	}

	// SUCCESS! Instance was launched.
	instanceID := *resp.Instance.Id
	w.Logger.Success(w.AccountName, fmt.Sprintf("Instance Launched: %s", instanceID))

	// Extended verification with longer timeout context
	verifyCtx, verifyCancel := context.WithTimeout(parentCtx, 6*time.Minute)
	defer verifyCancel()

	verified, verifyErr := w.VerifyInstance(verifyCtx, instanceID)
	if verifyErr != nil {
		w.Logger.Warn(w.AccountName, fmt.Sprintf("Verification warning: %v", verifyErr))
	}

	// Track success
	w.Tracker.IncSuccess()

	// Celebration Banner with terminal beep
	w.Logger.Celebrate(w.AccountName, verified)

	// Send notification with verified details - log any failures
	if err := w.Notifier.SendSuccessVerified(w.AccountName, verified); err != nil {
		w.Logger.Error(w.AccountName, fmt.Sprintf("Notification failed: %v", err))
	}

	return true, false, nil
}

// checkExisting queries OCI to see if an instance with the configured DisplayName already exists
// and is in a non-terminated state.
func (w *AccountWorker) checkExisting(ctx context.Context) (bool, error) {
	req := core.ListInstancesRequest{
		CompartmentId: common.String(w.Config.CompartmentOCID),
		DisplayName:   common.String(w.Config.DisplayName),
	}
	resp, err := w.ComputeClient.ListInstances(ctx, req)
	if err != nil {
		return false, err
	}
	for _, inst := range resp.Items {
		state := inst.LifecycleState
		// Check for active states strings
		if state == core.InstanceLifecycleStateRunning ||
			state == core.InstanceLifecycleStateProvisioning ||
			state == core.InstanceLifecycleStateStarting {
			return true, nil
		}
	}
	return false, nil
}
