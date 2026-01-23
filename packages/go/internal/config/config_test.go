package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	// Reset global config for testing
	configOnce = sync.Once{}
	globalConfig = nil
	configErr = nil

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.DefaultTool != DefaultDefaultTool {
		t.Errorf("DefaultTool = %q, want %q", cfg.DefaultTool, DefaultDefaultTool)
	}

	if cfg.HeartbeatInterval != DefaultHeartbeatInterval {
		t.Errorf("HeartbeatInterval = %v, want %v", cfg.HeartbeatInterval, DefaultHeartbeatInterval)
	}

	if cfg.RedisURL != DefaultRedisURL {
		t.Errorf("RedisURL = %q, want %q", cfg.RedisURL, DefaultRedisURL)
	}

	if cfg.DashboardPort != DefaultDashboardPort {
		t.Errorf("DashboardPort = %d, want %d", cfg.DashboardPort, DefaultDashboardPort)
	}

	if cfg.DefaultHeartbeat != DefaultDefaultHeartbeat {
		t.Errorf("DefaultHeartbeat = %t, want %t", cfg.DefaultHeartbeat, DefaultDefaultHeartbeat)
	}
}

func TestEnvOverrides(t *testing.T) {
	// Reset global config for testing
	configOnce = sync.Once{}
	globalConfig = nil
	configErr = nil

	// Set env vars
	os.Setenv("CODERS_DEFAULT_TOOL", "gemini")
	os.Setenv("CODERS_HEARTBEAT_INTERVAL", "60s")
	os.Setenv("CODERS_REDIS_URL", "redis://custom:6380")
	os.Setenv("CODERS_DASHBOARD_PORT", "8080")
	os.Setenv("CODERS_DEFAULT_HEARTBEAT", "false")
	defer func() {
		os.Unsetenv("CODERS_DEFAULT_TOOL")
		os.Unsetenv("CODERS_HEARTBEAT_INTERVAL")
		os.Unsetenv("CODERS_REDIS_URL")
		os.Unsetenv("CODERS_DASHBOARD_PORT")
		os.Unsetenv("CODERS_DEFAULT_HEARTBEAT")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.DefaultTool != "gemini" {
		t.Errorf("DefaultTool = %q, want %q", cfg.DefaultTool, "gemini")
	}

	if cfg.HeartbeatInterval != 60*time.Second {
		t.Errorf("HeartbeatInterval = %v, want %v", cfg.HeartbeatInterval, 60*time.Second)
	}

	if cfg.RedisURL != "redis://custom:6380" {
		t.Errorf("RedisURL = %q, want %q", cfg.RedisURL, "redis://custom:6380")
	}

	if cfg.DashboardPort != 8080 {
		t.Errorf("DashboardPort = %d, want %d", cfg.DashboardPort, 8080)
	}

	if cfg.DefaultHeartbeat != false {
		t.Errorf("DefaultHeartbeat = %t, want %t", cfg.DefaultHeartbeat, false)
	}
}

func TestHeartbeatIntervalSeconds(t *testing.T) {
	// Reset global config for testing
	configOnce = sync.Once{}
	globalConfig = nil
	configErr = nil

	// Test with plain seconds (no "s" suffix)
	os.Setenv("CODERS_HEARTBEAT_INTERVAL", "45")
	defer os.Unsetenv("CODERS_HEARTBEAT_INTERVAL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.HeartbeatInterval != 45*time.Second {
		t.Errorf("HeartbeatInterval = %v, want %v", cfg.HeartbeatInterval, 45*time.Second)
	}
}

func TestWriteExample(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	err := WriteExample(path)
	if err != nil {
		t.Fatalf("WriteExample() failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Config file was not created at %s", path)
	}

	// Read and verify content has expected keys
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	content := string(data)
	expected := []string{"default_tool", "heartbeat_interval", "redis_url", "dashboard_port", "ollama"}
	for _, key := range expected {
		if !contains(content, key) {
			t.Errorf("Config file missing key: %s", key)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
