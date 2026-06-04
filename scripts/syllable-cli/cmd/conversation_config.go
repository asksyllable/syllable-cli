package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func conversationConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conversation-config",
		Short: "Manage conversation configuration (bridge phrases)",
		Long: `Manage conversation configuration.

Bridge phrases — the Console calls these "Voices → Phrases" — are the filler
messages an agent speaks while it is delayed or a tool call is in progress
(first_slow_messages, very_slow_messages, tool_responses), with optional
per-language overrides.`,
		Example: `  # Get the current bridge phrases configuration
  syllable conversation-config bridges

  # Get bridge phrases configuration as JSON
  syllable conversation-config bridges --output json

  # Update the bridge phrases configuration from a JSON file
  syllable conversation-config bridges-update --file bridges.json`,
	}

	cmd.AddCommand(conversationConfigBridgesGetCmd())
	cmd.AddCommand(conversationConfigBridgesUpdateCmd())

	return cmd
}

func conversationConfigBridgesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bridges",
		Short: "Get bridge phrases configuration (Console: Voices → Phrases)",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/conversation-config/bridges")
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func conversationConfigBridgesUpdateCmd() *cobra.Command {
	var file string

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

			data, _, err := apiClient.Put("/api/v1/conversation-config/bridges", body)
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
