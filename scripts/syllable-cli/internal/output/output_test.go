package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

// captureStdout captures stdout output from a function call.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "he..."},
		{"hello world", 8, "hello..."},
		{"hello world", 11, "hello world"},
		{"ab", 1, "a"},
		{"ab", 2, "ab"},
		{"ab", 3, "ab"},
		{"abcdef", 3, "abc"},
		{"", 5, ""},
		{"a", 0, ""},
	}

	for _, tt := range tests {
		got := Truncate(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}

func TestTruncateMultibyteUTF8(t *testing.T) {
	// Each rune here is multiple bytes; byte-slicing would split one mid-rune
	// and emit invalid UTF-8 (#131).
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"日本語テキスト", 4, "日..."},     // 7 runes -> r[:1] + "..."
		{"日本語テキスト", 7, "日本語テキスト"},  // fits exactly by rune count
		{"Привет мир", 5, "Пр..."}, // Cyrillic
		{"한국어", 1, "한"},            // max<=3 path, no ellipsis
	}
	for _, tt := range tests {
		got := Truncate(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("Truncate(%q, %d) = %q is not valid UTF-8", tt.input, tt.max, got)
		}
	}
}

func TestFilterColumns(t *testing.T) {
	headers := []string{"ID", "NAME", "TYPE"}
	rows := [][]string{{"1", "alpha", "x"}, {"2", "beta", "y"}}

	// Case-insensitive match, reordered by request, unknown names reported.
	h, r, unknown := FilterColumns(headers, rows, []string{"name", "id", "bogus"})
	if strings.Join(h, ",") != "NAME,ID" {
		t.Errorf("expected NAME,ID, got %v", h)
	}
	if r[0][0] != "alpha" || r[0][1] != "1" {
		t.Errorf("expected reordered row [alpha 1], got %v", r[0])
	}
	if len(unknown) != 1 || unknown[0] != "bogus" {
		t.Errorf("expected unknown=[bogus], got %v", unknown)
	}

	// All-unknown → original table returned unchanged, unknowns still reported.
	h, r, unknown = FilterColumns(headers, rows, []string{"nope"})
	if len(h) != 3 || len(r) != 2 {
		t.Errorf("expected original table on zero matches, got headers=%v rows=%v", h, r)
	}
	if len(unknown) != 1 || unknown[0] != "nope" {
		t.Errorf("expected unknown=[nope], got %v", unknown)
	}

	// A short row is padded, not panicking.
	h, r, _ = FilterColumns(headers, [][]string{{"1"}}, []string{"id", "type"})
	if len(r[0]) != 2 || r[0][0] != "1" || r[0][1] != "" {
		t.Errorf("expected padded row [1 \"\"], got %v", r[0])
	}
}

func TestPrintTable(t *testing.T) {
	out := captureStdout(func() {
		PrintTable(
			[]string{"ID", "NAME"},
			[][]string{
				{"1", "alpha"},
				{"2", "beta"},
			},
		)
	})

	if !strings.Contains(out, "ID") {
		t.Error("output should contain header 'ID'")
	}
	if !strings.Contains(out, "NAME") {
		t.Error("output should contain header 'NAME'")
	}
	if !strings.Contains(out, "alpha") {
		t.Error("output should contain 'alpha'")
	}
	if !strings.Contains(out, "beta") {
		t.Error("output should contain 'beta'")
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 { // header + separator + 2 data rows
		t.Errorf("expected 4 lines, got %d: %v", len(lines), lines)
	}

	// Separator line should contain dashes
	if !strings.Contains(lines[1], "--") {
		t.Errorf("separator line should contain dashes, got %q", lines[1])
	}
}

func TestPrintTableEmpty(t *testing.T) {
	out := captureStdout(func() {
		PrintTable([]string{"ID", "NAME"}, [][]string{})
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 { // header + separator only
		t.Errorf("expected 2 lines for empty table, got %d", len(lines))
	}
}

func TestPrintJSON(t *testing.T) {
	out := captureStdout(func() {
		PrintJSON([]byte(`{"id":"1","name":"test"}`))
	})

	if !strings.Contains(out, "\"id\": \"1\"") {
		t.Errorf("expected pretty-printed JSON with indentation, got %q", out)
	}
	if !strings.Contains(out, "\"name\": \"test\"") {
		t.Errorf("expected pretty-printed JSON, got %q", out)
	}
}

func TestPrintJSONInvalid(t *testing.T) {
	// Invalid JSON should fall back to printing raw string
	out := captureStdout(func() {
		PrintJSON([]byte(`not json`))
	})

	if !strings.Contains(out, "not json") {
		t.Errorf("invalid JSON should print raw, got %q", out)
	}
}
