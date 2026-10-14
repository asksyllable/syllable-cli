package cmd

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/asksyllable/syllable-cli/internal/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// These fields were added inside existing request models. A matching embedded
// schema alone isn't enough: the corresponding commands must preserve nested
// JSON rather than silently rebuilding a body with only known fields.
func TestSDKCatchUpPayloadPassthrough(t *testing.T) {
	old := apiClient
	t.Cleanup(func() { apiClient = old })
	apiClient = client.New("https://api.example.invalid", "unit-test")
	apiClient.DryRun = true
	for _, tc := range []struct {
		name           string
		factory        func() *cobra.Command
		create, update []string
		body           string
	}{
		{"holiday greeting", customMessagesCmd, []string{"create"}, []string{"update", "17"},
			`{"id":17,"name":"audit","type":"greeting","text":"Hello","rules":[{"holidays":["christmas-day"]}]}`},
		{"tool response display", toolsCmd, []string{"create"}, []string{"update", "audit"},
			`{"id":17,"name":"audit","service_id":1,"definition":{"type":"endpoint","response_display":{"version":1,"type":"card-list","items_path":"items","mapping":{"content.header":{"path":"title"}}}}}`},
		{"directory SIP transfer", directoryCmd, []string{"create"}, []string{"update", "17"},
			`{"id":17,"name":"audit","type":"Operator","extensions":[{"name":"work","numbers":[{"number":"sip:audit@example.invalid","sip_transfer_mode":"refer"}]}]}`},
		{"campaign schedule and voicemail", outboundCmd, []string{"campaigns", "create"}, []string{"campaigns", "update", "17"},
			`{"id":17,"campaign_name":"audit","outreach_override_rules":{"days":{"fri":{"daily_end_time":"15:00:00","max_daily_calls":1300}},"dates":{"2026-11-26":{"active":false}}},"voicemail_detection":{"voicemail_detection_v2_pre_speech_timeout":3.5}}`},
	} {
		for _, argv := range [][]string{tc.create, tc.update} {
			t.Run(tc.name+"/"+argv[len(argv)-1], func(t *testing.T) {
				file := writeTempJSON(t, tc.body)
				args := append(append([]string(nil), argv...), "--file", file)
				cmd, remaining, err := tc.factory().Find(args)
				if err != nil {
					t.Fatal(err)
				}
				if err := cmd.ParseFlags(remaining); err != nil {
					t.Fatal(err)
				}
				positional := cmd.Flags().Args()
				if cmd.Args != nil {
					if err := cmd.Args(cmd, positional); err != nil {
						t.Fatal(err)
					}
				}
				var request *client.DryRunResult
				if err := cmd.RunE(cmd, positional); !errors.As(err, &request) {
					t.Fatalf("expected dry-run request: %v", err)
				}
				var output struct {
					Body interface{} `json:"body"`
				}
				if err := json.Unmarshal(request.Output, &output); err != nil {
					t.Fatal(err)
				}
				var expected interface{}
				if err := json.Unmarshal([]byte(tc.body), &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(output.Body, expected) {
					t.Fatalf("new nested SDK fields changed or disappeared: %s", request.Output)
				}
			})
		}
	}
}

func TestSDKRenamedSchemaLookups(t *testing.T) {
	previous := viper.GetString("output")
	viper.Set("output", "json")
	t.Cleanup(func() { viper.Set("output", previous) })
	schemas, err := loadSchemas()
	if err != nil {
		t.Fatal(err)
	}
	for alias, target := range map[string]string{
		"PromptHistory":                       "PromptHistoryResponse",
		"ListResponse_InsightToolOutput_":     "ListResponse_InsightToolResponse_",
		"ListResponse_InsightWorkflowOutput_": "ListResponse_InsightWorkflowResponse_",
	} {
		t.Run(alias, func(t *testing.T) {
			var runErr error
			var output string
			captureStderr(func() {
				output = captureStdout(func() { runErr = schemaGetCmd().RunE(nil, []string{strings.ToLower(alias)}) })
			})
			if runErr != nil {
				t.Fatal(runErr)
			}
			var got interface{}
			if err := json.Unmarshal([]byte(output), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, schemas[target]) {
				t.Fatalf("alias %s did not return current schema %s", alias, target)
			}
		})
	}
}
