---
name: syllable-cli
description: |
  **Syllable CLI**: Go CLI for the Syllable platform — full CRUD on every resource plus API schema exploration. Load on ANY Syllable resource operation so its references (gotchas, common-patterns, payload-examples, telephony, variables, sessions, insights, outbound) load before issuing commands.
  - MANDATORY TRIGGERS: syllable cli, syllable status, syllable setup, syllable schema, syllable api, list agents, get agent, list prompts, get prompt, list sessions, fetch transcript, list channels, create agent, update agent, copy agent, clone agent, copy prompt, copy message, clone greeting, copy tool, copy channel, copy campaign, clone campaign, bulk update, across all agents, in all orgs, change greeting, change message, update open hours, update holiday hours, update transfer number, add directory member, transfer contact, configure api key, add new org, what fields does, AgentCreate, PromptCreate, explore api schema
---

# Syllable CLI

A Go CLI for managing the Syllable AI platform. Source and releases: **https://github.com/asksyllable/syllable-cli**

> **Docs synced to CLI v1.8.0** (pending release) — bump this marker whenever these docs are updated for a new release.
>
> v1.8.0 highlights for CLI users: destructive `delete` commands now confirm interactively and need `--yes` when non-interactive; `channels delete` was removed (no such API op — use `channels targets delete`); `--output` is validated; unknown `--fields` columns warn; `--dry-run` lists any `missing_required_fields`; `--debug` redacts credentials; `data-sources create` inline now takes `--text` (the document-body field) in place of the old, non-functional `--content`.

## Installation

**Homebrew (recommended for macOS/Linux):**
```bash
brew tap asksyllable/syllable-cli https://github.com/asksyllable/syllable-cli
brew install --cask syllable
```

**Install script:**
```bash
curl -fsSL https://github.com/asksyllable/syllable-cli/releases/latest/download/install.sh | sh
```

## Setup — First Time and Any Config Changes

Any time the user needs to configure the CLI — whether it's the first time, adding a new org, adding a new environment, or updating an API key — run:

```bash
syllable setup
```

This opens a browser UI where the user can safely manage all orgs, API keys, and environments. Always use this command for config changes — never edit `~/.syllable/config.yaml` directly or ask the user to paste API keys into the terminal.

**Trigger `syllable setup` whenever the user says things like:**
- "I need to add a new org"
- "Add a new environment"
- "Update my API key"
- "Set up credentials for [org]"
- "I'm getting auth errors" (may need a key update)
- First use — if `~/.syllable/config.yaml` doesn't exist or has no orgs configured

**Check if setup is needed:**
```bash
test -f ~/.syllable/config.yaml && echo "configured" || echo "not configured"
```

**IMPORTANT — Never read or display config files.** The config at `~/.syllable/config.yaml` contains API keys. Never `cat`, `read`, `head`, or otherwise output its contents. Use `syllable setup` for any config changes and the `test -f` check above to verify it exists.

## Org & Environment Confirmation

**Before executing any command that reads or modifies platform resources, always confirm the target org and environment with the user.** Run `syllable status` first to see all configured orgs and environments:

```bash
syllable status
```

This shows each configured org, its environment(s), and whether an API key is present. Use it to:
- **Resolve vague user references** — e.g., "acme" → find the matching configured org name like `acme-health` or `acme`
- **Discover org naming conventions** — see the exact short names configured
- **Confirm the target before executing** — especially for create, update, or delete operations
- **Verify environment** — ensure you're targeting prod vs. staging vs. dev

**Always tell the user which org and environment you're about to run against and get confirmation before proceeding**, particularly for write operations.

## Quick Start

```bash
syllable --help
```

Select an org per command:
```bash
syllable --org acme agents list
syllable --org globex sessions list
```

If `default_org` is set in `~/.syllable/config.yaml`, it's used when `--org` is omitted.

## Global Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--org` | -- | `default_org` in config | Org name to look up API key (e.g. `acme`, `globex`) |
| `--base-url` | -- | `https://api.syllable.cloud` | API base URL (overrides `--env`) |
| `--env` | -- | -- | Named environment (e.g. `prod`, `staging`, `dev`) — sets base URL from config |
| `--output` / `-o` | -- | `table` | Output format: `table` or `json` |
| `--fields` | -- | -- | Comma-separated columns to show in table output (e.g. `id,name,type`) |
| `--config` | -- | `~/.syllable/config.yaml` | Config file path |
| `--dry-run` | -- | `false` | Preview API requests without executing them |
| `--debug` | -- | `false` | Print HTTP request/response details to stderr |

## Common List Flags

