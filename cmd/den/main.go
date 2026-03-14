package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	goruntime "runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/us/den/internal/api"
	"github.com/us/den/internal/api/handlers"
	"github.com/us/den/internal/config"
	"github.com/us/den/internal/engine"
	"github.com/us/den/internal/runtime/docker"
	"github.com/us/den/internal/store"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

var cfgFile string

func main() {
	handlers.SetVersion(version, commit, buildDate)

	rootCmd := &cobra.Command{
		Use:   "den",
		Short: "Self-hosted sandbox runtime for AI agents",
		Long:  "Den provides secure, isolated sandbox environments for AI agents to execute code.",
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: den.yaml)")
	rootCmd.PersistentFlags().String("server", "", "den server URL (default: http://localhost:8080)")

	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(createCmd())
	rootCmd.AddCommand(lsCmd())
	rootCmd.AddCommand(execCmd())
	rootCmd.AddCommand(rmCmd())
	rootCmd.AddCommand(snapshotCmd())
	rootCmd.AddCommand(statsCmd())
	rootCmd.AddCommand(mcpCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the den API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Setup logger
			logLevel := slog.LevelInfo
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: logLevel,
			}))
			slog.SetDefault(logger)

			// Load config
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}

			// Setup log level from config
			switch cfg.Log.Level {
			case "debug":
				logLevel = slog.LevelDebug
			case "warn":
				logLevel = slog.LevelWarn
			case "error":
				logLevel = slog.LevelError
			}
			opts := &slog.HandlerOptions{Level: logLevel}
			var handler slog.Handler
			if cfg.Log.Format == "json" {
				handler = slog.NewJSONHandler(os.Stdout, opts)
			} else {
				handler = slog.NewTextHandler(os.Stdout, opts)
			}
			logger = slog.New(handler)
			slog.SetDefault(logger)

			// Setup store
			st, err := store.NewBoltStore(cfg.Store.Path)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			// Setup runtime
			rt, err := docker.New(
				docker.WithNetworkID(cfg.Runtime.NetworkID),
				docker.WithLogger(logger),
			)
			if err != nil {
				return fmt.Errorf("creating docker runtime: %w", err)
			}

			ctx := context.Background()

			// Verify Docker connection
			if err := rt.Ping(ctx); err != nil {
				return fmt.Errorf("docker not available: %w", err)
			}

			// Ensure network
			if err := rt.EnsureNetwork(ctx); err != nil {
				logger.Warn("failed to create network", "error", err)
			}

			// Warn if auth is disabled
			if !cfg.Auth.Enabled {
				logger.Warn("authentication is DISABLED — API is publicly accessible, set auth.enabled=true in production")
			}

			// Protect Den process from OOM killer (Linux only)
			protectProcess(logger)

			// Setup engine
			eng := engine.NewEngine(rt, st, cfg.Sandbox, cfg.S3, cfg.Resource, logger)

			// Setup API server
			srv := api.NewServer(eng, rt, cfg, logger)

			// Graceful shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			errCh := make(chan error, 1)
			go func() {
				errCh <- srv.Start()
			}()

			logger.Info("den server started",
				"version", version,
				"addr", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
			)

			select {
			case err := <-errCh:
				return err
			case sig := <-sigCh:
				logger.Info("shutting down", "signal", sig)
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				eng.Shutdown(shutdownCtx)
				return srv.Shutdown(shutdownCtx)
			}
		},
	}
}

// protectProcess sets a low OOM score for the Den process on Linux.
// This makes it less likely to be killed when the host is under memory pressure.
func protectProcess(logger *slog.Logger) {
	if goruntime.GOOS != "linux" {
		return
	}
	// -900, not -1000 — kernel critical processes should be protected
	if err := os.WriteFile("/proc/self/oom_score_adj", []byte("-900"), 0644); err != nil {
		logger.Warn("failed to set OOM score adjustment", "error", err)
	} else {
		logger.Debug("set OOM score adjustment to -900")
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("den %s (commit: %s, built: %s)\n", version, commit, buildDate)
		},
	}
}
