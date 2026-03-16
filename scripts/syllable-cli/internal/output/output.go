package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// PrintTable prints data as a formatted table using tab-separated columns.
func PrintTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print headers
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "\t")
		}
		fmt.Fprint(w, h)
	}
	fmt.Fprintln(w)

	// Print separator
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "\t")
		}
		for range h {
			fmt.Fprint(w, "-")
		}
	}
	fmt.Fprintln(w)

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(w, "\t")
			}
			fmt.Fprint(w, cell)
		}
		fmt.Fprintln(w)
	}

	w.Flush()
}

// PrintJSON pretty-prints JSON data.
func PrintJSON(data []byte) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		// If we can't pretty-print, just print raw
		fmt.Println(string(data))
		return
	}
	fmt.Println(buf.String())
}

// FilterColumns filters headers and rows to only the requested column names (case-insensitive).
// Columns are returned in the order specified by fields. Unknown field names are ignored.
// If no valid fields are matched, the original headers and rows are returned unchanged.
func FilterColumns(headers []string, rows [][]string, fields []string) ([]string, [][]string) {
	// Map lowercase header name -> column index
	index := make(map[string]int, len(headers))
	for i, h := range headers {
		index[strings.ToLower(h)] = i
	}

	var keep []int
	var filteredHeaders []string
	for _, f := range fields {
		if idx, ok := index[strings.ToLower(strings.TrimSpace(f))]; ok {
			keep = append(keep, idx)
			filteredHeaders = append(filteredHeaders, headers[idx])
		}
	}

	if len(keep) == 0 {
		return headers, rows
	}

	filteredRows := make([][]string, len(rows))
	for i, row := range rows {
		filtered := make([]string, len(keep))
		for j, idx := range keep {
			if idx < len(row) {
				filtered[j] = row[idx]
			}
		}
		filteredRows[i] = filtered
	}

	return filteredHeaders, filteredRows
}

// Truncate truncates a string to max length, appending "..." if needed.
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
