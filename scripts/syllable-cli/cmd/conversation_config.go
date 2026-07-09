package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/asksyllable/syllable-cli/internal/output"
	"github.com/spf13/cobra"
)

// bridgesQuery builds the optional query string shared by the bridge phrases
// GET and PUT endpoints. Both accept an optional agent_id and tool_name to
// scope the config to a specific agent or tool; when neither is set the
// org-level default config is used. agent_id is only sent when the flag was
// explicitly set so an unset flag is not confused with a literal 0.
func bridgesQuery(cmd *cobra.Command, agentID int, toolName string) string {
	params := url.Values{}
	if cmd.Flags().Changed("agent-id") {
		params.Set("agent_id", strconv.Itoa(agentID))
	}
	if toolName != "" {
		params.Set("tool_name", toolName)
	}
	if len(params) == 0 {
		return ""
	}
	return "?" + params.Encode()
}

func conversationConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conversation-config",
		Short: "Manage conversation configuration (bridge phrases)",
		Long: `Manage conversation configuration.

Bridge phrases — the Console calls these "Voices → Phrases" — are the filler
messages an agent speaks while it is delayed or a tool call is in progress.
Set a unified ordered list via "messages" (with optional "randomize_messages"
for no-repeat shuffling); when "messages" is empty the legacy fields are used
(first_slow_messages, very_slow_messages, tool_responses). Per-language
overrides are also supported.`,
		Example: `  # Get the current bridge phrases configuration
  syllable conversation-config bridges

  # Get bridge phrases configuration as JSON
  syllable conversation-config bridges --output json

  # Get the config scoped to a specific agent or tool
  syllable conversation-config bridges --agent-id 42
  syllable conversation-config bridges --tool-name transfer_call

  # Update the bridge phrases configuration from a JSON file
  syllable conversation-config bridges-update --file bridges.json

  # Update the config scoped to a specific agent
  syllable conversation-config bridges-update --agent-id 42 --file bridges.json`,
	}

	cmd.AddCommand(conversationConfigBridgesGetCmd())
	cmd.AddCommand(conversationConfigBridgesUpdateCmd())

	return cmd
}

func conversationConfigBridgesGetCmd() *cobra.Command {
	var agentID int
	var toolName string

	cmd := &cobra.Command{
		Use:   "bridges",
		Short: "Get bridge phrases configuration (Console: Voices → Phrases)",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/conversation-config/bridges" + bridgesQuery(cmd, agentID, toolName)
			data, _, err := apiClient.Get(path)
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().IntVar(&agentID, "agent-id", 0, "Agent ID to fetch config for")
	cmd.Flags().StringVar(&toolName, "tool-name", "", "Tool name to fetch config for")
	return cmd
}

func conversationConfigBridgesUpdateCmd() *cobra.Command {
	var file string
	var agentID int
	var toolName string

	cmd := &cobra.Command{
		Use:   "bridges-update",
		Short: "Update bridge phrases configuration (Console: Voices → Phrases)",
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
				return fmt.Errorf("use --file to provide bridges update body")
			}

			path := "/api/v1/conversation-config/bridges" + bridgesQuery(cmd, agentID, toolName)
			data, _, err := apiClient.Put(path, body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().IntVar(&agentID, "agent-id", 0, "Agent ID to update config for")
	cmd.Flags().StringVar(&toolName, "tool-name", "", "Tool name to update config for")
	return cmd
}
