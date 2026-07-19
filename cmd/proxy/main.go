package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"vefr/internal/config"
	"vefr/internal/ippool"
	"vefr/internal/proxy"
)

var version = "dev"

var (
	configPath  string
	legacyCheck bool
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "vefr",
		Short:         "IPv6 HTTP forward proxy",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected argument %q", strings.Join(args, " "))
			}
			return runProxy(configPath, legacyCheck)
		},
	}
	root.SetVersionTemplate("vefr {{.Version}}\n")
	root.PersistentFlags().StringVarP(&configPath, "config", "c", "config.toml", "path to TOML configuration")
	root.PersistentFlags().BoolVar(&legacyCheck, "check", false, "validate configuration and exit (legacy alias)")
	root.AddCommand(
		&cobra.Command{Use: "run", Short: "Start the proxy", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return runProxy(configPath, false) }},
		&cobra.Command{Use: "check", Short: "Validate configuration and source addresses", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return runProxy(configPath, true) }},
		&cobra.Command{Use: "version", Short: "Print the binary version", Args: cobra.NoArgs, Run: func(*cobra.Command, []string) { fmt.Printf("vefr %s\n", version) }},
		newSystemdCommand(),
	)
	return root
}

func runProxy(path string, checkOnly bool) error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	pool, err := ippool.New(cfg.SourceIPs, cfg.SourceCIDRs, cfg.Rotation)
	if err != nil {
		return fmt.Errorf("create source pool: %w", err)
	}
	if checkOnly {
		logger.Info("configuration is valid", "path", path, "sources", pool.Size())
		return nil
	}
	server := proxy.NewServer(cfg, pool, logger)
	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           server,
		ReadHeaderTimeout: cfg.Timeouts.ReadHeader,
		IdleTimeout:       cfg.Timeouts.Idle,
		MaxHeaderBytes:    32 << 10,
	}
	go func() {
		logger.Info("proxy listening", "address", cfg.Listen, "sources", pool.Size(), "rotation", cfg.Rotation)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve", "error", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
