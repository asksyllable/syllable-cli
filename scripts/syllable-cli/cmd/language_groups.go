package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func languageGroupsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "language-groups",
		Short: "Manage language groups",
		Long:  "List, get, create, update, and delete language groups.",
		Example: `  # List all language groups
  syllable language-groups list

  # Search language groups by name
  syllable language-groups list --search "spanish"

  # Get a specific language group
  syllable language-groups get 2

  # Create a language group from a JSON file
  syllable language-groups create --file language-group.json

  # Update a language group
  syllable language-groups update 2 --file language-group.json

  # Delete a language group
  syllable language-groups delete 2`,
	}

	cmd.AddCommand(languageGroupsListCmd())
	cmd.AddCommand(languageGroupsGetCmd())
	cmd.AddCommand(languageGroupsCreateCmd())
	cmd.AddCommand(languageGroupsUpdateCmd())
	cmd.AddCommand(languageGroupsDeleteCmd())

	return cmd
}

func languageGroupsListCmd() *cobra.Command {
	var page, limit int
	var search, searchField string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List language groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/language_groups/?page=%d&limit=%d", page, limit)
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
					Name        string `json:"name"`
					Description string `json:"description"`
					UpdatedAt   string `json:"updated_at"`
					LastUpdBy   string `json:"last_updated_by"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "DESCRIPTION", "UPDATED_AT"}
			rows := make([][]string, len(result.Items))
			for i, g := range result.Items {
				rows[i] = []string{
					g.ID.String(),
					g.Name,
					output.Truncate(g.Description, 50),
					g.UpdatedAt,
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

func languageGroupsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <language-group-id>",
		Short: "Get a language group by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/language_groups/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var g struct {
				ID          json.Number `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				UpdatedAt   string `json:"updated_at"`
				LastUpdBy   string `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &g); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", g.ID.String()},
				{"Name", g.Name},
				{"Description", g.Description},
				{"Updated At", g.UpdatedAt},
				{"Last Updated By", g.LastUpdBy},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func languageGroupsCreateCmd() *cobra.Command {
	var file, name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a language group",
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
				if name == "" {
					return fmt.Errorf("required flags: --name (or use --file)")
				}
				body = map[string]interface{}{
					"name":                           name,
					"language_configs":               []interface{}{},
					"skip_current_language_in_message": false,
				}
			}

			data, _, err := apiClient.Post("/api/v1/language_groups/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Language group name")

	return cmd
}

func languageGroupsUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <language-group-id>",
		Short: "Update a language group",
		Args:  cobra.ExactArgs(1),
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

			data, _, err := apiClient.Put("/api/v1/language_groups/", body)
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

func languageGroupsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <language-group-id>",
		Short: "Delete a language group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/language_groups/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Language group %s deleted.\n", args[0])
			}
			return nil
		},
	}
}
