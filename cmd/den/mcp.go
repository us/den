package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/us/den/internal/config"
	"github.com/us/den/internal/engine"
	"github.com/us/den/internal/mcp"
	"github.com/us/den/internal/runtime/docker"
	"github.com/us/den/internal/store"
)

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server (stdio mode)",
		Long:  "Start an MCP server that communicates via stdin/stdout for AI agent integration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Log to stderr so stdout is reserved for MCP protocol
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelWarn,
			}))

			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}

			st, err := store.NewBoltStore(cfg.Store.Path)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			rt, err := docker.New(
				docker.WithNetworkID(cfg.Runtime.NetworkID),
				docker.WithLogger(logger),
			)
			if err != nil {
				return fmt.Errorf("creating docker runtime: %w", err)
			}

			ctx := context.Background()
			if err := rt.Ping(ctx); err != nil {
				return fmt.Errorf("docker not available: %w", err)
			}

			rt.EnsureNetwork(ctx)

			eng := engine.NewEngine(rt, st, cfg.Sandbox, cfg.S3, logger)

			srv := mcp.NewServer(eng, logger)
			return srv.Run(ctx)
		},
	}
}
