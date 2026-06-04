package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

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
	cmd.AddCommand(agentsSendTestMessageCmd())
	cmd.AddCommand(agentsVoicesCmd())
	cmd.AddCommand(agentsLabelsCmd())

	return cmd
}

func agentsLabelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "labels",
		Short: "List active agent labels",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/agents/labels")
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var labels []string
			if err := json.Unmarshal(data, &labels); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := make([][]string, len(labels))
			for i, l := range labels {
				rows[i] = []string{l}
			}
			printTable([]string{"LABEL"}, rows)
			fmt.Printf("\nTotal: %d\n", len(labels))
			return nil
		},
	}
}

func agentsVoicesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "voices",
		Short: "List available agent voices",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/agents/voices/available")
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var voices []struct {
				Provider    string `json:"provider"`
				DisplayName string `json:"display_name"`
				Gender      string `json:"gender"`
				Model       string `json:"model"`
			}
			if err := json.Unmarshal(data, &voices); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"PROVIDER", "DISPLAY_NAME", "GENDER", "MODEL"}
			rows := make([][]string, len(voices))
			for i, v := range voices {
				rows[i] = []string{v.Provider, v.DisplayName, v.Gender, v.Model}
			}
			printTable(headers, rows)
			fmt.Printf("\nTotal: %d\n", len(voices))
			return nil
		},
	}
}

func agentsListCmd() *cobra.Command {
	var page, limit int
	var search, searchField string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/agents/?page=%d&limit=%d", page, limit)
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
	cmd.Flags().StringVar(&searchField, "search-field", "name", "Field to search on (see API docs for valid values)")

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

			// The API routes this PUT by the id inside the body (#68).
			if err := ensureBodyIdentifier(body, "id", args[0], true); err != nil {
				return err
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

func agentsSendTestMessageCmd() *cobra.Command {
	var testID, text, serviceName, source, overrideTimestamp string
	var sessionStart bool
	var timeout int

	cmd := &cobra.Command{
		Use:   "send-test-message <agent-id>",
		Short: "Send a test message to an agent via the conversation test API",
		Long: `Send a test message to an agent using the conversation test API.

This calls POST /api/v1/agents/test/messages to exchange a single turn with
the agent under test. Use --session-start on the first call to begin a new
test session, then omit it for follow-up turns in the same session (reuse
the same --test-id).`,
		Example: `  # Start a new test session
  syllable agents send-test-message 42 --test-id my-test-1 --session-start --text "Hi, I need help"

  # Send a follow-up message in the same session
  syllable agents send-test-message 42 --test-id my-test-1 --text "Yes, that's correct"

  # Start a session with no caller text (agent speaks first)
  syllable agents send-test-message 42 --test-id my-test-1 --session-start

  # Pin the agent's wall clock for deterministic date-sensitive tests (timezone-naive ISO 8601, interpreted in the agent's timezone)
  syllable agents send-test-message 42 --test-id my-test-1 --session-start --override-timestamp "2030-12-25T09:30:00" --text "I need to schedule an appointment"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID := args[0]

			body := map[string]interface{}{
				"agent_id":      agentID,
				"test_id":       testID,
				"service_name":  serviceName,
				"source":        source,
				"session_start": sessionStart,
			}
			if cmd.Flags().Changed("text") {
				body["text"] = text
			} else if sessionStart {
				body["text"] = ""
			}
			if cmd.Flags().Changed("override-timestamp") {
				body["override_timestamp"] = overrideTimestamp
			}

			data, _, err := apiClient.PostWithTimeout(
				"/api/v1/agents/test/messages",
				body,
				time.Duration(timeout)*time.Second,
			)
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var resp struct {
				TestID       string          `json:"test_id"`
				AgentID      string          `json:"agent_id"`
				ResponseText string          `json:"response_text"`
				Text         string          `json:"text"`
				Response     json.RawMessage `json:"response"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"Test ID", resp.TestID},
				{"Agent ID", resp.AgentID},
				{"Response Text", resp.ResponseText},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}

	cmd.Flags().StringVar(&testID, "test-id", "", "Test session identifier (required)")
	cmd.Flags().StringVar(&text, "text", "", "Message text to send to the agent")
	cmd.Flags().StringVar(&overrideTimestamp, "override-timestamp", "", "Timezone-naive ISO 8601 timestamp (e.g. 2030-12-25T09:30:00) to pin the agent's wall clock for this turn, interpreted in the agent's timezone. Offset (-07:00), 'Z', and space-separated forms are silently ignored by the server. Omit to use current time.")
	cmd.Flags().BoolVar(&sessionStart, "session-start", false, "Start a new test session")
	cmd.Flags().StringVar(&serviceName, "service-name", "test", "Service name for the test")
	cmd.Flags().StringVar(&source, "source", "tester@syllable.ai", "Source identifier for the test caller")
	cmd.Flags().IntVar(&timeout, "timeout", 90, "Request timeout in seconds")
	_ = cmd.MarkFlagRequired("test-id")

	return cmd
}
