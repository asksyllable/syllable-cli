package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func dataSourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data-sources",
		Short: "Manage data sources",
		Long:  "List, get, create, update, and delete data sources.",
		Example: `  # List all data sources
  syllable data-sources list

  # Search data sources by name
  syllable data-sources list --search "crm"

  # Get a specific data source
  syllable data-sources get 3

  # Get a data source as JSON
  syllable data-sources get 3 --output json

  # Create a data source from a JSON file
  syllable data-sources create --file datasource.json

  # Update a data source
  syllable data-sources update 3 --file datasource.json

  # Delete a data source
  syllable data-sources delete 3`,
	}

	cmd.AddCommand(dataSourcesListCmd())
	cmd.AddCommand(dataSourcesGetCmd())
	cmd.AddCommand(dataSourcesCreateCmd())
	cmd.AddCommand(dataSourcesUpdateCmd())
	cmd.AddCommand(dataSourcesDeleteCmd())

	return cmd
}

func dataSourcesListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List data sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/data_sources/?page=%d&limit=%d", page, limit)
			if search != "" {
				path += fmt.Sprintf("&search_fields=name&search_field_values=%s", search)
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
			for i, d := range result.Items {
				rows[i] = []string{
					d.ID.String(),
					d.Name,
					output.Truncate(d.Description, 50),
					d.LastUpdated,
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

	return cmd
}

func dataSourcesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <data-source-id>",
		Short: "Get a data source by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/data_sources/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var d struct {
				ID          json.Number `json:"id"`
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Content     string      `json:"content"`
				LastUpdated string      `json:"last_updated"`
				LastUpdBy   string      `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &d); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", d.ID.String()},
				{"Name", d.Name},
				{"Description", d.Description},
				{"Content", output.Truncate(d.Content, 100)},
				{"Last Updated", d.LastUpdated},
				{"Last Updated By", d.LastUpdBy},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func dataSourcesCreateCmd() *cobra.Command {
	var file, name, description, content string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a data source",
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
					"description": description,
					"content":     content,
				}
			}

			data, _, err := apiClient.Post("/api/v1/data_sources/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Data source name")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	cmd.Flags().StringVar(&content, "content", "", "Content text")

	return cmd
}

func dataSourcesUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a data source",
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

			data, _, err := apiClient.Put("/api/v1/data_sources/", body)
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

func dataSourcesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <data-source-id>",
		Short: "Delete a data source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/data_sources/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Data source %s deleted.\n", args[0])
			}
			return nil
		},
	}
}
