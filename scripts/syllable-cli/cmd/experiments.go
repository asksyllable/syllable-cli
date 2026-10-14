package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/asksyllable/syllable-cli/internal/output"
	"github.com/spf13/cobra"
)

type experiment struct {
	ID            json.Number `json:"id"`
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	TargetID      json.Number `json:"target_id"`
	Status        string      `json:"status"`
	StartedAt     string      `json:"started_at"`
	StoppedAt     string      `json:"stopped_at"`
	CreatedAt     string      `json:"created_at"`
	UpdatedAt     string      `json:"updated_at"`
	LastUpdatedBy string      `json:"last_updated_by"`
	Variants      []struct {
		ID        json.Number `json:"id"`
		Name      string      `json:"name"`
		Weight    int         `json:"weight"`
		IsControl bool        `json:"is_control"`
		AgentID   json.Number `json:"agent_id"`
		AgentName string      `json:"agent_name"`
	} `json:"variants"`
}

func experimentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "experiments", Short: "Manage agent traffic experiments",
		Long: `Split a channel target's traffic between two or more agent variants.
Variants use different agents, have weights totaling 100, and exactly one control.
Only draft experiments can be updated or started; running experiments must be
stopped before deletion. Updates send the complete variant set.`,
		Example: `  syllable experiments list
  syllable experiments create --file experiment.json
  syllable experiments get 42
  syllable experiments update 42 --file experiment.json
  syllable experiments start 42
  syllable experiments results 42
  syllable experiments stop 42
  syllable experiments delete 42`,
	}
	cmd.AddCommand(experimentsListCmd(), experimentsGetCmd(), experimentsWriteCmd(false), experimentsWriteCmd(true))
	cmd.AddCommand(experimentsActionCmd("start"), experimentsActionCmd("stop"), experimentsResultsCmd())
	cmd.AddCommand(&cobra.Command{
		Use: "delete <experiment-id>", Short: "Delete a non-running experiment", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, args, "/api/v1/experiments/"+url.PathEscape(args[0]), "Experiment")
		},
	})
	return cmd
}

func experimentsListCmd() *cobra.Command {
	var opts resourceListOptions
	cmd := &cobra.Command{
		Use: "list", Short: "List experiments", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(opts.path("/api/v1/experiments/"),
				[]string{"ID", "NAME", "STATUS", "TARGET_ID", "VARIANTS", "UPDATED_AT"},
				func(e experiment) []string {
					return []string{e.ID.String(), e.Name, e.Status, e.TargetID.String(), fmt.Sprint(len(e.Variants)), e.UpdatedAt}
				})
		},
	}
	opts.flags(cmd, "name", "Search by an ExperimentProperties field", true)
	return cmd
}

func experimentsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use: "get <experiment-id>", Short: "Get an experiment", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet("/api/v1/experiments/"+url.PathEscape(args[0]), func(e experiment) [][]string {
				var variants []string
				for _, v := range e.Variants {
					control := ""
					if v.IsControl {
						control = " (control)"
					}
					variants = append(variants, fmt.Sprintf("%s: %d%%, agent %s (%s)%s", v.Name, v.Weight, v.AgentID, v.AgentName, control))
				}
				return [][]string{
					{"ID", e.ID.String()}, {"Name", e.Name}, {"Description", e.Description},
					{"Status", e.Status}, {"Target ID", e.TargetID.String()}, {"Variants", strings.Join(variants, "; ")},
					{"Started At", e.StartedAt}, {"Stopped At", e.StoppedAt}, {"Created At", e.CreatedAt},
					{"Updated At", e.UpdatedAt}, {"Last Updated By", e.LastUpdatedBy},
				}
			})
		},
	}
}

func experimentsWriteCmd(update bool) *cobra.Command {
	var file string
	cmd := &cobra.Command{Use: "create", Short: "Create an experiment from JSON", Args: cobra.NoArgs}
	if update {
		cmd.Use, cmd.Short, cmd.Args = "update <experiment-id>", "Update a draft experiment from JSON", cobra.ExactArgs(1)
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if file == "" {
			return fmt.Errorf("use --file to provide experiment body")
		}
		body, err := readJSONBody(file)
		if err != nil {
			return err
		}
		var data []byte
		if update {
			data, _, err = apiClient.Put("/api/v1/experiments/"+url.PathEscape(args[0]), body)
		} else {
			data, _, err = apiClient.Post("/api/v1/experiments/", body)
		}
		if err != nil {
			return err
		}
		output.PrintJSON(data)
		return nil
	}
	cmd.Flags().StringVar(&file, "file", "", "Path to JSON body file (or - for stdin); variants are a complete set")
	return cmd
}

func experimentsActionCmd(action string) *cobra.Command {
	return &cobra.Command{
		Use: action + " <experiment-id>", Short: strings.ToUpper(action[:1]) + action[1:] + " an experiment", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Post("/api/v1/experiments/"+url.PathEscape(args[0])+"/"+action, nil)
			if err != nil {
				return err
			}
			// Starting returns both the experiment and warnings; keep both visible.
			output.PrintJSON(data)
			return nil
		},
	}
}

func experimentsResultsCmd() *cobra.Command {
	return &cobra.Command{
		Use: "results <experiment-id>", Short: "Get session counts for each experiment variant", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, _, err := apiClient.Get("/api/v1/experiments/" + url.PathEscape(args[0]) + "/results")
			if err != nil {
				return err
			}
			if getOutputFmt() == "json" {
				output.PrintJSON(data)
				return nil
			}
			var result struct {
				Variants []struct {
					VariantID    json.Number `json:"variant_id"`
					SessionCount int         `json:"session_count"`
				} `json:"variants"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				return err
			}
			rows := make([][]string, len(result.Variants))
			for i, v := range result.Variants {
				rows[i] = []string{v.VariantID.String(), fmt.Sprint(v.SessionCount)}
			}
			printTable([]string{"VARIANT_ID", "SESSION_COUNT"}, rows)
			return nil
		},
	}
}
