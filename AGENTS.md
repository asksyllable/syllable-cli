# Syllable CLI — Agent Reference

Machine-readable reference for AI agents using the Syllable CLI. The `syllable` binary manages all resources on the Syllable AI platform: agents, channels, prompts, tools, sessions, outbound campaigns/batches, users, directory, insights, custom messages, language groups, organizations, data sources, voice groups, services, roles, incidents, pronunciations, session labels, session debug, takeouts, events, permissions, conversation config, and dashboards.

Run via: `cd scripts/syllable-cli && go run .` or build with `go build -o syllable .` (no binary is checked in; use releases or local build).

---

## Resource Overview

Quick reference for every resource: what it is, key fields, and hard constraints.

| Resource | Purpose | Key Fields | Constraints |
|----------|---------|------------|-------------|
| **Agents** | AI system that talks to users via a channel | prompt_id, timezone, opening_message, session_variables, tool_headers, voice_group_id, bridge_phrases_id | 1:1 with channel target — one agent per target. Test channel is auto-generated. `bridge_phrases_id` attaches a **Bridge Phrases** config; set it via `--file` (no dedicated flag). |
| **Prompts** | Natural language instructions defining agent behavior, backed by an LLM | name, provider, model, temperature, seed, tools, version | Every edit auto-creates a new version. Previous versions can be previewed and restored via `prompts history`. |
| **Tools** | APIs agents call during sessions (look up data, transfer calls, schedule, etc.) | endpoint_url, method, function definition (name, description, JSON Schema params), service_id, static_params, dynamic vars (`{{ vars.field }}`) | Requires a Service for auth. Three types: agent, step, system. `tools history <id>` lists past versions (version_number, operation, updated_by). |
| **Services** | Centralized credential store for tool authentication | name, auth_type (basic/bearer/custom), credentials | Credentials are masked and not stored by Syllable after configuration. Multiple tools may share one service. |
| **Channels** | Communication modes (voice, SMS, chat) | name, service, is_system | System channels include Freeswitch Twilio (voice) and Syllable Webchat (chat). Twilio channels configurable via CLI; others via SDK. |
| **Channel Targets** | Specific address (phone number or chat ID) where an agent operates | target (e.g., +18002832940 or chat-1-org-3), agent_id | 1:1 with agent — one agent per target. |
| **Data Sources** | Text knowledge bases agents can search | name (no whitespace), description, content | Connect to agents via a tool — not directly. Content should start with contextual framing for better search. Lower latency than embedding knowledge in a prompt. |
| **Sessions** | Individual voice or chat conversations | session_id, transcript, summary, tool_calls, duration, channel, user_id, labels, recording, is_test | Read-only. |
| **Session Labels** | Manual annotations on sessions | session_id, rating (Bad/OK/Good/N/A), issue_category, notes | **Immutable — once added, a label cannot be updated or deleted.** |
| **Outbound Campaigns** | Planned outreach efforts (agent makes calls) | name, channel_source, display_id (caller ID), rate_per_hour, operating_hours | — |
| **Outbound Batches** | Contact lists within a campaign | reference_id (unique), target (E.164 phone), arbitrary columns (accessible as agent prompt variables) | CSV required. Status values: PENDING, INITIATED, CONNECTED, CALL_COMPLETED, FAILED. |
| **Insights Workflows** | LLM-based automated evaluation of call sessions or recordings | name, type (Agent/Transfer/Folder), tools, sample_rate, duration_range, date_range | Types: Agent (AI portion), Transfer (includes post-transfer legs), Folder (uploaded recordings). |
| **Custom Messages** | Opening greetings for agents | name, text | Configured per agent. |
| **Voice Groups** | Voice configuration for agents | name, voices | Replaces language groups. |
| **Language Groups** | Voice configuration (deprecated) | name | **DEPRECATED — use voice-groups instead.** |
| **Users** | Platform accounts | email, role_id, first_name, last_name | `users me` looks up the email configured in `syllable setup`; it errors if no email is set (the API has no key-identity endpoint). |
| **Directory** | Member/contact list for call routing and transfers | name, type, phone | Used by agents at runtime for transfer targets and contact lookups. |
| **Roles** | User roles with associated permissions | name, permissions | — |
| **Pronunciations** | Custom TTS pronunciation dictionary | word, pronunciation | Downloadable as CSV. |
| **Dashboards** | Performance analytics | name | `sessions`, `session-events`, `session-transfers`, `session-summary` endpoints are **DEPRECATED** — use `list` and `fetch-info`. |
| **Takeouts** | Data export jobs | job_id, status, file_name | Workflow: create → poll with get → download. |
| **Incidents** | Platform incident tracking | name, status | — |
| **Organizations** | Current organization | display_name, slug, description | API exposes the **current** org only — `get` returns it; `update` modifies it. Create and delete are intentionally not exposed via CLI — use the web console for both. `sip-ip-ranges` (list/create/update/delete) manages signaling & media SIP IP ranges in CIDR notation. |
| **Permissions** | System-wide permissions | name | Read-only. |
| **Bridge Phrases** | Named, reusable hold-phrase configs spoken while a tool call is in flight | name, description, is_default, config (phrases.messages, phrases.localized, tools[].tool_name, smart_turn_timeout_seconds, randomize_bridge_phrases) | At most one non-deleted config per suborg may be the default. Name is unique per suborg. Attach to an agent via the agent's `bridge_phrases_id`; delete is blocked (400) while agents are attached. `update` is a PUT on the collection keyed by the body `id`, and replaces the fields you send. Distinct from **Conversation Config (Bridges)**. |
| **Conversation Config (Bridges)** | Bridge phrases — filler messages spoken during delays and tool calls (Console: Voices → Phrases) | first_slow_messages, very_slow_messages, tool_responses, messages, randomize_messages | Supports per-language overrides via `localized`. Scoped by optional `--agent-id` / `--tool-name`; a single config, not a named resource — see **Bridge Phrases** for those. |
| **Schema** | Explore API request/response shapes from embedded OpenAPI spec | type name | No API call made. Use before create/update to find required fields. |

