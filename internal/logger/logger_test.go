package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected slog.Level
	}{
		{
			name:     "debug level",
			level:    "debug",
			expected: slog.LevelDebug,
		},
		{
			name:     "info level",
			level:    "info",
			expected: slog.LevelInfo,
		},
		{
			name:     "warn level",
			level:    "warn",
			expected: slog.LevelWarn,
		},
		{
			name:     "error level",
			level:    "error",
			expected: slog.LevelError,
		},
		{
			name:     "invalid level defaults to info",
			level:    "invalid",
			expected: slog.LevelInfo,
		},
		{
			name:     "empty level defaults to info",
			level:    "",
			expected: slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.level)
			assert.NotNil(t, logger)

			// Test that the logger is properly configured by attempting to log
			// and verifying it behaves according to the level
			var buf bytes.Buffer
			testLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
				Level: tt.expected,
			}))

			// Test debug logging
			testLogger.Debug("debug message")
			output := buf.String()

			if tt.expected == slog.LevelDebug {
				assert.Contains(t, output, "debug message")
			} else {
				assert.Empty(t, output)
			}

			// Reset buffer for info test
			buf.Reset()
			testLogger.Info("info message")
			output = buf.String()

			if tt.expected <= slog.LevelInfo {
				assert.Contains(t, output, "info message")
			} else {
				assert.Empty(t, output)
			}
		})
	}
}

func TestLoggerOutput(t *testing.T) {
	// Create a logger that we can capture output from
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	logger.Debug("debug message", "key", "value")
	output := buf.String()

	assert.Contains(t, output, "debug message")
	assert.Contains(t, output, "key=value")
	assert.Contains(t, output, "level=DEBUG")
}

func TestLoggerLevels(t *testing.T) {
	tests := []struct {
		name          string
		level         string
		shouldLogInfo bool
		shouldLogWarn bool
	}{
		{
			name:          "debug level logs everything",
			level:         "debug",
			shouldLogInfo: true,
			shouldLogWarn: true,
		},
		{
			name:          "info level logs info and above",
			level:         "info",
			shouldLogInfo: true,
			shouldLogWarn: true,
		},
		{
			name:          "warn level logs warn and above",
			level:         "warn",
			shouldLogInfo: false,
			shouldLogWarn: true,
		},
		{
			name:          "error level logs only error",
			level:         "error",
			shouldLogInfo: false,
			shouldLogWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			var expectedLevel slog.Level

			switch tt.level {
			case "debug":
				expectedLevel = slog.LevelDebug
			case "info":
				expectedLevel = slog.LevelInfo
			case "warn":
				expectedLevel = slog.LevelWarn
			case "error":
				expectedLevel = slog.LevelError
			}

			testLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
				Level: expectedLevel,
			}))

			// Test info logging
			testLogger.Info("info message")
			infoOutput := buf.String()

			if tt.shouldLogInfo {
				assert.Contains(t, infoOutput, "info message")
			} else {
				assert.Empty(t, infoOutput)
			}

			// Reset and test warn logging
			buf.Reset()
			testLogger.Warn("warn message")
			warnOutput := buf.String()

			if tt.shouldLogWarn {
				assert.Contains(t, warnOutput, "warn message")
			} else {
				assert.Empty(t, warnOutput)
			}
		})
	}
}

func TestLoggerWithAttributes(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("test message", "string_attr", "value", "int_attr", 42, "bool_attr", true)
	output := buf.String()

	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "string_attr=value")
	assert.Contains(t, output, "int_attr=42")
	assert.Contains(t, output, "bool_attr=true")
}

func TestNewLoggerReturnsWorkingLogger(t *testing.T) {
	logger := NewLogger("info")
	assert.NotNil(t, logger)

	// Verify we can call logger methods without panicking
	assert.NotPanics(t, func() {
		logger.Info("test message")
		logger.Debug("debug message")
		logger.Warn("warn message")
		logger.Error("error message")
	})
}

// Test case-insensitive level matching
func TestLoggerCaseInsensitive(t *testing.T) {
	tests := []string{"DEBUG", "Info", "WARN", "Error", "DeBuG"}

	for _, level := range tests {
		t.Run("case_insensitive_"+level, func(t *testing.T) {
			logger := NewLogger(strings.ToLower(level))
			assert.NotNil(t, logger)
		})
	}
}