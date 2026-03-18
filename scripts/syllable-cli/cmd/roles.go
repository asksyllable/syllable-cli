package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

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
			path := fmt.Sprintf("/api/v1/roles/?page=%d&limit=%d", page, limit)
			if search != "" {
				path += fmt.Sprintf("&search_fields=%s&search_field_values=%s", searchField, url.QueryEscape(search))
			}

			data, _, err := apiClient.Get(path)
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var result struct {
				Items []struct {
					ID          json.Number `json:"id"`
					Name        string      `json:"name"`
					Description string      `json:"description"`
					LastUpdated string      `json:"last_updated"`
					LastUpdBy   string      `json:"last_updated_by"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "DESCRIPTION", "LAST_UPDATED"}
			rows := make([][]string, len(result.Items))
			for i, r := range result.Items {
				rows[i] = []string{
					r.ID.String(),
					r.Name,
					output.Truncate(r.Description, 50),
					r.LastUpdated,
				}
			}
			printTable(headers, rows)
			fmt.Printf("\nTotal: %d\n", result.TotalCount)
			return nil
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
			data, _, err := apiClient.Get("/api/v1/roles/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var r struct {
				ID          json.Number `json:"id"`
				Name        string      `json:"name"`
				Description string      `json:"description"`
				LastUpdated string      `json:"last_updated"`
				LastUpdBy   string      `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &r); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", r.ID.String()},
				{"Name", r.Name},
				{"Description", r.Description},
				{"Last Updated", r.LastUpdated},
				{"Last Updated By", r.LastUpdBy},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
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
				fileData, err := readFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(fileData, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
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
		Use:   "update",
		Short: "Update a role",
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}

			if file != "" {
				data, err := readFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(data, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide update body")
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
			data, _, err := apiClient.Delete("/api/v1/roles/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Role %s deleted.\n", args[0])
			}
			return nil
		},
	}
}
