package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func organizationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "organizations",
		Short: "Manage organizations",
		Long:  "List organizations.",
		Example: `  # List organizations
  syllable organizations list

  # List organizations as JSON
  syllable organizations list --output json`,
	}

	cmd.AddCommand(organizationsListCmd())

	return cmd
}

func organizationsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List organizations",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/organizations/")
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			// The API returns a single organization object, not a paginated list.
			var org struct {
				ID          json.Number `json:"id"`
				DisplayName string      `json:"display_name"`
				Slug        string      `json:"slug"`
				Description *string     `json:"description"`
				LastUpdated string      `json:"last_updated"`
			}
			if err := json.Unmarshal(data, &org); err != nil {
				output.PrintJSON(data)
				return nil
			}

			desc := ""
			if org.Description != nil {
				desc = output.Truncate(*org.Description, 50)
			}

			headers := []string{"ID", "DISPLAY_NAME", "SLUG", "DESCRIPTION", "LAST_UPDATED"}
			rows := [][]string{
				{org.ID.String(), org.DisplayName, org.Slug, desc, org.LastUpdated},
			}
			printTable(headers, rows)
			return nil
		},
	}

	return cmd
}
