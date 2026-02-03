package tui

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yourusername/oci-arm-provisioner/internal/config"
	"github.com/yourusername/oci-arm-provisioner/internal/logger"
	"github.com/yourusername/oci-arm-provisioner/internal/notifier"
	"github.com/yourusername/oci-arm-provisioner/internal/provisioner"
)

// ProvisionerRunner manages the background provisioning process
type ProvisionerRunner struct {
	Config      *config.Config
	Logger      *logger.Logger
	Tracker     *notifier.Tracker
	Provisioner *provisioner.Provisioner

	// Communication channels
	statusChan chan AccountStatusUpdate
	logChan    chan LogEntry
	pauseChan  chan bool
	stopChan   chan struct{}

	// State
	mu       sync.RWMutex
	paused   bool
	running  bool
	accounts map[string]*AccountStatus
}

// AccountStatusUpdate is sent when an account's status changes
type AccountStatusUpdate struct {
	Name   string
	Status AccountStatus
}

// LogEntry represents a log message for the TUI
type LogEntry struct {
	Time    time.Time
	Level   string // "info", "warn", "error", "success"
	Account string
	Message string
}

// NewProvisionerRunner creates a new runner
func NewProvisionerRunner(cfg *config.Config, l *logger.Logger, tracker *notifier.Tracker) *ProvisionerRunner {
	accounts := make(map[string]*AccountStatus)
	for name, acc := range cfg.Accounts {
		if acc.Enabled {
			accounts[name] = &AccountStatus{
				Name:     name,
				Region:   acc.Region,
				State:    "waiting",
				OCPUs:    acc.OCPUs,
				MemoryGB: acc.MemoryGB,
			}
		}
	}

	return &ProvisionerRunner{
		Config:      cfg,
		Logger:      l,
		Tracker:     tracker,
		Provisioner: provisioner.New(cfg, l, tracker),
		statusChan:  make(chan AccountStatusUpdate, 100),
		logChan:     make(chan LogEntry, 1000),
		pauseChan:   make(chan bool),
		stopChan:    make(chan struct{}),
		accounts:    accounts,
	}
}

// Start begins the provisioning loop in a goroutine
func (r *ProvisionerRunner) Start(ctx context.Context) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	go r.runLoop(ctx)
}

// Stop stops the provisioner
func (r *ProvisionerRunner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		close(r.stopChan)
		r.running = false
	}
}

// SetPaused pauses or resumes provisioning
func (r *ProvisionerRunner) SetPaused(paused bool) {
	r.mu.Lock()
	r.paused = paused
	r.mu.Unlock()
}

// IsPaused returns whether provisioning is paused
func (r *ProvisionerRunner) IsPaused() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.paused
}

// StatusChan returns the channel for status updates
func (r *ProvisionerRunner) StatusChan() <-chan AccountStatusUpdate {
	return r.statusChan
}

// LogChan returns the channel for log entries
func (r *ProvisionerRunner) LogChan() <-chan LogEntry {
	return r.logChan
}

// GetAccounts returns current account statuses
func (r *ProvisionerRunner) GetAccounts() []AccountStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	accounts := make([]AccountStatus, 0, len(r.accounts))
	for _, acc := range r.accounts {
		accounts = append(accounts, *acc)
	}
	return accounts
}

// runLoop is the main provisioning loop
func (r *ProvisionerRunner) runLoop(ctx context.Context) {
	interval := time.Duration(r.Config.Scheduler.CycleIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	cycleCount := 0

	// Run first cycle immediately
	r.runCycle(ctx, &cycleCount)

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		case <-ticker.C:
			r.mu.RLock()
			paused := r.paused
			r.mu.RUnlock()

			if !paused {
				r.runCycle(ctx, &cycleCount)
			}
		}
	}
}

// runCycle executes a single provisioning cycle
func (r *ProvisionerRunner) runCycle(ctx context.Context, cycleCount *int) {
	*cycleCount++

	// Update all accounts to "running" state at start of cycle
	for name := range r.accounts {
		// Skip already provisioned
		if r.Provisioner.Provisioned[name] {
			r.updateAccountStatus(name, func(s *AccountStatus) {
				s.State = "provisioned"
				s.Provisioned = true
			})
			continue
		}

		r.updateAccountStatus(name, func(s *AccountStatus) {
			s.State = "running"
		})
	}

	// Run the actual provisioner cycle
	r.Provisioner.RunCycle(ctx)

	// Update statuses based on provisioner state
	for name := range r.accounts {
		if r.Provisioner.Provisioned[name] {
			r.updateAccountStatus(name, func(s *AccountStatus) {
				s.State = "provisioned"
				s.Provisioned = true
			})
		} else {
			r.updateAccountStatus(name, func(s *AccountStatus) {
				if s.State == "running" {
					s.State = "waiting"
				}
			})
		}
	}

	// Update capacity hits from tracker
	stats := r.Tracker.Snapshot()
	for name := range r.accounts {
		r.updateAccountStatus(name, func(s *AccountStatus) {
			// Distribute capacity errors evenly for now (could be per-account)
			s.CapacityHits = stats.CapacityErrors / max(1, len(r.accounts))
		})
	}
}

// updateAccountStatus updates an account and sends the update
func (r *ProvisionerRunner) updateAccountStatus(name string, update func(*AccountStatus)) {
	r.mu.Lock()
	acc, ok := r.accounts[name]
	if ok {
		update(acc)
		// Send non-blocking update
		select {
		case r.statusChan <- AccountStatusUpdate{Name: name, Status: *acc}:
		default:
			// Channel full, skip update
		}
	}
	r.mu.Unlock()
}

// accountUpdateCmd creates a tea.Cmd that waits for account updates
func accountUpdateCmd(statusChan <-chan AccountStatusUpdate) tea.Cmd {
	return func() tea.Msg {
		return accountUpdateMsg(<-statusChan)
	}
}

// accountUpdateMsg is sent when an account status changes
type accountUpdateMsg AccountStatusUpdate

// logUpdateCmd creates a tea.Cmd that waits for log entries
func logUpdateCmd(logChan <-chan LogEntry) tea.Cmd {
	return func() tea.Msg {
		return logUpdateMsg(<-logChan)
	}
}

// logUpdateMsg is sent when a new log entry arrives
type logUpdateMsg LogEntry
