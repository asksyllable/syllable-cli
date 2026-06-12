# Syllable CLI: Known Gotchas

Consolidated list of CLI and API pitfalls encountered during agent builds. Each entry includes the fix or workaround.

## Payload Format

| Issue | Details |
|-------|---------|
| `prompt_tool_defaults` `doc` format | Must be a JSON array (`["name"]`), not a string. The API silently accepts strings but they don't work correctly. |
| Tool endpoint `method` must be lowercase | Use `"get"`, `"post"`, `"put"`, `"delete"` -- not uppercase. Uppercase values are rejected by the API. |
| Agent create requires `tool_headers` | Include `"tool_headers": {}` in the agent create payload even when empty. Omitting the field can cause validation errors. |
| Data source `text` field | Data source create uses `text` (not `content`) for the document body. Also requires `chunk: true` as a boolean field. |
| Greeting `subject` vs `text` field | The `subject` field on `CustomMessageCreateRequest` is only valid for `email_template` type greetings. Using `subject` on a regular greeting causes an API rejection. Use the `text` field for the greeting message body. The `preamble` field holds an optional uninterruptible prefix (e.g., legal disclaimers). |
| Static parameters API format | The API expects `{"name", "default", "type", "required"}` per static parameter. Some shorthand formats use `{"name", "value"}`. Translate: `value` -> `default`, add `"type": "string"` and `"required": false`. |
| Agent `variables` format | Use a flat dictionary `{"key": "value"}`, not an array of objects. The array format is silently rejected. Store each custom variable with both keys -- e.g., `{"agent_name": "Anna", "vars.agent_name": "Anna"}` -- so the value renders in both greetings (bare key) and prompts (`vars.` prefix). |
| `definition.type` required | Tool definitions must include `"type": "endpoint"`. Omitting this causes validation errors. |

## CLI Flags & Identifiers

| Issue | Details |
|-------|---------|
| CLI JSON input flag | Use `--file /tmp/payload.json` (not `--from-json`). The flag is `--file` for all create and update commands. |
| Destructive deletes need `--yes` non-interactively (v1.8) | `delete` prompts for a y/N confirmation; with no TTY (scripts, CI, **an agent driving the CLI**) it **refuses unless `--yes`/`-y` is passed** and makes no request. `--dry-run` previews without deleting. Related v1.8 flag changes: `--output` now rejects anything other than `table`/`json`; unknown `--fields` columns warn on stderr (output unchanged); a create/update under `--dry-run` lists any `missing_required_fields` from the embedded spec; `--debug` redacts credential fields (`auth_values`, tokens, secrets); config/auth errors print as JSON under `-o json`. |
| `prompts update` requires positional ID | `syllable prompts update <id> --file /tmp/prompt.json --org <org>`. Omitting the positional ID causes "accepts 1 arg(s), received 0". |
| CLI get/update identifier format | `tools update`/`delete` use the tool **name** (string); `prompts get/update` use the numeric **ID**. Since CLI v1.7, `tools get` accepts **either** (an all-digits arg is resolved ID → name via the list endpoint). Still capture both name and ID from create responses — updates and deletes remain name-keyed. |
| Update body `id` vs positional ID (v1.7) | Body-routed update commands (`agents`, `prompts`, `custom-messages`, `language-groups`, `users` by email, `tools` by name; and since **v1.8** `data-sources`, `services`, `voice-groups`, `roles`, `incidents`, which previously took no positional) now inject the positional identifier into the body when missing, and **hard-error before sending** if the body carries a different one. Pre-1.7 the positional was silently ignored and the body's `id` decided what got updated — on older CLI versions, always carry the right `id` in the body (the standard get → edit → put pipe does). |
| Session ID vs Twilio call SID | Routes/SDK calls keyed by `session_id` (e.g., `sessions get`, `GET /api/v1/sessions/full-summary/{session_id}`, `sdk.sessions.getById`) require the **numeric** Syllable session ID (e.g., `824`). Passing a Twilio-style `CA<hex>` call SID returns **400** — that value is the session's `channel_manager_sid`, a *different* field. To resolve a `CA…` SID to the session ID in one call: `sessions list --search-field channel_manager_sid --search "<full SID>" -o json` (verified live with a full SID — returned exactly the matching session). On older CLIs without `--search-field`, match the `channel_manager_sid` column on `sessions list -o json`. |
| Boolean session search fields don't filter | Verified live 2026-06-04 on CLI v1.7.1: `sessions list --search-field is_outbound --search <v>` returns **zero results** for every value form tried (`true`/`false`/`True`/`TRUE`/`1`/`t`/`outbound`) even though the org's sessions had `is_outbound: true` and the field is in the `SessionProperties` enum. `--search-field is_test --search true` doesn't restrict to test sessions either — it returned **more** rows than unfiltered (25 vs 18), behaving like `--include-test`; `false` matched the unfiltered default (18). String fields (`agent_name`, `channel_manager_sid`) filter correctly. **Fix:** fetch with `-o json` and filter client-side, e.g. `… -o json \| jq '.items[] \| select(.is_outbound == true)'`. To *include* test sessions use the dedicated `--include-test` flag. (Re-checked 2026-06-08 on CLI v1.7.3: `is_outbound` now **fails loud with 422** at query validation rather than silently returning zero, even though the v1.7.3 spec still lists `is_outbound` in `SessionProperties` — symptom changed, drift unchanged; client-side `jq` fix still required. Tracked in [cli#99](https://github.com/asksyllable/syllable-cli/issues/99).) |

