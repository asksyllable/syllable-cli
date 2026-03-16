package cmd

import (
	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func permissionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permissions",
		Short: "Manage permissions",
		Example: `  # List all permissions
  syllable permissions list

  # List permissions as JSON
  syllable permissions list --output json`,
	}

	cmd.AddCommand(permissionsListCmd())

	return cmd
}

func permissionsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List permissions",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/permissions/")
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}
