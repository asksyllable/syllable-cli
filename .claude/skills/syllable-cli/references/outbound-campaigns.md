# Outbound Campaigns Reference

Full lifecycle reference for outbound campaigns, batches, and requests. Sources: `syllable schema get` and [docs.syllable.ai](https://docs.syllable.ai).

## Concept Overview

```
Campaign (defines who calls, when, and how)
    ↓ contains
Batch (a named group of contacts, with a lifecycle)
    ↓ contains
Requests (individual contact records with variables)
    ↓ produces
Results (per-request status, session links, insights)
```

## Campaign Fields

See `references/payload-examples.md` for the full campaign create JSON template.

Key field distinctions:

| Field | Description |
|-------|-------------|
| `caller_id` | What the recipient sees on their phone (the display number) |
| `source` | The actual phone number or email making the call/sending the message |
| `mode` | `voice`, `sms`, or `email` |
| `active_days` | Days the campaign runs. Three-letter abbreviations: `mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun` |
| `daily_start_time` / `daily_end_time` | Operating window in `HH:MM:SS` format |
| `hourly_rate` | Target calls per hour (default: 1) |
| `max_daily_calls` | Hard cap on daily calls |
| `retry_count` | Number of retry attempts per target (default: 0) |
| `retry_interval` | Wait before retrying: `30m`, `12h`, `7d`, etc. |
| `campaign_variables` | Key-value pairs available to all batches in this campaign (rendered via `{{vars.*}}` in prompts) |
| `voicemail_detection` | See `references/telephony-and-channels.md` for field details. Set to `null` to disable. |
| `labels` | Array of strings (not singular `label` — see deprecation in `references/gotchas.md`) |

**Note:** `agent_id` on a campaign is informational — it does not determine which agent answers the call. The agent is determined by the channel target that matches the `source` phone number.

## Batch Lifecycle

### Batch Statuses (`BatchStatus`)

| Status | Description |
|--------|-------------|
| `PENDING` | Created, awaiting requests or activation |
| `ACTIVE` | Running — requests are being dialed/sent |
| `PAUSED` | On hold — no outreach while paused |
| `FAILED` | Upload or processing error |
| `CANCELED` | Manually canceled |
| `EXPIRED` | Past `expires_on` timestamp |

### Creating a Batch

```bash
# Create a batch (starts in PENDING)
syllable outbound batches create --file batch.json
```

```json
{
  "batch_id": "appt-reminders-2025-04-15",
  "campaign_id": 42,
  "paused": true,
  "expires_on": "2025-04-16T00:00:00Z"
}
```

**Required:** `batch_id`, `campaign_id`. The `batch_id` is a string you choose — must be unique within the campaign.

**Tip:** Create batches with `"paused": true` so you can add requests before any calls go out.

### Adding Requests

Requests are individual contact records. Add them to a batch:

```bash
syllable outbound batches requests <batch_id> --file requests.json
```

```json
[
  {
    "reference_id": "appt-001",
    "target": "+15125551234",
    "request_variables": {
      "patient_name": "Jane Doe",
      "appointment_date": "April 15, 2025",
      "provider_name": "Dr. Smith"
    }
  }
]
```

**Required per request:** `reference_id` (unique within batch), `target` (phone/email), `request_variables` (key-value pairs).

**Gotcha — deduplication:** Batches deduplicate by `target`. If multiple requests share the same phone number in a single batch, only the first is dialed — the rest are marked `DUPLICATE`. Use sequential batches for repeated calls to the same number.

**Gotcha — variable rendering:** Request variables render in prompts via `{{vars.*}}` but do NOT render in greetings. Greetings only read agent-level variables.

### Activating a Batch

Unpause to start dialing:

```bash
syllable outbound batches update <batch_id> --file unpause.json
```

```json
{
  "batch_id": "appt-reminders-2025-04-15",
  "campaign_id": 42,
  "paused": false
}
```

### Removing Requests

Remove specific requests from a batch before they're dialed:

```bash
syllable outbound batches remove-requests <batch_id> --file remove.json
```

## Request Statuses (`RequestStatus`)

Each request in a batch transitions through these states:

| Status | Description |
|--------|-------------|
| `PENDING` | Queued, not yet attempted |
| `INITIATED` | Call/message initiated |
| `CONNECTED` | Call connected to recipient |
| `COMPLETED` | Successfully finished |
| `FAILED` | Call/message failed |
| `DUPLICATE` | Skipped — same `target` as another request in this batch |
| `CANCELED` | Manually canceled |
| `INVALID` | Invalid target (bad phone number, etc.) |
| `UNSUBSCRIBED` | Target has opted out |

## Monitoring Results

### Batch Status

```bash
# Quick status check
syllable outbound batches get <batch_id>

# Full JSON with status_counts and detailed_status_counts
syllable outbound batches get <batch_id> -o json
```

The response includes:
- `status_counts` — `{"PENDING": 10, "CONNECTED": 95, "FAILED": 5}` etc.
- `detailed_status_counts` — Per-request-status breakdown with channel manager sub-statuses:
  ```json
  {
    "CONNECTED": {
      "total_count": 100,
      "counts": {"COMPLETED": 95, "NO-ANSWER": 5}
    }
  }
  ```

### Per-Request Results

```bash
syllable outbound batches results <batch_id> -o json
```

Each `CommunicationRequestResult` contains:

| Field | Type | Description |
|-------|------|-------------|
| `reference_id` | string | Your unique request ID |
| `target` | string | Phone number / email |
| `request_status` | enum | See status table above |
| `channel_manager_status` | string | Lower-level status: `COMPLETED`, `FAILED`, `NO-ANSWER`, `BUSY` |
| `channel_manager_sid` | string | External session ID (e.g., Twilio call SID) |
| `session_id` | integer | Syllable session ID (for fetching transcripts) |
| `conversation_id` | integer | Syllable conversation ID |
| `attempt_count` | integer | Number of attempts made |
| `sent_at` | datetime | When the request was dispatched |
| `request_variables` | object | The variables you submitted |
| `insights` | object | Insight results if a workflow processed this session |
| `insights_status` | string | `PENDING`, `PROCESSING`, `COMPLETED`, `FAILED` |

**Correlating results to sessions:** Use `session_id` from the results to fetch the full transcript:
```bash
syllable sessions transcript <session_id> -o json
```

## CSV Upload (Alternative to JSON Requests)

Batches can also be populated by uploading a CSV file:

```bash
syllable outbound batches create --file batch.csv
```

CSV columns: `reference_id`, `target`, plus any custom columns that become `request_variables`.

## Campaign Variable Scopes

Variables flow into prompts from multiple sources, with this precedence:

| Source | Scope | Renders in prompts | Renders in greetings |
|--------|-------|-------------------|---------------------|
| Agent `variables` | All sessions for this agent | Yes (via `{{vars.*}}`) | Yes (via `{{ key }}`) |
| Campaign `campaign_variables` | All batches in this campaign | Yes (via `{{vars.*}}`) | No |
| Request `request_variables` | Single request only | Yes (via `{{vars.*}}`) | No |

Request variables override campaign variables of the same name. Campaign variables override agent variables of the same name.