---

## Resource Dependencies

Resources must be created in dependency order. You cannot reference a resource that does not yet exist.

```
Data Sources
    └── referenced by Tools (data source tool points at the data source)
            └── referenced by Prompts (tools listed in prompt definition)
                    └── required by Agents (agent has a prompt_id)
                            └── deployed on Channel Targets (target has an agent_id)
                                    └── reached by Users via Channels

Services
    └── required by Tools (every tool has a service_id for auth)
```

**Dependency summary:**

- **Agent** requires a **Prompt** (`prompt_id`).
- **Prompt** can reference any number of **Tools** (by name/ID).
- **Tool** requires a **Service** (`service_id`) for authentication.
- **Data Source** connects to an agent via a **Tool** — create the data source, create a tool that references it, add that tool to a prompt, and attach the prompt to an agent.
- **Channel Target** requires an **Agent** (`agent_id`).
- **Outbound Batch** belongs to an **Outbound Campaign**.
- **Insights Workflow** references **Insights Tool Configs** and **Tool Definitions**.
- **User** requires a **Role** (`role_id`).

**To fully set up a new agent from scratch:**
1. Create a **Service** (credentials for any external APIs you need).
2. Create **Tools** that use that service (and optionally reference Data Sources).
3. Create (or update) a **Prompt** that includes those tools.
4. Create an **Agent** with that prompt.
5. Assign the agent to a **Channel Target**.

---

## Setup — First Time and Any Config Changes

Any time the user needs to configure the CLI — whether it's the first time, adding a new org, adding a new environment, or updating an API key — run the setup tool:

```bash
syllable setup
```

This opens a browser UI where the user can safely manage all orgs, API keys, and environments. Always use this tool for config changes — never edit `~/.syllable/config.yaml` directly or ask the user to paste API keys into the terminal.

