package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/asksyllable/syllable-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func envsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "envs",
		Short:   "List configured environments and their orgs",
		Long:    "Lists all environments from the config, their base URLs, and which orgs are configured for each.\nNo API calls are made — reads from local config only.",
		Example: `  syllable envs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Build env → orgs mapping by scanning each org's envs sub-map.
			orgsCfg := viper.GetStringMap("orgs")
			envOrgs := make(map[string][]string)
			for orgName, orgData := range orgsCfg {
				orgMap, ok := orgData.(map[string]interface{})
				if !ok {
					continue
				}
				envs, ok := orgMap["envs"].(map[string]interface{})
				if !ok {
					continue
				}
				for envName := range envs {
					envOrgs[envName] = append(envOrgs[envName], orgName)
				}
			}

			// Collect configured environments (which carry base_url entries).
			environments := viper.GetStringMap("environments")

			// Union of all env names from both sources.
			envSet := make(map[string]struct{})
			for e := range envOrgs {
				envSet[e] = struct{}{}
			}
			for e := range environments {
				envSet[e] = struct{}{}
			}

			if len(envSet) == 0 {
				fmt.Println("No environments configured. Run `syllable setup` to get started.")
				return nil
			}

			// Print defaults header.
			defaultOrg := viper.GetString("default_org")
			defaultEnv := viper.GetString("default_env")
			if defaultOrg == "" {
				defaultOrg = "(none)"
			}
			if defaultEnv == "" {
				defaultEnv = "prod"
			}
			fmt.Printf("Default org: %s\n", defaultOrg)
			fmt.Printf("Default env: %s\n\n", defaultEnv)


			sortedEnvs := make([]string, 0, len(envSet))
			for e := range envSet {
				sortedEnvs = append(sortedEnvs, e)
			}
			sort.Strings(sortedEnvs)

			headers := []string{"ENVIRONMENT", "BASE_URL", "ORGS"}
			rows := make([][]string, 0, len(sortedEnvs))

			for _, envName := range sortedEnvs {
				// Resolve base URL: config lookup, then built-in alias for prod.
				baseURL := ""
				if envData, ok := environments[envName]; ok {
					if envMap, ok := envData.(map[string]interface{}); ok {
						baseURL, _ = envMap["base_url"].(string)
					}
				}
				if baseURL == "" && envName == "prod" {
					baseURL = "https://api.syllable.cloud"
				}

				orgsForEnv := envOrgs[envName]
				sort.Strings(orgsForEnv)

				// Mark the active default env with an asterisk.
				label := envName
				if envName == defaultEnv {
					label = envName + " *"
				}

				rows = append(rows, []string{
					label,
					baseURL,
					strings.Join(orgsForEnv, ", "),
				})
			}

			output.PrintTable(headers, rows)
			fmt.Println("\n* = default environment")
			return nil
		},
	}
}
