// Package config handles loading and managing configuration for the coders CLI.
// It supports loading from YAML files, environment variables, and hardcoded defaults.
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration settings for the coders CLI.
type Config struct {
	// DefaultTool is the AI tool to use when not specified (claude, gemini, codex, opencode)
	DefaultTool string `yaml:"default_tool"`

	// HeartbeatInterval is how often to publish heartbeat data
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`

	// RedisURL is the Redis connection URL
	RedisURL string `yaml:"redis_url"`

	// DashboardPort is the port for the dashboard server
	DashboardPort int `yaml:"dashboard_port"`

	// DefaultModel is the default model to use (tool-specific)
	DefaultModel string `yaml:"default_model"`

	// DefaultHeartbeat controls whether heartbeat is enabled by default
	DefaultHeartbeat bool `yaml:"default_heartbeat"`

	// Ollama configuration
	Ollama OllamaConfig `yaml:"ollama"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging"`
}

// LoggingConfig holds logging-specific configuration.
type LoggingConfig struct {
	// Level is the minimum log level (debug, info, warn, error)
	Level string `yaml:"level"`

	// FilePath is the path to the log file (empty for console only)
	FilePath string `yaml:"file_path"`

	// JSON enables JSON output format
	JSON bool `yaml:"json"`

	// Console enables console output in addition to file output
	Console bool `yaml:"console"`

	// MaxSize is the maximum size in megabytes before rotation
	MaxSize int `yaml:"max_size"`

	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int `yaml:"max_backups"`

	// MaxAge is the maximum number of days to retain old log files
	MaxAge int `yaml:"max_age"`

	// Compress enables gzip compression of rotated files
	Compress bool `yaml:"compress"`
}

// OllamaConfig holds Ollama-specific configuration.
type OllamaConfig struct {
	// BaseURL is the Ollama API base URL
	BaseURL string `yaml:"base_url"`

	// AuthToken is the authentication token for Ollama
	AuthToken string `yaml:"auth_token"`

	// APIKey is an alternative to AuthToken
	APIKey string `yaml:"api_key"`
}

// Default configuration values
const (
	DefaultDefaultTool       = "claude"
	DefaultHeartbeatInterval = 30 * time.Second
	DefaultRedisURL          = "redis://localhost:6379"
	DefaultDashboardPort     = 3000
	DefaultDefaultModel      = ""
	DefaultDefaultHeartbeat  = true
	DefaultLogLevel          = "info"
	DefaultLogJSON           = true
	DefaultLogMaxSize        = 10 // 10 MB
	DefaultLogMaxBackups     = 5
	DefaultLogMaxAge         = 7 // 7 days
	DefaultLogCompress       = true
)

var (
	globalConfig *Config
	configOnce   sync.Once
	configErr    error
)

// Get returns the global configuration, loading it if necessary.
// This function is safe for concurrent use.
func Get() (*Config, error) {
	configOnce.Do(func() {
		globalConfig, configErr = Load()
	})
	return globalConfig, configErr
}

// MustGet returns the global configuration, panicking if loading fails.
func MustGet() *Config {
	cfg, err := Get()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}
	return cfg
}