**Trigger `syllable setup` whenever the user says things like:**
- "I need to add a new org"
- "Add a new environment"
- "Update my API key"
- "Set up credentials for [org]"
- "I'm getting auth errors" (may need a key update)
- First use — if `~/.syllable/config.yaml` doesn't exist or has no orgs configured

**Check if setup is needed:**
```bash
syllable status
```
Exits 0 if at least one org is configured, exits 1 if not. Never cat the config file directly.

## Quick Start

Run the CLI from source — no binary is checked in:

```bash
cd scripts/syllable-cli && go run . --help
```

If you need to rebuild after modifying source:
```bash
cd scripts/syllable-cli
go build -o syllable .

# Build with a version string:
go build -ldflags "-X github.com/asksyllable/syllable-cli/cmd.Version=1.2.3" -o syllable .
```

Run the tests:
```bash
cd scripts/syllable-cli
go test ./...
```

Check the version:
```bash
syllable --version
```

Select an org per command:
```bash
syllable --org acme agents list
syllable --org globex sessions list
```

If `default_org` is set in `~/.syllable/config.yaml`, it's used when `--org` is omitted.

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--org` | `default_org` in config | Org name — selects the API key from config |
| `--env` | `default_env` in config (or `prod`) | Named environment — selects base URL from config |
| `--output` / `-o` | `table` | Output format: `table` or `json` |
| `--fields` | — | Comma-separated columns to show in table output (e.g. `id,name,type`) |
| `--dry-run` | `false` | Print the HTTP request that would be sent without executing it |
| `--debug` | `false` | Print HTTP request and response details to stderr |
| `--config` | `~/.syllable/config.yaml` | Config file path |
| `--version` / `-v` | — | Print the CLI version and exit |

Auth and base URL are sourced from `~/.syllable/config.yaml`. Use `--org` and `--env` to select the right credentials. Run `syllable status` to see what is configured. For non-interactive use (CI, scripts), the `SYLLABLE_API_KEY` environment variable takes priority over the config file — no `syllable setup` needed.

## Common List Flags

All list commands support:

| Flag | Default | Description |
|------|---------|-------------|
| `--page` | `0` | Page number (0-based) |
| `--limit` | `25` | Max items to return |
| `--search` | — | Search filter (field varies by resource) |

**Search fields by resource:**

| Resource | `--search` filters on |
|----------|----------------------|
| agents | name |
| prompts | name |
| tools | name |
| channels | name |
| sessions | agent_name |
| conversations | agent_name |
| outbound batches | batch_id |
| outbound campaigns | campaign_name |
| users | email |
| directory | name |
| insights workflows | name |
| insights folders | name |
| custom messages | name |
| language groups | name |
| organizations | display_name |

## Output and Scripting

### JSON output
Use `--output json` to get the raw API response:
```bash
syllable agents get 42 --output json
syllable agents list --output json | jq '.items[].name'
```

### Field filtering (table only)
Use `--fields` to show only specific columns. Case-insensitive, ignored for JSON output:
```bash
syllable agents list --fields id,name
syllable sessions list --fields session_id,timestamp,agent
```

### Stdin piping with `--file -`
All `--file` arguments accept `-` to read from stdin:
```bash
# Fetch an agent, modify it, push it back
syllable agents get 42 --output json | jq '.name = "New Name"' | syllable agents update 42 --file -
```

### Dry run
Use `--dry-run` to inspect what would be sent without making an API call. Exits 0. Output is always JSON:
```bash
syllable agents create --name "Test" --type voice --prompt-id 10 --timezone UTC --dry-run
syllable agents delete 42 --dry-run
```

### Debug
Use `--debug` to print HTTP request/response details to stderr (API key is masked):
```bash
syllable agents list --debug 2>debug.log
```

## Error Handling

Errors go to stderr; normal output goes to stdout. Exit code is non-zero on error.

With `--output json`, errors are structured:
```json
{"error": {"status_code": 404, "detail": {"detail": "Agent with ID 99 not found"}, "hint": "Resource not found. Use the `list` subcommand to find valid IDs."}}
```

Common hints:
- **401**: API key invalid — run `syllable users me` to verify
- **403**: Permission denied — check `syllable permissions list`
- **404**: Resource not found — use `list` to find valid IDs
- **409**: Resource already exists — use `list` to find it
- **422/400**: Validation failed — hint names the specific field(s) and suggests `syllable schema get <TypeName>`

## Shell Completion

Cobra generates completion scripts automatically:
```bash
syllable completion bash > /etc/bash_completion.d/syllable
syllable completion zsh > "${fpath[1]}/_syllable"
syllable completion fish > ~/.config/fish/completions/syllable.fish
syllable completion powershell > syllable.ps1
```

## Commands

### Agents
Agents are AI systems that communicate with users via channels. Configured with a prompt, timezone, optional opening message, session variables (`{vars.session_variable}` syntax), tool headers, and optional voice group. **Constraint: 1:1 with channel target.** A test channel is auto-generated for each agent. `send-test-message` exchanges a single turn with an agent via the conversation-test API (`POST /api/v1/agents/test/messages`): pass `--session-start` on the first turn and reuse `--test-id` for follow-ups; `--override-timestamp` pins the agent's wall clock for deterministic date-sensitive tests and must be a **timezone-naive** ISO 8601 value like `2030-12-25T09:30:00` (interpreted in the agent's timezone) — offset, `Z`, and space-separated forms are silently ignored by the server (omit to use the current time).
```bash
syllable agents list [--page N] [--limit N] [--search TEXT]
syllable agents get <agent-id>
syllable agents create --file agent.json
syllable agents create --name NAME --type TYPE --prompt-id ID --timezone TZ
syllable agents update <agent-id> --file agent.json
syllable agents delete <agent-id>
syllable agents voices
syllable agents labels
syllable agents send-test-message <agent-id> --test-id ID [--session-start] [--text TEXT] [--override-timestamp ISO8601]
```
**Table columns:** ID, NAME, TYPE, LABEL, DESCRIPTION, UPDATED

### Channels
Channels define communication modes (voice, SMS, chat). Targets are the specific phone number or chat ID where an agent operates. **Constraint: 1:1 agent-to-target.** Twilio channels configurable via CLI; other channels via SDK.
```bash
syllable channels list [--page N] [--limit N] [--search TEXT]
syllable channels get <channel-id>   # resolved via the list endpoint (the API has no by-ID GET)
syllable channels create --file channel.json
syllable channels create --name NAME --service SERVICE
syllable channels update <channel-id> --file channel.json

