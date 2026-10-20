package main

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupLogging already takes an io.Writer, so the level tests write to a
// buffer. The stderr-routing test goes through the root command's real
// PersistentPreRunE with os.Stdout and os.Stderr redirected to pipes, which
// is the call site that decides where logs go.

func TestSetupLoggingAcceptsLevelsCaseInsensitively(t *testing.T) {
	tests := []struct {
		name  string
		level slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"Info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupLoggingTest(t)
			flags.logFormat = logFormatText
			flags.logLevel = tt.name

			var buf bytes.Buffer
			require.NoError(t, setupLogging(&buf))

			logger := slog.Default()
			assert.True(t, logger.Enabled(t.Context(), tt.level), "%s must be enabled", tt.level)
			if tt.level > slog.LevelDebug {
				assert.False(t, logger.Enabled(t.Context(), tt.level-1),
					"levels below %s must be filtered", tt.level)
			}
		})
	}
}

func TestSetupLoggingRejectsUnknownLevel(t *testing.T) {
	setupLoggingTest(t)
	flags.logFormat = logFormatText
	flags.logLevel = "verbose"

	err := setupLogging(&bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"verbose"`)
	for _, accepted := range []string{"debug", "info", "warn", "error"} {
		assert.Contains(t, err.Error(), accepted)
	}
}

func TestLogsGoToStderrNotStdout(t *testing.T) {
	setupLoggingTest(t)
	flags.logFormat = logFormatText
	flags.logLevel = "info"

	stdoutR, stdoutW, err := os.Pipe()
	require.NoError(t, err)
	stderrR, stderrW, err := os.Pipe()
	require.NoError(t, err)

	savedStdout, savedStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutW, stderrW
	t.Cleanup(func() { os.Stdout, os.Stderr = savedStdout, savedStderr })

	cmd := newRootCmd()
	require.NotNil(t, cmd.PersistentPreRunE)
	runErr := cmd.PersistentPreRunE(cmd, nil)
	slog.Info("routing check", "alias", "counter")

	os.Stdout, os.Stderr = savedStdout, savedStderr
	require.NoError(t, stdoutW.Close())
	require.NoError(t, stderrW.Close())
	require.NoError(t, runErr)

	stdout, err := io.ReadAll(stdoutR)
	require.NoError(t, err)
	stderr, err := io.ReadAll(stderrR)
	require.NoError(t, err)

	assert.Empty(t, string(stdout), "log lines must never reach stdout, where --json output goes")
	assert.True(t, strings.Contains(string(stderr), "msg=\"routing check\""), "log line must reach stderr: %q", stderr)
}
