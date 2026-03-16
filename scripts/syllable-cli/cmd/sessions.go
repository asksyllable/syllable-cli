package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func sessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage sessions",
		Long:  "List, get, transcript, and summary for sessions.",
		Example: `  # List all sessions
  syllable sessions list

  # Filter sessions by date range
  syllable sessions list --start-date 2024-01-01 --end-date 2024-01-31

  # Search sessions by agent name
  syllable sessions list --search "support"

  # Get a specific session
  syllable sessions get abc-123-def

  # Get the transcript for a session
  syllable sessions transcript abc-123-def

  # Get the summary for a session
  syllable sessions summary abc-123-def

  # Get latency information for a session
  syllable sessions latency abc-123-def

  # Get the recording for a session
  syllable sessions recording abc-123-def`,
	}

	cmd.AddCommand(sessionsListCmd())
	cmd.AddCommand(sessionsGetCmd())
	cmd.AddCommand(sessionsTranscriptCmd())
	cmd.AddCommand(sessionsSummaryCmd())
	cmd.AddCommand(sessionsLatencyCmd())
	cmd.AddCommand(sessionsRecordingCmd())

	return cmd
}

func sessionsListCmd() *cobra.Command {
	var page, limit int
	var search, startDate, endDate string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/sessions/?page=%d&limit=%d", page, limit)
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
					SessionID    string  `json:"session_id"`
					Timestamp    string  `json:"timestamp"`
					AgentName    string  `json:"agent_name"`
					AgentType    string  `json:"agent_type"`
					Duration     float64 `json:"duration"`
					Source       string  `json:"source"`
					Target       string  `json:"target"`
					IsTest       bool    `json:"is_test"`
				} `json:"items"`
				TotalCount *int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"SESSION_ID", "TIMESTAMP", "AGENT", "DURATION", "SOURCE", "TARGET", "IS_TEST"}
			rows := make([][]string, len(result.Items))
			for i, s := range result.Items {
				isTest := "no"
				if s.IsTest {
					isTest = "yes"
				}
				rows[i] = []string{
					s.SessionID,
					s.Timestamp,
					s.AgentName,
					fmt.Sprintf("%.1fs", s.Duration),
					s.Source,
					s.Target,
					isTest,
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

func sessionsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <session-id>",
		Short: "Get a session by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/sessions/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var s struct {
				SessionID      string  `json:"session_id"`
				ConversationID string  `json:"conversation_id"`
				Timestamp      string  `json:"timestamp"`
				AgentName      string  `json:"agent_name"`
				AgentType      string  `json:"agent_type"`
				AgentTimezone  string  `json:"agent_timezone"`
				PromptName     string  `json:"prompt_name"`
				Duration       float64 `json:"duration"`
				Source         string  `json:"source"`
				Target         string  `json:"target"`
				IsTest         bool    `json:"is_test"`
			}
			if err := json.Unmarshal(data, &s); err != nil {
				output.PrintJSON(data)
				return nil
			}

			isTest := "no"
			if s.IsTest {
				isTest = "yes"
			}
			rows := [][]string{
				{"Session ID", s.SessionID},
				{"Conversation ID", s.ConversationID},
				{"Timestamp", s.Timestamp},
				{"Agent", s.AgentName},
				{"Agent Type", s.AgentType},
				{"Timezone", s.AgentTimezone},
				{"Prompt", s.PromptName},
				{"Duration", fmt.Sprintf("%.1fs", s.Duration)},
				{"Source", s.Source},
				{"Target", s.Target},
				{"Is Test", isTest},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func sessionsTranscriptCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "transcript <session-id>",
		Short: "Get transcript for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/sessions/transcript/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var result struct {
				SessionID    string `json:"session_id"`
				Transcription []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
					Time    string `json:"time"`
				} `json:"transcription"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			fmt.Printf("Session: %s\n\n", result.SessionID)
			headers := []string{"TIME", "ROLE", "CONTENT"}
			rows := make([][]string, len(result.Transcription))
			for i, t := range result.Transcription {
				rows[i] = []string{t.Time, t.Role, output.Truncate(t.Content, 80)}
			}
			printTable(headers, rows)
			return nil
		},
	}
}

func sessionsSummaryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "summary <session-id>",
		Short: "Get summary for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/sessions/full-summary/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var result struct {
				Summary string `json:"summary"`
				Rating  string `json:"rating"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"Rating", result.Rating},
				{"Summary", result.Summary},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func sessionsLatencyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "latency <session-id>",
		Short: "Get latency info for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/sessions/latency/" + args[0])
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func sessionsRecordingCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "recording <session-id>",
		Short: "Get recording for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Post("/api/v1/sessions/recording/"+args[0], map[string]interface{}{})
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}
