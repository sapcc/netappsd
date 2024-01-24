package utils

import (
	"log/slog"
	"os"
)

func NewLogger(lvl slog.Leveler, addSource bool) *slog.Logger {
	logOptions := new(slog.HandlerOptions)
	logOptions.AddSource = addSource
	logOptions.Level = lvl
	return slog.New(slog.NewTextHandler(os.Stderr, logOptions))
}
