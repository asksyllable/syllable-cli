package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/asksyllable/syllable-cli/internal/output"
	"github.com/spf13/cobra"
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
	cmd.AddCommand(sessionsTimelineCmd())
	cmd.AddCommand(sessionsSummaryCmd())
	cmd.AddCommand(sessionsLatencyCmd())
	cmd.AddCommand(sessionsRecordingCmd())
	cmd.AddCommand(sessionsRecordingStreamCmd())

	return cmd
}

func sessionsRecordingStreamCmd() *cobra.Command {
	var token string

	cmd := &cobra.Command{
		Use:   "recording-stream",
		Short: "Stream recording bytes for a token from sessions recording",
		Long: `Stream the binary recording bytes for a token returned by ` + "`syllable sessions recording`" + `.

The recording endpoint returns short-lived streaming tokens; pass one of those
to --token. Bytes are written to stdout for redirection to a file.`,
		Example: `  # Get streaming tokens, pull one out, fetch the audio
  syllable sessions recording <session-id> --output json | jq -r '.<token-field>' | \
    xargs -I{} syllable sessions recording-stream --token {} > recording.wav`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" {
				return fmt.Errorf("required flag: --token")
			}
			path := "/api/v1/sessions/recording/stream?token=" + url.QueryEscape(token)
			body, _, err := apiClient.GetStream(path)
			if err != nil {
				return err
			}
			defer body.Close()
			if _, err := io.Copy(os.Stdout, body); err != nil {
				return fmt.Errorf("streaming recording: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "Streaming token from `sessions recording`")
	return cmd
}

func sessionsListCmd() *cobra.Command {
	var page, limit int
	var search, searchField, startDate, endDate string
	var includeTest bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/sessions/?page=%d&limit=%d", page, limit)
			if search != "" {
				path += fmt.Sprintf("&search_fields=%s&search_field_values=%s", searchField, url.QueryEscape(search))
			}
			if startDate != "" {
				path += fmt.Sprintf("&start_datetime=%s", url.QueryEscape(startDate))
			}
			if endDate != "" {
				path += fmt.Sprintf("&end_datetime=%s", url.QueryEscape(endDate))
			}
			if includeTest {
				path += "&search_fields=is_test&search_field_values=true"
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
					SessionID string  `json:"session_id"`
					Timestamp string  `json:"timestamp"`
					AgentName string  `json:"agent_name"`
					AgentType string  `json:"agent_type"`
					Duration  float64 `json:"duration"`
					Source    string  `json:"source"`
					Target    string  `json:"target"`
					IsTest    bool    `json:"is_test"`
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
	cmd.Flags().StringVar(&searchField, "search-field", "agent_name", "Field to search on (see API docs for valid values)")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start datetime filter (e.g. 2024-01-01T00:00:00Z)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End datetime filter")
	cmd.Flags().BoolVar(&includeTest, "include-test", false, "Include sessions flagged is_test=true, which are hidden by default (e.g. conversation tests from 'agents send-test-message')")

	return cmd
}

func sessionsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <session-id>",
		Short: "Get a session by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/sessions/" + url.PathEscape(args[0]))
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var s struct {
				SessionID                 string  `json:"session_id"`
				ConversationID            string  `json:"conversation_id"`
				Timestamp                 string  `json:"timestamp"`
				AgentName                 string  `json:"agent_name"`
				AgentType                 string  `json:"agent_type"`
				AgentTimezone             string  `json:"agent_timezone"`
				PromptName                string  `json:"prompt_name"`
				Duration                  float64 `json:"duration"`
				Source                    string  `json:"source"`
				Target                    string  `json:"target"`
				IsTest                    bool    `json:"is_test"`
				TransferVoicemailDetected *bool   `json:"transfer_voicemail_detected"`
			}
			if err := json.Unmarshal(data, &s); err != nil {
				output.PrintJSON(data)
				return nil
			}

			isTest := "no"
			if s.IsTest {
				isTest = "yes"
			}
			transferVoicemail := ""
			if s.TransferVoicemailDetected != nil {
				if *s.TransferVoicemailDetected {
					transferVoicemail = "yes"
				} else {
					transferVoicemail = "no"
				}
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
				{"Transfer Voicemail", transferVoicemail},
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
			data, _, err := apiClient.Get("/api/v1/sessions/transcript/" + url.PathEscape(args[0]))
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var result struct {
				SessionID     string `json:"session_id"`
				Transcription []struct {
					Source    string `json:"source"`
					Text      string `json:"text"`
					Timestamp string `json:"timestamp"`
				} `json:"transcription"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			fmt.Printf("Session: %s\n\n", result.SessionID)
			headers := []string{"TIME", "SOURCE", "TEXT"}
			rows := make([][]string, len(result.Transcription))
			for i, t := range result.Transcription {
				rows[i] = []string{t.Timestamp, t.Source, output.Truncate(t.Text, 80)}
			}
			printTable(headers, rows)
			return nil
		},
	}
}

func sessionsTimelineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "timeline <session-id>",
		Short: "Get the consolidated, time-ordered timeline for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/sessions/timeline/" + url.PathEscape(args[0]))
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var result struct {
				SessionID string `json:"session_id"`
				Events    []struct {
					Kind       string   `json:"kind"`
					Offset     string   `json:"offset"`
					Source     *string  `json:"source"`
					Text       *string  `json:"text"`
					Label      *string  `json:"label"`
					DurationMs *float64 `json:"duration_ms"`
				} `json:"events"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			fmt.Printf("Session: %s\n\n", result.SessionID)
			headers := []string{"OFFSET", "KIND", "SOURCE", "LABEL", "DETAIL"}
			rows := make([][]string, len(result.Events))
			for i, e := range result.Events {
				src := ""
				if e.Source != nil {
					src = *e.Source
				}
				lbl := ""
				if e.Label != nil {
					lbl = *e.Label
				}
				detail := ""
				if e.Text != nil && *e.Text != "" {
					detail = output.Truncate(*e.Text, 60)
				} else if e.DurationMs != nil {
					detail = fmt.Sprintf("%.0f ms", *e.DurationMs)
				}
				rows[i] = []string{e.Offset, e.Kind, src, lbl, detail}
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
			data, _, err := apiClient.Get("/api/v1/sessions/full-summary/" + url.PathEscape(args[0]))
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
			data, _, err := apiClient.Get("/api/v1/sessions/latency/" + url.PathEscape(args[0]))
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
