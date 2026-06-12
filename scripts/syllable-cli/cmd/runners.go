package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/asksyllable/syllable-cli/internal/output"
	"github.com/spf13/cobra"
)

// This file holds small generic helpers that capture the list/get/delete/body
// boilerplate repeated across resource commands, so a plain CRUD resource is a
// few closures instead of ~200 hand-copied lines (#141). Migrate resources to
// these incrementally; anything with special behavior (binary output, ID
// fallback, custom timeouts, multipart) keeps its hand-written command.

// readJSONBody reads a --file path (or "-" for stdin) and unmarshals it into a
// generic body, with the same error wording every create/update used.
func readJSONBody(file string) (interface{}, error) {
	data, err := readFile(file)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	var body interface{}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("parsing JSON file: %w", err)
	}
	return body, nil
}

// listQuery builds the standard paginated list path with an optional search
// filter (the value is URL-escaped).
func listQuery(base string, page, limit int, searchField, search string) string {
	path := fmt.Sprintf("%s?page=%d&limit=%d", base, page, limit)
	if search != "" {
		path += fmt.Sprintf("&search_fields=%s&search_field_values=%s", searchField, url.QueryEscape(search))
	}
	return path
}

// runList GETs a paginated collection and prints JSON (with -o json) or a table
// followed by a "Total: N" footer. row maps one decoded item to its cells.
func runList[T any](path string, headers []string, row func(T) []string) error {
	data, _, err := apiClient.Get(path)
	if err != nil {
		return err
	}
	if getOutputFmt() == "json" {
		output.PrintJSON(data)
		return nil
	}
	var result struct {
		Items      []T `json:"items"`
		TotalCount int `json:"total_count"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		output.PrintJSON(data)
		return nil
	}
	rows := make([][]string, len(result.Items))
	for i, it := range result.Items {
		rows[i] = row(it)
	}
	printTable(headers, rows)
	fmt.Printf("\nTotal: %d\n", result.TotalCount)
	return nil
}

// runGet GETs a single resource and prints JSON (with -o json) or a FIELD/VALUE
// table. fields maps the decoded item to its rows.
func runGet[T any](path string, fields func(T) [][]string) error {
	data, _, err := apiClient.Get(path)
	if err != nil {
		return err
	}
	if getOutputFmt() == "json" {
		output.PrintJSON(data)
		return nil
	}
	var item T
	if err := json.Unmarshal(data, &item); err != nil {
		output.PrintJSON(data)
		return nil
	}
	printTable([]string{"FIELD", "VALUE"}, fields(item))
	return nil
}

// runDelete confirms (unless --yes / --dry-run), DELETEs path, and prints the
// response body or a "<Label> <id> deleted." line.
func runDelete(cmd *cobra.Command, args []string, path, label string) error {
	if err := confirmDelete(cmd, args); err != nil {
		return err
	}
	data, _, err := apiClient.Delete(path)
	if err != nil {
		return err
	}
	if len(data) > 0 {
		output.PrintJSON(data)
	} else {
		fmt.Printf("%s %s deleted.\n", label, args[0])
	}
	return nil
}
