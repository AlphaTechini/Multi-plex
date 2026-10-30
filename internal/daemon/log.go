package daemon

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// EnvDebug turns on debug logging (hook latency timings) when set to "1".
const EnvDebug = "MPX_DEBUG"

// NewFileLogger opens Paths.LogDir/daemon.log for appending and returns a
// logger writing to it, plus the file so the caller can close it. It logs at
// debug level when MPX_DEBUG=1. With an empty dir it returns a discarding
// logger. Secrets, tool inputs and environment values are never logged by the
// daemon, whatever the level.
func NewFileLogger(logDir string) (*slog.Logger, io.Closer) {
	level := slog.LevelInfo
	if os.Getenv(EnvDebug) == "1" {
		level = slog.LevelDebug
	}
	if logDir == "" {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), nopCloser{}
	}
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), nopCloser{}
	}
	f, err := os.OpenFile(filepath.Join(logDir, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), nopCloser{}
	}
	return slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: level})), f
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
