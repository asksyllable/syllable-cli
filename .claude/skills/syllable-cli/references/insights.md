# Insights Reference

Reference for the Insights system: folders, tool definitions, tool configs, workflows, and outputs. Sources: `syllable schema get` and [docs.syllable.ai](https://docs.syllable.ai).

## Concept Overview

Insights is a post-call analysis pipeline. The pieces fit together like this:

```
Tool Definition (template)
    ↓ referenced by
Tool Config (instance with specific arguments)
    ↓ chained in
Workflow (pipeline that triggers on conditions)
    ↓ processes
Sessions or Uploaded Files
    ↓ produces
Outputs (string, numeric, and JSON values)
```

| Component | What it is | CLI command group |
|-----------|-----------|-------------------|
| **Folder** | Storage bucket for uploaded audio/files | `insights folders` |
| **Tool Definition** | Reusable template (e.g., `llm_tool`, `deepgram_transcribe`) | `insights tool-definitions` |
| **Tool Config** | Instance of a definition with specific arguments (e.g., a summary prompt) | `insights tool-configs` |
| **Workflow** | Pipeline that chains tool configs and triggers on conditions | `insights workflows` |
| **Output** | Result from a tool config run against a session/file | (via dashboard or API) |

## Folders

Storage for uploaded audio files (call recordings, etc.).

```bash
syllable insights folders list
syllable insights folders get <folder_id>
syllable insights folders create --file folder.json
syllable insights folders files <folder_id>          # List files in folder
```

### Create Payload

```json
{
  "name": "q2-call-recordings",
  "description": "Call recordings for Q2 analysis",
  "label": "recordings"
}
```

**Required:** `name` only. `description` and `label` are optional.

## Tool Definitions

Read-only templates that define what a tool does and what parameters it accepts.

```bash
syllable insights tool-definitions list
```

Each definition has:

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Definition ID (used when creating tool configs) |
| `name` | string | Template name (e.g., `llm_tool`, `deepgram_transcribe`) |
| `type` | string | Tool type |
| `tool_parameters` | object | Parameters the tool accepts (e.g., `{"prompt": "string"}`) |
| `tool_result_set` | object | Output keys and their types (e.g., `{"summary": "string"}`) |

The `tool_result_set` determines what output fields the tool produces. Common patterns:
- `{"summary": "string"}` — text output
- `{"score": "number"}` — numeric output
- `{"is_resolved": "boolean"}` — boolean output (stored as string `"true"`/`"false"`)

## Tool Configs

Instances of a tool definition with specific argument values.

```bash
syllable insights tool-configs list
syllable insights tool-configs get <config_id>
syllable insights tool-configs create --file config.json
syllable insights tool-configs update --file config.json
syllable insights tool-configs delete <config_id>
```

### Create Payload

```json
{
  "name": "call-summary-gpt41",
  "description": "Summarizes call transcripts using GPT-4.1",
  "version": 1,
  "insight_tool_definition_id": 1,
  "tool_arguments": {
    "prompt": "Provide a concise summary of the conversation, focusing on the caller's goal and resolution."
  }
}
```

**Required:** `name`, `description`, `version`, `insight_tool_definition_id`, `tool_arguments`.

**Important:** Tool config names can be misleading. Always inspect the `tool_arguments` JSON to understand what the config actually does — don't rely on the name alone.

## Workflows

Pipelines that chain tool configs and trigger on conditions.

```bash
syllable insights workflows list
syllable insights workflows get <workflow_id>
syllable insights workflows create --file workflow.json
syllable insights workflows update --file workflow.json
syllable insights workflows delete <workflow_id>
syllable insights workflows activate <workflow_id>
syllable insights workflows inactivate <workflow_id>
```

### Workflow States

| Status | Description |
|--------|-------------|
| `INACTIVE` | Created but not running. Can be edited. |
| `ACTIVE` | Running and processing sessions/files that match conditions. |

Workflows start as `INACTIVE`. Activate to begin processing. Inactivate to stop.

### Create Payload

```json
{
  "name": "session-summary-workflow",
  "source": "agent",
  "description": "Summarize all sessions from agent 42",
  "insight_tool_ids": [15, 22],
  "conditions": {
    "agent_list": [42],
    "min_duration": 30,
    "max_duration": 600
  },
  "start_datetime": "2025-04-01T00:00:00Z",
  "end_datetime": null
}
```

**Required:** `name`, `source`, `description`, `insight_tool_ids`, `conditions`.

### Source Types

The `source` field determines what the workflow processes:

| Source | Triggers on | Use case |
|--------|------------|----------|
| `agent` | Live agent sessions | Ongoing QA analysis |
| `upload` | Files uploaded to a folder | Transcribing call recordings |
| `transfer` | Transfer events | Transfer analysis |
| `sheet` | Google Sheet data | External data processing |
| `manual` | Manual trigger | One-off analysis |

### Conditions (`InsightWorkflowCondition`)

Filter which sessions/files the workflow processes:

| Field | Type | Description |
|-------|------|-------------|
| `agent_list` | int[] or string[] | Agent IDs or names to filter on |
| `prompt_list` | string[] | Prompt IDs to filter on |
| `folder_list` | int[] | Folder IDs (for `upload` source workflows) |
| `min_duration` | integer | Minimum session duration in seconds |
| `max_duration` | integer | Maximum session duration in seconds |
| `sample_rate` | number | Percentage of sessions to sample (e.g., `0.1` for 10%) |
| `sheet_info` | object | Google Sheet reference (for `sheet` source) |

### Date Range

- `start_datetime` — Backfill start. `null` = process only new sessions from activation.
- `end_datetime` — Backfill end. `null` = continue processing live sessions until inactivated.

### Activation

Activating a workflow requires acknowledging the cost estimate:

```bash
# First, get the workflow to see the estimate
syllable insights workflows get <workflow_id> -o json
# Check estimate.backfill_count and estimate.estimated_backfill_cost
# Then activate
syllable insights workflows activate <workflow_id>
```

The activate command sends `{"is_acknowledged": true, "estimate": {...}}` with the estimate from the workflow's current state.

### Monitoring Progress

Poll `queue_count` on the workflow to track processing:

```bash
syllable insights workflows get <workflow_id> -o json
# Check "queue_count" — 0 means all items processed
```

## Outputs (`InsightsOutput`)

Results from tool config runs. Each output contains:

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Output ID |
| `insight_tool_id` | integer | Tool config that produced this output |
| `insight_key` | string | Result key (from `tool_result_set`) |
| `string_value` | string | Text output (summaries, classifications) |
| `numeric_value` | number | Numeric output (scores, counts) |
| `json_value` | object | Structured data (always present) |
| `session_id` | integer | Session this output relates to |
| `upload_file_id` | integer | Uploaded file this output relates to |