# Channel Targets
syllable channels targets list <channel_id>
syllable channels targets get <channel_id> <target_id>
syllable channels targets create <channel_id> --file target.json
syllable channels targets update <channel_id> <target_id> --file target.json
syllable channels targets delete <channel_id> <target_id>

# Available Targets
syllable channels available-targets

# Twilio
syllable channels twilio get
syllable channels twilio create --file twilio.json
syllable channels twilio update --file twilio.json
syllable channels twilio numbers-list
syllable channels twilio numbers-add --file number.json
syllable channels twilio numbers-update --file number.json
syllable channels twilio numbers-verify-a2p-compliance <channel-id> --phone +18042221111
```
**Table columns:** ID, NAME, SERVICE, IS_SYSTEM

### Conversations
```bash
syllable conversations list [--page N] [--limit N] [--search TEXT] [--start-date DATE] [--end-date DATE]
```
**Table columns:** CONVERSATION_ID, TIMESTAMP, AGENT, TYPE, PROMPT
**Date format:** ISO 8601 (e.g. `2024-01-01T00:00:00Z`)

### Prompts
Natural language instruction sets defining agent behavior. Backed by an LLM (Azure OpenAI, Google, or OpenAI). Key fields: provider, model (e.g., `gpt-4o`, `gpt-4.1`, `gemini-2.0-flash`), temperature (0=deterministic), seed (integer for reproducible outputs), tools list. **Every edit auto-creates a new version.** Use `history` to view and restore previous versions.
```bash
syllable prompts list [--page N] [--limit N] [--search TEXT]
syllable prompts get <prompt-id>
syllable prompts create --file prompt.json
syllable prompts update <prompt-id> --file prompt.json
syllable prompts delete <prompt-id>
syllable prompts history <prompt-id>
syllable prompts supported-llms
```
**Table columns:** ID, NAME, TYPE, VERSION, AGENTS, LAST_UPDATED

### Tools
APIs agents call during sessions. Requires a Service for auth. Three types: agent tools (called during user sessions), step tools (structured multi-step workflows), system tools (pre-built: `hangup`, `transfer`, `web_search`, `summary-tool`). Dynamic variables use `{{ vars.field }}` syntax.
```bash
syllable tools list [--page N] [--limit N] [--search TEXT]
syllable tools get <tool-name>       # numeric IDs from the list's ID column are resolved to the name
syllable tools create --file tool.json
syllable tools create --name NAME --service-id ID
syllable tools update <tool-name> --file tool.json
syllable tools delete <tool-name>
syllable tools history <tool-id> [--page N] [--limit N] [--order-by-direction asc|desc]
```
**Table columns:** ID, NAME, SERVICE, LAST_UPDATED, LAST_UPDATED_BY
**`history` columns:** VERSION, OPERATION, NAME, UPDATED_BY, CREATED_AT, COMMENTS

### Sessions
Individual voice or chat conversations. Captures transcript, AI summary, tool calls with arguments and API responses, duration, channel, user ID, labels, recording, and is_test flag. Read-only — no create/update/delete.
```bash
syllable sessions list [--page N] [--limit N] [--start-date DATE] [--end-date DATE] [--search TEXT] [--search-field FIELD] [--include-test]
syllable sessions get <session-id>
syllable sessions transcript <session-id>
syllable sessions timeline <session-id>
syllable sessions summary <session-id>
syllable sessions latency <session-id>
syllable sessions recording <session-id>
syllable sessions recording-stream --token <token>
```
**List columns:** SESSION_ID, TIMESTAMP, AGENT, DURATION, SOURCE, TARGET, IS_TEST

**`--include-test`:** test sessions are hidden from `sessions list` by default; this flag adds them back. It keys off the **session-level** `is_test` flag, not off channel targets — conversation tests from `agents send-test-message` are returned even in an org with no channel targets.

### Outbound
```bash
# Batches
syllable outbound batches list [--page N] [--limit N] [--search TEXT]
syllable outbound batches get <batch-id>
syllable outbound batches create --file batch.json
syllable outbound batches update <batch-id> --file batch.json
syllable outbound batches delete <batch-id>
syllable outbound batches results <batch-id>
syllable outbound batches add-requests <batch-id> --file requests.json
syllable outbound batches remove-requests <batch-id> --file requests.json
syllable outbound batches upload <batch-id> --file contacts.csv

