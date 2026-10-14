package cmd

import (
	"net/http"
	"testing"

	"github.com/spf13/cobra"
)

func TestSDKSpecializedBodiesPreserveExplicitValues(t *testing.T) {
	for _, tc := range []struct {
		name    string
		factory func() *cobra.Command
		args    []string
		body    string
	}{
		{"empty-phone", channelsTwilioNumbersVerifyA2pCmd, []string{"17", "--phone", ""}, `{"phone":""}`},
		{"phone-file", channelsTwilioNumbersVerifyA2pCmd, []string{"17", "--file", "$body"}, `{"phone":"","extra":{"null":null,"flag":false}}`},
		{"empty-comments", directoryRestoreCmd, []string{"17", "--comments", ""}, `{"comments":""}`},
		{"comments-file", directoryRestoreCmd, []string{"17", "--file", "$body"}, `{"comments":"","extra":{"value":0}}`},
		{"explicit-null", insightsToolsTestCmd, []string{"--file", "$body"}, `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) { assertRequestJSON(t, r, tc.body); w.Write([]byte(`{}`)) })
			defer server.Close()
			args := append([]string{}, tc.args...)
			for i := range args {
				if args[i] == "$body" {
					args[i] = bodyFile(t, tc.body)
				}
			}
			executeBodyCommand(t, tc.factory(), args...)
		})
	}
}
