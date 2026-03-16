package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/asksyllable/syllable-cli/internal/client"
	"github.com/asksyllable/syllable-cli/internal/output"
)

// Version is set at build time via -ldflags "-X github.com/asksyllable/syllable-cli/cmd.Version=x.y.z"
var Version = "dev"

var (
	cfgFile    string
	apiKey     string
	baseURL    string
	orgName    string
	envName    string
	outputFmt  string
	dryRun     bool
	debugMode  bool
	fieldsFlag string
	apiClient  *client.Client
)

// rootCmd is the base command for the syllable CLI.
var rootCmd = &cobra.Command{
	Use:          "syllable",
	Version:      Version,
	Short:        "Syllable CLI - manage your Syllable AI platform",
	SilenceUsage: true,
	Long: `syllable is a CLI tool for managing your Syllable AI platform resources.

It supports agents, channels, conversations, prompts, tools, sessions,
outbound campaigns, users, directory, insights, custom messages, language groups, and organizations.

Feedback: https://github.com/asksyllable/syllable-cli/issues`,
	Example: `  # List agents
  syllable agents list

  # Get JSON output for scripting
  syllable agents get 42 --output json

  # Use a specific org and environment
  syllable --org acme --env staging agents list

  # Enable shell completion (bash)
  syllable completion bash > /etc/bash_completion.d/syllable

  # Enable shell completion (zsh)
  syllable completion zsh > "${fpath[1]}/_syllable"`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip for commands that don't need auth
		if cmd.Name() == "help" || cmd.Name() == "completion" || cmd.Name() == "version" {
			return nil
		}
		initClient()
		return nil
	},
}

// Execute runs the root command.
func Execute() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		var dryRun *client.DryRunResult
		if errors.As(err, &dryRun) {
			output.PrintJSON(dryRun.Output)
			return
		}
		printError(err)
		os.Exit(1)
	}
}

func printError(err error) {
	hint := hintForError(err)
	if getOutputFmt() == "json" {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			var detail json.RawMessage
			if json.Unmarshal(apiErr.Body, &detail) == nil {
				obj := map[string]interface{}{
					"status_code": apiErr.StatusCode,
					"detail":      detail,
				}
				if hint != "" {
					obj["hint"] = hint
				}
				out, _ := json.Marshal(map[string]interface{}{"error": obj})
				fmt.Fprintln(os.Stderr, string(out))
				return
			}
		}
		obj := map[string]interface{}{"message": err.Error()}
		if hint != "" {
			obj["hint"] = hint
		}
		out, _ := json.Marshal(map[string]interface{}{"error": obj})
		fmt.Fprintln(os.Stderr, string(out))
		return
	}
	fmt.Fprintln(os.Stderr, "Error:", err)
	if hint != "" {
		fmt.Fprintln(os.Stderr, "Hint: "+hint)
	}
}

// hintForError returns an actionable suggestion for common errors.
func hintForError(err error) string {
	// Non-API errors: missing required flags
	msg := err.Error()
	if strings.Contains(msg, "required flags") || strings.Contains(msg, "use --file") {
		return "Use `syllable schema list` to browse schemas, then `syllable schema get <TypeName>` to see all fields."
	}

	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		return ""
	}

	switch apiErr.StatusCode {
	case 401:
		return "Your API key may be invalid or expired. Verify it with: syllable users me"
	case 403:
		return "You don't have permission for this action. Check: syllable permissions list"
	case 404:
		return "Resource not found. Use the `list` subcommand to find valid IDs."
	case 409:
		return "A resource with this name already exists. Use the `list` subcommand to find it."
	case 422, 400:
		return hint422(apiErr.Body)
	case 500, 502, 503, 504:
		return "Server error. This may be temporary — try again shortly."
	}
	return ""
}

// hint422 parses a FastAPI validation error body and returns a field-specific hint.
func hint422(body []byte) string {
	var resp struct {
		Detail []struct {
			Loc []string `json:"loc"`
			Msg string   `json:"msg"`
		} `json:"detail"`
	}
	if json.Unmarshal(body, &resp) == nil && len(resp.Detail) > 0 {
		var fields []string
		for _, d := range resp.Detail {
			if len(d.Loc) > 0 {
				fields = append(fields, d.Loc[len(d.Loc)-1])
			}
		}
		if len(fields) > 0 {
			return fmt.Sprintf("Validation failed on: %s. Use `syllable schema list` to find the schema, then `syllable schema get <TypeName>` to see required fields.", strings.Join(fields, ", "))
		}
	}
	return "Validation failed. Use `syllable schema list` to find the schema for this resource, then `syllable schema get <TypeName>` to see required fields."
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
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Print the request that would be sent without executing it")
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Print HTTP request and response details to stderr")
	rootCmd.PersistentFlags().StringVar(&fieldsFlag, "fields", "", "Comma-separated columns to show in table output (e.g. id,name,type)")

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
	apiClient.DryRun = dryRun
	apiClient.Verbose = debugMode
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

// readFile reads a file by path, or reads from stdin if path is "-".
func readFile(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

// printTable prints a table, applying --fields column filtering if set.
func printTable(headers []string, rows [][]string) {
	if fieldsFlag != "" {
		fields := strings.Split(fieldsFlag, ",")
		headers, rows = output.FilterColumns(headers, rows, fields)
	}
	output.PrintTable(headers, rows)
}
