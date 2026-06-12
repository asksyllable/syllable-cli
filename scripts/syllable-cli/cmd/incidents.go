package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/asksyllable/syllable-cli/internal/output"
	"github.com/spf13/cobra"
)

// incident is the list view of an incident resource.
type incident struct {
	ID               json.Number `json:"id"`
	Description      string      `json:"description"`
	ImpactCategory   string      `json:"impact_category"`
	SessionsImpacted int         `json:"sessions_impacted"`
	StartDatetime    string      `json:"start_datetime"`
	CreatedAt        string      `json:"created_at"`
}

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
	var search, searchField string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List incidents",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(
				listQuery("/api/v1/incidents/", page, limit, searchField, search),
				[]string{"ID", "DESCRIPTION", "IMPACT_CATEGORY", "SESSIONS", "START", "CREATED_AT"},
				func(inc incident) []string {
					return []string{
						inc.ID.String(),
						output.Truncate(inc.Description, 50),
						inc.ImpactCategory,
						fmt.Sprintf("%d", inc.SessionsImpacted),
						inc.StartDatetime,
						inc.CreatedAt,
					}
				},
			)
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&search, "search", "", "Search by title")
	cmd.Flags().StringVar(&searchField, "search-field", "title", "Field to search on (see API docs for valid values)")

	return cmd
}

func incidentsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <incident-id>",
		Short: "Get an incident by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/incidents/" + url.PathEscape(args[0]))
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
			if file == "" {
				return fmt.Errorf("use --file to provide incident body")
			}
			body, err := readJSONBody(file)
			if err != nil {
				return err
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
		Use:   "update [id]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an incident",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("use --file to provide update body")
			}
			body, err := readJSONBody(file)
			if err != nil {
				return err
			}

			// An optional positional id is reconciled with the body id, which the
			// collection PUT routes by — matching the agents/prompts shape (#121, #68).
			if len(args) == 1 {
				if err := ensureBodyIdentifier(body, "id", args[0], true); err != nil {
					return err
				}
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
			return runDelete(cmd, args, "/api/v1/incidents/"+url.PathEscape(args[0]), "Incident")
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
