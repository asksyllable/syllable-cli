package cmd

import (
	"encoding/json"
	"testing"
)

func TestExactNumericIdentifierEquality(t *testing.T) {
	for _, tc := range []struct {
		body, argument string
		want           bool
	}{
		{"17", "017", true}, {"17.0", "17", true}, {"1.7e1", "17", true},
		{"1700e-2", "17", true}, {"9007199254740993", "9007199254740993", true},
		{"9007199254740993.0", "9007199254740992", false},
		{"17.00000000000000001", "17", false}, {"1e1000000000", "17", false},
		{"-9223372036854775808.0", "-9223372036854775808", true},
		{"9223372036854775808", "9223372036854775807", false},
	} {
		t.Run(tc.body+"/"+tc.argument, func(t *testing.T) {
			if got := identifierMatches(json.Number(tc.body), tc.argument); got != tc.want {
				t.Errorf("matched=%t want%t", got, tc.want)
			}
		})
	}
	if identifierMatches("17", "017") {
		t.Fatal("string identifiers must remain exact")
	}
}
