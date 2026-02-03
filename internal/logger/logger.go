package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ANSI Color Codes for console output formatting
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	Gray   = "\033[37m"
)

// LogHook is a function that receives structured log events
type LogHook func(level, account, msg string)

// Logger handles concurrent logging to both stdout (console) and a file.
// It ensures thread safety using a mutex.
type Logger struct {
	mu    sync.Mutex
	out   io.Writer // Console output (Standard Output)
	file  io.Writer // File output (Append only)
	hooks []LogHook
}

// New initializes a new Logger instance.
// It ensures the log directory exists and opens the 'provisioner.log' file for appending.
// If logDir is empty, it defaults to "logs".
func New(logDir string) (*Logger, error) {
	if logDir == "" {
		logDir = "logs"
	}
	// Create directory with standard permissions (rwxr-xr-x)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	// Open log file, create if missing, append if exists.
	f, err := os.OpenFile(filepath.Join(logDir, "provisioner.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &Logger{
		out:   os.Stdout,
		file:  f,
		hooks: make([]LogHook, 0),
	}, nil
}

// AddHook registers a function to be called on every log event
func (l *Logger) AddHook(hook LogHook) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks = append(l.hooks, hook)
}

// format constructs formatted strings for console (colored) and file (structured/timestamped) output.
// returns (consoleMsg, fileMsg)
func (l *Logger) format(level, color, icon, account, msg string) (string, string) {
	// Trigger hooks
	l.triggerHooks(level, account, msg)

	now := time.Now()
	tsConsole := now.Format("15:04:05")
	tsFile := now.Format("2006/01/02 15:04:05")

	// Console Format: [HH:mm:ss] ℹ️ [Account] Msg (Colored)
	// Example: [12:00:00] ⚠️ [personal] OCI Error 500
	console := fmt.Sprintf("%s[%s]%s %s %s[%s]%s %s%s%s\n",
		Gray, tsConsole, Reset,
		icon,
		Cyan, account, Reset,
		color, msg, Reset,
	)

	// File Format: YYYY/MM/DD HH:mm:ss [Account] [LEVEL] Msg
	// Example: 2023/01/01 12:00:00 [personal] [WARN] OCI Error 500
	file := fmt.Sprintf("%s [%s] [%s] %s\n", tsFile, account, level, msg)
	return console, file
}

func (l *Logger) triggerHooks(level, account, msg string) {
	// We don't lock here to avoid deadlocks if hook uses logger (though it shouldn't)
	// But we need to protect the slice access? No, format is called under lock in most cases?
	// Actually format is helpers.
	// Let's protect hooks access.
	// But wait, Write calls lock. Format does NOT lock.
	// So we can lock/unlock here safely.
	// But wait! All logging methods call format then write.
	// format is just string construction.
	// Let's check callers.
	// Info/Warn etc call format() then write().
	// write() locks. format() does NOT lock.
	// So we can safely lock inside triggerHooks.
	l.mu.Lock()
	hooks := make([]LogHook, len(l.hooks))
	copy(hooks, l.hooks)
	l.mu.Unlock()

	for _, h := range hooks {
		h(level, account, msg)
	}
}

// write securely writes strings to both outputs under a mutex lock.
func (l *Logger) write(console, file string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.out != nil {
		fmt.Fprint(l.out, console)
	}
	if l.file != nil {
		fmt.Fprint(l.file, file)
	}
}

// SetConsoleOutput sets the destination for console logs.
// Use io.Discard to silence console output.
func (l *Logger) SetConsoleOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = w
}

// Info logs general informational messages.
func (l *Logger) Info(account, msg string) {
	c, f := l.format("INFO", "", "ℹ️", account, msg)
	l.write(c, f)
}

// Success logs positive outcomes (e.g., instance created).
func (l *Logger) Success(account, msg string) {
	c, f := l.format("SUCCESS", Green, "✅", account, msg)
	l.write(c, f)
}

// Warn logs warnings or recoverable errors (e.g., capacity limits).
func (l *Logger) Warn(account, msg string) {
	c, f := l.format("WARN", Yellow, "⚠️", account, msg)
	l.write(c, f)
}

// Error logs critical failures.
func (l *Logger) Error(account, msg string) {
	c, f := l.format("ERROR", Red, "❌", account, msg)
	l.write(c, f)
}

// Section logs a visual divider to separate logical execution blocks (cycles).
func (l *Logger) Section(msg string) {
	line := strings.Repeat("=", 60)
	// Console: Blue Divider (Visual only)
	l.mu.Lock()
	fmt.Fprintf(l.out, "%s%s\n%s%s\n", Blue, line, msg, Reset)

	// File: Timestamped Entry with generic tag
	ts := time.Now().Format("2006/01/02 15:04:05")
	fmt.Fprintf(l.file, "%s [SECTION] === %s ===\n", ts, msg)
	l.mu.Unlock()
}

// Plain logs a raw message without account context or icons (e.g., startup info).
func (l *Logger) Plain(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	// Console: Plain text
	fmt.Fprintln(l.out, msg)

	// File: Timestamped Info
	ts := time.Now().Format("2006/01/02 15:04:05")
	fmt.Fprintf(l.file, "%s [INFO] %s\n", ts, msg)
}

// Celebrate logs a prominent success banner with instance details and a terminal beep.
func (l *Logger) Celebrate(account string, details interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Terminal beep
	fmt.Fprint(l.out, "\a")

	// ASCII Art Banner
	banner := `
` + Green + `
╔═══════════════════════════════════════════════════════════════════════╗
║                                                                       ║
║   🚀🎉  SUCCESS! INSTANCE PROVISIONED SUCCESSFULLY!  🎉🚀            ║
║                                                                       ║
╚═══════════════════════════════════════════════════════════════════════╝
` + Reset

	fmt.Fprint(l.out, banner)

	// Extract details if available
	type verifiedDetails interface {
		GetInstanceID() string
		GetPublicIP() string
		GetOCPUs() float32
		GetMemoryGB() float32
		GetState() string
		GetRegion() string
	}

	// Try to extract structured details
	if v, ok := details.(verifiedDetails); ok {
		box := fmt.Sprintf(`
%s┌─────────────────────────────────────────────────────────────────────┐%s
%s│ Account:     %-55s │%s
%s│ Instance ID: %-55s │%s
%s│ Public IP:   %-55s │%s
%s│ Specs:       %-55s │%s
%s│ State:       %-55s │%s
%s│ Region:      %-55s │%s
%s└─────────────────────────────────────────────────────────────────────┘%s
`,
			Cyan, Reset,
			Cyan, account, Reset,
			Cyan, v.GetInstanceID(), Reset,
			Cyan, v.GetPublicIP(), Reset,
			Cyan, fmt.Sprintf("%.0f OCPUs / %.0f GB RAM", v.GetOCPUs(), v.GetMemoryGB()), Reset,
			Cyan, v.GetState()+" ✓", Reset,
			Cyan, v.GetRegion(), Reset,
			Cyan, Reset,
		)
		fmt.Fprint(l.out, box)
	}

	// File logging
	ts := time.Now().Format("2006/01/02 15:04:05")
	fmt.Fprintf(l.file, "%s [SUCCESS] === INSTANCE PROVISIONED FOR ACCOUNT [%s] ===\n", ts, account)
}
