package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/asksyllable/syllable-cli/internal/output"
	"github.com/spf13/cobra"
)

// bridgePhraseMessages is the phrase set for one scope (the default set, or a
// per-tool override). localized holds per-language overrides keyed by BCP-47 tag.
type bridgePhraseMessages struct {
	Messages  []string `json:"messages"`
	Localized map[string]struct {
		Messages []string `json:"messages"`
	} `json:"localized"`
}

// bridgePhraseConfig is the nested config payload: the default phrases plus
// per-tool overrides and turn-timing settings.
type bridgePhraseConfig struct {
	Phrases bridgePhraseMessages `json:"phrases"`
	Tools   []struct {
		ToolName string               `json:"tool_name"`
		Phrases  bridgePhraseMessages `json:"phrases"`
	} `json:"tools"`
	SmartTurnTimeoutSeconds *float64 `json:"smart_turn_timeout_seconds"`
	RandomizeBridgePhrases  bool     `json:"randomize_bridge_phrases"`
}

// bridgePhrases is the list/get view of a bridge phrases config.
type bridgePhrases struct {
	ID           json.Number        `json:"id"`
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	IsDefault    bool               `json:"is_default"`
	Config       bridgePhraseConfig `json:"config"`
	EditComments string             `json:"edit_comments"`
	AgentsInfo   []struct {
		ID   json.Number `json:"id"`
		Name string      `json:"name"`
	} `json:"agents_info"`
	UpdatedAt string `json:"updated_at"`
	LastUpdBy string `json:"last_updated_by"`
}

func bridgePhrasesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bridge-phrases",
		Short: "Manage bridge phrases configs",
		Long: `List, get, create, update, and delete bridge phrases configs.

Bridge phrases are the hold phrases an agent speaks while a tool call is in
flight ("One moment, please."). A config carries a default phrase set, optional
per-tool overrides, and optional per-language variants. At most one non-deleted
config per suborg may be the default; attach one to an agent by setting the
agent's bridge_phrases_id.`,
		Example: `  # List all bridge phrases configs
  syllable bridge-phrases list

  # Search by name
  syllable bridge-phrases list --search "Default"

  # Get a specific config
  syllable bridge-phrases get 1

  # Get a config as JSON
  syllable bridge-phrases get 1 --output json

  # Create from a JSON file
  syllable bridge-phrases create --file bridge-phrases.json

  # Create a minimal config inline
  syllable bridge-phrases create --name "Inbound Hold" --description "Standard hold phrases"

  # Update a config (fetch, modify, push)
  syllable bridge-phrases get 1 --output json | jq '.name = "Renamed"' | syllable bridge-phrases update 1 --file -

  # Delete a config
  syllable bridge-phrases delete 1`,
	}

	cmd.AddCommand(bridgePhrasesListCmd())
	cmd.AddCommand(bridgePhrasesGetCmd())
	cmd.AddCommand(bridgePhrasesCreateCmd())
	cmd.AddCommand(bridgePhrasesUpdateCmd())
	cmd.AddCommand(bridgePhrasesDeleteCmd())

	return cmd
}

func bridgePhrasesListCmd() *cobra.Command {
	var page, limit int
	var search, searchField string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List bridge phrases configs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(
				listQuery("/api/v1/bridge_phrases/", page, limit, searchField, search),
				[]string{"ID", "NAME", "DESCRIPTION", "DEFAULT", "PHRASES", "UPDATED_AT"},
				func(b bridgePhrases) []string {
					return []string{
						b.ID.String(),
						b.Name,
						output.Truncate(b.Description, 40),
						strconv.FormatBool(b.IsDefault),
						strconv.Itoa(len(b.Config.Phrases.Messages)),
						b.UpdatedAt,
					}
				},
			)
		},
	}

	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items to return")
	cmd.Flags().StringVar(&search, "search", "", "Search by name")
	cmd.Flags().StringVar(&searchField, "search-field", "name", "Field to search on: name, description, updated_at, last_updated_by")

	return cmd
}

func bridgePhrasesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <bridge-phrases-id>",
		Short: "Get a bridge phrases config by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet("/api/v1/bridge_phrases/"+url.PathEscape(args[0]), func(b bridgePhrases) [][]string {
				timeout := "(default)"
				if b.Config.SmartTurnTimeoutSeconds != nil {
					timeout = strconv.FormatFloat(*b.Config.SmartTurnTimeoutSeconds, 'f', -1, 64)
				}

				// The nested config is summarized here; --output json shows the
				// full phrase lists, localizations, and per-tool overrides.
				toolNames := make([]string, len(b.Config.Tools))
				for i, t := range b.Config.Tools {
					toolNames[i] = t.ToolName
				}
				agents := make([]string, len(b.AgentsInfo))
				for i, a := range b.AgentsInfo {
					agents[i] = fmt.Sprintf("%s (%s)", a.Name, a.ID.String())
				}
				langs := make([]string, 0, len(b.Config.Phrases.Localized))
				for tag := range b.Config.Phrases.Localized {
					langs = append(langs, tag)
				}

				return [][]string{
					{"ID", b.ID.String()},
					{"Name", b.Name},
					{"Description", b.Description},
					{"Is Default", strconv.FormatBool(b.IsDefault)},
					{"Phrases", output.Truncate(strings.Join(b.Config.Phrases.Messages, " | "), 60)},
					{"Localized", output.Truncate(strings.Join(langs, ", "), 60)},
					{"Tool Overrides", output.Truncate(strings.Join(toolNames, ", "), 60)},
					{"Smart Turn Timeout", timeout},
					{"Randomize", strconv.FormatBool(b.Config.RandomizeBridgePhrases)},
					{"Agents", output.Truncate(strings.Join(agents, ", "), 60)},
					{"Edit Comments", b.EditComments},
					{"Updated At", b.UpdatedAt},
					{"Last Updated By", b.LastUpdBy},
				}
			})
		},
	}
}

func bridgePhrasesCreateCmd() *cobra.Command {
	var file, name, description string
	var isDefault bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a bridge phrases config",
		Long: `Create a bridge phrases config.

Use --file for the full body (phrases, per-tool overrides, localizations) — see
"syllable schema get BridgePhrasesCreateRequest". The inline flags create a
config with an empty phrase set for you to fill in later.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}
			if file != "" {
				b, err := readJSONBody(file)
				if err != nil {
					return err
				}
				body = b
			} else {
				if name == "" {
					return fmt.Errorf("required flags: --name (or use --file)")
				}
				m := map[string]interface{}{
					"name":       name,
					"is_default": isDefault,
					"config": map[string]interface{}{
						"phrases": map[string]interface{}{"messages": []string{}},
						"tools":   []interface{}{},
					},
				}
				if description != "" {
					m["description"] = description
				}
				body = m
			}

			data, _, err := apiClient.Post("/api/v1/bridge_phrases/", body)
			if err != nil {
				return err
			}

			output.PrintJSON(data)
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file")
	cmd.Flags().StringVar(&name, "name", "", "Bridge phrases config name")
	cmd.Flags().StringVar(&description, "description", "", "Description of the config")
	cmd.Flags().BoolVar(&isDefault, "default", false, "Mark this config as the suborg default")

	return cmd
}

func bridgePhrasesUpdateCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "update [id]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a bridge phrases config",
		Long: `Update a bridge phrases config.

The update is a full replacement of the fields you send, so fetch the current
config first rather than sending a partial body. Omitting is_default preserves
the current default flag.`,
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
			data, _, err := apiClient.Put("/api/v1/bridge_phrases/", body)
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

func bridgePhrasesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <bridge-phrases-id>",
		Short: "Delete a bridge phrases config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, args, "/api/v1/bridge_phrases/"+url.PathEscape(args[0]), "Bridge phrases config")
		},
	}
}
