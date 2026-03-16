# Syllable CLI

A command-line interface for managing the [Syllable AI platform](https://syllable.ai). Built in Go using [Cobra](https://github.com/spf13/cobra).

For deeper platform reference, see [docs.syllable.ai](https://docs.syllable.ai).

## Contents

- [Installation](#installation)
- [Configuration](#configuration)
- [Quick Start](#quick-start)
- [How It Works](#how-it-works)
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

## How It Works

Syllable is a platform for building custom AI voice and chat agents. Resources are layered — each depends on the ones below it:

```
Data Sources → Tools + Services → Prompts → Agents → Channels/Targets → Users
```

**Data Sources** are knowledge bases (text blobs) that agents can search during a conversation. They connect to agents indirectly — you create a tool that references the data source, add that tool to a prompt, and attach the prompt to an agent. Data sources do not connect to agents directly.

**Services** are centralized credential stores. They handle authentication (Basic, Bearer, or custom headers) for tools, so you configure credentials once and reuse them across many tools.

**Tools** are APIs that agents call during live sessions — to look up data, schedule appointments, transfer calls, and more. Every tool is backed by a service for auth. Three types exist: agent tools (called during user sessions), step tools (structured multi-step workflows), and system tools (pre-built platform tools like hangup, transfer, and web_search).

**Prompts** are natural language instruction sets that define how an agent behaves. They reference the LLM to use (e.g., `gpt-4o`, `gemini-2.0-flash`), the tools available, and the tone and rules the agent follows. Prompts are automatically versioned — every edit creates a new version, and you can roll back at any time.

**Agents** are the deployable AI systems that talk to users. Each agent is configured with a prompt, an optional opening message, a timezone, speech-to-text settings, and session variables for personalization. Each agent maps one-to-one to a channel target (a specific phone number or chat ID). A test channel is auto-generated for every agent so you can test before going live.

**Channels** are the communication modes (voice, SMS, chat). A target is the specific address within a channel — a phone number like `+18002832940` or a chat ID like `chat-1-org-3` — where an agent operates.

For a visual overview of the platform, see [docs.syllable.ai](https://docs.syllable.ai).

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

Agents are the AI systems that communicate with users. Each agent is configured with a prompt, a timezone, an optional opening message, session variables for personalization, and optional tool headers. An agent maps one-to-one to a channel target — one phone number or chat ID, one agent. A test channel is auto-generated for every agent for pre-production testing.

```bash
syllable agents list [--search TEXT]
syllable agents get <id>
syllable agents create --name NAME --type TYPE --prompt-id ID --timezone TZ
syllable agents create --file agent.json
syllable agents update <id> --file agent.json
syllable agents delete <id>
```

### Channels

Channels define how users reach agents — voice, SMS, or chat. A target is the specific phone number or chat ID where an agent is active. One agent per target — you cannot assign two agents to the same target. Twilio voice channels can be fully configured via the CLI; other channel types should be configured through the SDK or platform UI.

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

Conversations are grouped views of session activity. Use date filters to scope results; dates are ISO 8601 (`2024-01-01T00:00:00Z`).

```bash
syllable conversations list [--start-date DATE] [--end-date DATE] [--search TEXT]
```

### Prompts

Prompts are natural language instruction sets that define agent behavior, backed by an LLM (Azure OpenAI, Google, or OpenAI). Key fields include the provider, model (e.g., `gpt-4o`, `gpt-4.1`, `gemini-2.0-flash`), temperature (0 = deterministic, higher = more creative), seed (for reproducible outputs), and the list of tools the agent may call. Every edit to a prompt creates a new version automatically — use `prompts history` to view past versions and restore them if needed.

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

Tools are APIs that agents call during live sessions — for example, to look up records, schedule appointments, transfer calls, or search the web. Each tool is backed by a Service that handles authentication. Tools are referenced in prompts, which makes them available to agents. Three types exist: agent tools (called during user sessions), step tools (structured multi-step workflows), and system tools (pre-built platform tools ready to use, such as `hangup`, `transfer`, and `web_search`).

```bash
syllable tools list [--search TEXT]
syllable tools get <id>
syllable tools create --name NAME --service-id ID
syllable tools create --file tool.json
syllable tools update <id> --file tool.json
syllable tools delete <id>
```

### Sessions

Sessions are individual voice or chat conversations. Each session captures the transcript, an AI summary, all tool calls with their arguments and API responses, duration (for voice), channel, and test/live status. Use `transcript`, `summary`, `latency`, and `recording` subcommands to pull specific session data.

```bash
syllable sessions list [--start-date DATE] [--end-date DATE]
syllable sessions get <id>
syllable sessions transcript <id>
syllable sessions summary <id>
syllable sessions latency <id>
syllable sessions recording <id>
```

### Outbound

Outbound campaigns are planned calling efforts — they define the agent making calls, the caller ID shown to recipients, the call rate per hour, and operating hours. Batches are CSV contact lists attached to a campaign; the CSV must include `reference_id` (unique identifier) and `target` (E.164 phone number, e.g., `+14155551234`). Additional CSV columns are passed as variables to the agent prompt. A campaign can have multiple batches.

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

Users are platform accounts. `users me` returns the currently authenticated user — useful for verifying that an API key is working.

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

The directory is a member/contact list used for call routing and agent-initiated transfers. It stores names, numbers, and metadata that agents can look up at runtime.

```bash
syllable directory list [--search TEXT]
syllable directory get <id>
syllable directory create --name NAME --type TYPE
syllable directory update <id> --file member.json
syllable directory delete <id>
```

### Insights

Insights workflows automatically evaluate call sessions or recordings using LLMs. Three workflow types are available: Agent (analyzes the AI agent portion of a call), Transfer (includes post-transfer legs), and Folder (processes uploaded recordings). Workflows produce structured outputs (boolean, string, integer, array) that feed into dashboards.

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

Custom messages are opening greetings that an agent plays or sends at the start of a session. They are configured per-agent.

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

| Command | Subcommands | Notes |
|---------|------------|-------|
| `data-sources` | list, get, create, update, delete | Text knowledge bases for agents. Name must have no whitespace. Connect to agents via a tool, not directly. |
| `services` | list, get, create, update, delete | Credential store for tool auth (Basic, Bearer, or custom headers). Credentials are masked and not stored by Syllable after configuration. |
| `voice-groups` | list, get, create, update, delete | Voice configuration for agents. |
| `language-groups` | list, get, create, update, delete | **Deprecated** — use `voice-groups` instead. |
| `roles` | list, get, create, update, delete | User roles with associated permissions. |
| `incidents` | list, get, create, update, delete, organizations | Platform incident tracking. |
| `pronunciations` | list, get-csv, metadata | Custom TTS pronunciation dictionary. Downloadable as CSV. |
| `session-labels` | list, get, create | Manual annotations on sessions (rating, issue category, notes). **Once a label is added to a session, it cannot be updated or deleted.** |
| `session-debug` | by-session-id, by-sid, tool-result | Low-level session diagnostics. |
| `takeouts` | create, get, download | Data export jobs — create, poll with get, then download. |
| `events` | list | Platform event log. |
| `permissions` | list | System-wide permissions (read-only). |
| `conversation-config` | bridges, bridges-update | Configuration for transfer/handoff phrases. |
| `dashboards` | list, fetch-info, ~~sessions~~, ~~session-events~~, ~~session-transfers~~, ~~session-summary~~ | The `sessions`, `session-events`, `session-transfers`, and `session-summary` endpoints are **deprecated** — use `list` and `fetch-info` instead. |
| `organizations` | list (read-only) | Org listing — no create, update, or delete. |

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
