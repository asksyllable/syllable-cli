package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/asksyllable/syllable-cli/internal/client"
	apispec "github.com/asksyllable/syllable-cli/internal/spec"
	"github.com/spf13/cobra"
)

// TestSDKOperationCoverage fails when a spec operation has no reviewed CLI
// mapping, or a registered command constructs a different method/path. It uses
// the real command constructors and a dry-run client; no API/config credentials
// are used, and no platform calls or mutations occur.
func TestSDKOperationCoverage(t *testing.T) {
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(apispec.OpenAPI, &doc); err != nil {
		t.Fatal(err)
	}
	body := writeTempJSON(t, `{"id":17,"name":"audit","email":"audit@example.invalid"}`)
	oldClient, oldDryRun := apiClient, dryRun
	t.Cleanup(func() { apiClient, dryRun = oldClient, oldDryRun })
	apiClient = client.New("https://api.example.invalid", "unit-test")
	apiClient.DryRun, dryRun = true, true
	factories := map[string]func() *cobra.Command{
		"agents": agentsCmd, "bridge-phrases": bridgePhrasesCmd, "channels": channelsCmd,
		"conversations": conversationsCmd, "conversation-config": conversationConfigCmd,
		"data-sources": dataSourcesCmd, "events": eventsCmd, "experiments": experimentsCmd,
		"incidents": incidentsCmd, "insights": insightsCmd, "custom-messages": customMessagesCmd,
		"permissions": permissionsCmd, "prompts": promptsCmd, "pronunciations": pronunciationsCmd,
		"roles": rolesCmd, "services": servicesCmd, "session-labels": sessionLabelsCmd,
		"sessions": sessionsCmd, "session-debug": sessionDebugCmd, "tools": toolsCmd,
		"directory": directoryCmd, "dashboards": dashboardsCmd, "organizations": organizationsCmd,
		"outbound": outboundCmd, "language-groups": languageGroupsCmd, "voice-groups": voiceGroupsCmd,
		"takeouts": takeoutsCmd, "users": usersCmd,
	}
	excluded := map[string]string{
		"organizations_create":                "Organization provisioning is intentionally console-only (AGENTS.md).",
		"organizations_delete":                "Organization deletion is intentionally console-only (AGENTS.md).",
		"language_groups_create_voice_sample": "Legacy language-group sampling is superseded by voice-groups sample.",
	}
	values := map[string]string{
		"tool_name": "audit", "test_id": "audit-test", "job_id": "audit-job", "file_name": "audit.bin",
		"user_email": "audit@example.invalid", "channel_manager_service": "twilio", "channel_manager_sid": "audit-sid", "tool_call_id": "audit-call",
	}
	seen := map[string]bool{}
	covered := 0
	for path, methods := range doc.Paths {
		for method, raw := range methods {
			switch method {
			case "get", "post", "put", "patch", "delete", "head", "options", "trace":
			default:
				continue
			}
			var operation struct {
				OperationID string `json:"operationId"`
			}
			if err := json.Unmarshal(raw, &operation); err != nil {
				t.Fatal(err)
			}
			id := operation.OperationID
			if id == "" || seen[id] {
				t.Fatalf("missing or duplicate SDK operationId at %s %s", method, path)
			}
			seen[id] = true
			if reason, ok := excluded[id]; ok {
				t.Logf("excluded %s: %s", id, reason)
				continue
			}
			argv, ok := sdkOperationCommands[id]
			if !ok {
				t.Errorf("unmapped SDK operation: %s %s (%s)", method, path, id)
				continue
			}
			covered++
			t.Run(id, func(t *testing.T) {
				if id == "insights_workflow_activate" {
					// Activation reads the current estimate before its PUT. Supply
					// that prerequisite with an in-process transport, then intercept
					// the mutation as a dry-run result. No network access is possible.
					previous := apiClient
					stub := *apiClient
					stub.DryRun = false
					stub.HTTPClient = &http.Client{Transport: sdkAuditTransport(func(r *http.Request) (*http.Response, error) {
						if r.Method == "GET" && r.URL.Path == "/api/v1/insights/workflows/17" {
							return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"estimate":{}}`))}, nil
						}
						data, _ := json.Marshal(map[string]string{"method": r.Method, "url": r.URL.String()})
						return nil, &client.DryRunResult{Output: data}
					})}
					apiClient = &stub
					defer func() { apiClient = previous }()
				}
				args := append([]string(nil), argv...)
				for i := range args {
					if args[i] == "$body" {
						args[i] = body
					}
				}
				registered, _, err := rootCmd.Find(args)
				if err != nil || registered.RunE == nil {
					t.Fatalf("CLI command is not registered: %v (%v)", args, err)
				}
				factory, ok := factories[args[0]]
				if !ok {
					t.Fatalf("missing command factory: %s", args[0])
				}
				cmd, remaining, err := factory().Find(args[1:])
				if err != nil {
					t.Fatal(err)
				}
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
				if err := cmd.ParseFlags(remaining); err != nil {
					t.Fatal(err)
				}
				if err := cmd.ValidateRequiredFlags(); err != nil {
					t.Fatal(err)
				}
				positional := cmd.Flags().Args()
				if cmd.Args != nil {
					if err := cmd.Args(cmd, positional); err != nil {
						t.Fatal(err)
					}
				}
				var result *client.DryRunResult
				if err := cmd.RunE(cmd, positional); !errors.As(err, &result) {
					t.Fatalf("command did not produce a dry-run request: %v", err)
				}
				var request struct {
					Method string `json:"method"`
					URL    string `json:"url"`
				}
				if err := json.Unmarshal(result.Output, &request); err != nil {
					t.Fatal(err)
				}
				u, err := url.Parse(request.URL)
				if err != nil {
					t.Fatal(err)
				}
				want := path
				for _, parameter := range strings.Split(path, "/") {
					if strings.HasPrefix(parameter, "{") {
						name := strings.Trim(parameter, "{}")
						value := values[name]
						if value == "" {
							value = "17"
						}
						want = strings.ReplaceAll(want, parameter, value)
					}
				}
				if request.Method != strings.ToUpper(method) || u.Path != want {
					t.Errorf("%v constructs %s %s, SDK expects %s %s", args, request.Method, u.Path, strings.ToUpper(method), want)
				}
			})
		}
	}
	for id := range sdkOperationCommands {
		if !seen[id] {
			t.Errorf("stale CLI mapping for removed SDK operation: %s", id)
		}
	}
	for id := range excluded {
		if !seen[id] {
			t.Errorf("stale SDK exclusion: %s", id)
		}
	}
	t.Logf("covered %d SDK operations; %d deliberate exclusions", covered, len(excluded))
}

type sdkAuditTransport func(*http.Request) (*http.Response, error)

func (f sdkAuditTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Each entry is a reviewed CLI invocation, frozen independently of the spec.
// New operations must be mapped here; do not derive these from the spec at test time.
var sdkOperationCommands = map[string][]string{
	"agent_create":                                 {"agents", "create", "--file", "$body"},
	"agent_delete":                                 {"agents", "delete", "17"},
	"agent_get_available_stt_providers":            {"agents", "stt-providers"},
	"agent_get_available_voices":                   {"agents", "voices"},
	"agent_get_by_id":                              {"agents", "get", "17"},
	"agent_list":                                   {"agents", "list"},
	"agent_list_active_labels":                     {"agents", "labels"},
	"agent_update":                                 {"agents", "update", "17", "--file", "$body"},
	"available_targets":                            {"channels", "available-targets"},
	"bridge_phrases_create":                        {"bridge-phrases", "create", "--file", "$body"},
	"bridge_phrases_delete":                        {"bridge-phrases", "delete", "17"},
	"bridge_phrases_get_by_id":                     {"bridge-phrases", "get", "17"},
	"bridge_phrases_list":                          {"bridge-phrases", "list"},
	"bridge_phrases_update":                        {"bridge-phrases", "update", "17", "--file", "$body"},
	"channel_targets_create":                       {"channels", "targets", "create", "17", "--file", "$body"},
	"channel_targets_delete":                       {"channels", "targets", "delete", "17", "17"},
	"channel_targets_get_by_id":                    {"channels", "targets", "get", "17", "17"},
	"channel_targets_list":                         {"channels", "targets", "list"},
	"channel_targets_update":                       {"channels", "targets", "update", "17", "17", "--file", "$body"},
	"channels_create":                              {"channels", "create", "--file", "$body"},
	"channels_list":                                {"channels", "list"},
	"channels_twilio_create":                       {"channels", "twilio", "create", "--file", "$body"},
	"channels_twilio_get_by_id":                    {"channels", "twilio", "get", "17"},
	"channels_twilio_numbers_a2p_compliance_check": {"channels", "twilio", "numbers-verify-a2p-compliance", "17", "--phone", "+12025550123"},
	"channels_twilio_numbers_add":                  {"channels", "twilio", "numbers-add", "17", "--file", "$body"},
	"channels_twilio_numbers_list":                 {"channels", "twilio", "numbers-list", "17"},
	"channels_twilio_numbers_update":               {"channels", "twilio", "numbers-update", "17", "--file", "$body"},
	"channels_twilio_update":                       {"channels", "twilio", "update", "--file", "$body"},
	"channels_update":                              {"channels", "update", "17", "--file", "$body"},
	"conversations_list":                           {"conversations", "list"},
	"custom_message_get_by_id":                     {"custom-messages", "get", "17"},
	"custom_messages_create":                       {"custom-messages", "create", "--file", "$body"},
	"custom_messages_delete":                       {"custom-messages", "delete", "17"},
	"custom_messages_list":                         {"custom-messages", "list"},
	"custom_messages_update":                       {"custom-messages", "update", "17", "--file", "$body"},
	"data_sources_create":                          {"data-sources", "create", "--file", "$body"},
	"data_sources_delete":                          {"data-sources", "delete", "17"},
	"data_sources_get_by_id":                       {"data-sources", "get", "17"},
	"data_sources_list":                            {"data-sources", "list"},
	"data_sources_update":                          {"data-sources", "update", "17", "--file", "$body"},
	"directory_member_bulk_load":                   {"directory", "upload", "--file", "$body"},
	"directory_member_create":                      {"directory", "create", "--file", "$body"},
	"directory_member_delete":                      {"directory", "delete", "17"},
	"directory_member_download":                    {"directory", "download"},
	"directory_member_get_by_id":                   {"directory", "get", "17"},
	"directory_member_history":                     {"directory", "history", "17"},
	"directory_member_list":                        {"directory", "list"},
	"directory_member_restore":                     {"directory", "restore", "17"},
	"directory_member_test_extension":              {"directory", "test", "17"},
	"directory_member_update":                      {"directory", "update", "17", "--file", "$body"},
	"events_list":                                  {"events", "list"},
	"experiments_create":                           {"experiments", "create", "--file", "$body"},
	"experiments_delete":                           {"experiments", "delete", "17"},
	"experiments_get_by_id":                        {"experiments", "get", "17"},
	"experiments_list":                             {"experiments", "list"},
	"experiments_results":                          {"experiments", "results", "17"},
	"experiments_start":                            {"experiments", "start", "17"},
	"experiments_stop":                             {"experiments", "stop", "17"},
	"experiments_update":                           {"experiments", "update", "17", "--file", "$body"},
	"generate_session_recording_urls":              {"sessions", "recording", "17"},
	"get_bridge_phrases_config":                    {"conversation-config", "bridges"},
	"get_session_data_by_session_id":               {"session-debug", "by-session-id", "17"},
	"get_session_data_by_sid":                      {"session-debug", "by-sid", "twilio", "audit-sid"},
	"get_session_tool_call_result_by_id":           {"session-debug", "tool-result", "17", "audit-call"},
	"incident_create":                              {"incidents", "create", "--file", "$body"},
	"incident_delete":                              {"incidents", "delete", "17"},
	"incident_get_by_id":                           {"incidents", "get", "17"},
	"incident_get_organizations":                   {"incidents", "organizations"},
	"incident_list":                                {"incidents", "list"},
	"incident_update":                              {"incidents", "update", "17", "--file", "$body"},
	"insight_tool_get_by_id":                       {"insights", "tool-configs", "get", "17"},
	"insight_tool_get_definitions":                 {"insights", "tool-definitions"},
	"insight_tool_list":                            {"insights", "tool-configs", "list"},
	"insights_folder_create":                       {"insights", "folders", "create", "--file", "$body"},
	"insights_folder_delete":                       {"insights", "folders", "delete", "17"},
	"insights_folder_get_by_id":                    {"insights", "folders", "get", "17"},
	"insights_folder_list":                         {"insights", "folders", "list"},
	"insights_folder_update":                       {"insights", "folders", "update", "17", "--file", "$body"},
	"insights_folder_upload_file":                  {"insights", "folders", "upload-file", "17", "--file", "$body", "--call-id", "audit-call"},
	"insights_list":                                {"insights", "list"},
	"insights_tool_create":                         {"insights", "tool-configs", "create", "--file", "$body"},
	"insights_tool_delete":                         {"insights", "tool-configs", "delete", "17"},
	"insights_tool_test":                           {"insights", "tools-test", "--file", "$body"},
	"insights_tool_update":                         {"insights", "tool-configs", "update", "17", "--file", "$body"},
	"insights_upload_list_files":                   {"insights", "folders", "files", "17"},
	"insights_upload_move_files":                   {"insights", "folders", "move-files", "17", "--file", "$body"},
	"insights_workflow_activate":                   {"insights", "workflows", "activate", "17"},
	"insights_workflow_create":                     {"insights", "workflows", "create", "--file", "$body"},
	"insights_workflow_delete":                     {"insights", "workflows", "delete", "17"},
	"insights_workflow_executions":                 {"insights", "workflows", "executions", "17"},
	"insights_workflow_executions_summary":         {"insights", "workflows", "executions-summary", "17"},
	"insights_workflow_files":                      {"insights", "workflows", "files", "17"},
	"insights_workflow_get_by_id":                  {"insights", "workflows", "get", "17"},
	"insights_workflow_inactivate":                 {"insights", "workflows", "inactivate", "17"},
	"insights_workflow_list":                       {"insights", "workflows", "list"},
	"insights_workflow_queue_work":                 {"insights", "workflows", "queue-work", "--file", "$body"},
	"insights_workflow_sessions":                   {"insights", "workflows", "sessions", "17"},
	"insights_workflow_update":                     {"insights", "workflows", "update", "17", "--file", "$body"},
	"language_groups_create":                       {"language-groups", "create", "--file", "$body"},
	"language_groups_delete":                       {"language-groups", "delete", "17"},
	"language_groups_get_by_id":                    {"language-groups", "get", "17"},
	"language_groups_list":                         {"language-groups", "list"},
	"language_groups_update":                       {"language-groups", "update", "17", "--file", "$body"},
	"organization_sip_ip_ranges_create":            {"organizations", "sip-ip-ranges", "create", "--type", "signaling", "--ip-range", "203.0.113.0/24"},
	"organization_sip_ip_ranges_delete":            {"organizations", "sip-ip-ranges", "delete", "17"},
	"organization_sip_ip_ranges_list":              {"organizations", "sip-ip-ranges", "list"},
	"organization_sip_ip_ranges_update":            {"organizations", "sip-ip-ranges", "update", "17", "--type", "signaling", "--ip-range", "203.0.113.0/25"},
	"organizations_get":                            {"organizations", "get"},
	"organizations_update":                         {"organizations", "update", "--display-name", "audit"},
	"outbound_batch_add":                           {"outbound", "batches", "add-requests", "17", "--file", "$body"},
	"outbound_batch_create":                        {"outbound", "batches", "create", "--file", "$body"},
	"outbound_batch_delete":                        {"outbound", "batches", "delete", "17"},
	"outbound_batch_get_by_id":                     {"outbound", "batches", "get", "17"},
	"outbound_batch_list":                          {"outbound", "batches", "list"},
	"outbound_batch_remove":                        {"outbound", "batches", "remove-requests", "17", "--file", "$body"},
	"outbound_batch_results":                       {"outbound", "batches", "results", "17"},
	"outbound_batch_update":                        {"outbound", "batches", "update", "17", "--file", "$body"},
	"outbound_batch_upload":                        {"outbound", "batches", "upload", "17", "--file", "$body"},
	"outbound_campaign_create":                     {"outbound", "campaigns", "create", "--file", "$body"},
	"outbound_campaign_delete":                     {"outbound", "campaigns", "delete", "17"},
	"outbound_campaign_get_by_id":                  {"outbound", "campaigns", "get", "17"},
	"outbound_campaign_list":                       {"outbound", "campaigns", "list"},
	"outbound_campaign_update":                     {"outbound", "campaigns", "update", "17", "--file", "$body"},
	"permissions_list":                             {"permissions", "list"},
	"post_card_click_event":                        {"agents", "card-click", "audit-test", "--action-id", "audit", "--action-type", "cta"},
	"post_get_dashboard":                           {"dashboards", "fetch-info", "--name", "audit"},
	"post_list_dashboard":                          {"dashboards", "list"},
	"post_session_events_dashboard":                {"dashboards", "session-events"},
	"post_session_summary_dashboard":               {"dashboards", "session-summary"},
	"post_session_transfers_dashboard":             {"dashboards", "session-transfers"},
	"post_sessions_dashboard":                      {"dashboards", "sessions"},
	"prompt_get_supported_llms":                    {"prompts", "supported-llms"},
	"prompts_create":                               {"prompts", "create", "--file", "$body"},
	"prompts_delete":                               {"prompts", "delete", "17"},
	"prompts_get_by_id":                            {"prompts", "get", "17"},
	"prompts_history":                              {"prompts", "history", "17"},
	"prompts_list":                                 {"prompts", "list"},
	"prompts_update":                               {"prompts", "update", "17", "--file", "$body"},
	"pronunciations_delete_csv":                    {"pronunciations", "delete-csv", "--confirm"},
	"pronunciations_download_csv":                  {"pronunciations", "get-csv"},
	"pronunciations_get":                           {"pronunciations", "list"},
	"pronunciations_get_metadata":                  {"pronunciations", "metadata"},
	"pronunciations_upload_csv":                    {"pronunciations", "upload-csv", "--confirm", "--file", "$body"},
	"roles_create":                                 {"roles", "create", "--file", "$body"},
	"roles_delete":                                 {"roles", "delete", "17"},
	"roles_get_by_id":                              {"roles", "get", "17"},
	"roles_list":                                   {"roles", "list"},
	"roles_update":                                 {"roles", "update", "17", "--file", "$body"},
	"send_test_message":                            {"agents", "send-test-message", "17", "--test-id", "audit-test", "--session-start", "--text", "audit"},
	"service_create":                               {"services", "create", "--file", "$body"},
	"service_delete":                               {"services", "delete", "17"},
	"service_list":                                 {"services", "list"},
	"service_update":                               {"services", "update", "17", "--file", "$body"},
	"services_get_by_id":                           {"services", "get", "17"},
	"session_full_summary_get_by_id":               {"sessions", "summary", "17"},
	"session_get_by_id":                            {"sessions", "get", "17"},
	"session_label_create":                         {"session-labels", "create", "--file", "$body"},
	"session_label_get_by_id":                      {"session-labels", "get", "17"},
	"session_labels_list":                          {"session-labels", "list"},
	"session_latency_get_by_id":                    {"sessions", "latency", "17"},
	"session_recording_stream":                     {"sessions", "recording-stream", "--token", "audit"},
	"session_timeline_get_by_id":                   {"sessions", "timeline", "17"},
	"session_transcript_get_by_id":                 {"sessions", "transcript", "17"},
	"sessions_list":                                {"sessions", "list"},
	"takeouts_create":                              {"takeouts", "create", "--file", "$body"},
	"takeouts_get_by_job_id":                       {"takeouts", "get", "audit-job"},
	"takeouts_get_file":                            {"takeouts", "download", "audit-job", "audit.bin"},
	"tool_create":                                  {"tools", "create", "--file", "$body"},
	"tool_delete":                                  {"tools", "delete", "audit"},
	"tool_get_by_name":                             {"tools", "get", "audit"},
	"tool_history":                                 {"tools", "history", "17"},
	"tool_list":                                    {"tools", "list"},
	"tool_update":                                  {"tools", "update", "audit", "--file", "$body"},
	"update_bridge_phrases_config":                 {"conversation-config", "bridges-update", "--file", "$body"},
	"users_create":                                 {"users", "create", "--file", "$body"},
	"users_delete":                                 {"users", "delete", "audit@example.invalid"},
	"users_delete_account":                         {"users", "delete-account", "--confirm"},
	"users_get_by_email":                           {"users", "get", "audit@example.invalid"},
	"users_list":                                   {"users", "list"},
	"users_send_email":                             {"users", "send-email", "audit@example.invalid"},
	"users_update":                                 {"users", "update", "audit@example.invalid", "--file", "$body"},
	"voice_groups_create":                          {"voice-groups", "create", "--file", "$body"},
	"voice_groups_create_voice_sample":             {"voice-groups", "sample", "--file", "$body"},
	"voice_groups_delete":                          {"voice-groups", "delete", "17"},
	"voice_groups_get_by_id":                       {"voice-groups", "get", "17"},
	"voice_groups_list":                            {"voice-groups", "list"},
	"voice_groups_update":                          {"voice-groups", "update", "17", "--file", "$body"},
}
