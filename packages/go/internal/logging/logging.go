// Package logging provides structured JSON logging for the coders CLI.
// It uses zerolog for high-performance logging with context fields and
// supports log rotation via lumberjack.
package logging

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Level represents a log level.
type Level = zerolog.Level

// Log levels for convenience.
const (
	DebugLevel = zerolog.DebugLevel
	InfoLevel  = zerolog.InfoLevel
	WarnLevel  = zerolog.WarnLevel
	ErrorLevel = zerolog.ErrorLevel
)

// Config holds logging configuration.
type Config struct {
	// Level is the minimum log level (debug, info, warn, error)
	Level Level

	// JSON enables JSON output format (default: true for file, false for console)
	JSON bool

	// FilePath is the path to the log file (empty for console only)
	FilePath string

	// MaxSize is the maximum size in megabytes before rotation
	MaxSize int

	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int

	// MaxAge is the maximum number of days to retain old log files
	MaxAge int

	// Compress enables gzip compression of rotated files
	Compress bool

	// Console enables console output in addition to file output
	Console bool
}

// DefaultConfig returns sensible default logging configuration.
func DefaultConfig() *Config {
	return &Config{
		Level:      InfoLevel,
		JSON:       true,
		FilePath:   "",
		MaxSize:    10,    // 10 MB
		MaxBackups: 5,     // Keep 5 backups
		MaxAge:     7,     // 7 days
		Compress:   true,  // Compress old files
		Console:    false, // File only by default when file is configured
	}
}

// Logger wraps zerolog.Logger with additional context capabilities.
type Logger struct {
	zl        zerolog.Logger
	sessionID string
	command   string
}

var (
	globalLogger *Logger
	loggerOnce   sync.Once
	loggerMu     sync.RWMutex
)

// Init initializes the global logger with the given configuration.
// If config is nil, defaults are used.
func Init(cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	var writers []io.Writer

	// Set up file writer with rotation if FilePath is specified
	if cfg.FilePath != "" {
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(cfg.FilePath), 0755); err != nil {
			return err
		}

		fileWriter := &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}
		writers = append(writers, fileWriter)
	}

	// Add console output if enabled or if no file path specified
	if cfg.Console || cfg.FilePath == "" {
		if cfg.JSON {
			writers = append(writers, os.Stderr)
		} else {
			// Use pretty console output for non-JSON mode
			writers = append(writers, zerolog.ConsoleWriter{
				Out:        os.Stderr,
				TimeFormat: time.RFC3339,
			})
		}
	}

	// Combine writers
	var output io.Writer
	if len(writers) == 1 {
		output = writers[0]
	} else {
		output = zerolog.MultiLevelWriter(writers...)
	}

	// Create base logger with timestamp
	zl := zerolog.New(output).
		Level(cfg.Level).
		With().
		Timestamp().
		Logger()

	loggerMu.Lock()
	globalLogger = &Logger{zl: zl}
	loggerMu.Unlock()

	return nil
}

// Get returns the global logger, initializing with defaults if needed.
func Get() *Logger {
	loggerOnce.Do(func() {
		if globalLogger == nil {
			// Initialize with defaults (console output)
			_ = Init(nil)
		}
	})

	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return globalLogger
}

// WithSessionID returns a new logger with the session_id field set.
func (l *Logger) WithSessionID(sessionID string) *Logger {
	return &Logger{
		zl:        l.zl.With().Str("session_id", sessionID).Logger(),
		sessionID: sessionID,
		command:   l.command,
	}
}

// WithCommand returns a new logger with the command field set.
func (l *Logger) WithCommand(command string) *Logger {
	return &Logger{
		zl:        l.zl.With().Str("command", command).Logger(),
		sessionID: l.sessionID,
		command:   command,
	}
}

// WithField returns a new logger with an additional field.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		zl:        l.zl.With().Interface(key, value).Logger(),
		sessionID: l.sessionID,
		command:   l.command,
	}
}

// WithFields returns a new logger with additional fields.
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	ctx := l.zl.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	return &Logger{
		zl:        ctx.Logger(),
		sessionID: l.sessionID,
		command:   l.command,
	}
}

// WithError returns a new logger with the error field set.
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		zl:        l.zl.With().Err(err).Logger(),
		sessionID: l.sessionID,
		command:   l.command,
	}
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string) {
	l.zl.Debug().Msg(msg)
}

