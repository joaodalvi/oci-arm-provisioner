package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/oci-arm-provisioner/internal/config"
	"github.com/yourusername/oci-arm-provisioner/internal/logger"
	"github.com/yourusername/oci-arm-provisioner/internal/provisioner"
)

// main is the entry point of the application.
// It initializes the logger, loads configuration, and starts the provisioning cycle loop.
// It handles OS signals (SIGINT, SIGTERM) to perform a graceful shutdown.
func main() {
	// 1. Setup Context with Cancellation on Signal (SIGINT, SIGTERM)
	// This ensures that hitting Ctrl+C cancels any active OCI requests immediately.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 2. Initialize Logger
	// Start with a default logger pointing to the "logs" directory.
	l, err := logger.New("logs")
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	l.Section("Starting OCI ARM Provisioner (Go V2)")
	l.Plain(fmt.Sprintf("Version: %s", "0.1.0"))

	// 3. Load Configuration
	// We look for config.yaml in standard locations.
	cfg, path, err := config.LoadConfig("")
	if err != nil {
		l.Error("INIT", fmt.Sprintf("Failed to load config: %v", err))
		os.Exit(1)
	}
	l.Plain(fmt.Sprintf("Loaded Configuration: %s", path))

	// If the config specifies a different log directory, re-initialize the logger.
	if cfg.Logging.LogDir != "logs" {
		if newLogger, err := logger.New(cfg.Logging.LogDir); err == nil {
			l = newLogger
		} else {
			l.Warn("INIT", fmt.Sprintf("Failed to switch to configured log_dir: %v. Using default.", err))
		}
	}

	// 4. Initialize Provisioner
	// The provisioner holds the OCI clients and logic for creating instances.
	prov := provisioner.New(cfg, l)

	// Count and list enabled accounts for user feedback.
	count := 0
	names := []string{}
	for title, acc := range cfg.Accounts {
		if acc.Enabled {
			count++
			names = append(names, title)
		}
	}
	l.Plain(fmt.Sprintf("Found %d enabled accounts: %v", count, names))

	// If only 1 account, log special mode
	if count == 1 {
		l.Plain("Running in Single Account Mode")
	}

	if count == 0 {
		l.Warn("INIT", "No accounts enabled. Exiting.")
		return
	}

	// 5. Main Execution Loop
	// We use a Ticker to trigger the provisioning cycle at fixed intervals.
	interval := time.Duration(cfg.Scheduler.CycleIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	cycleCount := 1

	// Run the first cycle immediately before waiting for the ticker.
	runCycle(ctx, l, prov, interval, cycleCount)
	cycleCount++

	for {
		select {
		case <-ctx.Done():
			// Handle graceful shutdown when signal is received.
			l.Section("Shutdown Signal Received")
			l.Plain("Exiting gracefully...")
			return
		case <-ticker.C:
			// Trigger a new provisioning cycle.
			runCycle(ctx, l, prov, interval, cycleCount)
			cycleCount++
		}
	}
}

// runCycle executes a single pass of the provisioning logic.
// It wraps the provisioner call with logging for cycle duration.
func runCycle(ctx context.Context, l *logger.Logger, prov *provisioner.Provisioner, interval time.Duration, count int) {
	start := time.Now()
	l.Section(fmt.Sprintf("Cycle %d Started at %s", count, start.Format("2006-01-02 15:04:05")))

	// Execute the provisioning logic for all accounts.
	// We pass the context so requests can be cancelled if the app is stopping.
	prov.RunCycle(ctx)

	elapsed := time.Since(start)
	nextRun := time.Now().Add(interval)

	l.Section("Cycle Finished")
	l.Plain(fmt.Sprintf("Elapsed: %v", elapsed.Round(time.Second)))
	l.Plain(fmt.Sprintf("Sleeping %v until next cycle (Next run at %s)...",
		interval, nextRun.Format("15:04:05")))
}
