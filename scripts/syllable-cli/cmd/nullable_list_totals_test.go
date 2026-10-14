package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// These four SDK lists permit a missing/null total_count. A populated page
// must report how many rows it shows without inventing a collection total.
func TestSDKListsPreserveUnknownTotals(t *testing.T) {
	commands := []struct {
		name, path, item, rowMarker string
		factory                     func() *cobra.Command
		args                        []string
	}{
		{"experiments", "/api/v1/experiments/", `{"id":7,"name":"fixture","status":"draft","target_id":42,"variants":[]}`, "fixture", experimentsListCmd, nil},
		{"executions", "/api/v1/insights/workflows/42/executions", `{"id":7,"status":"completed","session_id":9,"priority":0,"retry_count":0}`, "completed", func() *cobra.Command { return insightsWorkflowRowsCmd("executions") }, []string{"42"}},
		{"sessions", "/api/v1/insights/workflows/42/sessions", `{"session_id":9,"status":"completed"}`, "completed", func() *cobra.Command { return insightsWorkflowRowsCmd("sessions") }, []string{"42"}},
		{"files", "/api/v1/insights/workflows/42/files", `{"upload_file_id":11,"filename":"fixture.mp3","status":"completed"}`, "fixture.mp3", func() *cobra.Command { return insightsWorkflowRowsCmd("files") }, []string{"42"}},
	}
	counts := []struct {
		name, field, footer string
	}{
		{"missing", "", "Showing 1 item(s)"},
		{"null", `,"total_count":null`, "Showing 1 item(s)"},
		{"known-zero", `,"total_count":0`, "Total: 0"},
		{"known-nonzero", `,"total_count":23`, "Total: 23"},
	}
	for _, resource := range commands {
		for _, count := range counts {
			response := fmt.Sprintf(`{"items":[%s]%s}`, resource.item, count.field)
			t.Run(resource.name+"/"+count.name, func(t *testing.T) {
				oldOutput, oldFields := viper.GetString("output"), fieldsFlag
				t.Cleanup(func() { viper.Set("output", oldOutput); fieldsFlag = oldFields })
				fieldsFlag = ""
				server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
					if r.Method != "GET" || r.URL.Path != resource.path {
						t.Errorf("request = %s %s, want GET %s", r.Method, r.URL.Path, resource.path)
					}
					io.WriteString(w, response)
				})
				defer server.Close()
				viper.Set("output", "table")
				cmd := resource.factory()
				cmd.SetArgs(resource.args)
				var err error
				out := captureStdout(func() { err = cmd.Execute() })
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(out, resource.rowMarker) {
					t.Fatalf("fixture row was not displayed: %s", out)
				}
				if !strings.HasSuffix(out, "\n"+count.footer+"\n") {
					t.Errorf("expected footer %q after populated row; output:\n%s", count.footer, out)
				}

				// Table rendering must not normalize null/omitted counts in JSON.
				viper.Set("output", "json")
				cmd = resource.factory()
				cmd.SetArgs(resource.args)
				out = captureStdout(func() { err = cmd.Execute() })
				if err != nil {
					t.Fatal(err)
				}
				var compact bytes.Buffer
				if err := json.Compact(&compact, []byte(out)); err != nil {
					t.Fatalf("invalid JSON response: %v", err)
				}
				if compact.String() != response {
					t.Errorf("raw JSON changed: %s, want %s", compact.String(), response)
				}
			})
		}
	}
}

func TestUnknownListTotalReportsDisplayedRowCount(t *testing.T) {
	for _, count := range []int{0, 2} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			oldOutput := viper.GetString("output")
			t.Cleanup(func() { viper.Set("output", oldOutput) })
			items := make([]map[string]interface{}, count)
			for i := range items {
				items[i] = map[string]interface{}{"id": i + 1, "name": "fixture"}
			}
			server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]interface{}{"items": items, "total_count": nil})
			})
			defer server.Close()
			viper.Set("output", "table")
			cmd := experimentsListCmd()
			var err error
			out := captureStdout(func() { err = cmd.Execute() })
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(out, fmt.Sprintf("\nShowing %d item(s)\n", count)) {
				t.Errorf("incorrect displayed row count: %s", out)
			}
		})
	}
}
