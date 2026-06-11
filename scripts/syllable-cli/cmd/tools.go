package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/client"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func toolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Manage tools",
		Long:  "List, get, create, update, and delete tools.",
		Example: `  # List all tools
  syllable tools list

  # Get a tool as JSON (inspect full config) — tools are name-keyed
  syllable tools get my_tool --output json

  # Numeric IDs from the list's ID column are resolved to the tool's name
  syllable tools get 425

  # Create a tool from a JSON file
  syllable tools create --file tool.json

  # Update a tool
  syllable tools update my_tool --file tool.json

  # Delete a tool
  syllable tools delete my_tool`,
	}

	cmd.AddCommand(toolsListCmd())
	cmd.AddCommand(toolsGetCmd())
	cmd.AddCommand(toolsCreateCmd())
	cmd.AddCommand(toolsUpdateCmd())
	cmd.AddCommand(toolsDeleteCmd())
	cmd.AddCommand(toolsHistoryCmd())

	return cmd
}

func toolsHistoryCmd() *cobra.Command {
	var page, limit int
	var orderByDirection string

	cmd := &cobra.Command{
		Use:   "history <tool-id>",
		Short: "List version history for a tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/tools/%s/history?page=%d&limit=%d", url.PathEscape(args[0]), page, limit)
			if orderByDirection != "" {
				path += "&order_by_direction=" + orderByDirection
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
					VersionNumber json.Number `json:"version_number"`
					Operation     string      `json:"operation"`
					Name          string      `json:"name"`
					UpdatedBy     string      `json:"updated_by"`
					CreatedAt     string      `json:"created_at"`
					Comments      *string     `json:"comments"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"VERSION", "OPERATION", "NAME", "UPDATED_BY", "CREATED_AT", "COMMENTS"}
			rows := make([][]string, len(result.Items))
			for i, h := range result.Items {
				comments := ""
				if h.Comments != nil {
					comments = output.Truncate(*h.Comments, 40)
				}
				rows[i] = []string{
					h.VersionNumber.String(),
					h.Operation,
					h.Name,
					h.UpdatedBy,
					h.CreatedAt,
					comments,
				}
			}
			printTable(headers, rows)
			fmt.Printf("\nTotal: %d\n", result.TotalCount)
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&orderByDirection, "order-by-direction", "", "Sort direction: asc or desc")

	return cmd
}

func toolsListCmd() *cobra.Command {
	var page, limit int
	var search, searchField string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/tools/?page=%d&limit=%d", page, limit)
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
					ServiceName string      `json:"service_name"`
					ServiceID   json.Number `json:"service_id"`
					LastUpdated string `json:"last_updated"`
					LastUpdBy   string `json:"last_updated_by"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "SERVICE", "LAST_UPDATED", "LAST_UPDATED_BY"}
			rows := make([][]string, len(result.Items))
			for i, t := range result.Items {
				rows[i] = []string{
					t.ID.String(),
					t.Name,
					t.ServiceName,
					t.LastUpdated,
					t.LastUpdBy,
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

// resolveToolIDFallback maps a failed by-name lookup to a tool name when the
// argument looks like a numeric ID copied from the `tools list` ID column (#69).
// The tools get endpoint is name-keyed only, so the ID is resolved via the list
// endpoint's id search, with an exact client-side match in case the server
// searches by substring. Returns ok=false when the fallback doesn't apply or
// finds nothing — callers keep the original by-name error.
func resolveToolIDFallback(err error, arg string) (string, bool) {
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 || !isAllDigits(arg) {
		return "", false
	}
	data, _, lerr := apiClient.Get("/api/v1/tools/?page=0&limit=100&search_fields=id&search_field_values=" + url.QueryEscape(arg))
	if lerr != nil {
		return "", false
	}
	var result struct {
		Items []struct {
			ID   json.Number `json:"id"`
			Name string      `json:"name"`
		} `json:"items"`
	}
	if json.Unmarshal(data, &result) != nil {
		return "", false
	}
	for _, item := range result.Items {
		if item.ID.String() == arg {
			return item.Name, true
		}
	}
	return "", false
}

// isAllDigits reports whether s is a non-empty ASCII digit string.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func toolsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <tool-name>",
		Short: "Get a tool by name (numeric IDs are resolved via the list endpoint)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/tools/" + url.PathEscape(args[0]))
			if err != nil {
				// The get endpoint is name-keyed, but `tools list` leads with
				// the ID column — resolve an all-digits arg by ID and retry (#69).
				name, ok := resolveToolIDFallback(err, args[0])
				if !ok {
					return err
				}
				if data, _, err = apiClient.Get("/api/v1/tools/" + url.PathEscape(name)); err != nil {
					return err
				}
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var t struct {
				ID          json.Number `json:"id"`
				Name        string      `json:"name"`
				ServiceName string      `json:"service_name"`
				ServiceID   json.Number `json:"service_id"`
				LastUpdated string `json:"last_updated"`
				LastUpdBy   string `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &t); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", t.ID.String()},
				{"Name", t.Name},
				{"Service Name", t.ServiceName},
				{"Service ID", t.ServiceID.String()},
				{"Last Updated", t.LastUpdated},
				{"Last Updated By", t.LastUpdBy},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func toolsCreateCmd() *cobra.Command {
	var file, name, serviceID string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a tool",
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
				if name == "" || serviceID == "" {
					return fmt.Errorf("required flags: --name, --service-id (or use --file)")
				}
				serviceIDInt, err := parseIDFlag("service-id", serviceID)
				if err != nil {
					return err
				}
				body = map[string]interface{}{
					"name":       name,
					"service_id": serviceIDInt,
					"definition": map[string]interface{}{},
				}
			}

			data, _, err := apiClient.Post("/api/v1/tools/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Tool name")
	cmd.Flags().StringVar(&serviceID, "service-id", "", "Service ID")

	return cmd
}

func toolsUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <tool-name>",
		Short: "Update a tool",
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

			// The positional is the tool name; the API routes this PUT by the
			// body's numeric id, so the name is validated/injected only (#68).
			if err := ensureBodyIdentifier(body, "name", args[0], false); err != nil {
				return err
			}

			data, _, err := apiClient.Put("/api/v1/tools/", body)
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

func toolsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <tool-name>",
		Short: "Delete a tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := confirmDelete(cmd, args); err != nil {
				return err
			}
			data, _, err := apiClient.Delete("/api/v1/tools/" + url.PathEscape(args[0]))
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Tool %s deleted.\n", args[0])
			}
			return nil
		},
	}
}
