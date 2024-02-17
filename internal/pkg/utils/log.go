package utils

import (
	"log/slog"
	"os"
)

func NewHandler(lvl slog.Leveler, addSource bool) slog.Handler {
	logOptions := new(slog.HandlerOptions)
	logOptions.AddSource = addSource
	logOptions.Level = lvl
	return slog.NewTextHandler(os.Stderr, logOptions)
}