# Campaigns
syllable outbound campaigns list [--page N] [--limit N] [--search TEXT]
syllable outbound campaigns get <campaign-id>
syllable outbound campaigns create --file campaign.json
syllable outbound campaigns create --name NAME --caller-id PHONE
syllable outbound campaigns update <campaign-id> --file campaign.json
syllable outbound campaigns delete <campaign-id>
```

### Users
```bash
syllable users list [--page N] [--limit N] [--search TEXT]
syllable users get <email>
syllable users create --file user.json
syllable users create --email EMAIL --role-id ID [--first-name NAME] [--last-name NAME]
syllable users update <email> --file user.json
syllable users delete <email>
syllable users me
syllable users send-email <email>
syllable users delete-account --confirm
```
**Table columns:** ID, EMAIL, NAME, ROLE, STATUS, LAST_UPDATED

### Directory
Member/contact list for call routing and agent-initiated transfers.
```bash
syllable directory list [--page N] [--limit N] [--search TEXT]
syllable directory get <member-id>
syllable directory create --file member.json
syllable directory create --name NAME --type TYPE
syllable directory update <member-id> --file member.json
syllable directory delete <member-id>
syllable directory upload --file members.csv
syllable directory download [--format normalized|raw]
syllable directory history <member-id> [--page N] [--limit N] [--order-by-direction asc|desc]
syllable directory restore <member-id> [--comments TEXT]
syllable directory test <member-id> [--timestamp ISO8601] [--language-code BCP47]
```

### Insights
Automated LLM-based evaluation of call sessions or recordings. Three workflow types: Agent (analyzes AI agent portion), Transfer (includes post-transfer legs), Folder (processes uploaded recordings). Produces structured outputs (boolean, string, integer, array) that appear in Dashboards.
```bash
# Workflows
syllable insights workflows list
syllable insights workflows get <workflow-id>
syllable insights workflows create --file workflow.json
syllable insights workflows update --file workflow.json
syllable insights workflows delete <workflow-id>
syllable insights workflows activate <workflow-id>
syllable insights workflows inactivate <workflow-id>
syllable insights workflows queue-work --file work.json

