package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func conversationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conversations",
		Short: "Manage conversations",
		Long:  "List conversations.",
		Example: `  # List all conversations
  syllable conversations list

  # List conversations with pagination
  syllable conversations list --page 1 --limit 50

  # Search conversations by agent name
  syllable conversations list --search "support"

  # Filter conversations by date range
  syllable conversations list --start-date 2024-01-01 --end-date 2024-01-31

  # List conversations as JSON
  syllable conversations list --output json`,
	}

	cmd.AddCommand(conversationsListCmd())

	return cmd
}

func conversationsListCmd() *cobra.Command {
	var page, limit int
	var search, startDate, endDate string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List conversations",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/conversations/?page=%d&limit=%d", page, limit)
			if search != "" {
				path += fmt.Sprintf("&search_fields=agent_name&search_field_values=%s", search)
			}
			if startDate != "" {
				path += fmt.Sprintf("&start_datetime=%s", startDate)
			}
			if endDate != "" {
				path += fmt.Sprintf("&end_datetime=%s", endDate)
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
					ConversationID string `json:"conversation_id"`
					Timestamp      string `json:"timestamp"`
					AgentID        string `json:"agent_id"`
					AgentName      string `json:"agent_name"`
					AgentType      string `json:"agent_type"`
					PromptName     string `json:"prompt_name"`
				} `json:"items"`
				TotalCount *int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"CONVERSATION_ID", "TIMESTAMP", "AGENT", "TYPE", "PROMPT"}
			rows := make([][]string, len(result.Items))
			for i, c := range result.Items {
				rows[i] = []string{
					c.ConversationID,
					c.Timestamp,
					c.AgentName,
					c.AgentType,
					c.PromptName,
				}
			}
			printTable(headers, rows)
			if result.TotalCount != nil {
				fmt.Printf("\nTotal: %d\n", *result.TotalCount)
			} else {
				fmt.Printf("\nShowing %d item(s)\n", len(result.Items))
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&search, "search", "", "Search by agent name")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start datetime filter (e.g. 2024-01-01T00:00:00Z)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End datetime filter")

	return cmd
}
