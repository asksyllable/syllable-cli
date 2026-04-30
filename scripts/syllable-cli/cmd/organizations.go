package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func organizationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "organizations",
		Short: "Manage organizations",
		Long:  "Get, create, update, and delete the current organization.",
		Example: `  # Show the current organization
  syllable organizations get

  # Show as JSON
  syllable organizations get --output json

  # Create a new organization
  syllable organizations create --file org.json

  # Update the current organization
  syllable organizations update --file org.json

  # Delete the current organization (requires --confirm)
  syllable organizations delete --confirm`,
	}

	cmd.AddCommand(organizationsGetCmd())
	cmd.AddCommand(organizationsListCmd())
	cmd.AddCommand(organizationsCreateCmd())
	cmd.AddCommand(organizationsUpdateCmd())
	cmd.AddCommand(organizationsDeleteCmd())

	return cmd
}

func organizationsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Get the current organization",
		RunE:  organizationsGetRunE,
	}
}

// organizationsListCmd is a hidden back-compat alias for `organizations get`.
// The spec calls this endpoint "Get Current Organization" (singular) — not a
// list — but earlier CLI versions exposed it as `list`. Keeping the old name
// works avoids breaking scripts.
func organizationsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "list",
		Short:  "Alias of `get` (deprecated)",
		Hidden: true,
		RunE:   organizationsGetRunE,
	}
}

func organizationsGetRunE(cmd *cobra.Command, args []string) error {
	data, _, err := apiClient.Get("/api/v1/organizations/")
	if err != nil {
		return err
	}

	if getOutputFmt() == "json" {
		output.PrintJSON(data)
		return nil
	}

	var org struct {
		ID          json.Number `json:"id"`
		Name        string      `json:"name"`
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

	headers := []string{"ID", "NAME", "DISPLAY_NAME", "SLUG", "DESCRIPTION", "LAST_UPDATED"}
	rows := [][]string{
		{org.ID.String(), org.Name, org.DisplayName, org.Slug, desc, org.LastUpdated},
	}
	printTable(headers, rows)
	return nil
}

func organizationsCreateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("required flag: --file")
			}
			fileData, err := readFile(file)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
			var body interface{}
			if err := json.Unmarshal(fileData, &body); err != nil {
				return fmt.Errorf("parsing JSON file: %w", err)
			}

			data, _, err := apiClient.Post("/api/v1/organizations/", body)
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file (use '-' for stdin)")
	return cmd
}

func organizationsUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the current organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("required flag: --file")
			}
			fileData, err := readFile(file)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
			var body interface{}
			if err := json.Unmarshal(fileData, &body); err != nil {
				return fmt.Errorf("parsing JSON file: %w", err)
			}

			data, _, err := apiClient.Put("/api/v1/organizations/", body)
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file (use '-' for stdin)")
	return cmd
}

func organizationsDeleteCmd() *cobra.Command {
	var confirm bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete the current organization (requires --confirm)",
		Long: `Delete the current organization. Irreversible.

The --confirm flag is required so a typo or autocomplete can't trigger the
destructive call. There is no undo.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				return fmt.Errorf("refusing to delete the current organization without --confirm")
			}
			data, _, err := apiClient.Delete("/api/v1/organizations/")
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Println("Organization deleted.")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm the destructive operation")
	return cmd
}
