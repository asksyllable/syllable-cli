package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	orgName    string
	envName    string
	outputFmt  string
	dryRun     bool
	debugMode  bool
	fieldsFlag string
	assumeYes  bool
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
		if cmd == nil {
			return nil
		}
		if err := validateOutputFmt(); err != nil {
			return err
		}
		// Skip for commands that don't need auth. Cobra invokes this for each command in
		// the chain (root → completion → zsh); for the leaf we see cmd.Name()=="zsh", so
		// check this command and all ancestors. Also skip when root runs with no args
		// (e.g. "syllable" or "syllable --version").
		if cmdRequiresNoAuth(cmd) {
			return nil
		}
		if cmd.Root() == cmd && len(args) == 0 {
			return nil
		}
		return initClient()
	},
}

// Execute runs the root command.
func Execute() {
	rootCmd.SilenceErrors = true
	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		var dryRun *client.DryRunResult
		if errors.As(err, &dryRun) {
			output.PrintJSON(dryRun.Output)
			return
		}
		printError(cmd, err)
		os.Exit(1)
	}
}

func printError(cmd *cobra.Command, err error) {
	hint := hintForError(cmd, err)
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

// hintForError returns an actionable suggestion for common errors. cmd is the
// command that failed, used to tailor hints; it may be nil.
func hintForError(cmd *cobra.Command, err error) string {
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
		return hint404(cmd)
	case 409:
		if listPath := siblingListPath(cmd); listPath != "" {
			return fmt.Sprintf("A resource with this name already exists. Use `%s` to find it.", listPath)
		}
		return "A resource with this name already exists. Use the `list` subcommand to find it."
	case 422, 400:
		return hint422(apiErr.Body)
	case 500, 502, 503, 504:
		return "Server error. This may be temporary — try again shortly."
	}
	return ""
}

// hint404 builds a 404 hint aware of which command failed. A 404 from `list`
// means the org genuinely has nothing configured — pointing the user at `list`
// would be a wild goose chase, so the API's own detail message stands alone.
// Name-keyed commands (`get <tool-name>`, `update <user-email>`) steer users to
// the right column instead of blaming an ID.
func hint404(cmd *cobra.Command) string {
	if cmd == nil {
		return "Resource not found. Use the `list` subcommand to find valid IDs."
	}
	if cmd.Name() == "list" {
		return ""
	}
	listPath := siblingListPath(cmd)
	switch {
	case strings.Contains(cmd.Use, "-name>"):
		if listPath != "" {
			return fmt.Sprintf("This command takes a name, not a numeric ID — copy the NAME column from `%s`.", listPath)
		}
		return "This command takes a name, not a numeric ID."
	case strings.Contains(cmd.Use, "-email>"):
		if listPath != "" {
			return fmt.Sprintf("This command takes an email address — copy the EMAIL column from `%s`.", listPath)
		}
		return "This command takes an email address, not a numeric ID."
	case listPath != "":
		return fmt.Sprintf("Resource not found. Use `%s` to find valid IDs.", listPath)
	}
	return "Resource not found. Use the `list` subcommand to find valid IDs."
}

