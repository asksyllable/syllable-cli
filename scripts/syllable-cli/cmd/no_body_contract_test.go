package cmd

import (
	"io"
	"net/http"
	"testing"

	"github.com/spf13/cobra"
)

// OpenAPI declares no request body for these operations. An unsolicited empty
// JSON object still changes the wire contract and must not be synthesized.
func TestSDKBodylessOperationDefaults(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command func() *cobra.Command
		args    []string
	}{
		{"inactivate", insightsWorkflowsInactivateCmd, []string{"17"}},
		{"recording", sessionsRecordingCmd, []string{"17"}},
		{"send-email", usersSendEmailCmd, []string{"audit@example.invalid"}},
		{"takeout", takeoutsCreateCmd, nil},
		{"dashboard-list", dashboardsListCmd, nil},
		{"dashboard-events", dashboardsSessionEventsCmd, nil},
		{"dashboard-summary", dashboardsSessionSummaryCmd, nil},
		{"dashboard-transfers", dashboardsSessionTransfersCmd, nil},
		{"dashboard-sessions", dashboardsSessionsCmd, nil},
		{"dashboard-info", dashboardsFetchInfoCmd, []string{"--name", "audit"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				called = true
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if len(data) != 0 || r.Header.Get("Content-Type") != "" {
					t.Errorf("unexpected body/content type: %q / %q", data, r.Header.Get("Content-Type"))
				}
				io.WriteString(w, `{}`)
			})
			defer server.Close()
			cmd := tc.command()
			cmd.SetArgs(tc.args)
			var err error
			captureStdout(func() { err = cmd.Execute() })
			if err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("no request")
			}
		})
	}
}