# Folders
syllable insights folders list
syllable insights folders get <folder-id>
syllable insights folders create --file folder.json
syllable insights folders update --file folder.json
syllable insights folders delete <folder-id>
syllable insights folders files <folder-id>
syllable insights folders move-files <folder-id> --file move.json

# Tool Configs
syllable insights tool-configs list
syllable insights tool-configs get <config-id>
syllable insights tool-configs create --file config.json
syllable insights tool-configs update --file config.json
syllable insights tool-configs delete <config-id>

# Tool Definitions

# Test a tool against sample input
syllable insights tools-test --file sample.json
```

### Custom Messages
Opening greetings for agents, configured per agent.
```bash
syllable custom-messages list [--page N] [--limit N] [--search TEXT]
syllable custom-messages get <message-id>
syllable custom-messages create --file message.json
syllable custom-messages create --name NAME --text TEXT
syllable custom-messages update <message-id> --file message.json
syllable custom-messages delete <message-id>
```

### Language Groups
**DEPRECATED — use `voice-groups` instead.** Commands still functional for legacy resources.
```bash
syllable language-groups list [--page N] [--limit N] [--search TEXT]
syllable language-groups get <group-id>
syllable language-groups create --file group.json
syllable language-groups create --name NAME
syllable language-groups update <group-id> --file group.json
syllable language-groups delete <group-id>
```

### Data Sources
Text knowledge bases for agents. Name must have no whitespace. Content should start with contextual framing for better search. **Connect to agents via a tool — not directly.** Workflow: create data source → create tool referencing it → add tool to prompt → attach prompt to agent.
```bash
syllable data-sources list [--page N] [--limit N] [--search TEXT]
syllable data-sources get <data-source-id>
syllable data-sources create --file source.json
syllable data-sources create --name NAME --description DESC --text TEXT
syllable data-sources update --file source.json
syllable data-sources delete <data-source-id>
```

### Voice Groups
Voice configuration for agents. Replaces language groups.
```bash
syllable voice-groups list
syllable voice-groups get <voice-group-id>
syllable voice-groups create --file vg.json
syllable voice-groups update --file vg.json
syllable voice-groups delete <voice-group-id>
syllable voice-groups sample --file voice.json > sample.mp3
```

### Services
Centralized credential store for tool authentication. Auth types: `basic` (username/password), `bearer` (token), `custom` (multiple key-value headers). Credentials are masked and not stored by Syllable after configuration. Multiple tools can share one service.
```bash
syllable services list
syllable services get <service-id>
syllable services create --file service.json
syllable services create --name NAME --auth-type bearer
syllable services update --file service.json
syllable services delete <service-id>
```

### Roles
```bash
syllable roles list
syllable roles get <role-id>
syllable roles create --file role.json
syllable roles update --file role.json
syllable roles delete <role-id>
```

### Incidents
```bash
syllable incidents list [--page N] [--limit N] [--search TEXT]
syllable incidents get <incident-id>
syllable incidents create --file incident.json
syllable incidents update --file incident.json
syllable incidents delete <incident-id>
syllable incidents organizations
```

### Pronunciations
Custom TTS pronunciation dictionary. Downloadable as CSV.
```bash
syllable pronunciations list
syllable pronunciations get-csv
syllable pronunciations upload-csv --file pronunciations.csv --confirm
syllable pronunciations delete-csv --confirm
syllable pronunciations metadata
```
**`upload-csv` and `delete-csv` modify the org-wide pronunciation dictionary, which every live agent picks up immediately. Both require `--confirm`.**

### Session Labels
Manual annotations on sessions: rating (Bad/OK/Good/N/A), issue category, freetext notes. **CONSTRAINT: Immutable — once a label is added to a session, it cannot be updated or deleted.** There is no update or delete subcommand.
```bash
syllable session-labels list
syllable session-labels get <label-id>
syllable session-labels create --file label.json
```

### Session Debug
```bash
syllable session-debug by-session-id <session-id>
syllable session-debug by-sid <session-id> <sid>
syllable session-debug tool-result <session-id> <tool-result-id>
```

### Takeouts
Data export jobs. Workflow: create → poll with get until complete → download file.
```bash
syllable takeouts create --file takeout.json
syllable takeouts get <job-id>
syllable takeouts download <job-id> <file-name>
```

### Events
```bash
syllable events list [--page N] [--limit N]
```

### Permissions
```bash
syllable permissions list
```

### Bridge Phrases
Named, reusable bridge-phrase configs — the hold phrases an agent speaks while a tool call is in flight. Each config carries a default phrase set, optional per-tool overrides, and optional per-language variants. Attach one to an agent by setting the agent's `bridge_phrases_id`.
```bash
syllable bridge-phrases list [--page N] [--limit N] [--search TEXT]
syllable bridge-phrases get <bridge-phrases-id>
syllable bridge-phrases create --file bridge-phrases.json
syllable bridge-phrases create --name NAME [--description TEXT] [--default]
syllable bridge-phrases update <bridge-phrases-id> --file bridge-phrases.json
syllable bridge-phrases delete <bridge-phrases-id>
```
`config` fields: `phrases` (`{messages: [...], localized: {"<bcp-47>": {messages: [...]}}}`), `tools` (a list of `{tool_name, phrases}` overrides), `smart_turn_timeout_seconds` (0.25–30), `randomize_bridge_phrases`.

Constraints and gotchas:
- **`update` is a full replacement of the fields you send** — fetch the current config first rather than sending a partial body. Omitting `is_default` (or sending `null`) preserves the current flag.
- **`update` is a PUT on the collection**, keyed by the `id` in the body. The positional id is optional and is reconciled with the body — a mismatch is an error.
- **At most one non-deleted config per suborg may be the default.**
- **Names are unique per suborg** among non-deleted configs — a duplicate name is a 400 on create or update.
- **Delete is blocked while agents are attached** (400: `Cannot delete bridge phrases config with associated agents`) — clear or repoint each agent's `bridge_phrases_id` first.
- The inline `create` flags produce an **empty phrase set**; use `--file` to set the phrases themselves. See `syllable schema get BridgePhrasesCreateRequest`.
- `get` **summarizes** the nested config as a table (phrase preview, language tags, tool names). Use `--output json` for the full phrase lists.

**Not the same thing as `conversation-config bridges`** (below). `bridge-phrases` manages *named configs you attach to agents*; `conversation-config bridges` reads and writes a *single* config scoped to the org, an agent, or a tool. They share vocabulary, overlap in effect, and have **separate storage**.

**Confirm which surface you want before writing either — a write to the wrong one returns 200 and changes nothing the agent uses.** If you're not sure, check the Console: does the agent's config page let you set a bridge phrase config per agent? **Yes** → `bridge-phrases`. **No** → `conversation-config bridges-update`. `conversation-config` is slated for deprecation, so `bridge-phrases` is the forward-looking choice.

### Conversation Config
Bridge phrases — the filler messages an agent speaks while it is delayed or a tool call is in progress. This is the feature the Console shows under **Voices → Phrases**. Config fields: `first_slow_messages`, `very_slow_messages`, `tool_responses`, `smart_turn_timeout_seconds` (seconds of caller silence before the first bridge phrase; subsequent intervals are 2x/3x/4x this base), plus per-language overrides under `localized`. A unified `messages` list with `randomize_messages` supersedes the three legacy `*_messages` fields when non-empty.

**Slated for deprecation** in favor of `bridge-phrases` (above) — the named, agent-attachable config resource, a separate surface with its own storage. Confirm which surface an org uses before writing either; see the note under *Bridge Phrases*.
```bash
syllable conversation-config bridges
syllable conversation-config bridges-update --file bridges.json
```
Both commands accept optional `--agent-id <id>` and `--tool-name <name>` query parameters to scope the config to a specific agent or tool; omit both for the org-level default config.
```bash
syllable conversation-config bridges --agent-id 42
syllable conversation-config bridges --tool-name transfer_call
syllable conversation-config bridges-update --agent-id 42 --file bridges.json
```

### Dashboards
**DEPRECATION WARNING:** The `sessions`, `session-events`, `session-transfers`, and `session-summary` subcommands are deprecated. Use `list` and `fetch-info` instead.
```bash
syllable dashboards list
syllable dashboards fetch-info --name <dashboard-name>
# Deprecated (still functional but do not use for new work):
syllable dashboards sessions [--file query.json]
syllable dashboards session-events [--file query.json]
syllable dashboards session-transfers [--file query.json]
syllable dashboards session-summary [--file query.json]
```

### Organizations
Operations on the **current** organization (the one your API key is scoped to). `list` is a hidden alias of `get` for back-compat.

`update` uses multipart/form-data with explicit flags; logo is optional and replaces the current one if supplied.

Org provisioning (create) and deletion are intentionally not exposed via the CLI — both are platform-side admin operations with high blast radius. Use the web console for both.
```bash
syllable organizations get
syllable organizations update --display-name NAME [--logo PATH] [--description TEXT] [--domains a.com,b.com] [--saml-provider-id ID] [--update-comments TEXT]

