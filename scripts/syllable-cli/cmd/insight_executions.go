package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/asksyllable/syllable-cli/internal/output"
	"github.com/spf13/cobra"
)

type insightWorkflowRow struct {
	ID           json.Number `json:"id"`
	SessionID    json.Number `json:"session_id"`
	UploadFileID json.Number `json:"upload_file_id"`
	Filename     string      `json:"filename"`
	Status       string      `json:"status"`
	Priority     int         `json:"priority"`
	RetryCount   int         `json:"retry_count"`
	ErrorMessage string      `json:"error_message"`
	StartedAt    string      `json:"started_at"`
	CreatedAt    string      `json:"created_at"`
	AnalyzedAt   string      `json:"analyzed_at"`
	QueuedAt     string      `json:"queued_at"`
}

func insightsWorkflowRowsCmd(resource string) *cobra.Command {
	var opts resourceListOptions
	searchField := "status"
	searchHelp := "Search by status"
	if resource == "sessions" {
		searchField, searchHelp = "session_id", "Search by session_id or status"
	} else if resource == "files" {
		searchField, searchHelp = "filename", "Search by filename or status"
	}
	cmd := &cobra.Command{
		Use: resource + " <workflow-id>", Short: "List insight workflow " + resource, Args: cobra.ExactArgs(1),
		Long: "List workflow " + resource + ". Use --output json for full results and execution details.",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := opts.path("/api/v1/insights/workflows/" + url.PathEscape(args[0]) + "/" + resource)
			headers := []string{"ID", "STATUS", "SESSION_ID", "UPLOAD_FILE_ID", "PRIORITY", "RETRIES", "CREATED_AT", "STARTED_AT", "ERROR"}
			if resource == "sessions" {
				headers = []string{"SESSION_ID", "STATUS", "QUEUED_AT", "ANALYZED_AT", "ERROR"}
			} else if resource == "files" {
				headers = []string{"UPLOAD_FILE_ID", "FILENAME", "STATUS", "QUEUED_AT", "ANALYZED_AT", "ERROR"}
			}
			return runList(path, headers, func(r insightWorkflowRow) []string {
				errorMessage := output.Truncate(r.ErrorMessage, 80)
				if resource == "sessions" {
					return []string{r.SessionID.String(), r.Status, r.QueuedAt, r.AnalyzedAt, errorMessage}
				}
				if resource == "files" {
					return []string{r.UploadFileID.String(), r.Filename, r.Status, r.QueuedAt, r.AnalyzedAt, errorMessage}
				}
				return []string{r.ID.String(), r.Status, r.SessionID.String(), r.UploadFileID.String(), fmt.Sprint(r.Priority), fmt.Sprint(r.RetryCount), r.CreatedAt, r.StartedAt, errorMessage}
			})
		},
	}
	opts.flags(cmd, searchField, searchHelp, resource == "executions")
	return cmd
}

func insightsWorkflowExecutionSummaryCmd() *cobra.Command {
	var startDate, endDate string
	cmd := &cobra.Command{
		Use: "executions-summary <workflow-id>", Short: "Summarize insight workflow execution counts", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/insights/workflows/" + url.PathEscape(args[0]) + "/executions/summary"
			query := url.Values{}
			if startDate != "" {
				query.Set("start_datetime", startDate)
			}
			if endDate != "" {
				query.Set("end_datetime", endDate)
			}
			if len(query) > 0 {
				path += "?" + query.Encode()
			}
			return runGet(path, func(s struct {
				Pending    int `json:"pending"`
				Processing int `json:"processing"`
				Completed  int `json:"completed"`
				Failed     int `json:"failed"`
				Total      int `json:"total"`
			}) [][]string {
				return [][]string{{"Pending", fmt.Sprint(s.Pending)}, {"Processing", fmt.Sprint(s.Processing)}, {"Completed", fmt.Sprint(s.Completed)}, {"Failed", fmt.Sprint(s.Failed)}, {"Total", fmt.Sprint(s.Total)}}
			})
		},
	}
	cmd.Flags().StringVar(&startDate, "start-date", "", "Inclusive created_at start datetime (ISO 8601)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "Exclusive created_at end datetime (ISO 8601)")
	return cmd
}
