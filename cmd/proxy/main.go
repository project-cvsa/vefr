package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vefr/internal/config"
	"vefr/internal/ippool"
	"vefr/internal/proxy"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to TOML configuration")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	pool, err := ippool.New(cfg.SourceIPs, cfg.SourceCIDRs, cfg.Rotation)
	if err != nil {
		logger.Error("create source pool", "error", err)
		os.Exit(1)
	}

	server := proxy.NewServer(cfg, pool, logger)
	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           server,
		ReadHeaderTimeout: cfg.Timeouts.ReadHeader,
		IdleTimeout:       cfg.Timeouts.Idle,
	}

	go func() {
		logger.Info("proxy listening", "address", cfg.Listen, "sources", pool.Size(), "rotation", cfg.Rotation)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "error", err)
	}
}