# SIP IP ranges (signaling or media, CIDR notation)
syllable organizations sip-ip-ranges list
syllable organizations sip-ip-ranges create --type signaling --ip-range 192.168.1.0/24
syllable organizations sip-ip-ranges update <sip-ip-range-id> [--type TYPE] [--ip-range CIDR]
syllable organizations sip-ip-ranges delete <sip-ip-range-id>
```
**`sip-ip-ranges list` columns:** ID, TYPE, IP_RANGE, VERIFIED, CREATED_AT

### Schema
Explores API data structures from the embedded OpenAPI spec (no API call made). Use before create/update operations to discover required fields and body structure.
```bash
syllable schema list
syllable schema list --filter agent
syllable schema get AgentCreate
syllable schema get ChannelCreateRequest --output json
```
Schema names are case-insensitive.

## Auth

All requests use the header `Syllable-API-Key: <api-key>`. Auth is skipped for `help`, `completion`, and `version` commands. HTTP client timeout is 30 seconds.

## Dual Input Pattern

Most create commands support two modes:

1. **File mode:** `--file path/to/body.json` — reads a full JSON body from disk or stdin (`-`)
2. **Inline flags:** `--name`, `--type`, etc. — builds a minimal body with default empty maps/arrays

Update commands require `--file` (no inline flags).

## Source Layout

```
scripts/syllable-cli/
  main.go
  go.mod
  cmd/
    root.go              # cobra root, global flags, viper config, error hints
    agents.go
    channels.go
    ... (one file per resource)
    cmd_test.go
  internal/
    client/
      client.go          # HTTP client (DryRun, Verbose, APIError, DryRunResult)
      client_test.go
    output/
      output.go          # table, JSON, truncation, FilterColumns
      output_test.go
    spec/
      spec.go            # embeds openapi.json at build time
      spec_test.go
      openapi.json
```

## Testing

```bash
cd scripts/syllable-cli
go test ./...          # all packages
go test ./cmd/ -v      # command tests
go test ./internal/... # client, output, spec
```
