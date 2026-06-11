package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/asksyllable/syllable-cli/internal/output"
	"github.com/spf13/cobra"
)

func directoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "directory",
		Short: "Manage directory members",
		Long:  "List, get, create, update, and delete directory members.",
		Example: `  # List all directory members
  syllable directory list

  # Search directory members by name
  syllable directory list --search "billing"

  # Get a specific directory member
  syllable directory get 12

  # Create a directory member from a JSON file
  syllable directory create --file member.json

  # Update a directory member
  syllable directory update 12 --file member.json

  # Delete a directory member
  syllable directory delete 12

  # Bulk-import directory members from a CSV
  syllable directory upload --file members.csv

  # Export all directory members as CSV
  syllable directory download > members.csv

  # Show version history for a directory member
  syllable directory history 12

  # Restore a soft-deleted directory member
  syllable directory restore 12

  # Test extension lookup for a directory member
  syllable directory test 12`,
	}

	cmd.AddCommand(directoryListCmd())
	cmd.AddCommand(directoryGetCmd())
	cmd.AddCommand(directoryCreateCmd())
	cmd.AddCommand(directoryUpdateCmd())
	cmd.AddCommand(directoryDeleteCmd())
	cmd.AddCommand(directoryUploadCmd())
	cmd.AddCommand(directoryDownloadCmd())
	cmd.AddCommand(directoryHistoryCmd())
	cmd.AddCommand(directoryRestoreCmd())
	cmd.AddCommand(directoryTestCmd())

	return cmd
}

func directoryListCmd() *cobra.Command {
	var page, limit int
	var search, searchField string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List directory members",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/directory_members/?page=%d&limit=%d", page, limit)
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
					ID        json.Number `json:"id"`
					Name      string      `json:"name"`
					Type      string      `json:"type"`
					CreatedAt string      `json:"created_at"`
					UpdatedAt string      `json:"updated_at"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "TYPE", "CREATED_AT", "UPDATED_AT"}
			rows := make([][]string, len(result.Items))
			for i, m := range result.Items {
				rows[i] = []string{m.ID.String(), m.Name, m.Type, m.CreatedAt, m.UpdatedAt}
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

func directoryGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <member-id>",
		Short: "Get a directory member by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/directory_members/" + url.PathEscape(args[0]))
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var m struct {
				ID        json.Number `json:"id"`
				Name      string      `json:"name"`
				Type      string      `json:"type"`
				Comments  string      `json:"comments"`
				CreatedBy string      `json:"created_by"`
				CreatedAt string      `json:"created_at"`
				UpdatedAt string      `json:"updated_at"`
				LastUpdBy string      `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &m); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", m.ID.String()},
				{"Name", m.Name},
				{"Type", m.Type},
				{"Comments", m.Comments},
				{"Created By", m.CreatedBy},
				{"Created At", m.CreatedAt},
				{"Updated At", m.UpdatedAt},
				{"Last Updated By", m.LastUpdBy},
			}
			printTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func directoryCreateCmd() *cobra.Command {
	var file, name, memberType, comments string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a directory member",
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
				if name == "" || memberType == "" {
					return fmt.Errorf("required flags: --name, --type (or use --file)")
				}
				b := map[string]interface{}{
					"name": name,
					"type": memberType,
				}
				if comments != "" {
					b["comments"] = comments
				}
				body = b
			}

			data, _, err := apiClient.Post("/api/v1/directory_members/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Member name")
	cmd.Flags().StringVar(&memberType, "type", "", "Member type")
	cmd.Flags().StringVar(&comments, "comments", "", "Comments for version history")

	return cmd
}

func directoryUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update <member-id>",
		Short: "Update a directory member",
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

			data, _, err := apiClient.Put("/api/v1/directory_members/"+args[0], body)
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

func directoryDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <member-id>",
		Short: "Delete a directory member",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := confirmDelete(cmd, args); err != nil {
				return err
			}
			data, _, err := apiClient.Delete("/api/v1/directory_members/" + url.PathEscape(args[0]) + "?comment=deleted+via+cli")
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Directory member %s deleted.\n", args[0])
			}
			return nil
		},
	}
}

func directoryUploadCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Bulk-import directory members from a file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("required flag: --file")
			}
			data, _, err := apiClient.PutMultipart("/api/v1/directory_members/upload/", "file", file)
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to local file to upload")
	return cmd
}

func directoryDownloadCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Bulk-export directory members",
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "normalized" && format != "raw" {
				return fmt.Errorf("--format must be 'normalized' or 'raw' (got %q)", format)
			}
			path := "/api/v1/directory_members/download/?response_format=" + format
			data, _, err := apiClient.Get(path)
			if err != nil {
				return err
			}
			os.Stdout.Write(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "normalized", "Response format: normalized or raw")
	return cmd
}

func directoryHistoryCmd() *cobra.Command {
	var page, limit int
	var orderByDirection, responseFormat string

	cmd := &cobra.Command{
		Use:   "history <member-id>",
		Short: "Show version history for a directory member",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/directory_members/%s/history?page=%d&limit=%d", url.PathEscape(args[0]), page, limit)
			if orderByDirection != "" {
				path += "&order_by_direction=" + url.QueryEscape(orderByDirection)
			}
			if responseFormat != "" {
				path += "&response_format=" + url.QueryEscape(responseFormat)
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
	cmd.Flags().StringVar(&orderByDirection, "order-by-direction", "", "Sort direction (e.g. asc, desc)")
	cmd.Flags().StringVar(&responseFormat, "response-format", "", "Response format passthrough to API")
	return cmd
}

func directoryRestoreCmd() *cobra.Command {
	var comments string

	cmd := &cobra.Command{
		Use:   "restore <member-id>",
		Short: "Restore a soft-deleted directory member",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]interface{}{}
			if comments != "" {
				body["comments"] = comments
			}
			data, _, err := apiClient.Put("/api/v1/directory_members/"+url.PathEscape(args[0])+"/restore", body)
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Directory member %s restored.\n", args[0])
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&comments, "comments", "", "Comment stored in version history (defaults to API default \"Restored\")")
	return cmd
}

func directoryTestCmd() *cobra.Command {
	var timestamp, languageCode string

	cmd := &cobra.Command{
		Use:   "test <member-id>",
		Short: "Test extension lookup for a directory member at a given time",
		Long: `Test extension lookup for a directory member at a given time.

The API requires a timestamp (ISO 8601). Without --timestamp the CLI
substitutes the current time in UTC.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if timestamp == "" {
				timestamp = time.Now().UTC().Format(time.RFC3339)
			}
			path := fmt.Sprintf(
				"/api/v1/directory_members/%s/test?timestamp=%s",
				args[0],
				url.QueryEscape(timestamp),
			)
			if languageCode != "" {
				path += "&language_code=" + url.QueryEscape(languageCode)
			}
			data, _, err := apiClient.Get(path)
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&timestamp, "timestamp", "", "ISO 8601 timestamp (defaults to now UTC)")
	cmd.Flags().StringVar(&languageCode, "language-code", "", "Optional BCP 47 language code (e.g. en-US)")
	return cmd
}
