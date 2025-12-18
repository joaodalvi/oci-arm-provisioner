package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/yourusername/oci-arm-provisioner/internal/config"
	"github.com/yourusername/oci-arm-provisioner/internal/logger"
	"github.com/yourusername/oci-arm-provisioner/internal/provisioner"
)

func main() {
	// 1. Setup Context with Cancellation
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 2. Initialize Logger
	l, err := logger.New("logs")
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	l.Section("🚀 OCI ARM Provisioner")
	l.Plain(fmt.Sprintf("Version: %s", "0.1.0"))

	// 3. Load Initial Configuration
	cfg, path, err := config.LoadConfig("")
	if err != nil {
		l.Error("INIT", fmt.Sprintf("Failed to load config: %v", err))
		os.Exit(1)
	}
	l.Plain(fmt.Sprintf("📂 Config: %s", path))

	// 4. Initialize Provisioner
	prov := provisioner.New(cfg, l)
	logAccountSummary(l, cfg)

	// 5. Setup Config Watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		l.Error("INIT", fmt.Sprintf("Failed to create file watcher: %v", err))
	} else {
		defer watcher.Close()
		// Watch the directory, not the file, to handle atomic replacements (sed/vim) checks.
		configDir := filepath.Dir(path)
		if err := watcher.Add(configDir); err != nil {
			l.Error("INIT", fmt.Sprintf("Failed to watch config dir: %v", err))
		} else {
			l.Plain(fmt.Sprintf("👀 Live Config Reload: Enabled (Watching %s)", configDir))
		}
	}

	// Channel to receive new configs from the watcher goroutine
	configUpdates := make(chan *config.Config)

	go func() {
		lastModTime := time.Now()
		// Polling ticker as fallback for Docker bind mount issues
		poll := time.NewTicker(5 * time.Second)
		defer poll.Stop()

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if filepath.Base(event.Name) == filepath.Base(path) {
					if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Rename == fsnotify.Rename {
						l.Plain("🔄 Config change detected (fsnotify). Reloading...")
						reload(l, path, configUpdates)
						// Update mod time to prevent double-reload by poller
						if info, err := os.Stat(path); err == nil {
							lastModTime = info.ModTime()
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				l.Error("WATCH", fmt.Sprintf("Watcher error: %v", err))

			case <-poll.C:
				// Fallback Polling
				info, err := os.Stat(path)
				if err == nil {
					if info.ModTime().After(lastModTime) {
						lastModTime = info.ModTime()
						l.Plain("🔄 Config change detected (poller). Reloading...")
						reload(l, path, configUpdates)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 6. Main Execution Loop
	interval := time.Duration(cfg.Scheduler.CycleIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	cycleCount := 1

	// Run first cycle immediately
	runCycle(ctx, l, prov, interval, cycleCount)
	cycleCount++

	for {
		select {
		case <-ctx.Done():
			l.Section("Shutdown Signal Received")
			l.Plain("Exiting gracefully...")
			return

		case newCfg := <-configUpdates:
			// Apply New Configuration
			l.Success("RELOAD", "Configuration applied successfully!")

			// 1. Update Provisioner
			cfg = newCfg
			prov = provisioner.New(cfg, l)
			logAccountSummary(l, cfg)

			// 2. Update Ticker if interval changed
			newInterval := time.Duration(cfg.Scheduler.CycleIntervalSeconds) * time.Second
			if newInterval != interval {
				l.Plain(fmt.Sprintf("⏱️  Updating Schedule: %v -> %v", interval, newInterval))
				interval = newInterval
				ticker.Reset(interval)
			}

		case <-ticker.C:
			runCycle(ctx, l, prov, interval, cycleCount)
			cycleCount++
		}
	}
}

func logAccountSummary(l *logger.Logger, cfg *config.Config) {
	count := 0
	names := []string{}
	for title, acc := range cfg.Accounts {
		if acc.Enabled {
			count++
			names = append(names, title)
		}
	}
	l.Plain(fmt.Sprintf("👥 Accounts: %v", names))

	if count == 1 {
		l.Plain("ℹ️  Single Account Mode Active")
	}
	if count == 0 {
		l.Warn("INIT", "No accounts enabled. The tool will just idle.")
	}
}

// runCycle executes a single pass of the provisioning logic.
func runCycle(ctx context.Context, l *logger.Logger, prov *provisioner.Provisioner, interval time.Duration, count int) {
	start := time.Now()
	l.Section(fmt.Sprintf("Cycle %d Started at %s", count, start.Format("2006-01-02 15:04:05")))

	prov.RunCycle(ctx)

	elapsed := time.Since(start)
	nextRun := time.Now().Add(interval)

	l.Section(fmt.Sprintf("Cycle Finished | Elapsed: %v", elapsed.Round(time.Second)))
	l.Plain(fmt.Sprintf("💤 Sleeping %v (Next run at %s)...",
		interval, nextRun.Format("15:04:05")))
}
