package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	mu     sync.RWMutex
	logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
)

// New builds a structured logger. Output is JSON, deterministic field ordering is handled by encoding/json.
func New(level string) (*slog.Logger, error) {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		lvl = slog.LevelInfo
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		return nil, fmt.Errorf("unknown log level: %q", level)
	}

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	return slog.New(h), nil
}

func SetDefault(l *slog.Logger) {
	mu.Lock()
	defer mu.Unlock()
	logger = l
}

func Default() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return logger
}
