package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func sessionLabelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session-labels",
		Short: "Manage session labels",
		Long:  "List, get, and create session labels.",
		Example: `  # List all session labels
  syllable session-labels list

  # Get a specific session label
  syllable session-labels get 9

  # Get a session label as JSON
  syllable session-labels get 9 --output json

  # Create a session label from a JSON file
  syllable session-labels create --file label.json`,
	}

	cmd.AddCommand(sessionLabelsListCmd())
	cmd.AddCommand(sessionLabelsGetCmd())
	cmd.AddCommand(sessionLabelsCreateCmd())

	return cmd
}

func sessionLabelsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List session labels",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/session_labels/")
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var result struct {
				Items []struct {
					ID        json.Number `json:"id"`
					SessionID json.Number `json:"session_id"`
					Type      string      `json:"type"`
					Code      string      `json:"code"`
					UserEmail string      `json:"user_email"`
					Comments  string      `json:"comments"`
					Timestamp string      `json:"timestamp"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "SESSION_ID", "TYPE", "CODE", "USER", "COMMENTS", "TIMESTAMP"}
			rows := make([][]string, len(result.Items))
			for i, l := range result.Items {
				rows[i] = []string{l.ID.String(), l.SessionID.String(), l.Type, l.Code, l.UserEmail, output.Truncate(l.Comments, 30), l.Timestamp}
			}
			printTable(headers, rows)
			fmt.Printf("\nTotal: %d\n", result.TotalCount)
			return nil
		},
	}
}

func sessionLabelsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <label-id>",
		Short: "Get a session label by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/session_labels/" + args[0])
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func sessionLabelsCreateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a session label",
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
				return fmt.Errorf("use --file to provide label body")
			}

			data, _, err := apiClient.Post("/api/v1/session_labels/", body)
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