// siblingListPath returns the full command path of the `list` sibling of cmd
// (e.g. "syllable tools list"), or "" when cmd has no list sibling.
func siblingListPath(cmd *cobra.Command) string {
	if cmd == nil || cmd.Parent() == nil {
		return ""
	}
	for _, sib := range cmd.Parent().Commands() {
		if sib.Name() == "list" {
			return sib.CommandPath()
		}
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

// noAuthCommandNames is the set of Cobra command names that run without API config.
var noAuthCommandNames = map[string]struct{}{
	"help": {}, "completion": {}, "version": {}, "setup": {}, "status": {}, "envs": {},
}

// cmdRequiresNoAuth reports whether the command or any ancestor is a no-auth command
// (help, completion, version, setup). Used so that e.g. "syllable completion zsh" works
// without ~/.syllable/config.yaml or SYLLABLE_API_KEY.
func cmdRequiresNoAuth(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if _, ok := noAuthCommandNames[c.Name()]; ok {
			return true
		}
	}
	return false
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.syllable/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&orgName, "org", "", "Organization name (e.g. sandbox)")
	rootCmd.PersistentFlags().StringVar(&envName, "env", "", "Named environment — sets base URL from config (default: prod)")
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "table", "Output format: table or json")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Print the request that would be sent without executing it")
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Print HTTP request and response details to stderr")
	rootCmd.PersistentFlags().StringVar(&fieldsFlag, "fields", "", "Comma-separated columns to show in table output (e.g. id,name,type)")
	rootCmd.PersistentFlags().BoolVarP(&assumeYes, "yes", "y", false, "Skip the confirmation prompt on destructive commands (for non-interactive use)")

	// Bind flags to viper
	viper.BindPFlag("org", rootCmd.PersistentFlags().Lookup("org"))
	viper.BindPFlag("env", rootCmd.PersistentFlags().Lookup("env"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	// Register subcommands
	rootCmd.AddCommand(setupCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(envsCmd())
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

	// Defaults
	viper.SetDefault("output", "table")

	// Read config file (ignore errors if not found)
	viper.ReadInConfig()
}

func initClient() error {
	url, err := resolveBaseURL()
	if err != nil {
		return err
	}
	key, err := resolveAPIKey()
	if err != nil {
		return err
	}

	if key == "" {
		return errors.New("not configured — run `syllable setup` and select an org with --org <name>, or set SYLLABLE_API_KEY for non-interactive use")
	}

	apiClient = client.New(url, key)
	apiClient.DryRun = dryRun
	apiClient.Verbose = debugMode
	return nil
}

// resolveAPIKey determines the API key to use from config.
// Priority: orgs.<org>.envs.<env>.api_key > orgs.<org>.api_key
// Org is taken from --org flag or default_org in config. It returns "" with a
// nil error when nothing is configured, so the caller can render a single
// "not configured" message; config problems (unknown org, missing key) are
// returned as errors that flow through the normal --output json error path (#128).
func resolveAPIKey() (string, error) {
	// Non-interactive auth: an explicit key in the environment takes priority,
	// so the CLI works in CI and automation without a ~/.syllable/config.yaml.
	if k := os.Getenv("SYLLABLE_API_KEY"); k != "" {
		return k, nil
	}

	org := strings.ToLower(viper.GetString("org"))
	if org == "" {
		org = strings.ToLower(viper.GetString("default_org"))
	}

	if org == "" {
		return "", nil
	}

	orgs := viper.GetStringMap("orgs")
	orgData, ok := orgs[org]
	if !ok {
		return "", fmt.Errorf("org %q not found in ~/.syllable/config.yaml — run `syllable setup` to add it", org)
	}
	orgMap, ok := orgData.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid config for org %q in ~/.syllable/config.yaml", org)
	}

	// env-specific key: orgs.<org>.envs.<env>.api_key
	if env := resolveEnvName(); env != "" {
		if envs, ok := orgMap["envs"].(map[string]interface{}); ok {
			if envData, ok := envs[env].(map[string]interface{}); ok {
				if k, _ := envData["api_key"].(string); k != "" {
					return k, nil
				}
			}
		}
	}

	// org-level key
	k, _ := orgMap["api_key"].(string)
	if k == "" {
		// If the org has per-env keys but no env was specified, give a targeted error.
		if envs, ok := orgMap["envs"].(map[string]interface{}); ok && len(envs) > 0 {
			names := make([]string, 0, len(envs))
			for e := range envs {
				names = append(names, e)
			}
			sort.Strings(names)
			return "", fmt.Errorf("org %q has per-environment keys (%s) but no environment was specified — use --env <name> or set default_env in `syllable setup`", org, strings.Join(names, ", "))
		}
		return "", fmt.Errorf("no api_key found for org %q — run `syllable setup` to configure it", org)
	}
	return k, nil
}

// resolveEnvName returns the active environment name from --env flag, SYLLABLE_ENV,
// or default_env in config (in that priority order).
func resolveEnvName() string {
	if env := viper.GetString("env"); env != "" {
		return env
	}
	return viper.GetString("default_env")
}

// resolveBaseURL determines the base URL from config.
// Priority: --env config lookup > --env builtin alias (prod) > https://api.syllable.cloud
func resolveBaseURL() (string, error) {
	env := resolveEnvName()
	if env != "" {
		// Check config-defined environments first
		environments := viper.GetStringMap("environments")
		if envData, ok := environments[env]; ok {
			if envMap, ok := envData.(map[string]interface{}); ok {
				if u, _ := envMap["base_url"].(string); u != "" {
					return u, nil
				}
			}
		}
		// Built-in alias: prod
		if env == "prod" {
			return "https://api.syllable.cloud", nil
		}
		// Unknown env — surface a clear error through the normal error path.
		return "", fmt.Errorf("environment %q not found in ~/.syllable/config.yaml — add it with `syllable setup`", env)
	}

	return "https://api.syllable.cloud", nil
}

func getOutputFmt() string {
	return viper.GetString("output")
}

// validateOutputFmt rejects --output values other than table or json, so a typo
// like `-o jsno` fails loudly instead of silently rendering a table into a
// script that expected JSON (#119).
func validateOutputFmt() error {
	switch getOutputFmt() {
	case "", "table", "json":
		return nil
	default:
		return fmt.Errorf("invalid --output %q: must be \"table\" or \"json\"", getOutputFmt())
	}
}

// readFile reads a file by path, or reads from stdin if path is "-".
func readFile(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

// ensureBodyIdentifier reconciles an update command's positional identifier
// with its --file body. The platform's collection-style PUT endpoints (e.g.
// PUT /api/v1/agents/) route by an identifier field inside the body, not the
// URL — without this guard the positional arg is accepted but ignored, and the
// body's value silently decides which resource gets updated (#68).
//
// When the body lacks key, the positional value is injected (as a JSON number
// when numeric is true). When the body carries a conflicting value, the update
// is refused rather than guessing which resource the user meant.
func ensureBodyIdentifier(body interface{}, key, positional string, numeric bool) error {
	m, ok := body.(map[string]interface{})
	if !ok {
		// Non-object bodies cannot carry the identifier; let the API validate.
		return nil
	}
	if existing, ok := m[key]; ok && existing != nil {
		if !identifierMatches(existing, positional) {
			return fmt.Errorf("positional argument %q conflicts with %s=%v in the request body — make them match or drop %s from the body", positional, key, existing, key)
		}
		return nil
	}
	if numeric {
		n, err := strconv.ParseInt(positional, 10, 64)
		if err != nil {
			return fmt.Errorf("expected a numeric identifier, got %q", positional)
		}
		m[key] = n
		return nil
	}
	m[key] = positional
	return nil
}

// parseIDFlag converts a string flag whose value the API expects as a JSON
// integer. Inline create bodies historically sent these as strings, which only
// worked while the backend coerced them; sending the right type also yields a
// clearer error than a server-side 422 (#116).
func parseIDFlag(flagName, value string) (int64, error) {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("--%s must be a whole number, got %q", flagName, value)
	}
	return n, nil
}

// confirmDelete prompts for confirmation before a destructive command runs (#118).
// It returns nil — proceed without prompting — when --yes is set or --dry-run is
// in effect (dry-run never executes the call). When stdin is not a terminal and
// --yes was not passed, it refuses with an actionable error so scripts can't
// delete by accident; otherwise it asks for y/N on stderr and reads the answer
// from stdin. The target shown is derived from the command path and positional
// args, e.g. "syllable agents delete 42".
func confirmDelete(cmd *cobra.Command, args []string) error {
	if assumeYes || dryRun {
		return nil
	}
	target := cmd.CommandPath()
	if len(args) > 0 {
		target += " " + strings.Join(args, " ")
	}
	if !isStdinTTY() {
		return fmt.Errorf("refusing to run %q without confirmation; pass --yes to confirm non-interactively", target)
	}
	fmt.Fprintf(os.Stderr, "About to run: %s\nThis is destructive and cannot be undone. Continue? [y/N]: ", target)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return fmt.Errorf("aborted")
	}
}

// isStdinTTY reports whether stdin is connected to a terminal (vs a pipe/file),
// used to decide whether an interactive confirmation prompt is possible.
func isStdinTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// identifierMatches compares a JSON body value against a positional CLI string.
func identifierMatches(existing interface{}, positional string) bool {
	switch v := existing.(type) {
	case string:
		return v == positional
	case float64: // encoding/json decodes JSON numbers to float64
		p, err := strconv.ParseFloat(positional, 64)
		return err == nil && p == v
	case json.Number:
		return v.String() == positional
	}
	return false
}

// printTable prints a table, applying --fields column filtering if set.
func printTable(headers []string, rows [][]string) {
	if fieldsFlag != "" {
		fields := strings.Split(fieldsFlag, ",")
		var unknown []string
		headers, rows, unknown = output.FilterColumns(headers, rows, fields)
		if len(unknown) > 0 {
			fmt.Fprintf(os.Stderr, "warning: unknown --fields column(s): %s\n", strings.Join(unknown, ", "))
		}
	}
	output.PrintTable(headers, rows)
}
