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
	cmd.AddCommand(organizationsSipIPRangesCmd())

	return cmd
}

// ── SIP IP ranges ──────────────────────────────────────────────────────────────

func organizationsSipIPRangesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sip-ip-ranges",
		Short: "Manage organization SIP IP ranges",
		Long:  "List, create, update, and delete the current organization's SIP IP ranges (signaling or media, in CIDR notation).",
		Example: `  # List SIP IP ranges
  syllable organizations sip-ip-ranges list

  # Create a signaling range
  syllable organizations sip-ip-ranges create --type signaling --ip-range 192.168.1.0/24

  # Update a range
  syllable organizations sip-ip-ranges update 7 --ip-range 10.0.0.0/24

  # Delete a range
  syllable organizations sip-ip-ranges delete 7`,
	}

	cmd.AddCommand(organizationsSipIPRangesListCmd())
	cmd.AddCommand(organizationsSipIPRangesCreateCmd())
	cmd.AddCommand(organizationsSipIPRangesUpdateCmd())
	cmd.AddCommand(organizationsSipIPRangesDeleteCmd())

	return cmd
}

func organizationsSipIPRangesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the organization's SIP IP ranges",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/organizations/sip_ip_ranges/")
			if err != nil {
				return err
			}

			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}

			var ranges []struct {
				ID        json.Number `json:"id"`
				Type      string      `json:"type"`
				IPRange   string      `json:"ip_range"`
				Verified  bool        `json:"verified"`
				CreatedAt *string     `json:"created_at"`
			}
			if err := json.Unmarshal(data, &ranges); err != nil {
				output.PrintJSON(data)
				return nil
			}

			headers := []string{"ID", "TYPE", "IP_RANGE", "VERIFIED", "CREATED_AT"}
			rows := make([][]string, len(ranges))
			for i, r := range ranges {
				verified := "no"
				if r.Verified {
					verified = "yes"
				}
				createdAt := ""
				if r.CreatedAt != nil {
					createdAt = *r.CreatedAt
				}
				rows[i] = []string{r.ID.String(), r.Type, r.IPRange, verified, createdAt}
			}
			printTable(headers, rows)
			fmt.Printf("\nTotal: %d\n", len(ranges))
			return nil
		},
	}
}

func organizationsSipIPRangesCreateCmd() *cobra.Command {
	var file, rangeType, ipRange string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a SIP IP range",
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
				if rangeType == "" || ipRange == "" {
					return fmt.Errorf("required flags: --type, --ip-range (or use --file)")
				}
				body = map[string]interface{}{
					"type":     rangeType,
					"ip_range": ipRange,
				}
			}

			data, _, err := apiClient.Post("/api/v1/organizations/sip_ip_ranges/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&rangeType, "type", "", "SIP IP range type: signaling or media")
	cmd.Flags().StringVar(&ipRange, "ip-range", "", "SIP IP range in CIDR notation, e.g. 192.168.1.0/24")

	return cmd
}

func organizationsSipIPRangesUpdateCmd() *cobra.Command {
	var file, rangeType, ipRange string

	cmd := &cobra.Command{
		Use:   "update <sip-ip-range-id>",
		Short: "Update a SIP IP range",
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
				m := map[string]interface{}{}
				if rangeType != "" {
					m["type"] = rangeType
				}
				if ipRange != "" {
					m["ip_range"] = ipRange
				}
				if len(m) == 0 {
					return fmt.Errorf("provide at least one of --type, --ip-range (or use --file)")
				}
				body = m
			}

			path := fmt.Sprintf("/api/v1/organizations/sip_ip_ranges/%s", args[0])
			data, _, err := apiClient.Put(path, body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&rangeType, "type", "", "SIP IP range type: signaling or media")
	cmd.Flags().StringVar(&ipRange, "ip-range", "", "SIP IP range in CIDR notation, e.g. 192.168.1.0/24")

	return cmd
}

func organizationsSipIPRangesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <sip-ip-range-id>",
		Short: "Delete a SIP IP range",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/organizations/sip_ip_ranges/%s", args[0])
			data, _, err := apiClient.Delete(path)
			if err != nil {
				return err
			}
			if len(data) > 0 {
				output.PrintJSON(data)
			} else {
				fmt.Printf("SIP IP range %s deleted.\n", args[0])
			}
			return nil
		},
	}
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
