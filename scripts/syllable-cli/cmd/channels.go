package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func channelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "Manage channels",
		Long:  "List, get, create, update, and delete channels.",
		Example: `  # List all channels
  syllable channels list

  # Get a single channel by ID
  syllable channels get 5

  # Search channels by name
  syllable channels list --search "support"

  # Create a channel from a JSON file
  syllable channels create --file channel.json

  # Update a channel
  syllable channels update 5 --file channel.json

  # Delete a channel
  syllable channels delete 5

  # Get targets for a channel
  syllable channels targets get 5

  # Create a target for a channel
  syllable channels targets create 5 --file target.json

  # List available targets
  syllable channels available-targets

  # Get Twilio configuration for a channel
  syllable channels twilio get 5

  # List Twilio phone numbers for a channel
  syllable channels twilio numbers-list 5`,
	}

	cmd.AddCommand(channelsListCmd())
	cmd.AddCommand(channelsGetCmd())
	cmd.AddCommand(channelsCreateCmd())
	cmd.AddCommand(channelsUpdateCmd())
	cmd.AddCommand(channelsTargetsCmd())
	cmd.AddCommand(channelsAvailableTargetsCmd())
	cmd.AddCommand(channelsTwilioCmd())

	return cmd
}

func channelsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <channel-id>",
		Short: "Get a channel by ID",
		Long: `Get a single channel by ID.

The API has no by-ID GET endpoint for channels (#70), so this resolves the
channel via the list endpoint's id search and prints the matching item — the
same shape as a single item from ` + "`channels list -o json`" + `.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/channels/?page=0&limit=100&search_fields=id&search_field_values=" + url.QueryEscape(args[0])
			data, _, err := apiClient.Get(path)
			if err != nil {
				return err
			}

			var result struct {
				Items []json.RawMessage `json:"items"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parsing channels list response: %w", err)
			}
			// Exact client-side match in case the server searches by substring.
			var match json.RawMessage
			for _, raw := range result.Items {
				var probe struct {
					ID json.Number `json:"id"`
				}
				if json.Unmarshal(raw, &probe) == nil && probe.ID.String() == args[0] {
					match = raw
					break
				}
			}
			if match == nil {
				return fmt.Errorf("channel %s not found — run `syllable channels list` to see available channels", args[0])
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(match)
				return nil
			}

			var c struct {
				ID             json.Number `json:"id"`
				Name           string      `json:"name"`
				ChannelService string      `json:"channel_service"`
				IsSystem       bool        `json:"is_system_channel"`
			}
			if err := json.Unmarshal(match, &c); err != nil {
				output.PrintJSON(match)
				return nil
			}
			isSystem := "no"
			if c.IsSystem {
				isSystem = "yes"
			}
			rows := [][]string{
				{"ID", c.ID.String()},
				{"Name", c.Name},
				{"Service", c.ChannelService},
				{"Is System", isSystem},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func channelsListCmd() *cobra.Command {
	var page, limit int
	var search, searchField string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List channels",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/channels/?page=%d&limit=%d", page, limit)
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

func channelsCreateCmd() *cobra.Command {
	var file, name, service string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a channel",
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

			// PUT exists only on the collection; the API routes by the id inside
			// the body, so reconcile the positional with it (#68, #114).
			if err := ensureBodyIdentifier(body, "id", args[0], true); err != nil {
				return err
			}

			data, _, err := apiClient.Put("/api/v1/channels/", body)
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
	var search, searchField string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all channel targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/channels/targets?page=%d&limit=%d", page, limit)
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
			printTable(headers, rows)
			fmt.Printf("\nTotal: %d\n", result.TotalCount)
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&search, "search", "", "Search value (searches target phone number by default)")
	cmd.Flags().StringVar(&searchField, "search-field", "target", "Field to search on (e.g. target, agent_id)")

	return cmd
}

func channelsTargetsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <channel-id> <target-id>",
		Short: "Get a channel target",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/channels/%s/targets/%s", url.PathEscape(args[0]), url.PathEscape(args[1]))
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
				fileData, err := readFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(fileData, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide target body")
			}

			path := fmt.Sprintf("/api/v1/channels/%s/targets", url.PathEscape(args[0]))
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
				fileData, err := readFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(fileData, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide update body")
			}

			path := fmt.Sprintf("/api/v1/channels/%s/targets/%s", url.PathEscape(args[0]), url.PathEscape(args[1]))
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
			if err := confirmDelete(cmd, args); err != nil {
				return err
			}
			// Delete-target is DELETE /channels/{channel_id}?target_id=<id>; the
			// /targets/{target_id} path only supports GET/PUT (#114). The query
			// string also suppresses the client's auto-reason param, which this
			// operation does not declare.
			path := fmt.Sprintf("/api/v1/channels/%s?target_id=%s",
				url.PathEscape(args[0]), url.QueryEscape(args[1]))
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
	cmd.AddCommand(channelsTwilioNumbersVerifyA2pCmd())

	return cmd
}

func channelsTwilioNumbersVerifyA2pCmd() *cobra.Command {
	var phone string

	cmd := &cobra.Command{
		Use:   "numbers-verify-a2p-compliance <channel-id>",
		Short: "Check US A2P / Messaging Service compliance for a Twilio number",
		Long: `Check whether a number on a Twilio channel is on a Messaging Service with an
approved US A2P brand and verified campaign. Reflects Twilio configuration, not
carrier per-number registration or legal/content compliance.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if phone == "" {
				return fmt.Errorf("required flag: --phone")
			}

			body := map[string]interface{}{"phone": phone}
			path := fmt.Sprintf("/api/v1/channels/twilio/%s/numbers/verify-a2p-compliance", url.PathEscape(args[0]))
			data, _, err := apiClient.Post(path, body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&phone, "phone", "", "E.164 phone number exactly as Twilio stores it, e.g. +18042221111 (required)")
	_ = cmd.MarkFlagRequired("phone")
	return cmd
}

func channelsTwilioGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <channel-id>",
		Short: "Get a Twilio channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/channels/twilio/" + url.PathEscape(args[0]))
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
				fileData, err := readFile(file)
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
				fileData, err := readFile(file)
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
			data, _, err := apiClient.Get("/api/v1/channels/twilio/" + url.PathEscape(args[0]) + "/numbers")
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
				fileData, err := readFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(fileData, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide numbers body")
			}

			data, _, err := apiClient.Post("/api/v1/channels/twilio/"+ url.PathEscape(args[0]) +"/numbers", body)
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
				fileData, err := readFile(file)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				if err := json.Unmarshal(fileData, &body); err != nil {
					return fmt.Errorf("parsing JSON file: %w", err)
				}
			} else {
				return fmt.Errorf("use --file to provide update body")
			}

			data, _, err := apiClient.Put("/api/v1/channels/twilio/"+ url.PathEscape(args[0]) +"/numbers", body)
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
