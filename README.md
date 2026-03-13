---
name: syllable-cli
description: Use when the user wants to interact with the Syllable platform via the Go CLI at `.claude/skills/syllable-cli/scripts/syllable-cli/`. Use for listing, getting, creating, updating, or deleting agents, channels, prompts, tools, sessions, outbound campaigns/batches, users, directory members, insights workflows/folders/tool-configs/tool-definitions, custom messages, language groups, organizations, data sources, voice groups, services, roles, incidents, pronunciations, session labels, session debug, takeouts, events, permissions, conversation config, or dashboards using the `syllable` binary. Also use for exploring API schemas and data structures.
---

# Syllable CLI

A Go CLI for managing the Syllable AI platform. Source is at `.claude/skills/syllable-cli/scripts/syllable-cli/`.

## Setup — First Time and Any Config Changes

Any time the user needs to configure the CLI — whether it's the first time, adding a new org, adding a new environment, or updating an API key — run the setup tool:

```bash
cd .claude/skills/syllable-cli/scripts/syllable-setup
./syllable-setup
```

This opens a browser UI where the user can safely manage all orgs, API keys, and environments. Always use this tool for config changes — never edit `~/.syllable/config.yaml` directly or ask the user to paste API keys into the terminal.

**Trigger `syllable-setup` whenever the user says things like:**
- "I need to add a new org"
- "Add a new environment"
- "Update my API key"
- "Set up credentials for [org]"
- "I'm getting auth errors" (may need a key update)
- First use — if `~/.syllable/config.yaml` doesn't exist or has no orgs configured

**Check if setup is needed:**
```bash
cat ~/.syllable/config.yaml 2>/dev/null || echo "not configured"
```

## Quick Start

The CLI binary (`syllable`) is pre-built and checked into the repo — no compilation needed. Just run it directly:

```bash
.claude/skills/syllable-cli/scripts/syllable-cli/syllable --help
```

If you need to rebuild after modifying source:
```bash
cd .claude/skills/syllable-cli/scripts/syllable-cli
go build -o syllable .
```

Run the tests:
```bash
cd .claude/skills/syllable-cli/scripts/syllable-cli
go test ./...
```

Select an org per command:
```bash
syllable --org nyp agents list
syllable --org hm sessions list
```

If `default_org` is set in `~/.syllable/config.yaml`, it's used when `--org` is omitted.

## Global Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--org` | -- | `default_org` in config | Org name to look up API key (e.g. `nyp`, `hm`) |
| `--api-key` | `SYLLABLE_API_KEY` | -- | API key directly (overrides `--org`) |
| `--base-url` | `SYLLABLE_BASE_URL` | `https://api.syllable.cloud` | API base URL |
| `--output` / `-o` | -- | `table` | Output format: `table` or `json` |
| `--config` | -- | `~/.syllable/config.yaml` | Config file path |

## Common List Flags

All list commands support these flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--page` | `0` | Page number (0-based) |
| `--limit` | `25` | Max items to return |
| `--search` | — | Search filter (field varies by resource — see table below) |

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

## Commands

### Agents
```bash
syllable agents list [--page N] [--limit N] [--search TEXT]
syllable agents get <agent-id>
syllable agents create --file agent.json
syllable agents create --name NAME --type TYPE --prompt-id ID --timezone TZ
syllable agents update <agent-id> --file agent.json
syllable agents delete <agent-id>
```

**Table columns:** ID, NAME, TYPE, LABEL, DESCRIPTION, UPDATED

**Get fields:** ID, Name, Type, Label, Description, Prompt ID, Timezone, Updated At, Last Updated By

**Inline create defaults:** Includes empty `variables` and `tool_headers` maps.

### Channels
```bash
syllable channels list [--page N] [--limit N] [--search TEXT]
syllable channels get <channel-id>
syllable channels create --file channel.json
syllable channels create --name NAME --service SERVICE
syllable channels update <channel-id> --file channel.json
syllable channels delete <channel-id>
```

**Table columns:** ID, NAME, SERVICE, IS_SYSTEM

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
```

**Table columns:** ID, CHANNEL, TARGET, MODE, AGENT_ID, IS_TEST

**Note:** `targets list` lists all targets across all channels (no channel ID argument). `targets get/create/update/delete` require a channel ID.

### Conversations
```bash
syllable conversations list [--page N] [--limit N] [--search TEXT] [--start-date DATE] [--end-date DATE]
```

**Table columns:** CONVERSATION_ID, TIMESTAMP, AGENT, TYPE, PROMPT

**Date format:** ISO 8601 (e.g. `2024-01-01T00:00:00Z`)

### Prompts
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

### Tools
```bash
syllable tools list [--page N] [--limit N] [--search TEXT]
syllable tools get <tool-name>
syllable tools create --file tool.json
syllable tools create --name NAME --service-id ID
syllable tools update <tool-name> --file tool.json
syllable tools delete <tool-name>
```

