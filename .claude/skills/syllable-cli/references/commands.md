# Syllable CLI command reference

Per-resource command syntax, table-column listings, get-fields, and inline-create defaults for every resource the `syllable` CLI exposes. Read this file when the inline `Resources & Verbs` summary in `SKILL.md` doesn't cover what you need (rarer verbs, exact flag names, get-fields, table-column lists).

> **Every `delete` below is gated (v1.8):** it prompts for confirmation interactively and **refuses non-interactively unless you pass `--yes`/`-y`**. The examples omit `--yes` for brevity — add it when scripting or driving the CLI from an agent. `--dry-run` previews without deleting. `update` commands also now accept an optional positional id (reconciled with the body's id) in addition to the `--file` body.

For everyday read/write verbs (`list`, `get`, `create --file`, `update --file`, `delete`) the inline summary is enough. Reach for this file when:

- The verb is resource-specific (`prompts history`, `outbound batches results`, `users send-email`, `channels available-targets`).
- You need to know which table columns or get-fields the CLI prints.
- You need an inline-create flag set (e.g. `agents create --name ... --type ... --prompt-id ... --timezone ...`).
- You're constructing a `--file body.json` and want to know what defaults the inline-create path provides.

## Status
```bash
syllable status
```

Shows all configured orgs and environments with API key presence. No API calls are made — reads from local config only. Use this to resolve ambiguous org references, confirm naming conventions, and verify the target before running commands.

## Agents
```bash
syllable agents list [--page N] [--limit N] [--search TEXT]
syllable agents get <agent-id>
syllable agents create --file agent.json
syllable agents create --name NAME --type TYPE --prompt-id ID --timezone TZ
syllable agents update <agent-id> --file agent.json
syllable agents delete <agent-id>
syllable agents send-test-message <agent-id> --test-id <id> [--session-start] --text "<msg>"
syllable agents labels    # list active agent labels across the org (v1.7)
syllable agents voices    # list available voices (PROVIDER, DISPLAY_NAME, GENDER, MODEL)
```

**Table columns:** ID, NAME, TYPE, LABEL, DESCRIPTION, UPDATED

**Get fields:** ID, Name, Type, Label, Description, Prompt ID, Timezone, Updated At, Last Updated By

**Inline create defaults:** Includes empty `variables` and `tool_headers` maps.

### Conversation Test API (`send-test-message`, v1.3.1+)

One turn at a time against the agent under test, no phone number or channel required. JSON output (`-o json`) returns `.response_text`, `.response.content.session_id`, and `.response.content.action`.

| Turn type | Command shape |
|---|---|
| Greeting (first turn) | `--test-id <id> --session-start --text "" -o json` |
| Follow-up | `--test-id <id> --text "<message>" -o json` (reuse the same `<id>`) |
| Silence | `--test-id <id> --text "" -o json` |
| DTMF | `--test-id <id> --text "9" -o json` |

**Always pass `--text`** — even `--text ""`. Omitting it on `--session-start` returns 500 ([cli#50](https://github.com/asksyllable/syllable-cli/issues/50)).

**Pin the clock for date-sensitive tests** — add `--override-timestamp <iso8601>` to any turn to fix the agent's perceived "now" (drives `get_current_datetime` and time-based greeting/message rules), so relative-date scenarios ("two weeks from today") stay deterministic instead of drifting daily.

```bash
# correct: timezone-NAIVE, read in the agent's timezone, repeated on every turn you want pinned
syllable agents send-test-message <id> --test-id t1 --session-start --override-timestamp "2030-12-25T09:30:00" --text ""
syllable agents send-test-message <id> --test-id t1 --override-timestamp "2030-12-25T09:30:00" --text "what's today's date?"
```

- **Format must be timezone-naive** `YYYY-MM-DDTHH:MM:SS` (no `-06:00` offset, no `Z`, no space separator). Other forms are ignored and fall back to the real clock.
- **Per-turn, not sticky** — pass it on every turn you want pinned, including `--session-start`.
- **Verify it took** via the session transcript `actions[].tool_result` for `get_current_datetime`, not a green run.

Full failure-mode detail in [gotchas.md](gotchas.md) § *Conversation Test API*; a server-side strictness fix is pending.

For multi-turn scripted testing, loop these commands turn by turn, reusing the same `--test-id` for the whole conversation.

## Channels
```bash
syllable channels list [--page N] [--limit N] [--search TEXT]
syllable channels get <channel-id>
syllable channels create --file channel.json
syllable channels create --name NAME --service SERVICE
syllable channels update <channel-id> --file channel.json
# No `channels delete` — the API has no delete-channel op (v1.8). Remove a target:
syllable channels targets delete <channel-id> <target-id> --yes
```

**Table columns:** ID, NAME, SERVICE, IS_SYSTEM

**`channels get` (v1.7)** resolves client-side via the list endpoint's id search — the API has no by-ID GET for channels — and prints exactly the shape of one item from `channels list -o json`. Not-found exits 1.

```bash
# Channel Targets
syllable channels targets list <channel_id>
syllable channels targets get <channel_id> <target_id>
syllable channels targets create <channel_id> --file target.json
syllable channels targets update <channel_id> <target_id> --file target.json
syllable channels targets delete <channel_id> <target_id>

# Available Targets (all orgs)
syllable channels available-targets

# Twilio Integration
syllable channels twilio get
syllable channels twilio create --file twilio.json
syllable channels twilio update --file twilio.json
syllable channels twilio numbers-list
syllable channels twilio numbers-add --file number.json
syllable channels twilio numbers-update --file number.json
syllable channels twilio numbers-verify-a2p-compliance <channel-id> --phone +18042221111   # v1.7
```

**A2P compliance check (v1.7):** reports whether a number on a Twilio channel sits on a Messaging Service with an approved US A2P brand and verified campaign. Reflects Twilio configuration only — not carrier per-number registration or legal/content compliance. `--phone` must be E.164 exactly as Twilio stores it.

**Table columns:** ID, CHANNEL, TARGET, MODE, AGENT_ID, IS_TEST

**Note:** `targets list` lists all targets across all channels (no channel ID argument). `targets get/create/update/delete` require a channel ID.

See `telephony-and-channels.md` for full telephony and channel config fields. Do not infer settings from channel names — check actual field values.

## Conversations
```bash
syllable conversations list [--page N] [--limit N] [--search TEXT] [--start-date DATE] [--end-date DATE]
```

**Table columns:** CONVERSATION_ID, TIMESTAMP, AGENT, TYPE, PROMPT

**Date format:** ISO 8601 (e.g. `2024-01-01T00:00:00Z`)

## Prompts
```bash
syllable prompts list [--page N] [--limit N] [--search TEXT]
syllable prompts get <prompt-id>
syllable prompts create --file prompt.json
syllable prompts update <prompt_id> --file prompt.json
syllable prompts delete <prompt_id>
syllable prompts history <prompt_id>
syllable prompts supported-llms
```

**Table columns:** ID, NAME, TYPE, VERSION, AGENTS, LAST_UPDATED

**Get fields:** ID, Name, Type, Description, Version, Agent Count, Last Updated, Context

**Inline create defaults:** Includes empty `llm_config` map.

**LLM Config:** Always run `syllable prompts supported-llms` before setting LLM config — it returns valid provider/model/version combos. See `gotchas.md` for LLM config validation rules and other platform gotchas.

**Edit comments:** Always include `edit_comments` in `PromptCreateRequest` / `PromptUpdateRequest` payloads — a one-line description of the change. It's the label for that revision in version history. See `gotchas.md` § *Validation*.

## Tools
```bash
syllable tools list [--page N] [--limit N] [--search TEXT]
syllable tools get <tool-name-or-id>
syllable tools create --file tool.json
syllable tools create --name NAME --service-id ID
syllable tools update <tool-name> --file tool.json
syllable tools delete <tool-name>
syllable tools history <tool-id> [--page N] [--limit N] [--order-by-direction asc|desc]   # v1.7
```

**Table columns:** ID, NAME, SERVICE, LAST_UPDATED, LAST_UPDATED_BY

**Get fields:** ID, Name, Service Name, Service ID, Last Updated, Last Updated By

**Inline create defaults:** Includes empty `definition` map.

**`get` accepts name or numeric ID (v1.7):** the API endpoint is name-keyed; an all-digits argument that 404s by name is resolved ID → name via the list endpoint and retried. `update`/`delete` remain **name-keyed only** — copy the NAME column, not the ID. `history` takes the numeric **ID** (VERSION, OPERATION, NAME, UPDATED_BY, CREATED_AT, COMMENTS per row).

## Sessions
```bash
syllable sessions list [--page N] [--limit N] [--start-date DATE] [--end-date DATE] [--search TEXT] [--search-field FIELD] [--include-test]
syllable sessions get <session_id>
syllable sessions transcript <session_id>
syllable sessions summary <session_id>
syllable sessions latency <session_id>
syllable sessions recording <session_id>
syllable sessions recording-stream --token <token>   # stream recording bytes to stdout
```

**Recording download:** `sessions recording <id>` returns short-lived streaming tokens; pass one to `recording-stream --token`, which writes raw bytes to stdout for redirection: `syllable sessions recording-stream --token <token> > recording.wav`.

**Date format:** ISO 8601 (e.g. `2024-01-01T00:00:00Z`)

**Search:** `--search` defaults to `agent_name`; `--search-field` switches it to any `SessionProperties` enum value (`syllable schema get SessionProperties`): `session_id`, `conversation_id`, `channel_manager_sid`, `agent_name`, `prompt_name`, `source`, `target`, `is_outbound`, `is_test`, … Resolve a Twilio `CA…` SID to its session in one call:
```bash
syllable sessions list --search-field channel_manager_sid --search "CA…" -o json
```
**Filter boolean fields client-side:** server-side `--search-field` filtering is reliable for string fields; for booleans (`is_outbound`, `is_test`) fetch with `-o json` and filter with `jq` (see `gotchas.md`). `--include-test` adds sessions flagged `is_test=true` to the list (they are hidden by default). It keys off the **session-level** `is_test` flag, not off channel targets: on an org with zero channel targets, conversation-test sessions from `agents send-test-message` (`channel_manager_type: web_chat_v1`) are still returned. Verified on `cli-test`, 2026-08-12 ([ZOO-8613](https://syllable.atlassian.net/browse/ZOO-8613)).

**Get fields:** Session ID, Conversation ID, Timestamp, Agent, Agent Type, Timezone, Prompt, Duration, Source, Target, Is Test

**Transcript table columns:** TIME, ROLE, CONTENT (content truncated to 80 chars)

**Summary fields:** Rating, Summary

## Outbound

### Batches
```bash
syllable outbound batches list [--page N] [--limit N] [--search TEXT]
syllable outbound batches get <batch-id>
syllable outbound batches create --file batch.json
syllable outbound batches update <batch_id> --file batch.json
syllable outbound batches delete <batch_id>
syllable outbound batches results <batch_id>
syllable outbound batches requests <batch_id> --file requests.json
syllable outbound batches remove-requests <batch_id> --file requests.json
```

**Get fields:** Batch ID, Campaign ID, Status, Paused, Expires On, Created At, Last Updated At, Last Updated By, Error Message

**Inline create:** `--paused` flag defaults to `false`.

### Campaigns
```bash
syllable outbound campaigns list [--page N] [--limit N] [--search TEXT]
syllable outbound campaigns get <campaign-id>
syllable outbound campaigns create --file campaign.json
syllable outbound campaigns create --name NAME --caller-id PHONE
syllable outbound campaigns update <campaign-id> --file campaign.json
syllable outbound campaigns delete <campaign-id>
```

**Table columns:** ID, NAME, MODE, CALLER_ID, DESCRIPTION, UPDATED

**Get fields:** ID, Campaign Name, Description, Mode, Caller ID, Source, Hourly Rate, Retry Count, Updated At, Last Updated By

**Inline create defaults:** Includes empty `campaign_variables` and `active_days`.

## Users
```bash
syllable users list [--page N] [--limit N] [--search email=foo]
syllable users get <email>
syllable users create --file user.json
syllable users create --email EMAIL --role-id ID [--first-name NAME] [--last-name NAME]
syllable users update <email> --file user.json
syllable users delete <email>
syllable users me
syllable users send-email <email>
```

**Table columns:** ID, EMAIL, NAME, ROLE, STATUS, LAST_UPDATED

**Get fields:** ID, Email, First Name, Last Name, Role Name, Role ID, Activity Status, Last Updated, Last Updated By, Last Session At

**Note:** Delete sends a JSON body (`{ "email": …, "reason": … }`) to `DELETE /api/v1/users/`, not a path or query identifier — unlike other resources.

**`users me` requires a configured email (v1.7):** with your email set via `syllable setup` it does an exact lookup of your account; without one it fails loudly (`Error: no email configured — run \`syllable setup\` …`, exit 1). The API has no key→identity endpoint, so there is no fallback. Pre-1.7 it silently returned the org's first user.

## Directory
```bash
syllable directory list [--page N] [--limit N] [--search TEXT]
syllable directory get <member-id>
syllable directory create --file member.json
syllable directory create --name NAME --type TYPE
syllable directory update <member-id> --file member.json
syllable directory delete <member-id>
syllable directory history <member-id>                              # version history
syllable directory restore <member-id>                              # restore a soft-deleted member
syllable directory test <member-id> [--timestamp ISO8601] [--language-code en-US]   # test extension lookup at a given time (defaults to now UTC)
syllable directory download [--format normalized|raw]               # bulk-export members
syllable directory upload --file <path>                             # bulk-import members (run `download --format raw` first to see the file shape)
```

**Table columns:** ID, NAME, TYPE, CREATED_AT, UPDATED_AT

**Get fields:** ID, Name, Type, Comments, Created At, Updated At, Last Updated By

## Insights

### Workflows
```bash
# Workflows
syllable insights workflows list
syllable insights workflows get <workflow_id>
syllable insights workflows create --file workflow.json
syllable insights workflows update --file workflow.json
syllable insights workflows delete <workflow_id>
syllable insights workflows activate <workflow_id>
syllable insights workflows inactivate <workflow_id>

# Folders
syllable insights folders list
syllable insights folders get <folder_id>
syllable insights folders create --file folder.json
syllable insights folders update --file folder.json
syllable insights folders delete <folder_id>
syllable insights folders files <folder_id>

# Tool Configs
syllable insights tool-configs list
syllable insights tool-configs get <config_id>
syllable insights tool-configs create --file config.json
syllable insights tool-configs update --file config.json
syllable insights tool-configs delete <config_id>

# Tool Definitions
syllable insights tool-definitions list
```

**Table columns:** ID, NAME, LABEL, CREATED_AT, UPDATED_AT

**Get fields:** ID, Name, Label, Description, Created At, Updated At, Last Updated By

## Custom Messages
```bash
syllable custom-messages list [--page N] [--limit N] [--search TEXT]
syllable custom-messages get <message-id>
syllable custom-messages create --file message.json
syllable custom-messages create --name NAME --text TEXT
syllable custom-messages update <message-id> --file message.json
syllable custom-messages delete <message-id>
```

**Table columns:** ID, NAME, TYPE, AGENTS, UPDATED_AT

**Get fields:** ID, Name, Type, Preamble, Text, Agent Count, Updated At, Last Updated By

## Language Groups

> **Deprecated.** The platform renamed Language Groups to Voice Groups. Use `voice-groups` commands for new work.

```bash
syllable language-groups list [--page N] [--limit N] [--search TEXT]
syllable language-groups get <group-id>
syllable language-groups create --file group.json
syllable language-groups create --name NAME
syllable language-groups update <group-id> --file group.json
syllable language-groups delete <group-id>
```

## Data Sources
```bash
syllable data-sources list [--page N] [--limit N] [--search name=foo]
syllable data-sources get <data_source_id>
syllable data-sources create --file source.json           # body requires: name, text (document body), chunk
syllable data-sources create --name "FAQ" --text "Q&A…"   # inline; chunk defaults to false
syllable data-sources update --file source.json
syllable data-sources delete <data_source_id>
```

## Voice Groups
```bash
syllable voice-groups list
syllable voice-groups get <voice_group_id>
syllable voice-groups create --file vg.json
syllable voice-groups update --file vg.json
syllable voice-groups delete <voice_group_id>
```

## Services
```bash
syllable services list
syllable services get <service_id>
syllable services create --file service.json
syllable services create --name "My Service" --auth-type bearer
syllable services update --file service.json
syllable services delete <service_id>
```

## Roles
```bash
syllable roles list
syllable roles get <role_id>
syllable roles create --file role.json
syllable roles update --file role.json
syllable roles delete <role_id>
```

## Incidents
```bash
syllable incidents list [--page N] [--limit N] [--search title]
syllable incidents get <incident_id>
syllable incidents create --file incident.json
syllable incidents update --file incident.json
syllable incidents delete <incident_id>
syllable incidents organizations
```

## Pronunciations
```bash
syllable pronunciations list
syllable pronunciations get-csv
syllable pronunciations metadata
```

## Session Labels
```bash
syllable session-labels list
syllable session-labels get <label_id>
syllable session-labels create --file label.json
```

## Session Debug
```bash
syllable session-debug by-session-id <session_id>
syllable session-debug by-sid <session_id> <sid>
syllable session-debug tool-result <session_id> <tool_result_id>
```

## Takeouts
```bash
syllable takeouts create --file takeout.json
syllable takeouts get <job_id>
syllable takeouts download <job_id> <file_name>
```

## Events
```bash
syllable events list [--page N] [--limit N]
```

## Permissions
```bash
syllable permissions list
```

## Conversation Config
```bash
syllable conversation-config bridges                          # get org-default bridge phrases (Console: Voices → Phrases)
syllable conversation-config bridges --agent-id 768           # agent-scoped (falls back to org default if no override)
syllable conversation-config bridges --tool-name transfer_call # tool-scoped
syllable conversation-config bridges-update --file bridges.json
syllable conversation-config bridges-update --agent-id 768 --file bridges.json  # agent-scoped override
```

**Scoping (CLI v2.1+):** both `bridges` and `bridges-update` accept optional `--agent-id <id>` and `--tool-name <name>` query parameters (combinable). Omitting both targets the org-level default. A scoped GET falls back to the org default when no override exists; a scoped PUT creates/updates that override without touching the org default. `--agent-id` is only sent when explicitly set, so an unset flag is not confused with `--agent-id 0`.

**Unified messages (v1.7.1 spec):** the bridges body now supports an ordered `messages` list plus `randomize_messages` (bool, no-repeat shuffling). When `messages` is non-empty it replaces the three legacy lists (`first_slow_messages`, `very_slow_messages`, `tool_responses`); when empty, the legacy fields still apply. Both `bridges` commands are pass-through `--file` bodies. **Caveat:** production doesn't persist these fields yet — read back after updating to confirm (see `gotchas.md` § *Bridge Phrases*). Field semantics in `telephony-and-channels.md` § *Bridge Phrases*.

**Smart-turn timeout (v1.7.2 spec):** the top-level bridges body also accepts `smart_turn_timeout_seconds` (number, `0.25`–`30`, or `null`) — seconds of caller silence before the first bridge phrase fires (later intervals scale 2x/3x/4x); unset uses the service default. Passes through the same `--file` body, but production doesn't persist it yet — read back to confirm.

## Dashboards
```bash
syllable dashboards list
syllable dashboards sessions [--file query.json]
syllable dashboards session-events [--file query.json]
syllable dashboards session-transfers [--file query.json]
syllable dashboards session-summary [--file query.json]
syllable dashboards fetch-info --name <dashboard_name>
```

## Organizations
```bash
syllable organizations get                                      # current org (the one --org authenticates as)
syllable organizations update --display-name NAME [--description ...] [--domains ...] [--logo path.png] [--saml-provider-id ...] [--update-comments ...]   # multipart/form-data; --display-name required
syllable organizations sip-ip-ranges list                       # v1.7
syllable organizations sip-ip-ranges create --ip-range CIDR --type signaling|media   # v1.7 (or --file body.json)
syllable organizations sip-ip-ranges update <range-id> --ip-range CIDR --type ...    # v1.7
syllable organizations sip-ip-ranges delete <range-id>          # v1.7
```

**Get table columns:** ID, NAME, DISPLAY_NAME, SLUG, DESCRIPTION, LAST_UPDATED

**Note:** Scoped to the **current org only** — the API key determines which org you see. Org create and delete are intentionally not exposed (platform-side admin operations). The older `organizations list` spelling still works but hits the same single-org endpoint — it does not list other orgs.

**SIP IP ranges (v1.7):** signaling vs media CIDR ranges for custom-SIP telephony (`OrganizationSipIpRange*` schemas). List columns: ID, TYPE, IP_RANGE, VERIFIED, CREATED_AT.

**Console org slug:** The `SLUG` column contains the Console org slug used in all Console URLs (`https://syllable.cloud/<slug>/...`). Look it up with:
```bash
syllable --env prod --org <org> organizations get
```

## Schema
Explore API data structures using the embedded OpenAPI spec. Uses local data only (no API calls), but the CLI still requires an API key to be configured.
```bash
# List all schemas
syllable schema list

# Filter by substring (case-insensitive)
syllable schema list --filter agent
syllable schema list --filter tool

# Get a full schema definition (case-insensitive name match)
syllable schema get AgentCreate
syllable schema get ToolCreateRequest
syllable schema get Session
```

Schema names are case-insensitive. Use `--output json` to get raw JSON output — pure JSON since v1.7 (no markdown heading), so `syllable schema get AgentCreate -o json | jq .` works directly.

**List table columns:** SCHEMA, DESCRIPTION (description truncated to 60 chars)
