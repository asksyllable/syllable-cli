package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/asksyllable/syllable-cli/internal/output"
	"github.com/spf13/cobra"
)

func agentsSTTProvidersCmd() *cobra.Command {
	var selectedProvider string
	cmd := &cobra.Command{
		Use: "stt-providers", Short: "List available speech-to-text providers", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/agents/stt/available"
			if selectedProvider != "" {
				path += "?selected_provider=" + url.QueryEscape(selectedProvider)
			}
			data, _, err := apiClient.Get(path)
			if err != nil {
				return err
			}
			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}
			var providers []struct {
				Provider    string `json:"provider"`
				DisplayName string `json:"display_name"`
				Status      string `json:"status"`
				Deprecated  bool   `json:"deprecated"`
				Removed     bool   `json:"removed"`
				SunsetDate  string `json:"sunset_date"`
				Fallback    string `json:"fallback"`
			}
			if err := json.Unmarshal(data, &providers); err != nil {
				return err
			}
			rows := make([][]string, len(providers))
			for i, p := range providers {
				rows[i] = []string{p.Provider, p.DisplayName, p.Status, fmt.Sprint(p.Deprecated), fmt.Sprint(p.Removed), p.SunsetDate, p.Fallback}
			}
			printTable([]string{"PROVIDER", "DISPLAY_NAME", "STATUS", "DEPRECATED", "REMOVED", "SUNSET_DATE", "FALLBACK"}, rows)
			fmt.Printf("\nTotal: %d\n", len(providers))
			return nil
		},
	}
	cmd.Flags().StringVar(&selectedProvider, "selected-provider", "", "Include a saved STT provider even if retired")
	return cmd
}

func agentsCardClickCmd() *cobra.Command {
	var file, actionID, actionType string
	cmd := &cobra.Command{
		Use: "card-click <test-id>", Short: "Log a card interaction in a conversation test", Args: cobra.ExactArgs(1),
		Example: `  syllable agents card-click my-test-1 --action-id book --action-type cta
  syllable agents card-click my-test-1 --file click.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var body interface{}
			if file != "" {
				var err error
				body, err = readJSONBody(file)
				if err != nil {
					return err
				}
			} else {
				if actionID == "" || actionType == "" {
					return fmt.Errorf("required flags: --action-id and --action-type (or use --file)")
				}
				if actionType != "cta" && actionType != "action" {
					return fmt.Errorf("--action-type must be cta or action")
				}
				body = map[string]interface{}{"action_id": actionID, "action_type": actionType}
			}
			data, _, err := apiClient.Post("/api/v1/agents/test/"+url.PathEscape(args[0])+"/card-click", body)
			if err != nil {
				return err
			}
			output.PrintJSON(data)
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file (or - for stdin)")
	cmd.Flags().StringVar(&actionID, "action-id", "", "Identifier of the clicked card action")
	cmd.Flags().StringVar(&actionType, "action-type", "", "Card action type: cta or action")
	return cmd
}
