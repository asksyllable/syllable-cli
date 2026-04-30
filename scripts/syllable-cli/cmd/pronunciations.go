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
		Long: `List, download CSV, upload CSV, delete CSV, and get metadata for pronunciations.

The pronunciation dictionary is org-wide and applies to every agent at runtime.
upload-csv and delete-csv affect every live agent immediately, so they require
an explicit --confirm flag.`,
		Example: `  # List all pronunciations
  syllable pronunciations list

  # Download pronunciations as a CSV file
  syllable pronunciations get-csv

  # Save the CSV to a file
  syllable pronunciations get-csv > pronunciations.csv

  # Upload pronunciations from a CSV file (replaces the org dictionary)
  syllable pronunciations upload-csv --file pronunciations.csv --confirm

  # Wipe the pronunciations dictionary
  syllable pronunciations delete-csv --confirm

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
	var confirm bool

	cmd := &cobra.Command{
		Use:   "upload-csv",
		Short: "Upload a pronunciations CSV (requires --confirm)",
		Long: `Upload a pronunciations CSV.

This **replaces** the org-wide pronunciation dictionary. Every live agent
will pick up the new pronunciations on its next synthesis call. The
--confirm flag is required to prevent accidental dictionary clobbers.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("required flag: --file")
			}
			if !confirm {
				return fmt.Errorf("refusing to replace the org-wide pronunciation dictionary without --confirm")
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
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm replacing the org-wide pronunciation dictionary")
	return cmd
}

func pronunciationsDeleteCSVCmd() *cobra.Command {
	var confirm bool

	cmd := &cobra.Command{
		Use:   "delete-csv",
		Short: "Delete the pronunciations dictionary (requires --confirm)",
		Long: `Delete the org-wide pronunciation dictionary.

Every live agent loses its custom pronunciations immediately. The --confirm
flag is required to prevent a typo from wiping the dictionary.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				return fmt.Errorf("refusing to delete the org-wide pronunciation dictionary without --confirm")
			}
			data, _, err := apiClient.Delete("/api/v1/pronunciations/csv")
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Println("Pronunciations dictionary deleted.")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm deleting the org-wide pronunciation dictionary")
	return cmd
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
