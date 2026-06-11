package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"unicode/utf8"
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
// Columns are returned in the order specified by fields. The names of any requested fields that
// don't match a header are returned in unknown so the caller can warn (#120). If no valid fields
// are matched, the original headers and rows are returned unchanged (with unknown still populated).
func FilterColumns(headers []string, rows [][]string, fields []string) (filteredHeaders []string, filteredRows [][]string, unknown []string) {
	// Map lowercase header name -> column index
	index := make(map[string]int, len(headers))
	for i, h := range headers {
		index[strings.ToLower(h)] = i
	}

	var keep []int
	for _, f := range fields {
		trimmed := strings.TrimSpace(f)
		if trimmed == "" {
			continue
		}
		if idx, ok := index[strings.ToLower(trimmed)]; ok {
			keep = append(keep, idx)
			filteredHeaders = append(filteredHeaders, headers[idx])
		} else {
			unknown = append(unknown, trimmed)
		}
	}

	if len(keep) == 0 {
		return headers, rows, unknown
	}

	filteredRows = make([][]string, len(rows))
	for i, row := range rows {
		filtered := make([]string, len(keep))
		for j, idx := range keep {
			if idx < len(row) {
				filtered[j] = row[idx]
			}
		}
		filteredRows[i] = filtered
	}

	return filteredHeaders, filteredRows, unknown
}

// Truncate truncates a string to max runes, appending "..." if needed.
// It counts and slices by rune, not byte, so multibyte UTF-8 (e.g. Mandarin,
// Korean, Cyrillic transcripts) is never split mid-character (#131).
func Truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}
