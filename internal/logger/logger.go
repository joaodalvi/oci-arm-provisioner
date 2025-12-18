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

// Logger handles concurrent logging to both stdout (console) and a file.
// It ensures thread safety using a mutex.
type Logger struct {
	mu   sync.Mutex
	out  io.Writer // Console output (Standard Output)
	file io.Writer // File output (Append only)
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
		out:  os.Stdout,
		file: f,
	}, nil
}

// format constructs formatted strings for console (colored) and file (structured/timestamped) output.
// returns (consoleMsg, fileMsg)
func (l *Logger) format(level, color, icon, account, msg string) (string, string) {
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

// write securely writes strings to both outputs under a mutex lock.
func (l *Logger) write(console, file string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprint(l.out, console)
	fmt.Fprint(l.file, file)
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
