package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log/level"
)

func cancelCtxOnSigterm(ctx context.Context) context.Context {
	exitCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-exitCh
		cancel()
	}()
	return ctx
}

func info(msg string, keyvals ...interface{}) {
	keyvals = append([]interface{}{"msg", msg}, keyvals...)
	level.Info(logger).Log(keyvals...)
}

func warn(msg string, keyvals ...interface{}) {
	keyvals = append([]interface{}{"msg", msg}, keyvals...)
	level.Warn(logger).Log(keyvals...)
}

func erro(err interface{}, keyvals ...interface{}) {
	keyvals = append([]interface{}{"error", err}, keyvals...)
	level.Error(logger).Log(keyvals...)
}

func debug(msg string, keyvals ...interface{}) {
	keyvals = append([]interface{}{"msg", msg}, keyvals...)
	level.Debug(logger).Log(keyvals...)
}
