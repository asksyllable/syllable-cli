package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func eventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Manage events",
		Example: `  # List all events
  syllable events list

  # List events with pagination
  syllable events list --page 0 --limit 50

  # List events as JSON
  syllable events list --output json`,
	}

	cmd.AddCommand(eventsListCmd())

	return cmd
}

func eventsListCmd() *cobra.Command {
	var page, limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List events",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/events/?page=%d&limit=%d", page, limit)

			data, _, err := apiClient.Get(path)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")

	return cmd
}
