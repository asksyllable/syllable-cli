package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/asksyllable/syllable-cli/internal/output"
	"github.com/spf13/cobra"
)

// role is the list/get view of a role resource.
type role struct {
	ID          json.Number `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	LastUpdated string      `json:"last_updated"`
	LastUpdBy   string      `json:"last_updated_by"`
}

func rolesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "Manage roles",
		Long:  "List, get, create, update, and delete roles.",
		Example: `  # List all roles
  syllable roles list

  # Search roles by name
  syllable roles list --search "admin"

  # Get a specific role
  syllable roles get 1

  # Get a role as JSON
  syllable roles get 1 --output json

  # Create a role from a JSON file
  syllable roles create --file role.json

  # Update a role
  syllable roles update 1 --file role.json

  # Delete a role
  syllable roles delete 1`,
	}

	cmd.AddCommand(rolesListCmd())
	cmd.AddCommand(rolesGetCmd())
	cmd.AddCommand(rolesCreateCmd())
	cmd.AddCommand(rolesUpdateCmd())
	cmd.AddCommand(rolesDeleteCmd())

	return cmd
}

func rolesListCmd() *cobra.Command {
	var page, limit int
	var search, searchField string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List roles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(
				listQuery("/api/v1/roles/", page, limit, searchField, search),
				[]string{"ID", "NAME", "DESCRIPTION", "LAST_UPDATED"},
				func(r role) []string {
					return []string{r.ID.String(), r.Name, output.Truncate(r.Description, 50), r.LastUpdated}
				},
			)
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&search, "search", "", "Search by name")
	cmd.Flags().StringVar(&searchField, "search-field", "name", "Field to search on (see API docs for valid values)")

	return cmd
}

func rolesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <role-id>",
		Short: "Get a role by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet("/api/v1/roles/"+url.PathEscape(args[0]), func(r role) [][]string {
				return [][]string{
					{"ID", r.ID.String()},
					{"Name", r.Name},
					{"Description", r.Description},
					{"Last Updated", r.LastUpdated},
					{"Last Updated By", r.LastUpdBy},
				}
			})
		},
	}
}

func rolesCreateCmd() *cobra.Command {
	var file, name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a role",
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}
			if file != "" {
				b, err := readJSONBody(file)
				if err != nil {
					return err
				}
				body = b
			} else {
				if name == "" {
					return fmt.Errorf("required flags: --name (or use --file)")
				}
				body = map[string]interface{}{
					"name":        name,
					"permissions": []interface{}{},
				}
			}

			data, _, err := apiClient.Post("/api/v1/roles/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Role name")

	return cmd
}

func rolesUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update [id]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a role",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("use --file to provide update body")
			}
			body, err := readJSONBody(file)
			if err != nil {
				return err
			}

			// An optional positional id is reconciled with the body id, which the
			// collection PUT routes by — matching the agents/prompts shape (#121, #68).
			if len(args) == 1 {
				if err := ensureBodyIdentifier(body, "id", args[0], true); err != nil {
					return err
				}
			}
			data, _, err := apiClient.Put("/api/v1/roles/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	return cmd
}

func rolesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <role-id>",
		Short: "Delete a role",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, args, "/api/v1/roles/"+url.PathEscape(args[0]), "Role")
		},
	}
}
