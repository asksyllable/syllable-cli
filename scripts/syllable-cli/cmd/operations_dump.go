package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// operationEntry describes the command serving one SDK operation.
type operationEntry struct {
	Command string `json:"command"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	// Flags lists the command's own flags other than --file and the query
	// flags bound from the spec. On an operation with a request body, these are
	// treated as flags that build the body inline (an over-approximation, so
	// automation errs toward asking a human).
	Flags []string `json:"flags"`
}

// operationsDumpCmd prints the SDK operation → command map as JSON. The
// spec-sync automation (scripts/ci/classify_drift.py) uses it to tell a
// request body the CLI passes through from --file from one it builds out of
// inline flags. Hidden, and needs no API key.
func operationsDumpCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "__operations",
		Short:  "Print the SDK operation to command map as JSON (for automation)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := operationEntries(cmd.Root())
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(entries)
		},
	}
}

func operationEntries(root *cobra.Command) (map[string]operationEntry, error) {
	operations, err := loadSDKOperations()
	if err != nil {
		return nil, err
	}
	out := make(map[string]operationEntry, len(sdkOperationCommands))
	for id, argv := range sdkOperationCommands {
		target, _, err := root.Find(argv)
		if err != nil || target.RunE == nil {
			return nil, fmt.Errorf("SDK operation %s has no registered command: %v", id, err)
		}
		operation, ok := operations[id]
		if !ok {
			return nil, fmt.Errorf("CLI mapping references unknown SDK operation %s", id)
		}
		query := map[string]bool{}
		for _, parameter := range operation.Parameters {
			if parameter.In == "query" {
				query[sdkQueryFlagName(target, parameter.Name)] = true
			}
		}
		flags := []string{}
		target.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
			if f.Name == "file" || f.Name == "help" || query[f.Name] {
				return
			}
			flags = append(flags, f.Name)
		})
		sort.Strings(flags)
		out[id] = operationEntry{
			Command: strings.TrimPrefix(target.CommandPath(), root.Name()+" "),
			Method:  operation.Method,
			Path:    operation.Path,
			Flags:   flags,
		}
	}
	return out, nil
}