**Table columns:** ID, NAME, SERVICE, LAST_UPDATED, LAST_UPDATED_BY

**Get fields:** ID, Name, Service Name, Service ID, Last Updated, Last Updated By

**Inline create defaults:** Includes empty `definition` map.

### Sessions
```bash
syllable sessions list [--page N] [--limit N] [--start-date DATE] [--end-date DATE]
syllable sessions get <session_id>
syllable sessions transcript <session_id>
syllable sessions summary <session_id>
syllable sessions latency <session_id>
syllable sessions recording <session_id>
```

**Date format:** ISO 8601 (e.g. `2024-01-01T00:00:00Z`)

**List table columns:** SESSION_ID, TIMESTAMP, AGENT, DURATION, SOURCE, TARGET, IS_TEST

**Get fields:** Session ID, Conversation ID, Timestamp, Agent, Agent Type, Timezone, Prompt, Duration, Source, Target, Is Test

**Transcript table columns:** TIME, ROLE, CONTENT (content truncated to 80 chars)

**Summary fields:** Rating, Summary

### Outbound

#### Batches
```bash
syllable outbound batches list [--page N] [--limit N] [--search TEXT]
syllable outbound batches get <batch-id>
syllable outbound batches create --file batch.json
syllable outbound batches update <batch_id> --file batch.json
syllable outbound batches delete <batch_id>
syllable outbound batches results <batch_id>
syllable outbound batches requests <batch_id> --file requests.json
syllable outbound batches remove-requests <batch_id> --file requests.json

**Get fields:** Batch ID, Campaign ID, Status, Paused, Expires On, Created At, Last Updated At, Last Updated By, Error Message

**Inline create:** `--paused` flag defaults to `false`.

#### Campaigns
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

### Users
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

**Note:** Delete uses query parameter (`DELETE /api/v1/users/?email=...`) instead of URL path — unlike other resources.

### Directory
```bash
syllable directory list [--page N] [--limit N] [--search TEXT]
syllable directory get <member-id>
syllable directory create --file member.json
syllable directory create --name NAME --type TYPE
syllable directory update <member-id> --file member.json
syllable directory delete <member-id>
```

**Table columns:** ID, NAME, TYPE, CREATED_AT, UPDATED_AT

**Get fields:** ID, Name, Type, Comments, Created At, Updated At, Last Updated By

### Insights

#### Workflows
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

### Custom Messages
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

### Language Groups
```bash
syllable language-groups list [--page N] [--limit N] [--search TEXT]
syllable language-groups get <group-id>
syllable language-groups create --file group.json
syllable language-groups create --name NAME
syllable language-groups update <group-id> --file group.json
syllable language-groups delete <group-id>
```

### Data Sources
```bash
syllable data-sources list [--page N] [--limit N] [--search name=foo]
syllable data-sources get <data_source_id>
syllable data-sources create --file source.json
syllable data-sources create --name "FAQ" --description "FAQ data" --content "Q&A text..."
syllable data-sources update --file source.json
syllable data-sources delete <data_source_id>
```

### Voice Groups
```bash
syllable voice-groups list
syllable voice-groups get <voice_group_id>
syllable voice-groups create --file vg.json
syllable voice-groups update --file vg.json
syllable voice-groups delete <voice_group_id>
```

### Services
```bash
syllable services list
syllable services get <service_id>
syllable services create --file service.json
syllable services create --name "My Service" --auth-type bearer
syllable services update --file service.json
syllable services delete <service_id>
```

### Roles
```bash
syllable roles list
syllable roles get <role_id>
syllable roles create --file role.json
syllable roles update --file role.json
syllable roles delete <role_id>
```

### Incidents
```bash
syllable incidents list [--page N] [--limit N] [--search title]
syllable incidents get <incident_id>
syllable incidents create --file incident.json
syllable incidents update --file incident.json
syllable incidents delete <incident_id>
syllable incidents organizations
```

### Pronunciations
```bash
syllable pronunciations list
syllable pronunciations get-csv
syllable pronunciations metadata
```

### Session Labels
```bash
syllable session-labels list
syllable session-labels get <label_id>
syllable session-labels create --file label.json
```

### Session Debug
```bash
syllable session-debug by-session-id <session_id>
syllable session-debug by-sid <session_id> <sid>
syllable session-debug tool-result <session_id> <tool_result_id>
```

### Takeouts
```bash
syllable takeouts create --file takeout.json
syllable takeouts get <job_id>
syllable takeouts download <job_id> <file_name>
```

### Events
```bash
syllable events list [--page N] [--limit N]
```

### Permissions
```bash
syllable permissions list
```

### Conversation Config
```bash
syllable conversation-config bridges
syllable conversation-config bridges-update --file bridges.json
```

### Dashboards
```bash
syllable dashboards list
syllable dashboards sessions [--file query.json]
syllable dashboards session-events [--file query.json]
syllable dashboards session-transfers [--file query.json]
syllable dashboards session-summary [--file query.json]
syllable dashboards fetch-info --name <dashboard_name>
```

### Organizations
```bash
syllable organizations list [--page N] [--limit N] [--search TEXT]
```

**Table columns:** ID, DISPLAY_NAME, SLUG, DESCRIPTION, LAST_UPDATED

**Note:** Read-only — no create, update, or delete operations.

### Schema
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

Schema names are case-insensitive. Use `--output json` to get raw JSON output.

**List table columns:** SCHEMA, DESCRIPTION (description truncated to 60 chars)

## Auth

All requests use the header:
```
Syllable-API-Key: <api-key>
```

Auth is skipped for `help` and `completion` commands. HTTP client timeout is 30 seconds.

## Dual Input Pattern

Most create commands support two modes:

1. **File mode:** `--file path/to/body.json` — reads a full JSON body from disk
2. **Inline flags:** Individual flags like `--name`, `--type`, etc. — builds a minimal body with default empty maps/arrays for nested fields

Update commands require `--file` (no inline flags).

## API Patterns

- **Endpoints:** `/api/v1/{resource}/?page=...&limit=...`
- **Search:** `&search_fields=<field>&search_field_values=<value>`
- **Date filters:** `&start_datetime=ISO8601&end_datetime=ISO8601`
- **Default page size:** 25 items
- **Table fallback:** If table format parsing fails, falls back to JSON output

## Source Layout

```
syllable-cli/
  main.go
  go.mod
  cmd/
    root.go              # cobra root, global flags, viper config
    agents.go
    channels.go
    conversations.go
    conversation_config.go
    custom_messages.go
    dashboards.go
    data_sources.go
    directory.go
    events.go
    incidents.go
    insights.go
    language_groups.go
    organizations.go
    outbound.go
    permissions.go
    prompts.go
    pronunciations.go
    roles.go
    schema.go
    services.go
    session_debug.go
    session_labels.go
    sessions.go
    takeouts.go
    tools.go
    users.go
    voice_groups.go
    cmd_test.go          # 53 unit tests
  internal/
    client/
      client.go          # HTTP client with Syllable-API-Key auth
      client_test.go     # HTTP client tests (GET/POST/PUT/DELETE, errors, headers)
    output/
      output.go          # table (text/tabwriter) + JSON rendering + truncation
      output_test.go     # table rendering, JSON pretty-print, truncation tests
    spec/
      spec.go            # embeds openapi.json at build time
      spec_test.go       # validates embedded spec is valid JSON with schemas
      openapi.json       # Syllable OpenAPI spec (source of truth for schema cmd)
