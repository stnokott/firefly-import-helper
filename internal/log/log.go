package log

import (
	"log/slog"
	"os"
)

func Setup() {
	level := slog.LevelInfo
	if os.Getenv("DEBUG") != "" {
		level = slog.LevelDebug
	}
	slog.SetLogLoggerLevel(level)
}
