package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func toolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Manage tools",
		Long:  "List, get, create, update, and delete tools.",
		Example: `  # List all tools
  syllable tools list

  # Get a tool as JSON (inspect full config)
  syllable tools get 5 --output json

  # Create a tool from a JSON file
  syllable tools create --file tool.json

  # Update a tool
  syllable tools update 5 --file tool.json

  # Delete a tool
  syllable tools delete 5`,
	}

	cmd.AddCommand(toolsListCmd())
	cmd.AddCommand(toolsGetCmd())
	cmd.AddCommand(toolsCreateCmd())
	cmd.AddCommand(toolsUpdateCmd())
	cmd.AddCommand(toolsDeleteCmd())

	return cmd
}

func toolsListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/tools/?page=%d&limit=%d", page, limit)
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
					ServiceName string      `json:"service_name"`
					ServiceID   json.Number `json:"service_id"`
					LastUpdated string `json:"last_updated"`
					LastUpdBy   string `json:"last_updated_by"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "SERVICE", "LAST_UPDATED", "LAST_UPDATED_BY"}
			rows := make([][]string, len(result.Items))
			for i, t := range result.Items {
				rows[i] = []string{
					t.ID.String(),
					t.Name,
					t.ServiceName,
					t.LastUpdated,
					t.LastUpdBy,
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

func toolsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <tool-name>",
		Short: "Get a tool by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/tools/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var t struct {
				ID          json.Number `json:"id"`
				Name        string      `json:"name"`
				ServiceName string      `json:"service_name"`
				ServiceID   json.Number `json:"service_id"`
				LastUpdated string `json:"last_updated"`
				LastUpdBy   string `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &t); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", t.ID.String()},
				{"Name", t.Name},
				{"Service Name", t.ServiceName},
				{"Service ID", t.ServiceID.String()},
				{"Last Updated", t.LastUpdated},
				{"Last Updated By", t.LastUpdBy},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func toolsCreateCmd() *cobra.Command {
	var file, name, serviceID string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a tool",
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
				if name == "" || serviceID == "" {
					return fmt.Errorf("required flags: --name, --service-id (or use --file)")
				}
				body = map[string]interface{}{
					"name":       name,
					"service_id": serviceID,
					"definition": map[string]interface{}{},
				}
			}

			data, _, err := apiClient.Post("/api/v1/tools/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Tool name")
	cmd.Flags().StringVar(&serviceID, "service-id", "", "Service ID")

	return cmd
}

func toolsUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <tool-name>",
		Short: "Update a tool",
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

			data, _, err := apiClient.Put("/api/v1/tools/", body)
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

func toolsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <tool-name>",
		Short: "Delete a tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/tools/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Tool %s deleted.\n", args[0])
			}
			return nil
		},
	}
}
