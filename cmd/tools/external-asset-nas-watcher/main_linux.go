//go:build linux

package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
)

func main() {
	cfg, err := loadWatcherConfig()
	if err != nil {
		log.Fatalf("load watcher config: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	w, err := newNASWatcher(cfg)
	if err != nil {
		log.Fatalf("create watcher: %v", err)
	}
	defer w.Close()
	if err := w.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("watcher stopped: %v", err)
	}
}
