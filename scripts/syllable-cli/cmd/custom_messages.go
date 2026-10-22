package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/asksyllable/syllable-cli/internal/output"
	"github.com/spf13/cobra"
)

func customMessagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custom-messages",
		Short: "Manage custom messages",
		Long:  "List, get, create, update, and delete custom messages.",
		Example: `  # List all custom messages
  syllable custom-messages list

  # Search custom messages by name
  syllable custom-messages list --search "greeting"

  # Get a specific custom message
  syllable custom-messages get 7

  # Create a custom message from a JSON file
  syllable custom-messages create --file message.json

  # Update a custom message
  syllable custom-messages update 7 --file message.json

  # Delete a custom message
  syllable custom-messages delete 7

  # See the edit history for a custom message, then fetch one version
  syllable custom-messages history 7
  syllable custom-messages get-version 7 3`,
	}

	cmd.AddCommand(customMessagesListCmd())
	cmd.AddCommand(customMessagesGetCmd())
	cmd.AddCommand(customMessagesCreateCmd())
	cmd.AddCommand(customMessagesUpdateCmd())
	cmd.AddCommand(customMessagesDeleteCmd())
	cmd.AddCommand(customMessagesHistoryCmd())
	cmd.AddCommand(customMessagesGetVersionCmd())

	return cmd
}

func customMessagesListCmd() *cobra.Command {
	var page, limit int
	var search, searchField string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List custom messages",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/custom_messages/?page=%d&limit=%d", page, limit)
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
					ID         json.Number `json:"id"`
					Name       string      `json:"name"`
					Type       string      `json:"type"`
					AgentCount int         `json:"agent_count"`
					UpdatedAt  string      `json:"updated_at"`
					LastUpdBy  string      `json:"last_updated_by"`
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

func customMessagesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <custom-message-id>",
		Short: "Get a custom message by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/custom_messages/" + url.PathEscape(args[0]))
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var m struct {
				ID         json.Number `json:"id"`
				Name       string      `json:"name"`
				Type       string      `json:"type"`
				Preamble   string      `json:"preamble"`
				Text       string      `json:"text"`
				Subject    string      `json:"subject"`
				AgentCount int         `json:"agent_count"`
				UpdatedAt  string      `json:"updated_at"`
				LastUpdBy  string      `json:"last_updated_by"`
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
				{"Subject", m.Subject},
				{"Agent Count", fmt.Sprintf("%d", m.AgentCount)},
				{"Updated At", m.UpdatedAt},
				{"Last Updated By", m.LastUpdBy},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func customMessagesCreateCmd() *cobra.Command {
	var file, name, text, msgType, subject string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a custom message",
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}

			if file != "" {
				data, err := readFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := decodeRequestJSON(data, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				if name == "" || text == "" {
					return fmt.Errorf("required flags: --name, --text (or use --file)")
				}
				b := map[string]interface{}{
					"name": name,
					"text": text,
					"type": msgType,
				}
				if subject != "" {
					b["subject"] = subject
				}
				body = b
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
	cmd.Flags().StringVar(&text, "text", "", "Message text (body for email_template)")
	cmd.Flags().StringVar(&msgType, "type", "greeting", "Message type: greeting (voice) or email_template")
	cmd.Flags().StringVar(&subject, "subject", "", "Email subject (required for email_template)")

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
				data, err := readFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := decodeRequestJSON(data, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide update body")
			}

			// The API routes this PUT by the id inside the body (#68).
			if err := ensureBodyIdentifier(body, "id", args[0], true); err != nil {
				return err
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
			if err := confirmDelete(cmd, args); err != nil {
				return err
			}
			data, _, err := apiClient.Delete("/api/v1/custom_messages/" + url.PathEscape(args[0]))
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

func customMessagesHistoryCmd() *cobra.Command {
	var page, limit int
	var orderByDirection string

	cmd := &cobra.Command{
		Use:   "history <custom-message-id>",
		Short: "List version history for a custom message",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/custom_messages/%s/history?page=%d&limit=%d", url.PathEscape(args[0]), page, limit)
			if orderByDirection != "" {
				path += "&order_by_direction=" + url.QueryEscape(orderByDirection)
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
					VersionNumber json.Number `json:"version_number"`
					Operation     string      `json:"operation"`
					UpdatedBy     *string     `json:"updated_by"`
					CreatedAt     string      `json:"created_at"`
					Comments      *string     `json:"comments"`
				} `json:"items"`
				TotalCount *int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"VERSION", "OPERATION", "UPDATED_BY", "CREATED_AT", "COMMENTS"}
			rows := make([][]string, len(result.Items))
			for i, h := range result.Items {
				updatedBy, comments := "", ""
				if h.UpdatedBy != nil {
					updatedBy = *h.UpdatedBy
				}
				if h.Comments != nil {
					comments = output.Truncate(*h.Comments, 40)
				}
				rows[i] = []string{h.VersionNumber.String(), h.Operation, updatedBy, h.CreatedAt, comments}
			}
			printTable(headers, rows)
			if result.TotalCount != nil {
				fmt.Printf("\nTotal: %d\n", *result.TotalCount)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&orderByDirection, "order-by-direction", "", "Sort direction: asc or desc")

	return cmd
}

// Named get-version, not version: a command named version anywhere in the
// tree runs without credentials (noAuthCommandNames).
func customMessagesGetVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get-version <custom-message-id> <version>",
		Short: "Get one saved version of a custom message",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/custom_messages/" + url.PathEscape(args[0]) + "/history/" + url.PathEscape(args[1]))
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var m struct {
				ID            json.Number `json:"custom_message_id"`
				VersionNumber json.Number `json:"version_number"`
				Operation     string      `json:"operation"`
				Name          string      `json:"name"`
				Type          string      `json:"type"`
				Text          string      `json:"text"`
				UpdatedBy     *string     `json:"updated_by"`
				CreatedAt     string      `json:"created_at"`
				Comments      *string     `json:"comments"`
			}
			if err := json.Unmarshal(data, &m); err != nil {
				output.PrintJSON(data)
				return nil
			}

			updatedBy, comments := "", ""
			if m.UpdatedBy != nil {
				updatedBy = *m.UpdatedBy
			}
			if m.Comments != nil {
				comments = output.Truncate(*m.Comments, 80)
			}
			rows := [][]string{
				{"Custom Message ID", m.ID.String()},
				{"Version", m.VersionNumber.String()},
				{"Operation", m.Operation},
				{"Name", m.Name},
				{"Type", m.Type},
				{"Text", output.Truncate(m.Text, 80)},
				{"Updated By", updatedBy},
				{"Created At", m.CreatedAt},
				{"Comments", comments},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}
