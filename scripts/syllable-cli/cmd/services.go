package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func servicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "services",
		Short: "Manage services",
		Long:  "List, get, create, update, and delete services.",
	}

	cmd.AddCommand(servicesListCmd())
	cmd.AddCommand(servicesGetCmd())
	cmd.AddCommand(servicesCreateCmd())
	cmd.AddCommand(servicesUpdateCmd())
	cmd.AddCommand(servicesDeleteCmd())

	return cmd
}

func servicesListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List services",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/services/?page=%d&limit=%d", page, limit)
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
					AuthType    string      `json:"auth_type"`
					LastUpdated string      `json:"last_updated"`
					LastUpdBy   string      `json:"last_updated_by"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "NAME", "AUTH_TYPE", "LAST_UPDATED"}
			rows := make([][]string, len(result.Items))
			for i, s := range result.Items {
				rows[i] = []string{
					s.ID.String(),
					s.Name,
					s.AuthType,
					s.LastUpdated,
				}
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

func servicesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <service-id>",
		Short: "Get a service by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/services/" + args[0])
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var s struct {
				ID          json.Number `json:"id"`
				Name        string      `json:"name"`
				AuthType    string      `json:"auth_type"`
				LastUpdated string      `json:"last_updated"`
				LastUpdBy   string      `json:"last_updated_by"`
			}
			if err := json.Unmarshal(data, &s); err != nil {
				output.PrintJSON(data)
				return nil
			}

			rows := [][]string{
				{"ID", s.ID.String()},
				{"Name", s.Name},
				{"Auth Type", s.AuthType},
				{"Last Updated", s.LastUpdated},
				{"Last Updated By", s.LastUpdBy},
			}
			output.PrintTable([]string{"FIELD", "VALUE"}, rows)
			return nil
		},
	}
}

func servicesCreateCmd() *cobra.Command {
	var file, name, authType string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a service",
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
				if name == "" || authType == "" {
					return fmt.Errorf("required flags: --name, --auth-type (or use --file)")
				}
				body = map[string]interface{}{
					"name":      name,
					"auth_type": authType,
				}
			}

			data, _, err := apiClient.Post("/api/v1/services/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Service name")
	cmd.Flags().StringVar(&authType, "auth-type", "", "Auth type (e.g. bearer, basic, custom)")

	return cmd
}

func servicesUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a service",
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

			data, _, err := apiClient.Put("/api/v1/services/", body)
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

func servicesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <service-id>",
		Short: "Delete a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/services/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Service %s deleted.\n", args[0])
			}
			return nil
		},
	}
}