// Load reads configuration from files and environment variables.
// Priority (highest to lowest):
// 1. Environment variables
// 2. ~/.config/coders/config.yaml
// 3. ~/.coders.yaml
// 4. Hardcoded defaults
func Load() (*Config, error) {
	cfg := &Config{
		DefaultTool:       DefaultDefaultTool,
		HeartbeatInterval: DefaultHeartbeatInterval,
		RedisURL:          DefaultRedisURL,
		DashboardPort:     DefaultDashboardPort,
		DefaultModel:      DefaultDefaultModel,
		DefaultHeartbeat:  DefaultDefaultHeartbeat,
		Logging: LoggingConfig{
			Level:      DefaultLogLevel,
			JSON:       DefaultLogJSON,
			MaxSize:    DefaultLogMaxSize,
			MaxBackups: DefaultLogMaxBackups,
			MaxAge:     DefaultLogMaxAge,
			Compress:   DefaultLogCompress,
		},
	}

	// Try to load from config files (lowest priority file first)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		// Try ~/.coders.yaml first (will be overwritten by XDG config if present)
		legacyPath := filepath.Join(homeDir, ".coders.yaml")
		if data, err := os.ReadFile(legacyPath); err == nil {
			_ = yaml.Unmarshal(data, cfg)
		}

		// Then try ~/.config/coders/config.yaml (higher priority)
		xdgPath := filepath.Join(homeDir, ".config", "coders", "config.yaml")
		if data, err := os.ReadFile(xdgPath); err == nil {
			_ = yaml.Unmarshal(data, cfg)
		}

		// Also try config.yml extension
		xdgPathYml := filepath.Join(homeDir, ".config", "coders", "config.yml")
		if data, err := os.ReadFile(xdgPathYml); err == nil {
			_ = yaml.Unmarshal(data, cfg)
		}
	}

	// Override with environment variables (highest priority)
	cfg.applyEnvOverrides()

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config.
func (c *Config) applyEnvOverrides() {
	// Default tool
	if val := os.Getenv("CODERS_DEFAULT_TOOL"); val != "" {
		c.DefaultTool = val
	}

	// Heartbeat interval
	if val := os.Getenv("CODERS_HEARTBEAT_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.HeartbeatInterval = duration
		} else if secs, err := strconv.Atoi(val); err == nil {
			// Support plain seconds for convenience
			c.HeartbeatInterval = time.Duration(secs) * time.Second
		}
	}

	// Redis URL (support both REDIS_URL and CODERS_REDIS_URL)
	if val := os.Getenv("CODERS_REDIS_URL"); val != "" {
		c.RedisURL = val
	} else if val := os.Getenv("REDIS_URL"); val != "" {
		c.RedisURL = val
	}

	// Dashboard port
	if val := os.Getenv("CODERS_DASHBOARD_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			c.DashboardPort = port
		}
	}

	// Default model
	if val := os.Getenv("CODERS_DEFAULT_MODEL"); val != "" {
		c.DefaultModel = val
	}

	// Default heartbeat
	if val := os.Getenv("CODERS_DEFAULT_HEARTBEAT"); val != "" {
		c.DefaultHeartbeat = val == "true" || val == "1" || val == "yes"
	}

	// Ollama settings
	if val := os.Getenv("CODERS_OLLAMA_BASE_URL"); val != "" {
		c.Ollama.BaseURL = val
	}
	if val := os.Getenv("CODERS_OLLAMA_AUTH_TOKEN"); val != "" {
		c.Ollama.AuthToken = val
	}
	if val := os.Getenv("CODERS_OLLAMA_API_KEY"); val != "" {
		c.Ollama.APIKey = val
	}

	// Logging settings
	if val := os.Getenv("CODERS_LOG_LEVEL"); val != "" {
		c.Logging.Level = val
	}
	if val := os.Getenv("CODERS_LOG_FILE"); val != "" {
		c.Logging.FilePath = val
	}
	if val := os.Getenv("CODERS_LOG_JSON"); val != "" {
		c.Logging.JSON = val == "true" || val == "1" || val == "yes"
	}
	if val := os.Getenv("CODERS_LOG_CONSOLE"); val != "" {
		c.Logging.Console = val == "true" || val == "1" || val == "yes"
	}
	if val := os.Getenv("CODERS_LOG_MAX_SIZE"); val != "" {
		if size, err := strconv.Atoi(val); err == nil {
			c.Logging.MaxSize = size
		}
	}
	if val := os.Getenv("CODERS_LOG_MAX_BACKUPS"); val != "" {
		if backups, err := strconv.Atoi(val); err == nil {
			c.Logging.MaxBackups = backups
		}
	}
	if val := os.Getenv("CODERS_LOG_MAX_AGE"); val != "" {
		if age, err := strconv.Atoi(val); err == nil {
			c.Logging.MaxAge = age
		}
	}
	if val := os.Getenv("CODERS_LOG_COMPRESS"); val != "" {
		c.Logging.Compress = val == "true" || val == "1" || val == "yes"
	}
}

// Reload forces a reload of the configuration.
// This resets the global singleton and returns the newly loaded config.
func Reload() (*Config, error) {
	configOnce = sync.Once{}
	return Get()
}

// ConfigPaths returns the paths where config files are searched.
func ConfigPaths() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(homeDir, ".config", "coders", "config.yaml"),
		filepath.Join(homeDir, ".config", "coders", "config.yml"),
		filepath.Join(homeDir, ".coders.yaml"),
	}
}

// WriteExample writes an example configuration file to the specified path.
func WriteExample(path string) error {
	example := `# Coders configuration file
# Place this file at ~/.config/coders/config.yaml or ~/.coders.yaml

# Default AI tool to use (claude, gemini, codex, opencode)
default_tool: claude

# Heartbeat interval (Go duration format, e.g., "30s", "1m")
heartbeat_interval: 30s

# Redis connection URL
redis_url: redis://localhost:6379

# Dashboard server port
dashboard_port: 3000

# Default model (tool-specific, leave empty for tool default)
default_model: ""

# Enable heartbeat monitoring by default
default_heartbeat: true

# Ollama configuration (for using Ollama as backend)
ollama:
  base_url: ""
  auth_token: ""
  api_key: ""

# Logging configuration
logging:
  # Log level (debug, info, warn, error)
  level: info
  # Path to log file (leave empty for console only)
  file_path: ""
  # Enable JSON output format
  json: true
  # Enable console output in addition to file
  console: false
  # Maximum size in MB before rotation
  max_size: 10
  # Number of old log files to retain
  max_backups: 5
  # Days to retain old log files
  max_age: 7
  # Compress rotated files
  compress: true
`
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(example), 0644)
}
