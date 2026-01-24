package logging

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != InfoLevel {
		t.Errorf("expected default level to be InfoLevel, got %v", cfg.Level)
	}
	if cfg.MaxSize != 10 {
		t.Errorf("expected default MaxSize to be 10, got %d", cfg.MaxSize)
	}
	if cfg.MaxBackups != 5 {
		t.Errorf("expected default MaxBackups to be 5, got %d", cfg.MaxBackups)
	}
	if cfg.MaxAge != 7 {
		t.Errorf("expected default MaxAge to be 7, got %d", cfg.MaxAge)
	}
	if !cfg.JSON {
		t.Error("expected default JSON to be true")
	}
	if !cfg.Compress {
		t.Error("expected default Compress to be true")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
		wantErr  bool
	}{
		{"debug", DebugLevel, false},
		{"info", InfoLevel, false},
		{"warn", WarnLevel, false},
		{"error", ErrorLevel, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLevel(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if level != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, level)
			}
		})
	}
}

func TestLoggerWithContext(t *testing.T) {
	// Initialize with defaults to ensure we have a logger
	err := Init(nil)
	if err != nil {
		t.Fatalf("failed to initialize logger: %v", err)
	}

	// Test chaining context methods
	log := Get().
		WithSessionID("test-session").
		WithCommand("spawn").
		WithField("tool", "claude")

	if log.sessionID != "test-session" {
		t.Errorf("expected sessionID to be 'test-session', got %q", log.sessionID)
	}
	if log.command != "spawn" {
		t.Errorf("expected command to be 'spawn', got %q", log.command)
	}
}

func TestLoggerWithFields(t *testing.T) {
	err := Init(nil)
	if err != nil {
		t.Fatalf("failed to initialize logger: %v", err)
	}

	fields := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}

	log := Get().WithFields(fields)
	if log == nil {
		t.Error("WithFields returned nil")
	}
}

func TestInitFromLogConfig(t *testing.T) {
	lc := LoggingConfig{
		Level:      "debug",
		JSON:       true,
		MaxSize:    20,
		MaxBackups: 10,
		MaxAge:     14,
		Compress:   false,
	}

	err := InitFromLogConfig(lc)
	if err != nil {
		t.Fatalf("failed to initialize from config: %v", err)
	}
}

func TestFileOutput(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "logging-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFile := filepath.Join(tmpDir, "test.log")

	cfg := &Config{
		Level:      DebugLevel,
		JSON:       true,
		FilePath:   logFile,
		MaxSize:    1,
		MaxBackups: 1,
		MaxAge:     1,
		Compress:   false,
		Console:    false,
	}

	err = Init(cfg)
	if err != nil {
		t.Fatalf("failed to initialize logger: %v", err)
	}

	// Write a log entry
	log := Get().WithSessionID("file-test").WithCommand("test")
	log.Info("test message")

	// Read the log file
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// Parse the JSON
	var entry map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(content), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v (content: %s)", err, content)
	}

	// Verify fields
	if entry["level"] != "info" {
		t.Errorf("expected level 'info', got %v", entry["level"])
	}
	if entry["message"] != "test message" {
		t.Errorf("expected message 'test message', got %v", entry["message"])
	}
	if entry["session_id"] != "file-test" {
		t.Errorf("expected session_id 'file-test', got %v", entry["session_id"])
	}
	if entry["command"] != "test" {
		t.Errorf("expected command 'test', got %v", entry["command"])
	}
	if _, ok := entry["time"]; !ok {
		t.Error("expected timestamp field")
	}
}

func TestConvenienceFunctions(t *testing.T) {
	err := Init(nil)
	if err != nil {
		t.Fatalf("failed to initialize logger: %v", err)
	}

	// These should not panic
	Debug("debug message")
	Debugf("debug %s", "formatted")
	Info("info message")
	Infof("info %s", "formatted")
	Warn("warn message")
	Warnf("warn %s", "formatted")
	Error("error message")
	Errorf("error %s", "formatted")

	log := WithSessionID("test")
	if log == nil {
		t.Error("WithSessionID returned nil")
	}

	log = WithCommand("test")
	if log == nil {
		t.Error("WithCommand returned nil")
	}

	log = WithField("key", "value")
	if log == nil {
		t.Error("WithField returned nil")
	}

	log = WithFields(map[string]interface{}{"key": "value"})
	if log == nil {
		t.Error("WithFields returned nil")
	}

	err = nil // Create an error to test
	log = WithError(err)
	if log == nil {
		t.Error("WithError returned nil")
	}
}

func TestInvalidLevelInConfig(t *testing.T) {
	lc := LoggingConfig{
		Level: "invalid-level",
	}

	err := InitFromLogConfig(lc)
	if err == nil {
		t.Error("expected error for invalid log level")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected error to mention 'invalid', got: %v", err)
	}
}