## Variable Rendering

| Issue | Details |
|-------|---------|
| Greetings vs prompts | Greetings use bare keys: `{{ agent_name }}`. Prompts use the `vars.` prefix: `{{vars.agent_name}}`. To ensure a variable renders in both contexts, store it with both keys on the agent: `{"agent_name": "Anna", "vars.agent_name": "Anna"}`. |
| Variable syntax options | The platform supports three substitution formats: `{{ variable }}` (double-brace, renders blank if missing), `${variable=fallback}` (dollar-brace, renders fallback value if missing), `{variable}` (single-brace, preserves literal text if missing). See https://docs.syllable.ai/resources/Variables. |
| Greetings cannot use campaign batch variables | Campaign batch variables (from CSV `request_variables`) do NOT render in greetings -- greetings only read agent-level variables. Keep greetings minimal for outbound agents and have the prompt speak recipient name/details. |
| Outbound batch variables blank in test API | Campaign batch variables render as empty strings in conversation test API sessions because there is no actual campaign batch providing values. Write outbound prompt conditionals defensively: "If the value is blank, empty, missing, or anything other than exactly 'true', treat it as false." |

## Creation Ordering

| Issue | Details |
|-------|---------|
| Tools must exist before prompt | The `tools` array on prompt create expects tool names that already exist in the org. Create tools first. |
| Prompt create ignores tools array | The `tools` array on `prompts create` is silently ignored -- the created prompt will have an empty tools list. Always follow up with a `prompts update` call to attach tools. |
| `prompt_tool_defaults` on agent create | If the API rejects `prompt_tool_defaults` during agent create, create the agent first without them, then update the agent to add them. |

## Validation

| Issue | Details |
|-------|---------|
| Prompt updates require all fields | CLI `prompts update` requires `id`, `name`, `type`, and `llm_config` even for partial changes. Fetch the full prompt first (`prompts get <id> -o json`), modify the JSON, then resubmit. Sending only changed fields returns 422. |
| Always set `edit_comments` on prompt create/update | Both `PromptCreateRequest` and `PromptUpdateRequest` accept `edit_comments` (string). It surfaces in the prompt's version history as the label for that revision and is the only way to tell what changed without diffing. **Always include a one-line `edit_comments` describing the change** (e.g., `"Add wheelchair-routing branch"`, `"Bump LLM to gpt-5.1"`, `"Tighten transfer guardrail"`). Omitting it leaves the version unlabeled and forces future reviewers to diff to figure out intent. |
| Agent enum values | `stt_provider` expects values like `"Deepgram Nova 3"` (not `"deepgram"`). `wait_sound` expects `"Call Center"` (not `"office"`). Query schemas if unsure: `syllable schema get AgentSttProvider`, `syllable schema get AgentWaitSound`. |
| Prompt text with backticks in shell | When writing prompt JSON to `/tmp/`, use Python `json.dumps()` to generate the file, not shell heredoc or echo. Backtick characters in prompt text are interpreted as command substitution by zsh, silently stripping them from the output. |
| Service `auth_type` for public APIs | Set to `null` (not omitted) for APIs with no authentication. |
| Session labels are immutable | Session labels, once applied, cannot be removed or changed via the API. Apply labels carefully. |
| Day-of-week abbreviation mismatch | Message rules use two-letter abbreviations (`mo`, `tu`, `we`, `th`, `fr`, `sa`, `su` — `DayOfWeek` enum). Campaign `active_days` use three-letter abbreviations (`mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun` — `DaysOfWeek` enum). Using the wrong format silently fails. |

## Campaign Behavior

