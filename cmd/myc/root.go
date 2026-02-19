package main

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "myc",
	Short: "A local-only code intelligence tool that parses repositories, builds a structural graph of code relationships, and enables semantic search.",
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(coloniesCmd)
}
