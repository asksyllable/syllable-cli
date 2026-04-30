package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

func pronunciationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pronunciations",
		Short: "Manage pronunciations",
		Long:  "List, download CSV, upload CSV, delete CSV, and get metadata for pronunciations.",
		Example: `  # List all pronunciations
  syllable pronunciations list

  # Download pronunciations as a CSV file
  syllable pronunciations get-csv

  # Save the CSV to a file
  syllable pronunciations get-csv > pronunciations.csv

  # Upload pronunciations from a CSV file
  syllable pronunciations upload-csv --file pronunciations.csv

  # Wipe the pronunciations dictionary
  syllable pronunciations delete-csv

  # Get pronunciations metadata
  syllable pronunciations metadata`,
	}

	cmd.AddCommand(pronunciationsListCmd())
	cmd.AddCommand(pronunciationsGetCSVCmd())
	cmd.AddCommand(pronunciationsUploadCSVCmd())
	cmd.AddCommand(pronunciationsDeleteCSVCmd())
	cmd.AddCommand(pronunciationsMetadataCmd())

	return cmd
}

func pronunciationsUploadCSVCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "upload-csv",
		Short: "Upload a pronunciations CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("required flag: --file")
			}
			data, _, err := apiClient.PostMultipart("/api/v1/pronunciations/csv", "file", file)
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to local CSV file (use '-' for stdin)")
	return cmd
}

func pronunciationsDeleteCSVCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-csv",
		Short: "Delete the pronunciations dictionary",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Delete("/api/v1/pronunciations/csv")
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				// Match other delete commands (directory, agents, voice-groups,
				// etc.) which print success messages to stdout.
				fmt.Println("Pronunciations dictionary deleted.")
			}
			return nil
		},
	}
}

func pronunciationsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List pronunciations",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/pronunciations")
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}

func pronunciationsGetCSVCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get-csv",
		Short: "Download pronunciations CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/pronunciations/csv")
			if err != nil {
				return err
			}
			os.Stdout.Write(data)
			return nil
		},
	}
}

func pronunciationsMetadataCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "metadata",
		Short: "Get pronunciations metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/pronunciations/metadata")
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
}
