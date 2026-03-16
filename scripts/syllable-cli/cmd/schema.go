package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/syllable-ai/syllable-cli/internal/output"
	apispec "github.com/syllable-ai/syllable-cli/internal/spec"
)

func schemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Explore API data schemas",
		Example: `  # List all available schemas
  syllable schema list

  # Filter schemas by name
  syllable schema list --filter "agent"

  # Get the full schema for a specific type
  syllable schema get AgentResponse

  # Get a schema as JSON (useful for scripting create/update bodies)
  syllable schema get ChannelCreateRequest --output json`,
	}
	cmd.AddCommand(schemaListCmd())
	cmd.AddCommand(schemaGetCmd())
	return cmd
}

func schemaListCmd() *cobra.Command {
	var filter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available schemas",
		RunE: func(cmd *cobra.Command, args []string) error {
			schemas, err := loadSchemas()
			if err != nil {
				return err
			}

			names := make([]string, 0, len(schemas))
			for name := range schemas {
				if filter == "" || strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
					names = append(names, name)
				}
			}
			sort.Strings(names)

			if getOutputFmt() == "json" {
				b, _ := json.MarshalIndent(names, "", "  ")
				output.PrintJSON(b)
				return nil
			}

			rows := make([][]string, len(names))
			for i, name := range names {
				schema, _ := schemas[name].(map[string]interface{})
				desc, _ := schema["description"].(string)
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				rows[i] = []string{name, desc}
			}
			printTable([]string{"SCHEMA", "DESCRIPTION"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "Filter schema names by substring")
	return cmd
}

func schemaGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <SchemaName>",
		Short: "Print the schema definition for a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			schemas, err := loadSchemas()
			if err != nil {
				return err
			}

			name := args[0]
			schema, ok := schemas[name]
			if !ok {
				// Try case-insensitive match
				for k, v := range schemas {
					if strings.EqualFold(k, name) {
						schema = v
						name = k
						ok = true
						break
					}
				}
			}
			if !ok {
				fmt.Fprintf(os.Stderr, "Schema %q not found. Run `syllable schema list` to see available schemas.\n", args[0])
				os.Exit(1)
			}

			b, err := json.MarshalIndent(schema, "", "  ")
			if err != nil {
				return err
			}
			fmt.Printf("# %s\n\n", name)
			output.PrintJSON(b)
			return nil
		},
	}
}

func loadSchemas() (map[string]interface{}, error) {
	var apiSpec map[string]interface{}
	if err := json.Unmarshal(apispec.OpenAPI, &apiSpec); err != nil {
		return nil, fmt.Errorf("parsing openapi spec: %w", err)
	}
	components, _ := apiSpec["components"].(map[string]interface{})
	schemas, _ := components["schemas"].(map[string]interface{})
	if schemas == nil {
		return nil, fmt.Errorf("no schemas found in openapi spec")
	}
	return schemas, nil
}
