package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func insightsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Manage insights",
		Example: `  # List all insight workflows
  syllable insights workflows list

  # Get a specific workflow
  syllable insights workflows get 8

  # Create a workflow from a JSON file
  syllable insights workflows create --file workflow.json

  # Activate a workflow
  syllable insights workflows activate 8

  # Deactivate a workflow
  syllable insights workflows inactivate 8

  # List insight folders
  syllable insights folders list

  # Upload a file to an insight folder
  syllable insights folders upload-file 42 --file /path/to/recording.mp3

  # List tool configurations
  syllable insights tool-configs list

  # List tool definitions
  syllable insights tool-definitions`,
	}

	cmd.AddCommand(insightsListCmd())
	cmd.AddCommand(insightsWorkflowsCmd())
	cmd.AddCommand(insightsFoldersCmd())
	cmd.AddCommand(insightsToolConfigsCmd())
	cmd.AddCommand(insightsToolDefinitionsCmd())

	return cmd
}

// ── Insights Results ──────────────────────────────────────────────────────────

func insightsListCmd() *cobra.Command {
	var page, limit int
	var folderID, workflowID, uploadFileID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List insight results",
		Example: `  # List all insight results
  syllable insights list

  # Filter by folder
  syllable insights list --folder-id 124

  # Filter by workflow
  syllable insights list --workflow-id 263`,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/insights/?page=%d&limit=%d", page, limit)

			if folderID != "" {
				path += "&search_fields=upload_folder_id&search_field_values=" + url.QueryEscape(folderID)
			} else if workflowID != "" {
				path += "&search_fields=workflow_id&search_field_values=" + url.QueryEscape(workflowID)
			} else if uploadFileID != "" {
				path += "&search_fields=upload_file_id&search_field_values=" + url.QueryEscape(uploadFileID)
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
					ID            json.Number     `json:"id"`
					UploadFileID  *json.Number    `json:"upload_file_id"`
					InsightToolID json.Number     `json:"insight_tool_id"`
					InsightKey    string          `json:"insight_key"`
					JsonValue     json.RawMessage `json:"json_value"`
					CreatedAt     string          `json:"created_at"`
				} `json:"items"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "UPLOAD_FILE_ID", "TOOL_ID", "KEY", "CREATED_AT"}
			rows := make([][]string, len(result.Items))
			for i, r := range result.Items {
				fileID := ""
				if r.UploadFileID != nil {
					fileID = r.UploadFileID.String()
				}
				rows[i] = []string{r.ID.String(), fileID, r.InsightToolID.String(), r.InsightKey, r.CreatedAt}
			}
			printTable(headers, rows)
			fmt.Printf("\nShowing %d results\n", len(result.Items))
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&folderID, "folder-id", "", "Filter by upload folder ID")
	cmd.Flags().StringVar(&workflowID, "workflow-id", "", "Filter by workflow ID")
	cmd.Flags().StringVar(&uploadFileID, "upload-file-id", "", "Filter by upload file ID")
	return cmd
}

// ── Workflows ─────────────────────────────────────────────────────────────────

func insightsWorkflowsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflows",
		Short: "Manage insight workflows",
	}

	cmd.AddCommand(insightsWorkflowsListCmd())
	cmd.AddCommand(insightsWorkflowsGetCmd())
	cmd.AddCommand(insightsWorkflowsCreateCmd())
	cmd.AddCommand(insightsWorkflowsUpdateCmd())
	cmd.AddCommand(insightsWorkflowsDeleteCmd())
	cmd.AddCommand(insightsWorkflowsActivateCmd())
	cmd.AddCommand(insightsWorkflowsInactivateCmd())

	return cmd
}

func insightsWorkflowsListCmd() *cobra.Command {
	var page, limit int
	var search, searchField string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List insight workflows",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/insights/workflows/?page=%d&limit=%d", page, limit)
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
					Name        string      `json:"name"`
					Description string      `json:"description"`
					Source      string      `json:"source"`
					Status      string      `json:"status"`
					CreatedAt   string `json:"created_at"`
					UpdatedAt   string `json:"updated_at"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "SOURCE", "STATUS", "UPDATED_AT"}
			rows := make([][]string, len(result.Items))
			for i, w := range result.Items {
				rows[i] = []string{w.ID.String(), w.Name, w.Source, w.Status, w.UpdatedAt}
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

func insightsWorkflowsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <workflow-id>",
		Short: "Get an insight workflow by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/insights/workflows/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var w struct {
				ID          json.Number `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Source      string `json:"source"`
				Status      string `json:"status"`
				CreatedAt   string `json:"created_at"`
				UpdatedAt   string `json:"updated_at"`
				LastUpdBy   string `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &w); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", w.ID.String()},
				{"Name", w.Name},
				{"Description", w.Description},
				{"Source", w.Source},
				{"Status", w.Status},
				{"Created At", w.CreatedAt},
				{"Updated At", w.UpdatedAt},
				{"Last Updated By", w.LastUpdBy},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func insightsWorkflowsCreateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an insight workflow",
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
				return fmt.Errorf("use --file to provide workflow body")
			}

			data, _, err := apiClient.Post("/api/v1/insights/workflows/", body)
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

func insightsWorkflowsUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <workflow-id>",
		Short: "Update an insight workflow",
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

			data, _, err := apiClient.Put("/api/v1/insights/workflows/"+args[0], body)
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

func insightsWorkflowsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <workflow-id>",
		Short: "Delete an insight workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/insights/workflows/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Workflow %s deleted.\n", args[0])
			}
			return nil
		},
	}
}

func insightsWorkflowsActivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "activate <workflow-id>",
		Short: "Activate an insight workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fetch the current workflow to get the exact estimate the API requires.
			wfData, _, err := apiClient.Get("/api/v1/insights/workflows/" + args[0])
			if err != nil {
				return fmt.Errorf("fetching workflow: %w", err)
			}
			var wf struct {
				Estimate json.RawMessage `json:"estimate"`
			}
			if err := json.Unmarshal(wfData, &wf); err != nil {
				return fmt.Errorf("parsing workflow: %w", err)
			}

			body := map[string]interface{}{
				"is_acknowledged": true,
				"estimate":        wf.Estimate,
			}

			path := fmt.Sprintf("/api/v1/insights/workflows/%s/activate", args[0])
			data, _, err := apiClient.Put(path, body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}
}

func insightsWorkflowsInactivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inactivate <workflow-id>",
		Short: "Inactivate an insight workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/insights/workflows/%s/inactivate", args[0])
			data, _, err := apiClient.Put(path, map[string]interface{}{})
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}
}

// ── Folders ───────────────────────────────────────────────────────────────────

func insightsFoldersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "folders",
		Short: "Manage insight folders",
	}

	cmd.AddCommand(insightsFoldersListCmd())
	cmd.AddCommand(insightsFoldersGetCmd())
	cmd.AddCommand(insightsFoldersCreateCmd())
	cmd.AddCommand(insightsFoldersUpdateCmd())
	cmd.AddCommand(insightsFoldersDeleteCmd())
	cmd.AddCommand(insightsFoldersFilesCmd())
	cmd.AddCommand(insightsFoldersUploadFileCmd())

	return cmd
}

func insightsFoldersListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List insight folders",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/insights/folders/?page=%d&limit=%d", page, limit)
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
					ID        json.Number `json:"id"`
					Name      string      `json:"name"`
					Label     string      `json:"label"`
					CreatedAt string `json:"created_at"`
					UpdatedAt string `json:"updated_at"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "LABEL", "CREATED_AT", "UPDATED_AT"}
			rows := make([][]string, len(result.Items))
			for i, f := range result.Items {
				rows[i] = []string{f.ID.String(), f.Name, f.Label, f.CreatedAt, f.UpdatedAt}
			}
			printTable(headers, rows)
			fmt.Printf("\nTotal: %d\n", result.TotalCount)
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&search, "search", "", "Search by name")

	return cmd
}

func insightsFoldersGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <folder-id>",
		Short: "Get an insight folder by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/insights/folders/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var f struct {
				ID          json.Number `json:"id"`
				Name        string `json:"name"`
				Label       string `json:"label"`
				Description string `json:"description"`
				CreatedAt   string `json:"created_at"`
				UpdatedAt   string `json:"updated_at"`
				LastUpdBy   string `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &f); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", f.ID.String()},
				{"Name", f.Name},
				{"Label", f.Label},
				{"Description", f.Description},
				{"Created At", f.CreatedAt},
				{"Updated At", f.UpdatedAt},
				{"Last Updated By", f.LastUpdBy},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func insightsFoldersCreateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an insight folder",
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
				return fmt.Errorf("use --file to provide folder body")
			}

			data, _, err := apiClient.Post("/api/v1/insights/folders/", body)
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

func insightsFoldersUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <folder-id>",
		Short: "Update an insight folder",
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

			data, _, err := apiClient.Put("/api/v1/insights/folders/"+args[0], body)
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

func insightsFoldersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <folder-id>",
		Short: "Delete an insight folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/insights/folders/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Folder %s deleted.\n", args[0])
			}
			return nil
		},
	}
}

func insightsFoldersFilesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "files <folder-id>",
		Short: "List files in an insight folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/insights/folders/" + args[0] + "/files")
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func insightsFoldersUploadFileCmd() *cobra.Command {
	var filePath, callID, agentNumber, customerNumber, startTime, endTime, metadata string
	var duration float64

	cmd := &cobra.Command{
		Use:   "upload-file <folder-id>",
		Short: "Upload a file to an insight folder",
		Args:  cobra.ExactArgs(1),
		Example: `  # Upload a recording with a call ID
  syllable insights folders upload-file 42 --file /path/to/recording.mp3 --call-id my-call-001

  # Upload with optional metadata
  syllable insights folders upload-file 42 --file recording.mp3 --call-id call-001 --agent-number +15551234567 --customer-number +15559876543`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				return fmt.Errorf("required flag: --file")
			}
			if callID == "" {
				return fmt.Errorf("required flag: --call-id")
			}

			params := url.Values{}
			params.Set("call_id", callID)
			if agentNumber != "" {
				params.Set("agent_number", agentNumber)
			}
			if customerNumber != "" {
				params.Set("customer_number", customerNumber)
			}
			if startTime != "" {
				params.Set("start_time", startTime)
			}
			if endTime != "" {
				params.Set("end_time", endTime)
			}
			if duration > 0 {
				params.Set("duration", fmt.Sprintf("%g", duration))
			}
			if metadata != "" {
				params.Set("metadata", metadata)
			}

			path := fmt.Sprintf("/api/v1/insights/folders/%s/upload-file?%s", args[0], params.Encode())
			data, _, err := apiClient.PostMultipart(path, "file", filePath)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "Path to local file to upload")
	cmd.Flags().StringVar(&callID, "call-id", "", "Unique identifier for the call (required)")
	cmd.Flags().StringVar(&agentNumber, "agent-number", "", "Phone number or ID of the agent")
	cmd.Flags().StringVar(&customerNumber, "customer-number", "", "Phone number or ID of the customer")
	cmd.Flags().StringVar(&startTime, "start-time", "", "Call start timestamp (ISO 8601)")
	cmd.Flags().StringVar(&endTime, "end-time", "", "Call end timestamp (ISO 8601)")
	cmd.Flags().Float64Var(&duration, "duration", 0, "Call duration in seconds")
	cmd.Flags().StringVar(&metadata, "metadata", "", "Additional metadata string")
	return cmd
}

// ── Tool Configurations ──────────────────────────────────────────────────────

func insightsToolConfigsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool-configs",
		Short: "Manage insight tool configurations",
	}

	cmd.AddCommand(insightsToolConfigsListCmd())
	cmd.AddCommand(insightsToolConfigsGetCmd())
	cmd.AddCommand(insightsToolConfigsCreateCmd())
	cmd.AddCommand(insightsToolConfigsUpdateCmd())
	cmd.AddCommand(insightsToolConfigsDeleteCmd())

	return cmd
}

func insightsToolConfigsListCmd() *cobra.Command {
	var page, limit int
	var search, searchField string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List insight tool configurations",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/insights/tool-configurations?page=%d&limit=%d", page, limit)
			if search != "" {
				path += fmt.Sprintf("&search_fields=%s&search_field_values=%s", searchField, url.QueryEscape(search))
			}

			data, _, err := apiClient.Get(path)
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&search, "search", "", "Search by name")
	cmd.Flags().StringVar(&searchField, "search-field", "name", "Field to search on (see API docs for valid values)")

	return cmd
}

func insightsToolConfigsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <tool-id>",
		Short: "Get an insight tool configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/insights/tool-configurations/" + args[0])
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func insightsToolConfigsCreateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an insight tool configuration",
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
				return fmt.Errorf("use --file to provide tool config body")
			}

			data, _, err := apiClient.Post("/api/v1/insights/tool-configurations", body)
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

func insightsToolConfigsUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <tool-id>",
		Short: "Update an insight tool configuration",
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

			data, _, err := apiClient.Put("/api/v1/insights/tool-configurations/"+args[0], body)
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

func insightsToolConfigsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <tool-id>",
		Short: "Delete an insight tool configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/insights/tool-configurations/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Tool configuration %s deleted.\n", args[0])
			}
			return nil
		},
	}
}

// ── Tool Definitions ─────────────────────────────────────────────────────────

func insightsToolDefinitionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tool-definitions",
		Short: "List available insight tool definitions",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/insights/tool-definitions")
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}
