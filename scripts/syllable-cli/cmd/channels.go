package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func channelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "Manage channels",
		Long:  "List, get, create, update, and delete channels.",
	}

	cmd.AddCommand(channelsListCmd())
	cmd.AddCommand(channelsCreateCmd())
	cmd.AddCommand(channelsUpdateCmd())
	cmd.AddCommand(channelsDeleteCmd())
	cmd.AddCommand(channelsTargetsCmd())
	cmd.AddCommand(channelsAvailableTargetsCmd())
	cmd.AddCommand(channelsTwilioCmd())

	return cmd
}

func channelsListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List channels",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/channels/?page=%d&limit=%d", page, limit)
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
					ID             json.Number `json:"id"`
					Name           string      `json:"name"`
					ChannelService string      `json:"channel_service"`
					IsSystem       bool        `json:"is_system_channel"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "SERVICE", "IS_SYSTEM"}
			rows := make([][]string, len(result.Items))
			for i, c := range result.Items {
				isSystem := "no"
				if c.IsSystem {
					isSystem = "yes"
				}
				rows[i] = []string{c.ID.String(), c.Name, c.ChannelService, isSystem}
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

func channelsCreateCmd() *cobra.Command {
	var file, name, service string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a channel",
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
				if name == "" || service == "" {
					return fmt.Errorf("required flags: --name, --service (or use --file)")
				}
				body = map[string]interface{}{
					"name":            name,
					"channel_service": service,
					"config":          map[string]interface{}{},
				}
			}

			data, _, err := apiClient.Post("/api/v1/channels/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Channel name")
	cmd.Flags().StringVar(&service, "service", "", "Channel service")

	return cmd
}

func channelsUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <channel-id>",
		Short: "Update a channel",
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

			data, _, err := apiClient.Put("/api/v1/channels/"+args[0], body)
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

func channelsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <channel-id>",
		Short: "Delete a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/channels/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Channel %s deleted.\n", args[0])
			}
			return nil
		},
	}
}

func channelsTargetsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "targets",
		Short: "Manage channel targets",
	}

	cmd.AddCommand(channelsTargetsListCmd())
	cmd.AddCommand(channelsTargetsGetCmd())
	cmd.AddCommand(channelsTargetsCreateCmd())
	cmd.AddCommand(channelsTargetsUpdateCmd())
	cmd.AddCommand(channelsTargetsDeleteCmd())

	return cmd
}

func channelsTargetsListCmd() *cobra.Command {
	var page, limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all channel targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/channels/targets?page=%d&limit=%d", page, limit)

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
					AgentID     json.Number `json:"agent_id"`
					ChannelID   json.Number `json:"channel_id"`
					ChannelName string `json:"channel_name"`
					Target      string `json:"target"`
					TargetMode  string `json:"target_mode"`
					IsTest      bool   `json:"is_test"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "CHANNEL", "TARGET", "MODE", "AGENT_ID", "IS_TEST"}
			rows := make([][]string, len(result.Items))
			for i, t := range result.Items {
				isTest := "no"
				if t.IsTest {
					isTest = "yes"
				}
				rows[i] = []string{t.ID.String(), t.ChannelName, t.Target, t.TargetMode, t.AgentID.String(), isTest}
			}
			output.PrintTable(headers, rows)
			fmt.Printf("\nTotal: %d\n", result.TotalCount)
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")

	return cmd
}

func channelsTargetsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <channel-id> <target-id>",
		Short: "Get a channel target",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/channels/%s/targets/%s", args[0], args[1])
			data, _, err := apiClient.Get(path)
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func channelsTargetsCreateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "create <channel-id>",
		Short: "Create a channel target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}

			if file != "" {
				fileData, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(fileData, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide target body")
			}

			path := fmt.Sprintf("/api/v1/channels/%s/targets", args[0])
			data, _, err := apiClient.Post(path, body)
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

func channelsTargetsUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <channel-id> <target-id>",
		Short: "Update a channel target",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}

			if file != "" {
				fileData, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(fileData, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide update body")
			}

			path := fmt.Sprintf("/api/v1/channels/%s/targets/%s", args[0], args[1])
			data, _, err := apiClient.Put(path, body)
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

func channelsTargetsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <channel-id> <target-id>",
		Short: "Delete a channel target",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/channels/%s/targets/%s", args[0], args[1])
			data, _, err := apiClient.Delete(path)
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Channel target %s deleted.\n", args[1])
			}
			return nil
		},
	}
}

func channelsAvailableTargetsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "available-targets",
		Short: "List available channel targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/channels/available-targets")
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

// ── Twilio ───────────────────────────────────────────────────────────────────

func channelsTwilioCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "twilio",
		Short: "Manage Twilio channels",
	}

	cmd.AddCommand(channelsTwilioGetCmd())
	cmd.AddCommand(channelsTwilioCreateCmd())
	cmd.AddCommand(channelsTwilioUpdateCmd())
	cmd.AddCommand(channelsTwilioNumbersListCmd())
	cmd.AddCommand(channelsTwilioNumbersAddCmd())
	cmd.AddCommand(channelsTwilioNumbersUpdateCmd())

	return cmd
}

func channelsTwilioGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <channel-id>",
		Short: "Get a Twilio channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/channels/twilio/" + args[0])
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func channelsTwilioCreateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Twilio channel",
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}

			if file != "" {
				fileData, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(fileData, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide Twilio channel body")
			}

			data, _, err := apiClient.Post("/api/v1/channels/twilio/", body)
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

func channelsTwilioUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a Twilio channel",
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}

			if file != "" {
				fileData, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(fileData, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide update body")
			}

			data, _, err := apiClient.Put("/api/v1/channels/twilio/", body)
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

func channelsTwilioNumbersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "numbers-list <channel-id>",
		Short: "List phone numbers for a Twilio channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/channels/twilio/" + args[0] + "/numbers")
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func channelsTwilioNumbersAddCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "numbers-add <channel-id>",
		Short: "Add phone numbers to a Twilio channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}

			if file != "" {
				fileData, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(fileData, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide numbers body")
			}

			data, _, err := apiClient.Post("/api/v1/channels/twilio/"+args[0]+"/numbers", body)
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

func channelsTwilioNumbersUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "numbers-update <channel-id>",
		Short: "Update phone numbers for a Twilio channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}

			if file != "" {
				fileData, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(fileData, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide update body")
			}

			data, _, err := apiClient.Put("/api/v1/channels/twilio/"+args[0]+"/numbers", body)
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
