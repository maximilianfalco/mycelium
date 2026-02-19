package main

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/maximilianfalco/mycelium/internal/api"
	"github.com/maximilianfalco/mycelium/internal/config"
	"github.com/maximilianfalco/mycelium/internal/db"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()

		slog.Info("starting API server", "port", cfg.ServerPort)
		return api.Run(pool, cfg, cfg.ServerPort)
	},
}
