# Syllable CLI

A command-line interface for managing the [Syllable AI platform](https://syllable.ai). Built in Go using [Cobra](https://github.com/spf13/cobra).

## Contents

- [Installation](#installation)
- [Configuration](#configuration)
- [Quick Start](#quick-start)
- [Global Flags](#global-flags)
- [Output Formats](#output-formats)
- [Scripting and Automation](#scripting-and-automation)
- [Commands](#commands)
- [Troubleshooting](#troubleshooting)

---

## Installation

The pre-built binary is included in the repo. No compilation needed:

```bash
# Add the binary to your PATH, or run it directly:
./scripts/syllable-cli/syllable --help
```

To rebuild from source (requires Go 1.21+):

```bash
cd scripts/syllable-cli
go build -o syllable .
```

To build with a version string embedded:

```bash
go build -ldflags "-X github.com/syllable-ai/syllable-cli/cmd.Version=1.2.3" -o syllable .
```

### Shell Completion

Enable tab completion for your shell:

```bash
# Bash
syllable completion bash > /etc/bash_completion.d/syllable

# Zsh
syllable completion zsh > "${fpath[1]}/_syllable"

# Fish
syllable completion fish > ~/.config/fish/completions/syllable.fish
```

---

## Configuration

Run the setup tool to configure orgs, API keys, and environments:

```bash
cd scripts/syllable-setup
./syllable-setup
```

This opens a browser UI. Use it any time you need to add an org, add an environment, or rotate a key. Do not edit `~/.syllable/config.yaml` by hand.

### Config File Structure

`~/.syllable/config.yaml` supports multiple orgs and environments:

```yaml
default_org: acme

orgs:
  acme:
    api_key: sk-prod-...
    envs:
      staging:
        api_key: sk-staging-...

environments:
  staging:
    base_url: https://staging.syllable.cloud
```

Built-in environment aliases (no config needed): `prod`, `staging`, `dev`.

---

## Quick Start

```bash
# Check the version
syllable --version

# List agents
syllable agents list

# Get a specific agent
syllable agents get 42

# Use a different org or environment
syllable --org acme --env staging agents list

# Create a resource from a JSON file
syllable agents create --file agent.json

# Delete a resource
syllable agents delete 42
```

---

## Global Flags

These flags work on every command:

| Flag | Default | Description |
|------|---------|-------------|
| `--org NAME` | `default_org` in config | Select org (looks up API key from config) |
| `--env NAME` | `default_env` in config | Select environment: `prod`, `staging`, `dev` |
| `--api-key KEY` | `SYLLABLE_API_KEY` env var | Provide API key directly |
| `--base-url URL` | `https://api.syllable.cloud` | Override API endpoint |
| `--output` / `-o` | `table` | Output format: `table` or `json` |
| `--fields` | — | Columns to show in table output (e.g. `id,name`) |
| `--dry-run` | `false` | Show the request that would be sent, without sending it |
| `--debug` | `false` | Print full HTTP request/response to stderr |
| `--config PATH` | `~/.syllable/config.yaml` | Config file location |
| `--version` / `-v` | — | Print version and exit |

### List Flags

All `list` subcommands also support:

| Flag | Default | Description |
|------|---------|-------------|
| `--page N` | `0` | Page number (0-based) |
| `--limit N` | `25` | Results per page |
| `--search TEXT` | — | Filter results (see [Search Fields](#search-fields)) |

---

## Output Formats

### Table (default)

Human-readable, aligned columns. Good for browsing:

```
ID   NAME              TYPE   UPDATED
--   ----              ----   -------
42   Support Bot       ca_v1  2024-01-15
```

### JSON

Full API response, pretty-printed. Good for scripting:

```bash
syllable agents get 42 --output json
```

### Filtering Columns

Use `--fields` to show only the columns you care about (case-insensitive):

```bash
syllable agents list --fields id,name
syllable sessions list --fields session_id,timestamp,agent,duration
```

---

## Scripting and Automation

### Pipe to jq

```bash
syllable agents list --output json | jq '.items[] | {id, name}'
```

### Read from stdin with `--file -`

Any command that takes `--file` also accepts `-` to read JSON from stdin:

```bash
# Fetch an agent, rename it, push it back
syllable agents get 42 --output json \
  | jq '.name = "Updated Name"' \
  | syllable agents update 42 --file -
```

### Dry Run

See exactly what would be sent to the API — method, URL, and full request body — without making any changes:

```bash
syllable agents create --name "Test Bot" --type voice --prompt-id 10 --timezone UTC --dry-run
```

```json
{
  "body": {"name": "Test Bot", "type": "voice", "prompt_id": "10", "timezone": "UTC", ...},
  "dry_run": true,
  "method": "POST",
  "url": "https://api.syllable.cloud/api/v1/agents/"
}
```

### Debug Mode

Print full HTTP request and response details to stderr (API key is masked):

```bash
syllable agents list --debug 2>debug.log
```

### Structured Errors for Scripts

When using `--output json`, errors are machine-readable:

```json
{
  "error": {
    "status_code": 422,
    "detail": {"detail": [{"loc": ["body", "name"], "msg": "field required"}]},
    "hint": "Validation failed on: name. Use `syllable schema list` to find the schema..."
  }
}
```

---

## Commands

### Agents

```bash
syllable agents list [--search TEXT]
syllable agents get <id>
syllable agents create --name NAME --type TYPE --prompt-id ID --timezone TZ
syllable agents create --file agent.json
syllable agents update <id> --file agent.json
syllable agents delete <id>
```

### Channels

```bash
syllable channels list [--search TEXT]
syllable channels create --file channel.json
syllable channels update <id> --file channel.json
syllable channels delete <id>

# Targets
syllable channels targets list <channel-id>
syllable channels targets get <channel-id> <target-id>
syllable channels targets create <channel-id> --file target.json
syllable channels targets update <channel-id> <target-id> --file target.json
syllable channels targets delete <channel-id> <target-id>

# Twilio integration
syllable channels twilio get
syllable channels twilio create --file twilio.json
syllable channels twilio numbers-list
syllable channels twilio numbers-add --file number.json
```

### Conversations

```bash
syllable conversations list [--start-date DATE] [--end-date DATE] [--search TEXT]
```

Dates are ISO 8601: `2024-01-01T00:00:00Z`

### Prompts

```bash
syllable prompts list [--search TEXT]
syllable prompts get <id>
syllable prompts create --file prompt.json
syllable prompts update <id> --file prompt.json
syllable prompts delete <id>
syllable prompts history <id>
syllable prompts supported-llms
```

### Tools

```bash
syllable tools list [--search TEXT]
syllable tools get <id>
syllable tools create --name NAME --service-id ID
syllable tools create --file tool.json
syllable tools update <id> --file tool.json
syllable tools delete <id>
```

### Sessions

```bash
syllable sessions list [--start-date DATE] [--end-date DATE]
syllable sessions get <id>
syllable sessions transcript <id>
syllable sessions summary <id>
syllable sessions latency <id>
syllable sessions recording <id>
```

### Outbound

```bash
# Campaigns
syllable outbound campaigns list [--search TEXT]
syllable outbound campaigns get <id>
syllable outbound campaigns create --name NAME --caller-id PHONE
syllable outbound campaigns create --file campaign.json
syllable outbound campaigns update <id> --file campaign.json
syllable outbound campaigns delete <id>

# Batches
syllable outbound batches list [--search TEXT]
syllable outbound batches get <id>
syllable outbound batches create --file batch.json
syllable outbound batches results <id>
syllable outbound batches requests <id> --file requests.json
syllable outbound batches remove-requests <id> --file requests.json
```

### Users

```bash
syllable users list [--search TEXT]
syllable users get <email>
syllable users me
syllable users create --email EMAIL --role-id ID [--first-name NAME] [--last-name NAME]
syllable users create --file user.json
syllable users update <email> --file user.json
syllable users delete <email>
syllable users send-email <email>
```

### Directory

```bash
syllable directory list [--search TEXT]
syllable directory get <id>
syllable directory create --name NAME --type TYPE
syllable directory update <id> --file member.json
syllable directory delete <id>
```

### Insights

```bash
syllable insights workflows list
syllable insights workflows get <id>
syllable insights workflows create --file workflow.json
syllable insights workflows activate <id>
syllable insights workflows inactivate <id>

syllable insights folders list
syllable insights folders get <id>
syllable insights folders files <id>

syllable insights tool-configs list
syllable insights tool-configs get <id>

syllable insights tool-definitions list
```

### Custom Messages

```bash
syllable custom-messages list [--search TEXT]
syllable custom-messages get <id>
syllable custom-messages create --name NAME --text TEXT
syllable custom-messages update <id> --file message.json
syllable custom-messages delete <id>
```

### Schema Explorer

Browse and inspect API data schemas — uses embedded OpenAPI spec, no API call needed:

```bash
# List all schemas
syllable schema list

# Filter by name
syllable schema list --filter agent

# See a schema's full definition
syllable schema get AgentCreate
syllable schema get PromptCreateRequest --output json
```

Use this when you need to know what fields a create or update body requires.

### Other Resources

| Command | Subcommands |
|---------|------------|
| `language-groups` | list, get, create, update, delete |
| `data-sources` | list, get, create, update, delete |
| `voice-groups` | list, get, create, update, delete |
| `services` | list, get, create, update, delete |
| `roles` | list, get, create, update, delete |
| `incidents` | list, get, create, update, delete, organizations |
| `pronunciations` | list, get-csv, metadata |
| `session-labels` | list, get, create |
| `session-debug` | by-session-id, by-sid, tool-result |
| `takeouts` | create, get, download |
| `events` | list |
| `permissions` | list |
| `conversation-config` | bridges, bridges-update |
| `dashboards` | list, sessions, session-events, session-transfers, session-summary, fetch-info |
| `organizations` | list (read-only) |

---

## Search Fields

The `--search` flag filters on different fields per resource:

| Resource | Searches on |
|----------|-------------|
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
| custom messages | name |
| language groups | name |
| organizations | display_name |

---

## Troubleshooting

**Auth errors (401)**
Run `syllable users me` to verify your API key is working. Re-run `syllable-setup` to update it.

**Permission errors (403)**
Run `syllable permissions list` to see what your key can access.

**Resource not found (404)**
Use the `list` subcommand to find valid IDs: `syllable agents list --search "name"`.

**Validation errors (422)**
The error message names the specific field(s) that failed. Use `syllable schema list` and `syllable schema get <TypeName>` to see what the request body should look like.

**Unknown fields or unexpected output**
Add `--output json` to see the full raw API response. Add `--debug` to see the full HTTP exchange.

**Rebuilding after code changes**
```bash
cd scripts/syllable-cli
go build -o syllable .
go test ./...
```