```

## Testing

Run all tests:
```bash
cd .claude/skills/syllable-cli/scripts/syllable-cli
go test ./...
```

Run with verbose output:
```bash
go test ./... -v
```

Run a specific package:
```bash
go test ./internal/client/ -v
go test ./internal/output/ -v
go test ./internal/spec/ -v
go test ./cmd/ -v
```

### Test Coverage

**`internal/client/`** — HTTP client tests using `httptest.Server`:
- Constructor validation (base URL, API key, 30s timeout)
- GET/POST/PUT/DELETE methods with header verification (`Syllable-API-Key`, `Content-Type`, `Accept`)
- POST body serialization
- Error handling for 4xx/5xx responses
- Unreachable server handling
- Nil body handling

**`internal/output/`** — Output formatting tests:
- `Truncate()` — boundary cases (shorter than max, exact max, ellipsis, max ≤ 3, empty string)
- `PrintTable()` — header + separator + data rows, empty table
- `PrintJSON()` — pretty-printing with indentation, invalid JSON fallback

**`internal/spec/`** — Embedded OpenAPI spec validation:
- Spec is non-empty (embed succeeded)
- Spec is valid JSON
- Spec contains `components.schemas` with entries
- Spec contains `paths` with entries

**`cmd/`** — Command structure and functional tests:
- Root command registers all 14 subcommands
- Each resource command registers expected subcommands (list/get/create/update/delete)
- Agents list/get/create/delete against mock HTTP server
- Search parameter construction (`search_fields` + `search_field_values`)
- Inline flag create with default body structures
- Missing required flags produce errors
- File-based create reads and posts JSON
- Sessions date filter construction
- Sessions transcript and summary output
- Schema list, filter, get, and case-insensitive lookup
- Global flag presence and shorthand validation

## Workflow Tips

- Use `--output json` to get raw API responses for scripting or piping to `jq`
- Create/update commands read a JSON body from `--file path/to/body.json`
- List commands default to 25 results per page; use `--limit` and `--page` to paginate
- When building or modifying the CLI, always run `go build ./...` from `.claude/skills/syllable-cli/scripts/syllable-cli/` to verify compilation
- Run `go test ./cmd/ -v` to execute all 53 unit tests
