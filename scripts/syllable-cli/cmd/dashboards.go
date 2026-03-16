package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func dashboardsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboards",
		Short: "Query dashboard data",
		Long:  "Query dashboard data for sessions, events, transfers, summaries, and more.",
		Example: `  # List dashboard data with a JSON body file
  syllable dashboards list --file query.json

  # Get session data for the dashboard
  syllable dashboards sessions --file query.json

  # Get session events
  syllable dashboards session-events --file query.json

  # Get session transfer data
  syllable dashboards session-transfers --file query.json

  # Get session summary data
  syllable dashboards session-summary --file query.json

  # Fetch additional dashboard info
  syllable dashboards fetch-info --file query.json`,
	}

	cmd.AddCommand(dashboardsListCmd())
	cmd.AddCommand(dashboardsSessionsCmd())
	cmd.AddCommand(dashboardsSessionEventsCmd())
	cmd.AddCommand(dashboardsSessionTransfersCmd())
	cmd.AddCommand(dashboardsSessionSummaryCmd())
	cmd.AddCommand(dashboardsFetchInfoCmd())

	return cmd
}

func dashboardsListCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List dashboard data",
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
				body = map[string]interface{}{}
			}

			data, _, err := apiClient.Post("/api/v1/dashboards/list", body)
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

func dashboardsSessionsCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Query session dashboard data",
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
				body = map[string]interface{}{}
			}

			data, _, err := apiClient.Post("/api/v1/dashboards/sessions", body)
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

func dashboardsSessionEventsCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "session-events",
		Short: "Query session events dashboard data",
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
				body = map[string]interface{}{}
			}

			data, _, err := apiClient.Post("/api/v1/dashboards/session_events", body)
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

func dashboardsSessionTransfersCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "session-transfers",
		Short: "Query session transfers dashboard data",
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
				body = map[string]interface{}{}
			}

			data, _, err := apiClient.Post("/api/v1/dashboards/session_transfers", body)
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

func dashboardsSessionSummaryCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "session-summary",
		Short: "Query session summary dashboard data",
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
				body = map[string]interface{}{}
			}

			data, _, err := apiClient.Post("/api/v1/dashboards/session_summary", body)
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

func dashboardsFetchInfoCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "fetch-info",
		Short: "Fetch dashboard info",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required (e.g. session_events, session_summary)")
			}

			path := fmt.Sprintf("/api/v1/dashboards/fetch_info?dashboard_name=%s", name)
			data, _, err := apiClient.Post(path, map[string]interface{}{})
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Dashboard name (e.g. session_events, session_summary)")
	return cmd
}
