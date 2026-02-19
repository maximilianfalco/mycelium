package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/maximilianfalco/mycelium/internal/config"
	"github.com/maximilianfalco/mycelium/internal/db"
	"github.com/maximilianfalco/mycelium/internal/projects"
)

var coloniesCmd = &cobra.Command{
	Use:   "colonies",
	Short: "Manage colonies",
}

var coloniesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all colonies",
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

		ps, err := projects.ListProjects(context.Background(), pool)
		if err != nil {
			return fmt.Errorf("listing colonies: %w", err)
		}

		if len(ps) == 0 {
			fmt.Println("No colonies found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tDESCRIPTION\tCREATED")
		for _, p := range ps {
			desc := p.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.ID, p.Name, desc, p.CreatedAt.Format("2006-01-02"))
		}
		w.Flush()

		return nil
	},
}

func init() {
	coloniesCmd.AddCommand(coloniesListCmd)
}
