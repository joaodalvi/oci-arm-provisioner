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
	ListInstancesFunc  func(ctx context.Context, request core.ListInstancesRequest) (core.ListInstancesResponse, error)
	LaunchInstanceFunc func(ctx context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error)
	ListADsFunc        func(ctx context.Context, request identity.ListAvailabilityDomainsRequest) (identity.ListAvailabilityDomainsResponse, error)
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
		AccountName:    "test",
		Config:         &config.AccountConfig{},
		Logger:         newMockLogger(),
		Notifier:       notifier.New(config.NotificationConfig{Enabled: false}),
		Tracker:        notifier.NewTracker(),
		ComputeClient:  mock,
		IdentityClient: mock,
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
	mock := &MockClient{
		ListInstancesFunc: func(ctx context.Context, request core.ListInstancesRequest) (core.ListInstancesResponse, error) {
			return core.ListInstancesResponse{Items: []core.Instance{}}, nil
		},
		ListADsFunc: func(ctx context.Context, request identity.ListAvailabilityDomainsRequest) (identity.ListAvailabilityDomainsResponse, error) {
			ad := "AD-1"
			return identity.ListAvailabilityDomainsResponse{Items: []identity.AvailabilityDomain{{Name: &ad}}}, nil
		},
		LaunchInstanceFunc: func(ctx context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error) {
			id := "inst-1"
			return core.LaunchInstanceResponse{Instance: core.Instance{Id: &id}}, nil
		},
	}

	w := &AccountWorker{
		AccountName:    "test",
		Config:         &config.AccountConfig{AvailabilityDomain: "auto"},
		Logger:         newMockLogger(),
		Notifier:       notifier.New(config.NotificationConfig{Enabled: false}),
		Tracker:        notifier.NewTracker(),
		ComputeClient:  mock,
		IdentityClient: mock,
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
		AccountName:    "test",
		Config:         &config.AccountConfig{AvailabilityDomain: "AD-1"},
		Logger:         newMockLogger(),
		Notifier:       notifier.New(config.NotificationConfig{Enabled: false}),
		Tracker:        notifier.NewTracker(),
		ComputeClient:  mock,
		IdentityClient: mock,
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
		AccountName:    "test",
		Config:         &config.AccountConfig{},
		Logger:         newMockLogger(),
		Notifier:       notifier.New(config.NotificationConfig{Enabled: false}),
		Tracker:        notifier.NewTracker(),
		ComputeClient:  mock,
		IdentityClient: mock,
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
