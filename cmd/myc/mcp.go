package main

import (
	"context"
	"log/slog"

	mcpserver "github.com/mark3labs/mcp-go/server"
	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"

	"github.com/maximilianfalco/mycelium/internal/config"
	"github.com/maximilianfalco/mycelium/internal/db"
	"github.com/maximilianfalco/mycelium/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP server (stdio transport)",
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

		var oaiClient *openai.Client
		if cfg.OpenAIAPIKey != "" {
			oaiClient = openai.NewClient(cfg.OpenAIAPIKey)
		}

		s := mcp.NewServer(pool, oaiClient)

		slog.Info("starting MCP server (stdio)")
		return mcpserver.ServeStdio(s)
	},
}
