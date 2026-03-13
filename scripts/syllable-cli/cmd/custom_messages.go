package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func customMessagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custom-messages",
		Short: "Manage custom messages",
		Long:  "List, get, create, update, and delete custom messages.",
	}

	cmd.AddCommand(customMessagesListCmd())
	cmd.AddCommand(customMessagesGetCmd())
	cmd.AddCommand(customMessagesCreateCmd())
	cmd.AddCommand(customMessagesUpdateCmd())
	cmd.AddCommand(customMessagesDeleteCmd())

	return cmd
}

func customMessagesListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List custom messages",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/custom_messages/?page=%d&limit=%d", page, limit)
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
					ID         json.Number `json:"id"`
					Name       string `json:"name"`
					Type       string `json:"type"`
					AgentCount int    `json:"agent_count"`
					UpdatedAt  string `json:"updated_at"`
					LastUpdBy  string `json:"last_updated_by"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "TYPE", "AGENTS", "UPDATED_AT"}
			rows := make([][]string, len(result.Items))
			for i, m := range result.Items {
				rows[i] = []string{
					m.ID.String(),
					m.Name,
					m.Type,
					fmt.Sprintf("%d", m.AgentCount),
					m.UpdatedAt,
				}
			}
			output.PrintTable(headers, rows)
			fmt.Printf("\nTotal: %d\n", result.TotalCount)
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&search, "search", "", "Search by name")

	return cmd
}

func customMessagesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <custom-message-id>",
		Short: "Get a custom message by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/custom_messages/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var m struct {
				ID         json.Number `json:"id"`
				Name       string `json:"name"`
				Type       string `json:"type"`
				Preamble   string `json:"preamble"`
				Text       string `json:"text"`
				AgentCount int    `json:"agent_count"`
				UpdatedAt  string `json:"updated_at"`
				LastUpdBy  string `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &m); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", m.ID.String()},
				{"Name", m.Name},
				{"Type", m.Type},
				{"Preamble", output.Truncate(m.Preamble, 80)},
				{"Text", output.Truncate(m.Text, 80)},
				{"Agent Count", fmt.Sprintf("%d", m.AgentCount)},
				{"Updated At", m.UpdatedAt},
				{"Last Updated By", m.LastUpdBy},
			}
			output.PrintTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func customMessagesCreateCmd() *cobra.Command {
	var file, name, text string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a custom message",
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}

			if file != "" {
				data, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(data, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				if name == "" || text == "" {
					return fmt.Errorf("required flags: --name, --text (or use --file)")
				}
				body = map[string]interface{}{
					"name": name,
					"text": text,
				}
			}

			data, _, err := apiClient.Post("/api/v1/custom_messages/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Message name")
	cmd.Flags().StringVar(&text, "text", "", "Message text")

	return cmd
}

func customMessagesUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <custom-message-id>",
		Short: "Update a custom message",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}

			if file != "" {
				data, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(data, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide update body")
			}

			data, _, err := apiClient.Put("/api/v1/custom_messages/", body)
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

func customMessagesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <custom-message-id>",
		Short: "Delete a custom message",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/custom_messages/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Custom message %s deleted.\n", args[0])
			}
			return nil
		},
	}
}
