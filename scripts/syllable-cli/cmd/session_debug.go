package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func sessionDebugCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session-debug",
		Short: "Debug sessions",
		Long:  "Get debug info for sessions by session ID, SID, or tool result.",
	}

	cmd.AddCommand(sessionDebugBySessionIDCmd())
	cmd.AddCommand(sessionDebugBySIDCmd())
	cmd.AddCommand(sessionDebugToolResultCmd())

	return cmd
}

func sessionDebugBySessionIDCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "by-session-id <session-id>",
		Short: "Get debug info by session ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/session_debug/session_id/" + args[0])
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func sessionDebugBySIDCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "by-sid <channel-manager-service> <channel-manager-sid>",
		Short: "Get debug info by channel manager SID",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/session_debug/sid/%s/%s", args[0], args[1])
			data, _, err := apiClient.Get(path)
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func sessionDebugToolResultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tool-result <session-id> <tool-call-id>",
		Short: "Get tool result for a session",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/session_debug/tool_result/%s/%s", args[0], args[1])
			data, _, err := apiClient.Get(path)
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}
