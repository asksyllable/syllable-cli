package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/syllable-ai/syllable-cli/internal/client"
)

var (
	cfgFile   string
	apiKey    string
	baseURL   string
	orgName   string
	envName   string
	outputFmt string
	apiClient *client.Client
)

// rootCmd is the base command for the syllable CLI.
var rootCmd = &cobra.Command{
	Use:   "syllable",
	Short: "Syllable CLI - manage your Syllable AI platform",
	Long: `syllable is a CLI tool for managing your Syllable AI platform resources.

It supports agents, channels, conversations, prompts, tools, sessions,
outbound campaigns, users, directory, insights, custom messages, language groups, and organizations.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip for commands that don't need auth
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}
		initClient()
		return nil
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.syllable/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "Syllable API key (overrides org lookup)")
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "Syllable API base URL (overrides --env)")
	rootCmd.PersistentFlags().StringVar(&orgName, "org", "", "Organization name (e.g. sandbox, memorialcare)")
	rootCmd.PersistentFlags().StringVar(&envName, "env", "", "Named environment (e.g. prod, staging, dev) — sets base URL from config")
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "table", "Output format: table or json")

	// Bind flags to viper
	viper.BindPFlag("api_key", rootCmd.PersistentFlags().Lookup("api-key"))
	viper.BindPFlag("base_url", rootCmd.PersistentFlags().Lookup("base-url"))
	viper.BindPFlag("org", rootCmd.PersistentFlags().Lookup("org"))
	viper.BindPFlag("env", rootCmd.PersistentFlags().Lookup("env"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	// Register subcommands
	rootCmd.AddCommand(agentsCmd())
	rootCmd.AddCommand(channelsCmd())
	rootCmd.AddCommand(conversationsCmd())
	rootCmd.AddCommand(promptsCmd())
	rootCmd.AddCommand(toolsCmd())
	rootCmd.AddCommand(sessionsCmd())
	rootCmd.AddCommand(outboundCmd())
	rootCmd.AddCommand(usersCmd())
	rootCmd.AddCommand(directoryCmd())
	rootCmd.AddCommand(insightsCmd())
	rootCmd.AddCommand(customMessagesCmd())
	rootCmd.AddCommand(languageGroupsCmd())
	rootCmd.AddCommand(organizationsCmd())
	rootCmd.AddCommand(schemaCmd())
	rootCmd.AddCommand(dataSourcesCmd())
	rootCmd.AddCommand(voiceGroupsCmd())
	rootCmd.AddCommand(servicesCmd())
	rootCmd.AddCommand(rolesCmd())
	rootCmd.AddCommand(incidentsCmd())
	rootCmd.AddCommand(pronunciationsCmd())
	rootCmd.AddCommand(sessionLabelsCmd())
	rootCmd.AddCommand(sessionDebugCmd())
	rootCmd.AddCommand(takeoutsCmd())
	rootCmd.AddCommand(eventsCmd())
	rootCmd.AddCommand(permissionsCmd())
	rootCmd.AddCommand(conversationConfigCmd())
	rootCmd.AddCommand(dashboardsCmd())
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			configDir := filepath.Join(home, ".syllable")
			viper.AddConfigPath(configDir)
			viper.SetConfigName("config")
			viper.SetConfigType("yaml")
		}
	}

	// Environment variables
	viper.SetEnvPrefix("")
	viper.BindEnv("api_key", "SYLLABLE_API_KEY")
	viper.BindEnv("base_url", "SYLLABLE_BASE_URL")
	viper.BindEnv("env", "SYLLABLE_ENV")
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("base_url", "https://api.syllable.cloud")
	viper.SetDefault("output", "table")

	// Read config file (ignore errors if not found)
	viper.ReadInConfig()
}

func initClient() {
	url := resolveBaseURL()
	key := resolveAPIKey()

	if key == "" {
		fmt.Fprintln(os.Stderr, "Error: API key not set. Use --api-key, --org <name>, SYLLABLE_API_KEY, or configure orgs in ~/.syllable/config.yaml")
		os.Exit(1)
	}

	apiClient = client.New(url, key)
}

// resolveAPIKey determines the API key to use.
// Priority (when --org is set): orgs.<org>.envs.<env>.api_key > orgs.<org>.api_key
// Priority (no --org):          --api-key flag > SYLLABLE_API_KEY env var
func resolveAPIKey() string {
	org := viper.GetString("org")
	if org == "" {
		org = viper.GetString("default_org")
	}

	if org != "" {
		orgs := viper.GetStringMap("orgs")
		orgData, ok := orgs[org]
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: org %q not found in ~/.syllable/config.yaml\n", org)
			os.Exit(1)
		}
		orgMap, ok := orgData.(map[string]interface{})
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: invalid config for org %q in ~/.syllable/config.yaml\n", org)
			os.Exit(1)
		}

		// env-specific key: orgs.<org>.envs.<env>.api_key
		if env := resolveEnvName(); env != "" {
			if envs, ok := orgMap["envs"].(map[string]interface{}); ok {
				if envData, ok := envs[env].(map[string]interface{}); ok {
					if k, _ := envData["api_key"].(string); k != "" {
						return k
					}
				}
			}
		}

		// org-level key
		k, _ := orgMap["api_key"].(string)
		if k == "" {
			fmt.Fprintf(os.Stderr, "Error: no api_key found for org %q in ~/.syllable/config.yaml\n", org)
			os.Exit(1)
		}
		return k
	}

	// No org — fall back to --api-key flag or SYLLABLE_API_KEY env var
	return viper.GetString("api_key")
}

// resolveEnvName returns the active environment name from --env flag, SYLLABLE_ENV,
// or default_env in config (in that priority order).
func resolveEnvName() string {
	if env := viper.GetString("env"); env != "" {
		return env
	}
	return viper.GetString("default_env")
}

// builtinEnvURLs are recognized environment names that work without any config entry.
var builtinEnvURLs = map[string]string{
	"prod":    "https://api.syllable.cloud",
	"staging": "https://staging.syllable.cloud",
	"dev":     "https://dev.syllable.cloud",
}

// resolveBaseURL determines the base URL to use.
// Priority: --base-url flag > --env config lookup > --env builtin alias > https://api.syllable.cloud
func resolveBaseURL() string {
	// --base-url wins unconditionally
	if baseURL != "" {
		return baseURL
	}

	env := resolveEnvName()
	if env != "" {
		// Check config-defined environments first
		environments := viper.GetStringMap("environments")
		if envData, ok := environments[env]; ok {
			if envMap, ok := envData.(map[string]interface{}); ok {
				if u, _ := envMap["base_url"].(string); u != "" {
					return u
				}
			}
		}
		// Fall back to builtin aliases
		if u, ok := builtinEnvURLs[env]; ok {
			return u
		}
		// Unknown env — exit with a clear error
		fmt.Fprintf(os.Stderr, "Error: environment %q not found in ~/.syllable/config.yaml and is not a builtin (prod, staging, dev)\n", env)
		os.Exit(1)
	}

	// No env specified — use the hardcoded production default.
	// We intentionally do NOT use SYLLABLE_BASE_URL here to avoid shell env bleed.
	return "https://api.syllable.cloud"
}

func getOutputFmt() string {
	return viper.GetString("output")
}
