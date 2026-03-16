# Syllable CLI — Agent Reference

Machine-readable reference for AI agents using the Syllable CLI. The `syllable` binary manages all resources on the Syllable AI platform: agents, channels, prompts, tools, sessions, outbound campaigns/batches, users, directory, insights, custom messages, language groups, organizations, data sources, voice groups, services, roles, incidents, pronunciations, session labels, session debug, takeouts, events, permissions, conversation config, and dashboards.

Binary location: `scripts/syllable-cli/syllable`

## Setup — First Time and Any Config Changes

Any time the user needs to configure the CLI — whether it's the first time, adding a new org, adding a new environment, or updating an API key — run the setup tool:

```bash
cd scripts/syllable-setup && ./syllable-setup
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

The CLI binary (`syllable`) is pre-built and checked into the repo — no compilation needed:

```bash
scripts/syllable-cli/syllable --help
```

If you need to rebuild after modifying source:
```bash
cd scripts/syllable-cli
go build -o syllable .

# Build with a version string:
go build -ldflags "-X github.com/syllable-ai/syllable-cli/cmd.Version=1.2.3" -o syllable .
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
syllable --org nyp agents list
syllable --org hm sessions list
```

If `default_org` is set in `~/.syllable/config.yaml`, it's used when `--org` is omitted.

## Global Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--org` | — | `default_org` in config | Org name to look up API key |
| `--env` | `SYLLABLE_ENV` | `default_env` in config | Named environment: `prod`, `staging`, `dev` |
| `--api-key` | `SYLLABLE_API_KEY` | — | API key directly (overrides `--org`) |
| `--base-url` | — | `https://api.syllable.cloud` | API base URL (overrides `--env`) |
| `--output` / `-o` | — | `table` | Output format: `table` or `json` |
| `--fields` | — | — | Comma-separated columns to show in table output (e.g. `id,name,type`) |
| `--dry-run` | — | `false` | Print the HTTP request that would be sent without executing it |
| `--debug` | — | `false` | Print HTTP request and response details to stderr |
| `--config` | — | `~/.syllable/config.yaml` | Config file path |
| `--version` / `-v` | — | — | Print the CLI version and exit |

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
```bash
syllable agents list [--page N] [--limit N] [--search TEXT]
syllable agents get <agent-id>
syllable agents create --file agent.json
syllable agents create --name NAME --type TYPE --prompt-id ID --timezone TZ
syllable agents update <agent-id> --file agent.json
syllable agents delete <agent-id>
```
**Table columns:** ID, NAME, TYPE, LABEL, DESCRIPTION, UPDATED

### Channels
```bash
syllable channels list [--page N] [--limit N] [--search TEXT]
syllable channels create --file channel.json
syllable channels create --name NAME --service SERVICE
syllable channels update <channel-id> --file channel.json
syllable channels delete <channel-id>

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
```
**Table columns:** ID, NAME, SERVICE, IS_SYSTEM

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
syllable prompts update <prompt-id> --file prompt.json
syllable prompts delete <prompt-id>
syllable prompts history <prompt-id>
syllable prompts supported-llms
```
**Table columns:** ID, NAME, TYPE, VERSION, AGENTS, LAST_UPDATED

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

### Sessions
```bash
syllable sessions list [--page N] [--limit N] [--start-date DATE] [--end-date DATE]
syllable sessions get <session-id>
syllable sessions transcript <session-id>
syllable sessions summary <session-id>
syllable sessions latency <session-id>
syllable sessions recording <session-id>
```
**List columns:** SESSION_ID, TIMESTAMP, AGENT, DURATION, SOURCE, TARGET, IS_TEST

### Outbound
```bash
# Batches
syllable outbound batches list [--page N] [--limit N] [--search TEXT]
syllable outbound batches get <batch-id>
syllable outbound batches create --file batch.json
syllable outbound batches update <batch-id> --file batch.json
syllable outbound batches delete <batch-id>
syllable outbound batches results <batch-id>
syllable outbound batches requests <batch-id> --file requests.json
syllable outbound batches remove-requests <batch-id> --file requests.json

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
```
**Table columns:** ID, EMAIL, NAME, ROLE, STATUS, LAST_UPDATED

### Directory
```bash
syllable directory list [--page N] [--limit N] [--search TEXT]
syllable directory get <member-id>
syllable directory create --file member.json
syllable directory create --name NAME --type TYPE
syllable directory update <member-id> --file member.json
syllable directory delete <member-id>
```

### Insights
```bash
# Workflows
syllable insights workflows list
syllable insights workflows get <workflow-id>
syllable insights workflows create --file workflow.json
syllable insights workflows update --file workflow.json
syllable insights workflows delete <workflow-id>
syllable insights workflows activate <workflow-id>
syllable insights workflows inactivate <workflow-id>

# Folders
syllable insights folders list
syllable insights folders get <folder-id>
syllable insights folders create --file folder.json
syllable insights folders update --file folder.json
syllable insights folders delete <folder-id>
syllable insights folders files <folder-id>

# Tool Configs
syllable insights tool-configs list
syllable insights tool-configs get <config-id>
syllable insights tool-configs create --file config.json
syllable insights tool-configs update --file config.json
syllable insights tool-configs delete <config-id>

# Tool Definitions
syllable insights tool-definitions list
```

### Custom Messages
```bash
syllable custom-messages list [--page N] [--limit N] [--search TEXT]
syllable custom-messages get <message-id>
syllable custom-messages create --file message.json
syllable custom-messages create --name NAME --text TEXT
syllable custom-messages update <message-id> --file message.json
syllable custom-messages delete <message-id>
```

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
syllable data-sources list [--page N] [--limit N] [--search TEXT]
syllable data-sources get <data-source-id>
syllable data-sources create --file source.json
syllable data-sources create --name NAME --description DESC --content TEXT
syllable data-sources update --file source.json
syllable data-sources delete <data-source-id>
```

### Voice Groups
```bash
syllable voice-groups list
syllable voice-groups get <voice-group-id>
syllable voice-groups create --file vg.json
syllable voice-groups update --file vg.json
syllable voice-groups delete <voice-group-id>
```

### Services
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
```bash
syllable pronunciations list
syllable pronunciations get-csv
syllable pronunciations metadata
```

### Session Labels
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
syllable dashboards fetch-info --name <dashboard-name>
```

### Organizations
```bash
syllable organizations list [--page N] [--limit N] [--search TEXT]
```
Read-only — no create, update, or delete.

### Schema
Explores API data structures from the embedded OpenAPI spec (no API call made):
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
