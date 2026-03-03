package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/getden/den/internal/api"
	"github.com/getden/den/internal/api/handlers"
	"github.com/getden/den/internal/config"
	"github.com/getden/den/internal/engine"
	"github.com/getden/den/internal/runtime/docker"
	"github.com/getden/den/internal/store"
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
			logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: logLevel,
			}))
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

			// Setup engine
			eng := engine.NewEngine(rt, st, cfg.Sandbox, cfg.S3, logger)

			// Setup API server
			srv := api.NewServer(eng, cfg, logger)

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

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("den %s (commit: %s, built: %s)\n", version, commit, buildDate)
		},
	}
}
