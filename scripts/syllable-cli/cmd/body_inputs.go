package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// decodeRequestJSON preserves JSON numbers, including integers too large for
// float64, while retaining arbitrary nested fields in full-body input.
func decodeRequestJSON(data []byte, body interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(body); err != nil {
		return err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("expected one JSON value")
		}
		return err
	}
	// A supplied JSON null is distinct from an absent body in client.Do.
	if value, ok := body.(*interface{}); ok && *value == nil {
		*value = json.RawMessage("null")
	}
	return nil
}

// requestInteger compares number spellings without float64 rounding. Decimal
// points, exponent notation, and trailing zeroes may still describe an exact
// int64. Bound the resulting digits before padding, even for enormous exponents.
func requestInteger(value json.Number) (int64, bool) {
	text := value.String()
	sign := ""
	if strings.HasPrefix(text, "-") {
		sign = "-"
		text = text[1:]
	}
	parts := strings.FieldsFunc(text, func(r rune) bool { return r == 'e' || r == 'E' })
	if len(parts) == 0 || len(parts) > 2 {
		return 0, false
	}
	exponent := int64(0)
	if len(parts) == 2 {
		var err error
		exponent, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, false
		}
	}
	fraction := 0
	if dot := strings.IndexByte(parts[0], '.'); dot >= 0 {
		fraction = len(parts[0]) - dot - 1
	}
	digits := strings.TrimLeft(strings.ReplaceAll(parts[0], ".", ""), "0")
	if digits == "" {
		return 0, true
	}
	trimmed := strings.TrimRight(digits, "0")
	scale := int64(fraction - (len(digits) - len(trimmed)))
	if exponent < scale || exponent > scale+19 {
		return 0, false
	}
	zeroes := exponent - scale
	if int64(len(trimmed))+zeroes > 19 {
		return 0, false
	}
	number, err := strconv.ParseInt(sign+trimmed+strings.Repeat("0", int(zeroes)), 10, 64)
	return number, err == nil
}

// exclusiveBodyFile makes full JSON input authoritative without quietly
// ignoring, or overwriting it with, explicitly supplied convenience flags.
func exclusiveBodyFile(cmd *cobra.Command, file string, bodyFlags ...string) error {
	if file == "" {
		return nil
	}
	for _, flag := range bodyFlags {
		if cmd.Flags().Changed(flag) {
			return fmt.Errorf("--file cannot be combined with --%s; put all request fields in the JSON body", flag)
		}
	}
	return nil
}
