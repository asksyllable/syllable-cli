package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show configuration status",
		Long:  "Reports whether the CLI is configured and lists available orgs and environments.\nExits with code 1 if not configured (no orgs found).",
		Example: `  syllable status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			cfgPath := filepath.Join(home, ".syllable", "config.yaml")

			orgs := viper.GetStringMap("orgs")
			if len(orgs) == 0 {
				fmt.Fprintf(os.Stderr, "Not configured — run `syllable setup` to get started.\n")
				fmt.Fprintf(os.Stderr, "Config path: %s\n", cfgPath)
				os.Exit(1)
			}

			fmt.Printf("Config:      %s\n", cfgPath)

			defaultOrg := viper.GetString("default_org")
			defaultEnv := viper.GetString("default_env")
			if defaultOrg == "" {
				defaultOrg = "(none)"
			}
			if defaultEnv == "" {
				defaultEnv = "prod"
			}
			fmt.Printf("Default org: %s\n", defaultOrg)
			fmt.Printf("Default env: %s\n", defaultEnv)
			fmt.Println()

			orgNames := make([]string, 0, len(orgs))
			for n := range orgs {
				orgNames = append(orgNames, n)
			}
			sort.Strings(orgNames)

			for _, name := range orgNames {
				orgData, _ := orgs[name].(map[string]interface{})
				var envList []string
				if orgData != nil {
					if envs, ok := orgData["envs"].(map[string]interface{}); ok {
						for e := range envs {
							envList = append(envList, e)
						}
						sort.Strings(envList)
					}
				}
				if len(envList) > 0 {
					fmt.Printf("  org: %-20s  envs: %s\n", name, strings.Join(envList, ", "))
				} else {
					fmt.Printf("  org: %s\n", name)
				}
			}
			return nil
		},
	}
}
