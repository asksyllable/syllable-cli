package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func agentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Manage agents",
		Long:  "List, get, create, update, and delete agents.",
		Example: `  # List all agents
  syllable agents list

  # Search agents by name
  syllable agents list --search "support"

  # Get a specific agent (table view)
  syllable agents get 42

  # Get a specific agent as JSON (useful for scripting)
  syllable agents get 42 --output json

  # Create an agent from a JSON file
  syllable agents create --file agent.json

  # Create an agent with inline flags
  syllable agents create --name "Support Bot" --type voice --prompt-id 10 --timezone UTC

  # Update an agent
  syllable agents update 42 --file agent.json

  # Delete an agent
  syllable agents delete 42`,
	}

	cmd.AddCommand(agentsListCmd())
	cmd.AddCommand(agentsGetCmd())
	cmd.AddCommand(agentsCreateCmd())
	cmd.AddCommand(agentsUpdateCmd())
	cmd.AddCommand(agentsDeleteCmd())

	return cmd
}

func agentsListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/agents/?page=%d&limit=%d", page, limit)
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
					Label       string `json:"label"`
					UpdatedAt   string `json:"updated_at"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "TYPE", "LABEL", "DESCRIPTION", "UPDATED"}
			rows := make([][]string, len(result.Items))
			for i, a := range result.Items {
				rows[i] = []string{
					a.ID.String(),
					a.Name,
					a.Type,
					a.Label,
					output.Truncate(a.Description, 50),
					a.UpdatedAt,
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

func agentsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <agent-id>",
		Short: "Get an agent by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/agents/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var a struct {
				ID          json.Number `json:"id"`
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Type        string      `json:"type"`
				Label       string      `json:"label"`
				PromptID    json.Number `json:"prompt_id"`
				Timezone    string `json:"timezone"`
				UpdatedAt   string `json:"updated_at"`
				LastUpdBy   string `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &a); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", a.ID.String()},
				{"Name", a.Name},
				{"Type", a.Type},
				{"Label", a.Label},
				{"Description", a.Description},
				{"Prompt ID", a.PromptID.String()},
				{"Timezone", a.Timezone},
				{"Updated At", a.UpdatedAt},
				{"Last Updated By", a.LastUpdBy},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func agentsCreateCmd() *cobra.Command {
	var file, name, agentType, promptID, timezone string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an agent",
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
				if name == "" || agentType == "" || promptID == "" || timezone == "" {
					return fmt.Errorf("required flags: --name, --type, --prompt-id, --timezone (or use --file)")
				}
				body = map[string]interface{}{
					"name":         name,
					"type":         agentType,
					"prompt_id":    promptID,
					"timezone":     timezone,
					"variables":    map[string]interface{}{},
					"tool_headers": map[string]interface{}{},
				}
			}

			data, _, err := apiClient.Post("/api/v1/agents/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Agent name")
	cmd.Flags().StringVar(&agentType, "type", "", "Agent type")
	cmd.Flags().StringVar(&promptID, "prompt-id", "", "Prompt ID")
	cmd.Flags().StringVar(&timezone, "timezone", "", "Timezone")

	return cmd
}

func agentsUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <agent-id>",
		Short: "Update an agent",
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

			data, _, err := apiClient.Put("/api/v1/agents/", body)
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

func agentsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <agent-id>",
		Short: "Delete an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/agents/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Agent %s deleted.\n", args[0])
			}
			return nil
		},
	}
}
