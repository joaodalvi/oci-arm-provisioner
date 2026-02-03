package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/yourusername/oci-arm-provisioner/internal/config"
	"github.com/yourusername/oci-arm-provisioner/internal/logger"
	"github.com/yourusername/oci-arm-provisioner/internal/notifier"
	"github.com/yourusername/oci-arm-provisioner/internal/provisioner"
	"github.com/yourusername/oci-arm-provisioner/internal/tui"
	"github.com/yourusername/oci-arm-provisioner/internal/wizard"
)

func main() {
	// 0. Parse Flags
	setupNotifications := flag.Bool("setup-notifications", false, "Run the notification setup wizard")
	setupOCI := flag.Bool("setup", false, "Run the OCI setup wizard (config.yaml)")
	headless := flag.Bool("headless", false, "Run in headless mode (log-only, no TUI)")
	flag.Parse()

	// 1. Setup Context with Cancellation
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 2. Initialize Logger
	l, err := logger.New("logs")
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	// Wizard Modes
	if *setupNotifications {
		wizard.RunNotifications(l)
		return
	}
	if *setupOCI {
		wizard.RunOCI(l)
		return
	}

	// 3. Load Initial Configuration
	cfg, path, err := config.LoadConfig("")
	if err != nil {
		l.Error("INIT", fmt.Sprintf("Failed to load config: %v", err))
		os.Exit(1)
	}

	// 4. Initialize Tracker
	tracker := notifier.NewTracker()

	// 5. Run TUI or Headless mode
	if !*headless {
		// TUI Mode (default) - runs provisioner in background
		if err := tui.Run(cfg, tracker, l); err != nil {
			l.Error("TUI", fmt.Sprintf("TUI error: %v", err))
			os.Exit(1)
		}
		return
	}

	// Headless Mode (original behavior)
	l.Section("🚀 OCI ARM Provisioner (Headless Mode)")
	l.Plain(fmt.Sprintf("Version: %s", "0.2.0"))
	l.Plain(fmt.Sprintf("📂 Config: %s", path))

	// Initialize Provisioner for headless mode
	prov := provisioner.New(cfg, l, tracker)
	logAccountSummary(l, cfg)

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

	// Digest Ticker
	digestDuration, _ := time.ParseDuration(cfg.Notifications.DigestInterval)
	if digestDuration == 0 && cfg.Notifications.DigestInterval != "" {
		// Default failure or just disabled if empty
		l.Error("INIT", "Invalid digest interval, disabling digests.")
	}
	var digestTicker *time.Ticker
	if digestDuration > 0 {
		digestTicker = time.NewTicker(digestDuration)
		l.Plain(fmt.Sprintf("📊 Digest Scheduler: Enabled (Every %s)", digestDuration))
	} else {
		digestTicker = time.NewTicker(24 * 365 * time.Hour) // Dummy long wait
		digestTicker.Stop()                                 // Stop immediately
	}

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
			prov = provisioner.New(cfg, l, tracker)
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

		case <-digestTicker.C:
			if cfg.Notifications.Enabled {
				l.Plain("📊 Sending Digest...")
				n := notifier.New(cfg.Notifications) // Create temp notifier with current config
				if err := n.SendDigest(tracker.Snapshot()); err != nil {
					l.Error("NOTIFIER", fmt.Sprintf("Failed to send digest: %v", err))
				}
			}
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

// Helper to reload config safely
// Helper to reload config safely
func reload(l *logger.Logger, path string, updates chan<- *config.Config) {
	// Debounce/Settle
	time.Sleep(100 * time.Millisecond)
	newCfg, _, err := config.LoadConfig(path)
	if err != nil {
		l.Error("RELOAD", fmt.Sprintf("Failed to reload config: %v (keeping old config)", err))
		return
	}
	updates <- newCfg
}
