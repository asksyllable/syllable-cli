package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func outboundCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outbound",
		Short: "Manage outbound campaigns and batches",
		Example: `  # List all outbound batches
  syllable outbound batches list

  # Get a specific batch
  syllable outbound batches get abc-123

  # Create a batch from a JSON file
  syllable outbound batches create --file batch.json

  # Get results for a batch
  syllable outbound batches results abc-123

  # Get requests in a batch
  syllable outbound batches requests abc-123

  # Remove requests from a batch
  syllable outbound batches remove-requests abc-123 --file request-ids.json

  # List all outbound campaigns
  syllable outbound campaigns list

  # Create a campaign from a JSON file
  syllable outbound campaigns create --file campaign.json`,
	}

	cmd.AddCommand(outboundBatchesCmd())
	cmd.AddCommand(outboundCampaignsCmd())

	return cmd
}

// ── Batches ──────────────────────────────────────────────────────────────────

func outboundBatchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batches",
		Short: "Manage outbound batches",
	}

	cmd.AddCommand(outboundBatchesListCmd())
	cmd.AddCommand(outboundBatchesGetCmd())
	cmd.AddCommand(outboundBatchesCreateCmd())
	cmd.AddCommand(outboundBatchesUpdateCmd())
	cmd.AddCommand(outboundBatchesDeleteCmd())
	cmd.AddCommand(outboundBatchesResultsCmd())
	cmd.AddCommand(outboundBatchesRequestsCmd())
	cmd.AddCommand(outboundBatchesRemoveRequestsCmd())

	return cmd
}

func outboundBatchesListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List outbound batches",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/outbound/batches?page=%d&limit=%d", page, limit)
			if search != "" {
				path += fmt.Sprintf("&search_fields=batch_id&search_field_values=%s", search)
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
					BatchID    string      `json:"batch_id"`
					CampaignID json.Number `json:"campaign_id"`
					Status     string `json:"status"`
					Paused     bool   `json:"paused"`
					ExpiresOn  string `json:"expires_on"`
					CreatedAt  string `json:"created_at"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"BATCH_ID", "CAMPAIGN_ID", "STATUS", "PAUSED", "EXPIRES_ON", "CREATED_AT"}
			rows := make([][]string, len(result.Items))
			for i, b := range result.Items {
				paused := "no"
				if b.Paused {
					paused = "yes"
				}
				rows[i] = []string{
					b.BatchID,
					b.CampaignID.String(),
					b.Status,
					paused,
					b.ExpiresOn,
					b.CreatedAt,
				}
			}
			printTable(headers, rows)
			fmt.Printf("\nTotal: %d\n", result.TotalCount)
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&search, "search", "", "Search by batch ID")

	return cmd
}

func outboundBatchesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <batch-id>",
		Short: "Get an outbound batch by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/outbound/batches/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var b struct {
				BatchID       string      `json:"batch_id"`
				CampaignID    json.Number `json:"campaign_id"`
				Status        string      `json:"status"`
				Paused        bool        `json:"paused"`
				DispatchID    string      `json:"dispatch_id"`
				ExpiresOn     string      `json:"expires_on"`
				CreatedAt     string      `json:"created_at"`
				LastUpdatedAt string      `json:"last_updated_at"`
				LastUpdatedBy string      `json:"last_updated_by"`
				ErrorMessage  string      `json:"error_message"`
			}
			if err := json.Unmarshal(data, &b); err != nil {
				output.PrintJSON(data)
				return nil
			}

			paused := "no"
			if b.Paused {
				paused = "yes"
			}
			rows := [][]string{
				{"Batch ID", b.BatchID},
				{"Campaign ID", b.CampaignID.String()},
				{"Status", b.Status},
				{"Paused", paused},
				{"Dispatch ID", b.DispatchID},
				{"Expires On", b.ExpiresOn},
				{"Created At", b.CreatedAt},
				{"Last Updated At", b.LastUpdatedAt},
				{"Last Updated By", b.LastUpdatedBy},
				{"Error Message", b.ErrorMessage},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func outboundBatchesCreateCmd() *cobra.Command {
	var file, batchID, campaignID string
	var paused bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an outbound batch",
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
				if batchID == "" || campaignID == "" {
					return fmt.Errorf("required flags: --batch-id, --campaign-id (or use --file)")
				}
				body = map[string]interface{}{
					"batch_id":    batchID,
					"campaign_id": campaignID,
					"paused":      paused,
				}
			}

			data, _, err := apiClient.Post("/api/v1/outbound/batches", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&batchID, "batch-id", "", "Batch ID")
	cmd.Flags().StringVar(&campaignID, "campaign-id", "", "Campaign ID")
	cmd.Flags().BoolVar(&paused, "paused", false, "Start batch paused")

	return cmd
}

func outboundBatchesUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <batch-id>",
		Short: "Update an outbound batch",
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

			data, _, err := apiClient.Put("/api/v1/outbound/batches/"+args[0], body)
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

func outboundBatchesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <batch-id>",
		Short: "Delete an outbound batch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/outbound/batches/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Batch %s deleted.\n", args[0])
			}
			return nil
		},
	}
}

func outboundBatchesResultsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "results <batch-id>",
		Short: "Get results for an outbound batch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/outbound/batches/" + args[0] + "/results")
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func outboundBatchesRequestsCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "add-requests <batch-id>",
		Short: "Add requests to an outbound batch",
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
				return fmt.Errorf("use --file to provide requests body")
			}

			data, _, err := apiClient.Post("/api/v1/outbound/batches/"+args[0]+"/requests", body)
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

func outboundBatchesRemoveRequestsCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "remove-requests <batch-id>",
		Short: "Remove requests from an outbound batch",
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
				return fmt.Errorf("use --file to provide requests body")
			}

			data, _, err := apiClient.Post("/api/v1/outbound/batches/"+args[0]+"/remove-requests", body)
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

// ── Campaigns ────────────────────────────────────────────────────────────────

func outboundCampaignsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "campaigns",
		Short: "Manage outbound campaigns",
	}

	cmd.AddCommand(outboundCampaignsListCmd())
	cmd.AddCommand(outboundCampaignsGetCmd())
	cmd.AddCommand(outboundCampaignsCreateCmd())
	cmd.AddCommand(outboundCampaignsUpdateCmd())
	cmd.AddCommand(outboundCampaignsDeleteCmd())

	return cmd
}

func outboundCampaignsListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List outbound campaigns",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/outbound/campaigns?page=%d&limit=%d", page, limit)
			if search != "" {
				path += fmt.Sprintf("&search_fields=campaign_name&search_field_values=%s", search)
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
					ID           json.Number `json:"id"`
					CampaignName string      `json:"campaign_name"`
					Description  string `json:"description"`
					Mode         string `json:"mode"`
					CallerID     string `json:"caller_id"`
					UpdatedAt    string `json:"updated_at"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "MODE", "CALLER_ID", "DESCRIPTION", "UPDATED"}
			rows := make([][]string, len(result.Items))
			for i, c := range result.Items {
				rows[i] = []string{
					c.ID.String(),
					c.CampaignName,
					c.Mode,
					c.CallerID,
					output.Truncate(c.Description, 40),
					c.UpdatedAt,
				}
			}
			printTable(headers, rows)
			fmt.Printf("\nTotal: %d\n", result.TotalCount)
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&search, "search", "", "Search by campaign name")

	return cmd
}

func outboundCampaignsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <campaign-id>",
		Short: "Get an outbound campaign by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/outbound/campaigns/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var c struct {
				ID           json.Number `json:"id"`
				CampaignName string `json:"campaign_name"`
				Description  string `json:"description"`
				Mode         string `json:"mode"`
				CallerID     string `json:"caller_id"`
				Source       string `json:"source"`
				HourlyRate   int    `json:"hourly_rate"`
				RetryCount   int    `json:"retry_count"`
				UpdatedAt    string `json:"updated_at"`
				LastUpdBy    string `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &c); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", c.ID.String()},
				{"Campaign Name", c.CampaignName},
				{"Description", c.Description},
				{"Mode", c.Mode},
				{"Caller ID", c.CallerID},
				{"Source", c.Source},
				{"Hourly Rate", fmt.Sprintf("%d", c.HourlyRate)},
				{"Retry Count", fmt.Sprintf("%d", c.RetryCount)},
				{"Updated At", c.UpdatedAt},
				{"Last Updated By", c.LastUpdBy},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func outboundCampaignsCreateCmd() *cobra.Command {
	var file, name, callerID string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an outbound campaign",
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
				if name == "" || callerID == "" {
					return fmt.Errorf("required flags: --name, --caller-id (or use --file)")
				}
				body = map[string]interface{}{
					"campaign_name":      name,
					"caller_id":          callerID,
					"campaign_variables": map[string]interface{}{},
					"active_days":        []string{},
				}
			}

			data, _, err := apiClient.Post("/api/v1/outbound/campaigns", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Campaign name")
	cmd.Flags().StringVar(&callerID, "caller-id", "", "Caller ID (phone number)")

	return cmd
}

func outboundCampaignsUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <campaign-id>",
		Short: "Update an outbound campaign",
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

			data, _, err := apiClient.Put("/api/v1/outbound/campaigns/"+args[0], body)
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

func outboundCampaignsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <campaign-id>",
		Short: "Delete an outbound campaign",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/outbound/campaigns/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Campaign %s deleted.\n", args[0])
			}
			return nil
		},
	}
}
