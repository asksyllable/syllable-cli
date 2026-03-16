package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func voiceGroupsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "voice-groups",
		Short: "Manage voice groups",
		Long:  "List, get, create, update, and delete voice groups.",
		Example: `  # List all voice groups
  syllable voice-groups list

  # Search voice groups by name
  syllable voice-groups list --search "english"

  # Get a specific voice group
  syllable voice-groups get 3

  # Get a voice group as JSON
  syllable voice-groups get 3 --output json

  # Create a voice group from a JSON file
  syllable voice-groups create --file voice-group.json

  # Update a voice group
  syllable voice-groups update 3 --file voice-group.json

  # Delete a voice group
  syllable voice-groups delete 3`,
	}

	cmd.AddCommand(voiceGroupsListCmd())
	cmd.AddCommand(voiceGroupsGetCmd())
	cmd.AddCommand(voiceGroupsCreateCmd())
	cmd.AddCommand(voiceGroupsUpdateCmd())
	cmd.AddCommand(voiceGroupsDeleteCmd())

	return cmd
}

func voiceGroupsListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List voice groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/voice_groups/?page=%d&limit=%d", page, limit)
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
					UpdatedAt   string      `json:"updated_at"`
					LastUpdBy   string      `json:"last_updated_by"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "DESCRIPTION", "UPDATED_AT"}
			rows := make([][]string, len(result.Items))
			for i, v := range result.Items {
				rows[i] = []string{
					v.ID.String(),
					v.Name,
					output.Truncate(v.Description, 50),
					v.UpdatedAt,
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

func voiceGroupsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <voice-group-id>",
		Short: "Get a voice group by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/voice_groups/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var v struct {
				ID          json.Number `json:"id"`
				Name        string      `json:"name"`
				Description string      `json:"description"`
				UpdatedAt   string      `json:"updated_at"`
				LastUpdBy   string      `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &v); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", v.ID.String()},
				{"Name", v.Name},
				{"Description", v.Description},
				{"Updated At", v.UpdatedAt},
				{"Last Updated By", v.LastUpdBy},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func voiceGroupsCreateCmd() *cobra.Command {
	var file, name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a voice group",
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
					"name":             name,
					"language_configs": []interface{}{},
				}
			}

			data, _, err := apiClient.Post("/api/v1/voice_groups/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Voice group name")

	return cmd
}

func voiceGroupsUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a voice group",
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

			data, _, err := apiClient.Put("/api/v1/voice_groups/", body)
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

func voiceGroupsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <voice-group-id>",
		Short: "Delete a voice group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/voice_groups/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Voice group %s deleted.\n", args[0])
			}
			return nil
		},
	}
}