// Debugf logs a formatted debug message.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.zl.Debug().Msgf(format, args...)
}

// Info logs an info message.
func (l *Logger) Info(msg string) {
	l.zl.Info().Msg(msg)
}

// Infof logs a formatted info message.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.zl.Info().Msgf(format, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string) {
	l.zl.Warn().Msg(msg)
}

// Warnf logs a formatted warning message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.zl.Warn().Msgf(format, args...)
}

// Error logs an error message.
func (l *Logger) Error(msg string) {
	l.zl.Error().Msg(msg)
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.zl.Error().Msgf(format, args...)
}

// Fatal logs a fatal error message and exits.
func (l *Logger) Fatal(msg string) {
	l.zl.Fatal().Msg(msg)
}

// Fatalf logs a formatted fatal message and exits.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.zl.Fatal().Msgf(format, args...)
}

// Event returns a zerolog Event for advanced logging scenarios.
func (l *Logger) Event(level Level) *zerolog.Event {
	return l.zl.WithLevel(level)
}

// ParseLevel parses a level string into a Level.
func ParseLevel(level string) (Level, error) {
	return zerolog.ParseLevel(level)
}

// Convenience functions that use the global logger

// Debug logs a debug message using the global logger.
func Debug(msg string) {
	Get().Debug(msg)
}

// Debugf logs a formatted debug message using the global logger.
func Debugf(format string, args ...interface{}) {
	Get().Debugf(format, args...)
}

// Info logs an info message using the global logger.
func Info(msg string) {
	Get().Info(msg)
}

// Infof logs a formatted info message using the global logger.
func Infof(format string, args ...interface{}) {
	Get().Infof(format, args...)
}

// Warn logs a warning message using the global logger.
func Warn(msg string) {
	Get().Warn(msg)
}

// Warnf logs a formatted warning message using the global logger.
func Warnf(format string, args ...interface{}) {
	Get().Warnf(format, args...)
}

// Error logs an error message using the global logger.
func Error(msg string) {
	Get().Error(msg)
}

// Errorf logs a formatted error message using the global logger.
func Errorf(format string, args ...interface{}) {
	Get().Errorf(format, args...)
}

// Fatal logs a fatal message using the global logger and exits.
func Fatal(msg string) {
	Get().Fatal(msg)
}

// Fatalf logs a formatted fatal message using the global logger and exits.
func Fatalf(format string, args ...interface{}) {
	Get().Fatalf(format, args...)
}

// WithSessionID returns a new logger with session_id set.
func WithSessionID(sessionID string) *Logger {
	return Get().WithSessionID(sessionID)
}

// WithCommand returns a new logger with command set.
func WithCommand(command string) *Logger {
	return Get().WithCommand(command)
}

// WithField returns a new logger with an additional field.
func WithField(key string, value interface{}) *Logger {
	return Get().WithField(key, value)
}

// WithFields returns a new logger with additional fields.
func WithFields(fields map[string]interface{}) *Logger {
	return Get().WithFields(fields)
}

// WithError returns a new logger with the error set.
func WithError(err error) *Logger {
	return Get().WithError(err)
}

// InitFromConfig initializes the logger from a LoggingConfig.
// This is a convenience function for use with the config package.
type LoggingConfig struct {
	Level      string
	FilePath   string
	JSON       bool
	Console    bool
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
}

// InitFromLogConfig initializes the logger from a LoggingConfig struct.
func InitFromLogConfig(lc LoggingConfig) error {
	cfg := DefaultConfig()

	// Parse log level
	if lc.Level != "" {
		level, err := ParseLevel(lc.Level)
		if err != nil {
			return err
		}
		cfg.Level = level
	}

	cfg.FilePath = lc.FilePath
	cfg.JSON = lc.JSON
	cfg.Console = lc.Console

	if lc.MaxSize > 0 {
		cfg.MaxSize = lc.MaxSize
	}
	if lc.MaxBackups > 0 {
		cfg.MaxBackups = lc.MaxBackups
	}
	if lc.MaxAge > 0 {
		cfg.MaxAge = lc.MaxAge
	}
	cfg.Compress = lc.Compress

	return Init(cfg)
}
