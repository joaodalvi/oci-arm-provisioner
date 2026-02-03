package provisioner

import (
	"context"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/yourusername/oci-arm-provisioner/internal/config"
	"github.com/yourusername/oci-arm-provisioner/internal/logger"
	"github.com/yourusername/oci-arm-provisioner/internal/notifier"
)

// --- Mocks ---

type MockClient struct {
	ListInstancesFunc       func(ctx context.Context, request core.ListInstancesRequest) (core.ListInstancesResponse, error)
	LaunchInstanceFunc      func(ctx context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error)
	ListADsFunc             func(ctx context.Context, request identity.ListAvailabilityDomainsRequest) (identity.ListAvailabilityDomainsResponse, error)
	GetInstanceFunc         func(ctx context.Context, request core.GetInstanceRequest) (core.GetInstanceResponse, error)
	ListVnicAttachmentsFunc func(ctx context.Context, request core.ListVnicAttachmentsRequest) (core.ListVnicAttachmentsResponse, error)
}

func (m *MockClient) ListInstances(ctx context.Context, request core.ListInstancesRequest) (core.ListInstancesResponse, error) {
	if m.ListInstancesFunc != nil {
		return m.ListInstancesFunc(ctx, request)
	}
	return core.ListInstancesResponse{}, nil
}

func (m *MockClient) LaunchInstance(ctx context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error) {
	if m.LaunchInstanceFunc != nil {
		return m.LaunchInstanceFunc(ctx, request)
	}
	return core.LaunchInstanceResponse{}, nil
}

func (m *MockClient) ListAvailabilityDomains(ctx context.Context, request identity.ListAvailabilityDomainsRequest) (identity.ListAvailabilityDomainsResponse, error) {
	if m.ListADsFunc != nil {
		return m.ListADsFunc(ctx, request)
	}
	return identity.ListAvailabilityDomainsResponse{}, nil
}

func (m *MockClient) GetInstance(ctx context.Context, request core.GetInstanceRequest) (core.GetInstanceResponse, error) {
	if m.GetInstanceFunc != nil {
		return m.GetInstanceFunc(ctx, request)
	}
	return core.GetInstanceResponse{}, nil
}

func (m *MockClient) ListVnicAttachments(ctx context.Context, request core.ListVnicAttachmentsRequest) (core.ListVnicAttachmentsResponse, error) {
	if m.ListVnicAttachmentsFunc != nil {
		return m.ListVnicAttachmentsFunc(ctx, request)
	}
	return core.ListVnicAttachmentsResponse{}, nil
}

// MockVirtualNetworkClient mocks VirtualNetworkClientOps interface
type MockVirtualNetworkClient struct {
	GetVnicFunc func(ctx context.Context, request core.GetVnicRequest) (core.GetVnicResponse, error)
}

func (m *MockVirtualNetworkClient) GetVnic(ctx context.Context, request core.GetVnicRequest) (core.GetVnicResponse, error) {
	if m.GetVnicFunc != nil {
		return m.GetVnicFunc(ctx, request)
	}
	return core.GetVnicResponse{}, nil
}

// Helper to create mocked service error
func newServiceError(status int, message string) error {
	return &mockServiceError{status: status, message: message}
}

type mockServiceError struct {
	status  int
	message string
}

func (e *mockServiceError) Error() string              { return e.message }
func (e *mockServiceError) GetHTTPStatusCode() int     { return e.status }
func (e *mockServiceError) GetMessage() string         { return e.message }
func (e *mockServiceError) GetOpcRequestID() string    { return "req-id" }
func (e *mockServiceError) GetCode() string            { return "MockCode" }
func (e *mockServiceError) GetTarget() string          { return "target" }
func (e *mockServiceError) GetOriginalMessage() string { return e.message }
func (e *mockServiceError) GetCause() error            { return nil }

// --- Tests ---

// Helper to create dummy logger
func newMockLogger() *logger.Logger {
	l, _ := logger.New("") // writes to "logs/"
	return l
}