All list commands support these flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--page` | `0` | Page number (0-based) |
| `--limit` | `25` | Max items to return |
| `--search` | — | Search filter (field varies by resource — see table below) |
| `--search-field` | per-resource default below | Override which field `--search` filters on (free-form, passed to the API as `search_fields`) |

For sessions, the valid `--search-field` values are the `SessionProperties` enum — `syllable schema get SessionProperties` lists them (`agent_name`, `source`, `target`, `channel_manager_sid`, `prompt_name`, `is_outbound`, …). **String fields filter server-side; filter boolean fields (`is_test`, `is_outbound`) client-side with `jq`** — see `references/gotchas.md`.

**Default search field by resource:**

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

## Resources & Verbs (summary)

Every command takes the form `syllable [--org X] [--env Y] <resource> <verb> [args]`.

**Resources:** `agents`, `prompts`, `tools`, `services`, `channels` (+ `channels targets`, `channels twilio`, `channels available-targets`), `sessions`, `conversations`, `outbound batches`, `outbound campaigns`, `users`, `directory`, `insights workflows`, `insights folders`, `insights tool-configs`, `insights tool-definitions`, `custom-messages`, `language-groups` (deprecated → use `voice-groups`), `voice-groups`, `data-sources`, `roles`, `incidents`, `pronunciations`, `session-labels`, `session-debug`, `takeouts`, `events`, `permissions`, `conversation-config`, `dashboards`, `organizations` (current-org get/update + sip-ip-ranges; no create/delete), `schema`, `status`.

**Standard verbs** (most resources):
```bash
syllable <resource> list [--page N] [--limit N] [--search TEXT]
syllable <resource> get <id>
syllable <resource> create --file body.json     # or inline flags (see references/commands.md)
syllable <resource> update <id> --file body.json # PUT, full replacement — see warning below
syllable <resource> delete <id> --yes            # destructive — see note below
```

> **Destructive commands confirm first.** `delete` prompts for a y/N confirmation.
> Running non-interactively (scripts, CI, **an agent driving the CLI**) you **must** pass
> `--yes`/`-y` or the command refuses with an error and does nothing. `--dry-run` previews
> the request without deleting. (`users delete-account` and the pronunciation-CSV commands
> keep their own `--confirm` flag.)

**Resource-specific verbs worth knowing inline:**

| Resource | Verbs |
|---|---|
| `prompts` | `history <id>`, `supported-llms` |
| `tools` | `history <tool-id>` (v1.7, version history); `get` accepts name **or** numeric ID (v1.7 — IDs resolved via the list endpoint) |
| `sessions` | `transcript <id>`, `summary <id>`, `latency <id>`, `recording <id>`, `recording-stream --token <token>` |
| `outbound batches` | `results <id>`, `requests <id> --file …`, `remove-requests <id> --file …` |
| `channels` | `get <id>` (v1.7, resolved client-side via list), `targets list/get/create/update/delete`, `available-targets`, `twilio get/create/update/numbers-list/numbers-add/numbers-update/numbers-verify-a2p-compliance` (last is v1.7). **No top-level `channels delete`** — the API has no delete-channel op; remove a target with `channels targets delete <channel-id> <target-id>` (v1.8) |
| `agents` | `send-test-message <id> --test-id <id> [--session-start] --text "<msg>"` (conversation test API), `labels` (v1.7, active agent labels), `voices` (available voices) |
| `insights workflows` | `activate <id>`, `inactivate <id>` |
| `insights folders` | `files <id>` |
| `users` | `me` (requires your email in config — fails loudly otherwise, v1.7), `send-email <email>` |
| `directory` | `history <id>`, `restore <id>`, `test <id> [--timestamp …]`, `download`, `upload --file …` |
| `organizations` | `get` (current org incl. Console SLUG), `update`, `sip-ip-ranges list/create/update/delete` (v1.7) — no org create/delete (intentionally not exposed) |
| `pronunciations` | `list`, `get-csv`, `metadata` |
| `session-debug` | `by-session-id <id>`, `by-sid <id> <sid>`, `tool-result <id> <tool-result-id>` |
| `takeouts` | `create --file …`, `get <id>`, `download <id> <file>` |
| `incidents` | `organizations` |
| `conversation-config` | `bridges`, `bridges-update --file …` |
| `dashboards` | `list`, `sessions`, `session-events`, `session-transfers`, `session-summary`, `fetch-info --name <name>` |
| `schema` | `list [--filter X]`, `get <SchemaName>` (local OpenAPI lookup, no API call; `-o json` is pure JSON since v1.7 — pipe straight to `jq`) |
| `status` | no verb — prints all configured orgs/envs (read-only, local config) |

**`<id>` for sessions is the numeric Syllable session ID** (e.g., `824`) — *not* the Twilio `CA<hex>` call SID. Any `session_id`-keyed call (`sessions get/transcript/summary`, `GET /sessions/full-summary/{session_id}`, `sdk.sessions.getById`) returns **400** on a `CA…` SID; that value is the session's `channel_manager_sid`. To resolve one from the other: `sessions list --search-field channel_manager_sid --search "CA…" -o json`. See [`references/sessions-and-debugging.md`](references/sessions-and-debugging.md).

For exact flag syntax, table-column listings, get-fields, inline-create flag sets, and resource-specific notes (e.g. `users get`/`update`/`delete`/`send-email` are keyed by **email address**, not a numeric ID), see [`references/commands.md`](references/commands.md).

For send-test-message conversation flow details (greeting / follow-up / silence / DTMF), see `references/commands.md` § *Conversation Test API*.

## Auth

All requests use the header:
```
Syllable-API-Key: <api-key>
```

Auth is skipped for `help` and `completion` commands. HTTP client timeout is 30 seconds.

**Headless / CI auth (v1.7+):** the `SYLLABLE_API_KEY` environment variable, when set, **takes priority over** `~/.syllable/config.yaml` — the CLI runs with no `syllable setup` at all (CI pipelines, scheduled routines, scripts). For interactive use keep using `syllable setup`; never echo, log, or hardcode a key when wiring the env var (inject it from a secret store).

## Dual Input Pattern

Most create commands support two modes:

1. **File mode:** `--file path/to/body.json` — reads a full JSON body from disk
2. **Inline flags:** Individual flags like `--name`, `--type`, etc. — builds a minimal body with default empty maps/arrays for nested fields
3. **Stdin:** `--file -` — reads JSON from stdin (useful for piping)

Update commands require `--file` (no inline flags).

## API Patterns

- **Endpoints:** `/api/v1/{resource}/?page=...&limit=...`
- **Search:** `&search_fields=<field>&search_field_values=<value>`
- **Date filters:** `&start_datetime=ISO8601&end_datetime=ISO8601`
- **Default page size:** 25 items
- **Table fallback:** If table format parsing fails, falls back to JSON output

## Update Commands Are Full Replacements (PUT, Not PATCH)

All `update` commands send a full PUT request — the server replaces the entire resource. **Any field omitted from the update body will be set to null or its default.** This means:

- Always start your update body from the full resource JSON (use `get <id> -o json` first)
- Include all required fields — especially large text fields like `context` on prompts, which have NOT NULL constraints and will cause a 500 error if omitted
- The pattern is: **get → edit → put**, not just patch the fields you want to change

**Positional ID is enforced (v1.7+).** On the body-routed update commands (`agents`, `prompts`, `custom-messages`, `language-groups` update by `id`; `users` by `email`; `tools` by `name`), the positional argument is now injected into the body when missing, and a body carrying a **different** identifier is a hard error before any request is sent (`positional argument "574" conflicts with id=575 in the request body`). Pre-1.7, the positional was silently ignored and the body's `id` decided what got updated — make the two match.

Example for prompts:
```bash
# Get the current state first
syllable --env staging prompts get 950 -o json > prompt-update.json

