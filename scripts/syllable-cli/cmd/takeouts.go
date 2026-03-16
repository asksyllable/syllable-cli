package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func takeoutsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "takeouts",
		Short: "Manage data takeouts",
		Long:  "Create, get, and download data takeout exports.",
		Example: `  # Create a data takeout from a JSON file
  syllable takeouts create --file takeout-request.json

  # Get the status of a takeout
  syllable takeouts get abc-123

  # Download a completed takeout
  syllable takeouts download abc-123

  # Download a takeout to a specific file
  syllable takeouts download abc-123 --output takeout.zip`,
	}

	cmd.AddCommand(takeoutsCreateCmd())
	cmd.AddCommand(takeoutsGetCmd())
	cmd.AddCommand(takeoutsDownloadCmd())

	return cmd
}

func takeoutsCreateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a data takeout",
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
				return fmt.Errorf("use --file to provide takeout body")
			}

			data, _, err := apiClient.Post("/api/v1/takeouts/create", body)
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

func takeoutsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <job-id>",
		Short: "Get a takeout job status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/takeouts/get/" + args[0])
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func takeoutsDownloadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "download <job-id> <file-name>",
		Short: "Download a takeout file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/takeouts/get/%s/file/%s", args[0], args[1])
			data, _, err := apiClient.Get(path)
			if err != nil {
				return err
			}
			fmt.Print(string(data))
			return nil
		},
	}
}
