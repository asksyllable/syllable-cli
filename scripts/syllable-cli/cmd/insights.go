package cmd

import (
	"encoding/json"
	"fmt"

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

  # List tool configurations
  syllable insights tool-configs list

  # List tool definitions
  syllable insights tool-definitions`,
	}

	cmd.AddCommand(insightsWorkflowsCmd())
	cmd.AddCommand(insightsFoldersCmd())
	cmd.AddCommand(insightsToolConfigsCmd())
	cmd.AddCommand(insightsToolDefinitionsCmd())

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
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List insight workflows",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/insights/workflows/?page=%d&limit=%d", page, limit)
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
	var file string

	cmd := &cobra.Command{
		Use:   "activate <workflow-id>",
		Short: "Activate an insight workflow",
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
				body = map[string]interface{}{
					"is_acknowledged": true,
				}
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

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	return cmd
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
	return &cobra.Command{
		Use:   "list",
		Short: "List insight tool configurations",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/insights/tool-configurations")
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
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
