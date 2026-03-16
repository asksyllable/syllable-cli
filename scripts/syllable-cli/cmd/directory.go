package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func directoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "directory",
		Short: "Manage directory members",
		Long:  "List, get, create, update, and delete directory members.",
		Example: `  # List all directory members
  syllable directory list

  # Search directory members by name
  syllable directory list --search "billing"

  # Get a specific directory member
  syllable directory get 12

  # Create a directory member from a JSON file
  syllable directory create --file member.json

  # Update a directory member
  syllable directory update 12 --file member.json

  # Delete a directory member
  syllable directory delete 12`,
	}

	cmd.AddCommand(directoryListCmd())
	cmd.AddCommand(directoryGetCmd())
	cmd.AddCommand(directoryCreateCmd())
	cmd.AddCommand(directoryUpdateCmd())
	cmd.AddCommand(directoryDeleteCmd())

	return cmd
}

func directoryListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List directory members",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/directory_members/?page=%d&limit=%d", page, limit)
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
					ID        json.Number `json:"id"`
					Name      string `json:"name"`
					Type      string `json:"type"`
					CreatedAt string `json:"created_at"`
					UpdatedAt string `json:"updated_at"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "TYPE", "CREATED_AT", "UPDATED_AT"}
			rows := make([][]string, len(result.Items))
			for i, m := range result.Items {
				rows[i] = []string{m.ID.String(), m.Name, m.Type, m.CreatedAt, m.UpdatedAt}
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

func directoryGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <member-id>",
		Short: "Get a directory member by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/directory_members/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var m struct {
				ID        json.Number `json:"id"`
				Name      string `json:"name"`
				Type      string `json:"type"`
				Comments  string `json:"comments"`
				CreatedAt string `json:"created_at"`
				UpdatedAt string `json:"updated_at"`
				LastUpdBy string `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &m); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", m.ID.String()},
				{"Name", m.Name},
				{"Type", m.Type},
				{"Comments", m.Comments},
				{"Created At", m.CreatedAt},
				{"Updated At", m.UpdatedAt},
				{"Last Updated By", m.LastUpdBy},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func directoryCreateCmd() *cobra.Command {
	var file, name, memberType string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a directory member",
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
				if name == "" || memberType == "" {
					return fmt.Errorf("required flags: --name, --type (or use --file)")
				}
				body = map[string]interface{}{
					"name": name,
					"type": memberType,
				}
			}

			data, _, err := apiClient.Post("/api/v1/directory_members/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Member name")
	cmd.Flags().StringVar(&memberType, "type", "", "Member type")

	return cmd
}

func directoryUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <member-id>",
		Short: "Update a directory member",
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

			data, _, err := apiClient.Put("/api/v1/directory_members/"+args[0], body)
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

func directoryDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <member-id>",
		Short: "Delete a directory member",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/directory_members/" + args[0] + "?comment=deleted+via+cli")
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Directory member %s deleted.\n", args[0])
			}
			return nil
		},
	}
}
