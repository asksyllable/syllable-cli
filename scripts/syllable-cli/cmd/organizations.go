package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/asksyllable/syllable-cli/internal/output"
)

// orgFormFields builds the multipart form-field map for organization update.
// Optional values are only included when set, so the API doesn't receive
// empty strings for fields the caller didn't touch.
func orgFormFields(displayName, description, domains, samlProviderID, updateComments string) map[string]string {
	fields := map[string]string{}
	if displayName != "" {
		fields["display_name"] = displayName
	}
	if description != "" {
		fields["description"] = description
	}
	if domains != "" {
		fields["domains"] = domains
	}
	if samlProviderID != "" {
		fields["saml_provider_id"] = samlProviderID
	}
	if updateComments != "" {
		fields["update_comments"] = updateComments
	}
	return fields
}

func organizationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "organizations",
		Short: "Manage the current organization",
		Long: `Get and update the current organization.

Org provisioning (create) and deletion are intentionally not exposed via
the CLI — both are platform-side admin operations with high blast radius.
Use the web console if you really need them.`,
		Example: `  # Show the current organization
  syllable organizations get

  # Show as JSON
  syllable organizations get --output json

  # Update the current organization
  syllable organizations update --display-name NAME`,
	}

	cmd.AddCommand(organizationsGetCmd())
	cmd.AddCommand(organizationsListCmd())
	cmd.AddCommand(organizationsUpdateCmd())

	return cmd
}

func organizationsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Get the current organization",
		RunE:  organizationsGetRunE,
	}
}

// organizationsListCmd is a hidden back-compat alias for `organizations get`.
// The spec calls this endpoint "Get Current Organization" (singular) — not a
// list — but earlier CLI versions exposed it as `list`. Keeping the old name
// avoids breaking scripts that still invoke it.
func organizationsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "list",
		Short:  "Alias of `get` (deprecated)",
		Hidden: true,
		RunE:   organizationsGetRunE,
	}
}

func organizationsGetRunE(cmd *cobra.Command, args []string) error {
	data, _, err := apiClient.Get("/api/v1/organizations/")
	if err != nil {
		return err
	}

	if getOutputFmt() == "json" {
		output.PrintJSON(data)
		return nil
	}

	var org struct {
		ID          json.Number `json:"id"`
		Name        string      `json:"name"`
		DisplayName string      `json:"display_name"`
		Slug        string      `json:"slug"`
		Description *string     `json:"description"`
		LastUpdated string      `json:"last_updated"`
	}
	if err := json.Unmarshal(data, &org); err != nil {
		output.PrintJSON(data)
		return nil
	}

	desc := ""
	if org.Description != nil {
		desc = output.Truncate(*org.Description, 50)
	}

	headers := []string{"ID", "NAME", "DISPLAY_NAME", "SLUG", "DESCRIPTION", "LAST_UPDATED"}
	rows := [][]string{
		{org.ID.String(), org.Name, org.DisplayName, org.Slug, desc, org.LastUpdated},
	}
	printTable(headers, rows)
	return nil
}

func organizationsUpdateCmd() *cobra.Command {
	var displayName, logo, description, domains, samlProviderID, updateComments string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the current organization",
		Long: `Update the current organization. Multipart/form-data: --display-name
is required by the API; logo and other fields are optional.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if displayName == "" {
				return fmt.Errorf("required flag: --display-name")
			}

			fields := orgFormFields(displayName, description, domains, samlProviderID, updateComments)
			data, _, err := apiClient.PutMultipartForm("/api/v1/organizations/", fields, "logo", logo)
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&displayName, "display-name", "", "Human-readable display name (required)")
	cmd.Flags().StringVar(&logo, "logo", "", "Path to a new logo (120x120 PNG); omit to keep current")
	cmd.Flags().StringVar(&description, "description", "", "Description of the organization")
	cmd.Flags().StringVar(&domains, "domains", "", "Comma-delimited list of email domains")
	cmd.Flags().StringVar(&samlProviderID, "saml-provider-id", "", "SAML provider ID for SSO")
	cmd.Flags().StringVar(&updateComments, "update-comments", "", "Comments describing this update")
	return cmd
}
