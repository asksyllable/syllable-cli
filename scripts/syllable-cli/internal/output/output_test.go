package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
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
