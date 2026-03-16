package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func promptsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompts",
		Short: "Manage prompts",
		Long:  "List, get, create, update, and delete prompts.",
		Example: `  # List all prompts
  syllable prompts list

  # Get a prompt and its current content
  syllable prompts get 7

  # See the edit history for a prompt
  syllable prompts history 7

  # List supported LLM models
  syllable prompts supported-llms

  # Create a prompt from a JSON file
  syllable prompts create --file prompt.json

  # Update a prompt
  syllable prompts update 7 --file prompt.json`,
	}

	cmd.AddCommand(promptsListCmd())
	cmd.AddCommand(promptsGetCmd())
	cmd.AddCommand(promptsCreateCmd())
	cmd.AddCommand(promptsUpdateCmd())
	cmd.AddCommand(promptsDeleteCmd())
	cmd.AddCommand(promptsHistoryCmd())
	cmd.AddCommand(promptsSupportedLLMsCmd())

	return cmd
}

func promptsListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List prompts",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/prompts/?page=%d&limit=%d", page, limit)
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
					Name        string `json:"name"`
					Description string `json:"description"`
					Type        string `json:"type"`
					AgentCount  int    `json:"agent_count"`
					LastUpdated string `json:"last_updated"`
					LastUpdBy   string `json:"last_updated_by"`
					Version     int    `json:"version_number"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "TYPE", "VERSION", "AGENTS", "LAST_UPDATED"}
			rows := make([][]string, len(result.Items))
			for i, p := range result.Items {
				rows[i] = []string{
					p.ID.String(),
					p.Name,
					p.Type,
					fmt.Sprintf("%d", p.Version),
					fmt.Sprintf("%d", p.AgentCount),
					p.LastUpdated,
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

func promptsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <prompt-id>",
		Short: "Get a prompt by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/prompts/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var p struct {
				ID          json.Number `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Type        string `json:"type"`
				AgentCount  int    `json:"agent_count"`
				LastUpdated string `json:"last_updated"`
				LastUpdBy   string `json:"last_updated_by"`
				Version     int    `json:"version_number"`
				Context     string `json:"context"`
			}
			if err := json.Unmarshal(data, &p); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", p.ID.String()},
				{"Name", p.Name},
				{"Type", p.Type},
				{"Description", p.Description},
				{"Version", fmt.Sprintf("%d", p.Version)},
				{"Agent Count", fmt.Sprintf("%d", p.AgentCount)},
				{"Last Updated", p.LastUpdated},
				{"Last Updated By", p.LastUpdBy},
				{"Context", output.Truncate(p.Context, 100)},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func promptsCreateCmd() *cobra.Command {
	var file, name, promptType string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a prompt",
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
				if name == "" || promptType == "" {
					return fmt.Errorf("required flags: --name, --type (or use --file)")
				}
				body = map[string]interface{}{
					"name":       name,
					"type":       promptType,
					"llm_config": map[string]interface{}{},
				}
			}

			data, _, err := apiClient.Post("/api/v1/prompts/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Prompt name")
	cmd.Flags().StringVar(&promptType, "type", "", "Prompt type")

	return cmd
}

func promptsUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <prompt-id>",
		Short: "Update a prompt",
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

			data, _, err := apiClient.Put("/api/v1/prompts/", body)
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

func promptsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <prompt-id>",
		Short: "Delete a prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/prompts/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Prompt %s deleted.\n", args[0])
			}
			return nil
		},
	}
}

func promptsHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history <prompt-id>",
		Short: "Get version history for a prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/prompts/" + args[0] + "/history")
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func promptsSupportedLLMsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "supported-llms",
		Short: "List supported LLM providers and models",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/prompts/llms/supported")
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}
