# Sessions & Debugging Reference

Field-level reference for session commands, transcript structure, latency analysis, and debug tools. Sources: `syllable schema get` and [docs.syllable.ai/workspaces/Sessions](https://docs.syllable.ai/workspaces/Sessions).

## Session Fields

`sessions list -o json` returns a paginated list. Each session object contains:

| Field | Type | Description |
|-------|------|-------------|
| `session_id` | string | Internal Syllable session ID — a **numeric** value rendered as a string (e.g., `"824"`). Distinct from the Twilio `CA…` call SID (see `channel_manager_sid`). Routes keyed by this ID (e.g., `GET /sessions/full-summary/{session_id}`, `sessions get`) coerce it to an int and return **400** on non-numeric values like a `CA<hex>` SID. |
| `conversation_id` | string | Parent conversation ID (a conversation may contain multiple sessions) |
| `timestamp` | datetime | Session start time |
| `duration` | number | Duration in seconds |
| `agent_id` | string | Agent ID |
| `agent_name` | string | Agent name |
| `agent_type` | string | Agent type |
| `agent_timezone` | string | Agent's configured timezone |
| `prompt_id` | string | Prompt ID used during this session |
| `prompt_name` | string | Prompt name |
| `prompt_version_number` | integer | Prompt version number |
| `source` | string | Caller (inbound) or recipient (outbound) — phone number, email, or username |
| `target` | string | Channel target (the number/address the caller dialed) |
| `is_test` | boolean | Whether this was a test session. Set on the session, not inherited from a channel target — conversation tests via `agents send-test-message` are `is_test=true` even in orgs with no channel targets. Hidden from `sessions list` unless `--include-test` is passed |
| `user_terminated` | boolean | Whether the caller/recipient ended the call. `false` if transferred or errored. `null` for non-voice sessions |
| `session_label_id` | string | ID of the quality label applied to this session |
| `channel_manager_service` | string | Service facilitating the session (e.g., `hedy`, `console`) |
| `channel_manager_type` | string | Session type (e.g., `voice_sip_v1`, `voice_twilio_v1`, `web_chat_v1`) |
| `channel_manager_sid` | string | External session ID (Twilio call SID, etc.) — different from `session_id` |
| `is_outbound` | boolean | Whether the session was an outbound (campaign-initiated) call — added to `SessionProperties` in CLI v1.7.1's spec |
| `is_legacy` | boolean | Whether the session occurred on the legacy system |

## Filtering Sessions

```bash
# By date range
syllable sessions list --start-date 2025-04-01T00:00:00Z --end-date 2025-04-02T00:00:00Z

# By agent name (uses --search on sessions; agent_name is the default search field)
syllable sessions list --search "Anna"

# By any other SessionProperties field via --search-field
# (valid fields: syllable schema get SessionProperties)
syllable sessions list --search-field channel_manager_sid --search "CA…"   # resolve a Twilio call SID to its session
syllable sessions list --search-field source --search "+18045551234"

# Combine with pagination
syllable sessions list --start-date 2025-04-01T00:00:00Z --limit 100 --page 0

# Get full JSON for scripting
syllable sessions list --start-date 2025-04-01T00:00:00Z -o json
```

**Date format:** ISO 8601 (e.g., `2025-04-01T00:00:00Z`).

**Boolean fields don't filter server-side** (`is_outbound`, `is_test` — verified on CLI v1.7.1): filter client-side with `jq 'select(.is_outbound == true)'` instead, and use `--include-test` to surface test sessions. See `gotchas.md`.

**Important:** Test sessions (created via conversation test API) may not appear in `sessions list` results. Retrieve them by direct lookup with `sessions get <session_id>` using the session ID returned by the test API.

## Transcript Structure

`sessions transcript <session_id> -o json` returns:

```json
{
  "session_id": "abc123",
  "transcription": [ ... ],
  "actions": [ ... ]
}
```

### Transcription Messages (`SessionText`)

Each message in the `transcription` array:

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | datetime | When the message was sent |
| `source` | string | `user` or `agent` |
| `text` | string | Message content |
| `lang` | string | Language code (e.g., `en-US`, `es-US`). May be `unset` if not determined |

### Tool Actions (`SessionAction`)

Each entry in the `actions` array represents a tool invocation:

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | datetime | When the tool was called |
| `tool_name` | string | Name of the tool invoked |
| `tool_request` | string | Request payload sent to the tool API (JSON string) |
| `tool_result` | string | Response received from the tool API (JSON string) |
| `tool_error` | string | Error message if the tool call failed |

**Tip:** When troubleshooting tool failures, check both `tool_error` (API-level errors) and `tool_result` (successful responses that may contain application-level error messages). The `tool_request` shows exactly what parameters the agent sent.

## Session Summary

`sessions summary <session_id>` returns the AI-generated summary:

| Field | Type | Description |
|-------|------|-------------|
| `rating` | string | AI quality rating (e.g., `Good`) |
| `summary` | string | Natural language summary of the session |

## Latency Analysis

`sessions latency <session_id>` returns timing data with two sections:

### Timeline

Chronological list of operations. Each `LatencyEntry`:

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | datetime | When the operation occurred |
| `category` | enum | `tts`, `stt`, `llm`, `tool`, `http` |
| `label` | string | Specific operation label |
| `value` | number | Duration in milliseconds |
| `value_str` | string | Human-readable duration |
| `time_delta` | string | Time since previous entry |
| `metadata` | string[] | Additional context |

### Summary

Aggregated by category. Each `SummaryEntry`:

| Field | Type | Description |
|-------|------|-------------|
| `category` | enum | `tts`, `stt`, `llm`, `tool`, `http` |
| `sub_category` | string | Finer grouping within category |
| `event_count` | integer | Number of operations |
| `sum_ms` | number | Total milliseconds |
| `average_ms` | number | Average milliseconds per operation |

**Latency categories:**

| Category | What it measures |
|----------|-----------------|
| `tts` | Text-to-speech synthesis time |
| `stt` | Speech-to-text transcription time |
| `llm` | LLM inference time (prompt processing + generation) |
| `tool` | External tool API call time |
| `http` | Other HTTP request time |

## Session Recording

```bash
# Download recording URL
syllable sessions recording <session_id>
```

Returns a `SessionRecordingResponse` with URLs to the audio recording.

## Session Debug

Lower-level debugging commands for inspecting raw session internals:

```bash
# Full debug data for a session
syllable session-debug by-session-id <session_id>

# Debug by channel manager SID (Twilio call SID, etc.)
syllable session-debug by-sid <session_id> <sid>

# Inspect a specific tool result in detail
syllable session-debug tool-result <session_id> <tool_result_id>
```

**When to use each:**
- `by-session-id` — Starting point for any debug investigation. Shows the full internal event log.
- `by-sid` — When you have a Twilio call SID or other channel manager ID and need to find the corresponding Syllable session data.
- `tool-result` — Deep inspection of a specific tool invocation. Use when `sessions transcript` shows a tool error and you need the raw request/response details.

## Console URL Pattern

To link to a session in the Console:

```
https://syllable.cloud/<org_slug>/sessions?sessionId=<session_id>
```

Find the org slug (SLUG column; the command returns the current org only — the API key scopes it):
```bash
syllable organizations get --org <org>
```

The `SLUG` column contains the Console org slug (the `<slug>` in any Console URL).