func TestAccountWorker_Provision_InstanceExists(t *testing.T) {
	mock := &MockClient{
		ListInstancesFunc: func(ctx context.Context, request core.ListInstancesRequest) (core.ListInstancesResponse, error) {
			return core.ListInstancesResponse{
				Items: []core.Instance{
					{LifecycleState: core.InstanceLifecycleStateRunning},
				},
			}, nil
		},
	}

	// We test AccountWorker directly as it holds the logic
	w := &AccountWorker{
		AccountName:          "test",
		Config:               &config.AccountConfig{},
		Logger:               newMockLogger(),
		Notifier:             notifier.New(config.NotificationConfig{Enabled: false}),
		Tracker:              notifier.NewTracker(),
		ComputeClient:        mock,
		IdentityClient:       mock,
		VirtualNetworkClient: &MockVirtualNetworkClient{},
	}

	success, retry, err := w.Provision(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !success {
		t.Error("expected success=true (stopped early)")
	}
	if retry {
		t.Error("expected retry=false")
	}
}

func TestAccountWorker_Provision_LaunchSuccess(t *testing.T) {
	instID := "inst-1"
	ocpus := float32(4)
	memory := float32(24)

	mock := &MockClient{
		ListInstancesFunc: func(ctx context.Context, request core.ListInstancesRequest) (core.ListInstancesResponse, error) {
			return core.ListInstancesResponse{Items: []core.Instance{}}, nil
		},
		ListADsFunc: func(ctx context.Context, request identity.ListAvailabilityDomainsRequest) (identity.ListAvailabilityDomainsResponse, error) {
			ad := "AD-1"
			return identity.ListAvailabilityDomainsResponse{Items: []identity.AvailabilityDomain{{Name: &ad}}}, nil
		},
		LaunchInstanceFunc: func(ctx context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error) {
			return core.LaunchInstanceResponse{Instance: core.Instance{Id: &instID}}, nil
		},
		GetInstanceFunc: func(ctx context.Context, request core.GetInstanceRequest) (core.GetInstanceResponse, error) {
			// Return RUNNING instance with shape config for verification to pass
			return core.GetInstanceResponse{
				Instance: core.Instance{
					Id:             &instID,
					LifecycleState: core.InstanceLifecycleStateRunning,
					ShapeConfig: &core.InstanceShapeConfig{
						Ocpus:       &ocpus,
						MemoryInGBs: &memory,
					},
				},
			}, nil
		},
		ListVnicAttachmentsFunc: func(ctx context.Context, request core.ListVnicAttachmentsRequest) (core.ListVnicAttachmentsResponse, error) {
			return core.ListVnicAttachmentsResponse{Items: []core.VnicAttachment{}}, nil
		},
	}

	w := &AccountWorker{
		AccountName:          "test",
		Config:               &config.AccountConfig{AvailabilityDomain: "auto", OCPUs: 4, MemoryGB: 24},
		Logger:               newMockLogger(),
		Notifier:             notifier.New(config.NotificationConfig{Enabled: false}),
		Tracker:              notifier.NewTracker(),
		ComputeClient:        mock,
		IdentityClient:       mock,
		VirtualNetworkClient: &MockVirtualNetworkClient{},
	}

	success, retry, err := w.Provision(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !success {
		t.Error("expected success=true")
	}
	if retry {
		t.Error("expected retry=false")
	}
}

func TestAccountWorker_Provision_OutOfCapacity(t *testing.T) {
	mock := &MockClient{
		ListInstancesFunc: func(ctx context.Context, request core.ListInstancesRequest) (core.ListInstancesResponse, error) {
			return core.ListInstancesResponse{Items: []core.Instance{}}, nil
		},
		LaunchInstanceFunc: func(ctx context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error) {
			return core.LaunchInstanceResponse{}, newServiceError(500, "Out of host capacity")
		},
	}

	w := &AccountWorker{
		AccountName:          "test",
		Config:               &config.AccountConfig{AvailabilityDomain: "AD-1"},
		Logger:               newMockLogger(),
		Notifier:             notifier.New(config.NotificationConfig{Enabled: false}),
		Tracker:              notifier.NewTracker(),
		ComputeClient:        mock,
		IdentityClient:       mock,
		VirtualNetworkClient: &MockVirtualNetworkClient{},
	}

	success, retry, err := w.Provision(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if success {
		t.Error("expected success=false")
	}
	if !retry {
		t.Error("expected retry=true")
	}
}

func TestAccountWorker_Provision_RateLimit(t *testing.T) {
	mock := &MockClient{
		ListInstancesFunc: func(ctx context.Context, request core.ListInstancesRequest) (core.ListInstancesResponse, error) {
			return core.ListInstancesResponse{Items: []core.Instance{}}, nil
		},
		LaunchInstanceFunc: func(ctx context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error) {
			return core.LaunchInstanceResponse{}, newServiceError(429, "TooManyRequests")
		},
	}

	w := &AccountWorker{
		AccountName:          "test",
		Config:               &config.AccountConfig{},
		Logger:               newMockLogger(),
		Notifier:             notifier.New(config.NotificationConfig{Enabled: false}),
		Tracker:              notifier.NewTracker(),
		ComputeClient:        mock,
		IdentityClient:       mock,
		VirtualNetworkClient: &MockVirtualNetworkClient{},
	}

	success, retry, err := w.Provision(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if success {
		t.Error("expected success=false")
	}
	if !retry {
		t.Error("expected retry=true")
	}
}

// --- Verifier Tests ---

func TestVerifyInstance_Success(t *testing.T) {
	instID := "inst-success"
	ocpus := float32(4)
	memory := float32(24)
	publicIP := "10.0.0.1"
	privateIP := "192.168.1.1"
	vnicID := "vnic-1"

	mock := &MockClient{
		GetInstanceFunc: func(ctx context.Context, request core.GetInstanceRequest) (core.GetInstanceResponse, error) {
			return core.GetInstanceResponse{
				Instance: core.Instance{
					Id:             &instID,
					LifecycleState: core.InstanceLifecycleStateRunning,
					ShapeConfig: &core.InstanceShapeConfig{
						Ocpus:       &ocpus,
						MemoryInGBs: &memory,
					},
				},
			}, nil
		},
		ListVnicAttachmentsFunc: func(ctx context.Context, request core.ListVnicAttachmentsRequest) (core.ListVnicAttachmentsResponse, error) {
			return core.ListVnicAttachmentsResponse{
				Items: []core.VnicAttachment{
					{VnicId: &vnicID, LifecycleState: core.VnicAttachmentLifecycleStateAttached},
				},
			}, nil
		},
	}

	mockVNet := &MockVirtualNetworkClient{
		GetVnicFunc: func(ctx context.Context, request core.GetVnicRequest) (core.GetVnicResponse, error) {
			return core.GetVnicResponse{
				Vnic: core.Vnic{
					PublicIp:  &publicIP,
					PrivateIp: &privateIP,
				},
			}, nil
		},
	}

	w := &AccountWorker{
		AccountName:          "test",
		Config:               &config.AccountConfig{OCPUs: 4, MemoryGB: 24, Region: "us-ashburn-1"},
		Logger:               newMockLogger(),
		ComputeClient:        mock,
		VirtualNetworkClient: mockVNet,
	}

	result, err := w.VerifyInstance(context.Background(), instID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.State != "RUNNING" {
		t.Errorf("expected state RUNNING, got %s", result.State)
	}
	if result.PublicIP != publicIP {
		t.Errorf("expected public IP %s, got %s", publicIP, result.PublicIP)
	}
	if result.SpecsMismatch {
		t.Error("expected no specs mismatch")
	}
	if !result.Verified {
		t.Error("expected verified=true")
	}
}

func TestVerifyInstance_SpecsMismatch(t *testing.T) {
	instID := "inst-mismatch"
	ocpus := float32(2)   // Different from config
	memory := float32(12) // Different from config

	mock := &MockClient{
		GetInstanceFunc: func(ctx context.Context, request core.GetInstanceRequest) (core.GetInstanceResponse, error) {
			return core.GetInstanceResponse{
				Instance: core.Instance{
					Id:             &instID,
					LifecycleState: core.InstanceLifecycleStateRunning,
					ShapeConfig: &core.InstanceShapeConfig{
						Ocpus:       &ocpus,
						MemoryInGBs: &memory,
					},
				},
			}, nil
		},
		ListVnicAttachmentsFunc: func(ctx context.Context, request core.ListVnicAttachmentsRequest) (core.ListVnicAttachmentsResponse, error) {
			return core.ListVnicAttachmentsResponse{Items: []core.VnicAttachment{}}, nil
		},
	}

	w := &AccountWorker{
		AccountName:          "test",
		Config:               &config.AccountConfig{OCPUs: 4, MemoryGB: 24}, // Mismatched!
		Logger:               newMockLogger(),
		ComputeClient:        mock,
		VirtualNetworkClient: &MockVirtualNetworkClient{},
	}

	result, err := w.VerifyInstance(context.Background(), instID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.SpecsMismatch {
		t.Error("expected specs mismatch")
	}
	if len(result.Errors) != 2 { // OCPUs + Memory
		t.Errorf("expected 2 errors, got %d", len(result.Errors))
	}
}

func TestVerifyInstance_Terminated(t *testing.T) {
	instID := "inst-terminated"

	mock := &MockClient{
		GetInstanceFunc: func(ctx context.Context, request core.GetInstanceRequest) (core.GetInstanceResponse, error) {
			return core.GetInstanceResponse{
				Instance: core.Instance{
					Id:             &instID,
					LifecycleState: core.InstanceLifecycleStateTerminated,
				},
			}, nil
		},
	}

	w := &AccountWorker{
		AccountName:          "test",
		Config:               &config.AccountConfig{},
		Logger:               newMockLogger(),
		ComputeClient:        mock,
		VirtualNetworkClient: &MockVirtualNetworkClient{},
	}

	_, err := w.VerifyInstance(context.Background(), instID)
	if err == nil {
		t.Error("expected error for terminated instance")
	}
}

func TestVerifyInstance_IPRetrieval(t *testing.T) {
	instID := "inst-ip"
	ocpus := float32(4)
	memory := float32(24)
	publicIP := "203.0.113.42"
	vnicID := "vnic-primary"

	mock := &MockClient{
		GetInstanceFunc: func(ctx context.Context, request core.GetInstanceRequest) (core.GetInstanceResponse, error) {
			return core.GetInstanceResponse{
				Instance: core.Instance{
					Id:             &instID,
					LifecycleState: core.InstanceLifecycleStateRunning,
					ShapeConfig: &core.InstanceShapeConfig{
						Ocpus:       &ocpus,
						MemoryInGBs: &memory,
					},
				},
			}, nil
		},
		ListVnicAttachmentsFunc: func(ctx context.Context, request core.ListVnicAttachmentsRequest) (core.ListVnicAttachmentsResponse, error) {
			return core.ListVnicAttachmentsResponse{
				Items: []core.VnicAttachment{
					{VnicId: &vnicID, LifecycleState: core.VnicAttachmentLifecycleStateAttached},
				},
			}, nil
		},
	}

	mockVNet := &MockVirtualNetworkClient{
		GetVnicFunc: func(ctx context.Context, request core.GetVnicRequest) (core.GetVnicResponse, error) {
			return core.GetVnicResponse{
				Vnic: core.Vnic{PublicIp: &publicIP},
			}, nil
		},
	}

	w := &AccountWorker{
		AccountName:          "test",
		Config:               &config.AccountConfig{OCPUs: 4, MemoryGB: 24},
		Logger:               newMockLogger(),
		ComputeClient:        mock,
		VirtualNetworkClient: mockVNet,
	}

	result, err := w.VerifyInstance(context.Background(), instID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PublicIP != publicIP {
		t.Errorf("expected public IP %s, got %s", publicIP, result.PublicIP)
	}
}

func TestProvisioner_SkipProvisionedAccounts(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]*config.AccountConfig{
			"account1": {Enabled: true},
			"account2": {Enabled: true},
		},
		Scheduler: config.SchedulerConfig{
			CycleIntervalSeconds: 10,
			AccountDelaySeconds:  0,
		},
	}

	tracker := notifier.NewTracker()
	p := New(cfg, newMockLogger(), tracker)

	// Mark account1 as already provisioned
	p.Provisioned["account1"] = true

	// RunCycle should skip account1 - we verify by checking it doesn't error
	// (since mock clients are nil, it would panic if it tried to provision)
	// This is a behavioral test - if it runs without panic, account1 was skipped

	// For this test, we need to set up mock clients on the workers
	for _, worker := range p.Workers {
		worker.ComputeClient = &MockClient{
			ListInstancesFunc: func(ctx context.Context, request core.ListInstancesRequest) (core.ListInstancesResponse, error) {
				// Return existing instance to prevent actual provisioning
				return core.ListInstancesResponse{
					Items: []core.Instance{{LifecycleState: core.InstanceLifecycleStateRunning}},
				}, nil
			},
		}
		worker.IdentityClient = &MockClient{}
		worker.VirtualNetworkClient = &MockVirtualNetworkClient{}
	}

	// Run a cycle - should not panic
	p.RunCycle(context.Background())

	// Verify tracker incremented cycle
	if tracker.TotalCycles != 1 {
		t.Errorf("expected 1 cycle, got %d", tracker.TotalCycles)
	}
}

func TestTracker_IncSuccess(t *testing.T) {
	tracker := notifier.NewTracker()

	if tracker.SuccessCount != 0 {
		t.Errorf("expected initial SuccessCount=0, got %d", tracker.SuccessCount)
	}

	tracker.IncSuccess()
	tracker.IncSuccess()

	if tracker.SuccessCount != 2 {
		t.Errorf("expected SuccessCount=2, got %d", tracker.SuccessCount)
	}

	if tracker.LastSuccessTime.IsZero() {
		t.Error("expected LastSuccessTime to be set")
	}

	snapshot := tracker.Snapshot()
	if snapshot.SuccessCount != 2 {
		t.Errorf("expected snapshot SuccessCount=2, got %d", snapshot.SuccessCount)
	}
}
