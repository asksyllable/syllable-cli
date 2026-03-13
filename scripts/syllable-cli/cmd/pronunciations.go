package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
)

func pronunciationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pronunciations",
		Short: "Manage pronunciations",
		Long:  "List, download CSV, upload CSV, delete CSV, and get metadata for pronunciations.",
	}

	cmd.AddCommand(pronunciationsListCmd())
	cmd.AddCommand(pronunciationsGetCSVCmd())
	cmd.AddCommand(pronunciationsMetadataCmd())

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
			fmt.Print(string(data))
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
