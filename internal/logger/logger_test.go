package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	logDir := filepath.Join(tempDir, "logs")

	l, err := New(logDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Errorf("Log directory was not created")
	}

	// Verify log file exists
	logFile := filepath.Join(logDir, "provisioner.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("Log file was not created at %s", logFile)
	}

	// Clean up implies closing the file, but Logger doesn't expose Close().
	// In tests, we just let the temp dir cleanup handle it, though file lock might be an issue.
	// Since we are writing to it, better check if we can write.
	l.Plain("Test message")
}

func TestLogger_Output(t *testing.T) {
	tempDir := t.TempDir()
	logDir := filepath.Join(tempDir, "logs")

	// Capture stdout? slightly complex for parallel tests.
	// We will rely on FILE output verification.

	l, err := New(logDir)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Write various log types
	l.Section("Test Section")
	l.Plain("Test Plain Message")
	l.Error("TEST", "Test Error Message")
	l.Success("TEST", "Test Success Message")

	// Read file content
	logPath := filepath.Join(logDir, "provisioner.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	output := string(content)

	// Verify occurrences
	checks := []string{
		"[SECTION] === Test Section ===",
		"Test Plain Message",
		"[TEST] [ERROR] Test Error Message",
		"[TEST] [SUCCESS] Test Success Message",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("Log file missing expected content: %s\nGot:\n%s", check, output)
		}
	}
}

// TestConsole formatting is tricky without capturing stdout,
// but we can at least ensure the methods run without panic.
func TestLogger_Concurrency(t *testing.T) {
	tempDir := t.TempDir()
	l, _ := New(tempDir)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			l.Plain("Concurrent log")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
	// Pass if no data race/panic
}
