package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func incidentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "incidents",
		Short: "Manage incidents",
		Long:  "List, get, create, update, and delete incidents.",
		Example: `  # List all incidents
  syllable incidents list

  # Search incidents by title
  syllable incidents list --search "outage"

  # Get a specific incident
  syllable incidents get 4

  # Create an incident from a JSON file
  syllable incidents create --file incident.json

  # Update an incident
  syllable incidents update 4 --file incident.json

  # Delete an incident
  syllable incidents delete 4

  # List organizations affected by an incident
  syllable incidents organizations 4`,
	}

	cmd.AddCommand(incidentsListCmd())
	cmd.AddCommand(incidentsGetCmd())
	cmd.AddCommand(incidentsCreateCmd())
	cmd.AddCommand(incidentsUpdateCmd())
	cmd.AddCommand(incidentsDeleteCmd())
	cmd.AddCommand(incidentsOrganizationsCmd())

	return cmd
}

func incidentsListCmd() *cobra.Command {
	var page, limit int
	var search string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List incidents",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/incidents/?page=%d&limit=%d", page, limit)
			if search != "" {
				path += fmt.Sprintf("&search_fields=title&search_field_values=%s", search)
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
					ID              json.Number `json:"id"`
					Description     string      `json:"description"`
					ImpactCategory  string      `json:"impact_category"`
					SessionsImpacted int        `json:"sessions_impacted"`
					StartDatetime   string      `json:"start_datetime"`
					CreatedAt       string      `json:"created_at"`
				} `json:"items"`
				TotalCount int `json:"total_count"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "DESCRIPTION", "IMPACT_CATEGORY", "SESSIONS", "START", "CREATED_AT"}
			rows := make([][]string, len(result.Items))
			for i, inc := range result.Items {
				rows[i] = []string{
					inc.ID.String(),
					output.Truncate(inc.Description, 50),
					inc.ImpactCategory,
					fmt.Sprintf("%d", inc.SessionsImpacted),
					inc.StartDatetime,
					inc.CreatedAt,
				}
			}
			printTable(headers, rows)
			fmt.Printf("\nTotal: %d\n", result.TotalCount)
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&search, "search", "", "Search by title")

	return cmd
}

func incidentsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <incident-id>",
		Short: "Get an incident by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/incidents/" + args[0])
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func incidentsCreateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an incident",
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
				return fmt.Errorf("use --file to provide incident body")
			}

			data, _, err := apiClient.Post("/api/v1/incidents/", body)
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

func incidentsUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an incident",
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

			data, _, err := apiClient.Put("/api/v1/incidents/", body)
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

func incidentsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <incident-id>",
		Short: "Delete an incident",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/incidents/" + args[0])
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("Incident %s deleted.\n", args[0])
			}
			return nil
		},
	}
}

func incidentsOrganizationsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "organizations",
		Short: "List organizations for incidents",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/incidents/organizations")
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}