# Edit the file (change tools, llm_config, etc.)
# Then update with the full body
syllable --env staging prompts update 950 --file prompt-update.json
```

## Workflow Tips

- Use `--output json` to get raw API responses for scripting or piping to `jq`
- Use `--dry-run` to preview what API request would be sent before executing
- Use `--debug` to inspect HTTP request/response details when troubleshooting
- Create/update commands read a JSON body from `--file path/to/body.json` or `--file -` for stdin
- List commands default to 25 results per page; use `--limit` and `--page` to paginate
- **Bulk analysis with JSON list output:** `prompts list -o json` returns full prompt text (`context` field), `agent_count`, `llm_config`, `tools`, and all metadata for every prompt in a single call. This is far more efficient than calling `prompts get` on each prompt individually. The same applies to other resources — `agents list -o json`, `tools list -o json`, etc. all return complete objects. Use `--limit 100` (or higher) to fetch all resources in one request for audits, bulk analysis, or scripting.
- Source and releases: https://github.com/asksyllable/syllable-cli

## References

Detailed reference files (loaded on demand, not every session):

- `references/gotchas.md` — CLI/API gotchas: payload formats, validation rules, shell pitfalls, creation ordering, LLM config
- `references/common-patterns.md` — CLI recipes: data source wiring, cross-org cloning, tool reuse, variable config, batch testing
- `references/payload-examples.md` — JSON templates for tool, prompt, agent, and data source payloads
- `references/telephony-and-channels.md` — Telephony config fields, channel service enums, target modes, email/Twilio config, ReDi bridge phrases, campaign voicemail detection
- `references/variables-and-messages.md` — Variable syntax (3 formats with missing-value behaviors), system variables, message time-based rules, day-of-week enums
- `references/sessions-and-debugging.md` — Session fields, transcript structure, tool actions, latency analysis, session-debug commands, Console URL patterns
- `references/insights.md` — Insights system: folders, tool definitions, tool configs, workflows (lifecycle, conditions, activation), outputs
- `references/outbound-campaigns.md` — Campaign lifecycle, batch statuses, request statuses, results correlation, variable scoping