| Issue | Details |
|-------|---------|
| Batch deduplicates by target number | Outbound campaign batches deduplicate by `target` phone number -- if multiple batch records share the same target, only the first is dialed and the rest are marked `DUPLICATE`. Fix: use sequential batches (one record per batch) instead of a single batch with multiple records targeting the same number. |
| `caller_id` required despite schema marking it optional | `OutboundCampaignInput` lists `caller_id` as optional (`anyOf [string, null]`), but `syllable outbound campaigns create --file payload.json` rejects with `"No agent with assign number None found"` (status 400) when `caller_id` is omitted or set to a phone without a matching channel target. Both `caller_id` and `source` should be set to the same source phone number, and a voice channel target binding that phone to the campaign's agent must already exist on the org's Twilio channel. The CLI resolves `agent_id` server-side from `caller_id` → channel target lookup, even if you also pass `agent_id` in the body. |
| Channel target create takes channel-id as positional | `syllable channels targets create <channel-id> --file payload.json` -- the channel ID is a positional argument, not part of the JSON body (even though the body also includes a `channel_id` field). Omitting the positional fails with `accepts 1 arg(s), received 0`. |

## Conversation Test API (`send-test-message`)

| Issue | Details |
|-------|---------|
| `--override-timestamp` only honors **timezone-naive** ISO | `syllable agents send-test-message --override-timestamp` forwards `override_timestamp` to the conversation-test API, but the server applies it **only** when given timezone-naive `YYYY-MM-DDTHH:MM:SS` (e.g. `2030-12-25T09:30:00`, interpreted in the agent's timezone). Forms with a TZ offset (`...-06:00`), a `Z`/UTC suffix, or a space separator are **silently ignored** — the agent falls back to the real wall clock with **no error** (HTTP 200). Invalid strings also return 200, *not* a 422 (`TestMessage.override_timestamp` is typed `anyOf[string,null]`, so there is no format validation). (CLI **v1.6.1** corrected the `--help` example to the tz-naive form — earlier versions shipped the offset example, a silent no-op — [cli#76](https://github.com/asksyllable/syllable-cli/issues/76)/[cli#78](https://github.com/asksyllable/syllable-cli/issues/78). CLI **v1.7** adds a **stderr warning** on the three confirmed-ignored forms — trailing `Z`, `±HH:MM` offset, space separator — while still sending the value; the warning also fires under `--dry-run`, so preflight there. The server behavior is unchanged; a server-side strictness fix is pending.) **Fix:** pass naive `YYYY-MM-DDTHH:MM:SS`, and confirm it took by checking the session transcript `actions[].tool_result` for `get_current_datetime` (or whether a time-based greeting changed) rather than trusting a green run. When honored, the override drives both `get_current_datetime` and time-based message/greeting rule selection. |

## Bridge Phrases (`conversation-config`)

| Issue | Details |
|-------|---------|
| Unified `messages` accepted but not persisted (prod) | The v1.7.1 spec adds `messages` + `randomize_messages` to the bridges body, and `conversation-config bridges-update` sends them — but the **production API silently drops both** (verified live 2026-06-04: update with `messages` returned 200, immediate read-back omitted the fields). Until the backend deploys support, only the legacy lists (`first_slow_messages`, `very_slow_messages`, `tool_responses`) persist. **Always read back after a bridges-update** to confirm what actually stuck. |
| `smart_turn_timeout_seconds` accepted but not persisted (prod) | The v1.7.2 spec adds top-level `smart_turn_timeout_seconds` (number `0.25`–`30`, or `null`) to the bridges body, and `bridges-update` passes it through via `--file`. **Verified live 2026-06-08 (CLI v1.7.3): same drift as the sibling fields above** — a `bridges-update` carrying `smart_turn_timeout_seconds` returned 200 but both the response body and the immediate read-back omitted it (sent in the same PUT as `messages`/`randomize_messages`, all three dropped). Until the backend deploys support it does not persist; **read back to confirm**. Tracked alongside [cli#100](https://github.com/asksyllable/syllable-cli/issues/100). |

## Channels (`channels targets`)

| Issue | Details |
|-------|---------|
| `--search-field target_mode_list` rejected by prod (422) | The v1.7.3 spec sync ([cli#106](https://github.com/asksyllable/syllable-cli/issues/106)) added `target_mode_list` to the channel-target `search_fields` vocabulary — a filter-only field meant to match a comma-separated list of modes, used as `channels targets list --search-field target_mode_list --search "voice,sms"`. **Prod rejects it with 422** ("Input should be 'id', 'channel_id', … 'a2p_verified'") — prod's `search_fields` enum has the other 10 members but not `target_mode_list`. Verified live 2026-06-08 (CLI v1.7.3); a 422 enum error is raised before data evaluation, and the singular `target_mode` / default `target` search fields return 200 on the same org, so the search mechanism works — only this field is rejected. **Fix until the backend deploys it:** filter client-side, e.g. `channels targets list -o json \| jq '.items[] \| select(.target_mode == "voice" or .target_mode == "sms")'`. Tracked in [cli#109](https://github.com/asksyllable/syllable-cli/issues/109). (The companion `order_by` change in the same sync — new `ChannelTargetOrderProperties` enum — has **no CLI surface**: `channels targets list` exposes no `--order-by` flag, and the accepted sort fields are unchanged.) |

## Deprecations

| Issue | Details |
|-------|---------|
| Language Groups renamed to Voice Groups | The `language-groups` commands still work but the platform renamed them to Voice Groups. Use `voice-groups` commands for new work. The Console shows "Voices" not "Language Groups." |
| Agent `label` (singular) deprecated → `labels` (plural) | The agent field `label` (singular string) is deprecated. Use `labels` (array of strings). Old payloads with `label` may silently ignore the value. The same applies to campaigns — use `labels` (array) instead of `label` (string). |

## Known Gaps

| Issue | Details |
|-------|---------|
| Step tools — schema only, no CLI commands | The API schema defines `Step`, `StepTools`, `StepEventActions`, and `NextStep` types, but the CLI has no `steps` or `step-tools` commands. Manage step tools through the Console or SDK until CLI support is added. |

## Users

| Issue | Details |
|-------|---------|
| `users create` 500s for existing-globally emails | If the email already has an identity in Syllable (e.g., the user exists in another org, or has a Google Workspace account), `POST /api/v1/users/` returns an opaque `500 "An error occurred while creating the user"` unless the payload includes `"skip_auth": true` AND an explicit `"login_type"`. The OpenAPI schema lists `skip_auth` as defaulting to `false`, but the Console always sends `skipAuth: true`. Without it, the server tries to provision a fresh auth identity for an already-known email and crashes internally. See `payload-examples.md` § *User Create — Add to Org*. |
| Inline-create integer flags now send as integers (v1.8) | `--role-id` (`users create`), `--prompt-id` (`agents create`), `--service-id` (`tools create`), and `--campaign-id` (`outbound batches create`) now parse and send as JSON integers, with a clear error on a non-numeric value. Pre-1.8 they serialized as strings (e.g. `"19"`) and relied on the server coercing them — on older CLIs use `--file` with the unquoted integer (`"role_id": 19`) instead. |
| `users get <id>` not supported | The `users get` command takes the **email** as the positional argument, not the numeric ID. Passing an integer returns `"<id> is not a valid email address"`. |
| `users me` needs your email configured | The API has no key→identity endpoint, so `users me` can only do an exact lookup of the email set via `syllable setup`. Since CLI v1.7 it **fails loudly** without one (`Error: no email configured — run \`syllable setup\` …`, exit 1). Pre-1.7 it silently returned the **first user in the org** — never trust its output on older versions to confirm which account a key belongs to. |
| Choosing `login_type` | Use `"google"` for Google Workspace domains. Use `"username_and_password"` for everything else. The schema default is `"google"` only for `@gmail.com` — every other domain defaults to username/password, which is wrong for Workspace SSO orgs. |

## LLM Config

| Issue | Details |
|-------|---------|
| Always use `syllable prompts supported-llms` | The `supported-llms` command returns all valid provider/model/version/api_version combos. `syllable schema get PromptLlmProvider` is stale and does not reflect all supported providers (e.g., it omits `anthropic`). |
| LLM config requires exact `version` and `api_version` fields | Sending only `provider` + `model` will fail with HTTP 400. Copy the exact values from `supported-llms`. Examples: voice agents — `{ "provider": "openai", "model": "gpt-4.1", "version": "2025-04-14", "api_version": null, "temperature": 0.0 }`; chat agents — `{ "provider": "openai", "model": "gpt-5.1", "version": "2025-11-13", "api_version": null, "temperature": null }`. (gpt-4o is being discontinued — do not use as a default.) |
| GPT-5 family requires `temperature: null` | The models gpt-5, gpt-5-mini, gpt-5.1, and gpt-5.2 do not support the temperature parameter. Setting `temperature: 0.0` lets the prompt update succeed and the agent greeting fire, but the second user message returns HTTP 400. Set `temperature: null` for these models. |
