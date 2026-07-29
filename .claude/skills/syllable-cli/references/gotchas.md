# Syllable CLI: Known Gotchas

Practical notes on non-obvious CLI and platform behavior — exact payload shapes, field names, identifier formats, and the occasional read-back to confirm a setting applied. Each row pairs the behavior with what to do.

## Payload Format

| Topic | What to do |
|-------|-----------|
| `prompt_tool_defaults` `doc` format | Provide `doc` as a JSON array (`["name"]`), not a string — only the array form is applied. |
| Tool endpoint `method` is lowercase | Use `"get"`, `"post"`, `"put"`, `"delete"` (lowercase). |
| Agent create — include `tool_headers` | Send `"tool_headers": {}` in the agent create payload even when empty. |
| Data source body field | Data source create requires `name`, `text`, and `chunk` — the document body goes in `text` (not `content`). `chunk` is a required boolean placeholder (currently treated as `false` server-side regardless of value). Inline create takes `--name` + `--text`; use `--file` for full control. |
| Greeting `subject` vs `text` | `subject` is only accepted on `email_template` messages — including it on a `greeting` is rejected with a 422 (`subject is only allowed when type is email_template`). Put the greeting message in `text`; `preamble` holds an optional uninterruptible prefix (e.g., legal disclaimers). |
| Static parameter shape | Each static parameter is `{"name", "default", "type", "required"}`. If you have a `{"name", "value"}` shorthand, map `value` → `default` and add `"type": "string"`, `"required": false`. |
| Agent `variables` shape | Use a flat dictionary `{"key": "value"}`, not an array of objects. Reference each variable by the **same key everywhere** (greeting, prompt, tool parameter) — one key per value, no duplicates. The `vars.` prefix isn't needed for your own variables; it's reserved for system variables (`vars.session.*`) and outbound-campaign variables (auto-prefixed by the campaign system). See `references/variables-and-messages.md`. |
| Tool `definition.type` | Tool definitions include `"type": "endpoint"`. |

## CLI Flags & Identifiers

| Topic | What to do |
|-------|-----------|
| JSON input flag is `--file` | Use `--file /tmp/payload.json` (or `--file -` for stdin) on every create/update — there is no `--from-json`. |
| Destructive deletes need `--yes` non-interactively (v1.8) | `delete` prompts for a y/N confirmation; with no TTY (scripts, CI, **an agent driving the CLI**) pass `--yes`/`-y` or it makes no request. `--dry-run` previews without deleting. Related v1.8 conveniences: `--output` accepts only `table`/`json`; unknown `--fields` columns warn on stderr (output unchanged); a create/update under `--dry-run` lists any `missing_required_fields` from the embedded spec; `--debug` redacts credential fields (`auth_values`, tokens, secrets); config/auth errors print as JSON under `-o json`. |
| `prompts update` takes a positional ID | `syllable prompts update <id> --file /tmp/prompt.json --org <org>`. Without it: "accepts 1 arg(s), received 0". |
| get/update identifier types | `tools update`/`delete` use the tool **name** (string); `prompts get/update` use the numeric **ID**. Since v1.7 `tools get` accepts **either** (an all-digits arg is resolved ID → name via the list endpoint). Capture both name and ID from create responses — updates and deletes remain name-keyed. |
| Update body `id` vs positional (v1.7+) | Body-routed update commands (`agents`, `prompts`, `custom-messages`, `language-groups`, `users` by email, `tools` by name; and since **v1.8** `data-sources`, `services`, `voice-groups`, `roles`, `incidents`, which previously took no positional) inject the positional identifier into the body when it's missing, and hard-error before sending if the body carries a different one. On pre-1.7 CLIs the positional was ignored and the body's `id` decided the target — so always carry the right `id` in the body (the standard get → edit → put pipe does this for you). |
| Session ID vs Twilio call SID | `session_id`-keyed calls (`sessions get`, `GET /api/v1/sessions/full-summary/{session_id}`, `sdk.sessions.getById`) take the **numeric** Syllable session ID (e.g. `824`). A Twilio `CA<hex>` call SID is a different field — the session's `channel_manager_sid` — and returns **400** on these routes. Resolve a `CA…` SID to its session in one call: `sessions list --search-field channel_manager_sid --search "<full SID>" -o json`. (On older CLIs without `--search-field`, match the `channel_manager_sid` column on `sessions list -o json`.) |
| Filter sessions on boolean fields client-side | Server-side `--search-field` filtering is reliable for string fields (`agent_name`, `channel_manager_sid`, `prompt_name`, …). For boolean fields (`is_outbound`, `is_test`), fetch with `-o json` and filter with `jq`, e.g. `… -o json \| jq '.items[] \| select(.is_outbound == true)'`. To *include* test sessions in a listing, use the dedicated `--include-test` flag. (Tracked in [cli#99](https://github.com/asksyllable/syllable-cli/issues/99).) |

## Variable Rendering

| Topic | What to do |
|-------|-----------|
| Custom variables — one key, no `vars.` prefix | Reference a custom variable by the same bare key in greetings, prompts, and tool parameters — e.g. `{{ agent_name }}` everywhere. Don't add a `vars.` prefix or store a duplicate `vars.`-prefixed copy: per the docs, `{{ vars.agent_name }}` resolves to empty when the stored key is `agent_name`. The `vars.` prefix is only for system variables (`{{ vars.session.* }}`) and outbound-campaign variables (which the campaign system auto-prefixes). |
| Variable syntax options | Three substitution formats: `{{ variable }}` (renders blank if missing), `${variable=fallback}` (renders the fallback if missing), `{variable}` (keeps the literal text if missing). See https://docs.syllable.ai/resources/Variables. |
| Campaign batch variables don't reach greetings | Greetings read agent-level variables only — campaign batch variables (from CSV `request_variables`) aren't available there. Keep outbound greetings minimal and have the prompt speak recipient name/details. |
| Batch variables in the test API | In conversation-test sessions there's no campaign batch, so batch variables resolve to empty strings. Write outbound prompt conditionals defensively: "If the value is blank, empty, missing, or anything other than exactly 'true', treat it as false." |

## Creation Ordering

| Topic | What to do |
|-------|-----------|
| Tools before prompt | The `tools` array on prompt create references tool names that must already exist in the org — create tools first. |
| Attach tools in a follow-up update | `prompts create` doesn't attach the `tools` array — the new prompt starts with an empty tools list. Create the prompt, then `prompts update` to attach tools. |
| `prompt_tool_defaults` on agent create | If agent create doesn't accept `prompt_tool_defaults`, create the agent first, then update it to add them. |

## Validation

| Topic | What to do |
|-------|-----------|
| Prompt updates send the full object | `prompts update` is a full PUT — include `id`, `name`, `type`, and `llm_config` (and large fields like `context`) even for a one-field change. Fetch first (`prompts get <id> -o json`), edit the JSON, resubmit; a partial body returns 422. |
| Always set `edit_comments` on prompt create/update | Both `PromptCreateRequest` and `PromptUpdateRequest` accept `edit_comments` (string) — it's the label for that revision in version history and the quickest way to see what changed without diffing. **Always include a one-line `edit_comments`** (e.g., `"Add wheelchair-routing branch"`, `"Bump LLM to gpt-5.1"`, `"Tighten transfer guardrail"`). |
| Agent enum values | `stt_provider` expects display values like `"Deepgram Nova 3"` (not `"deepgram"`); `wait_sound` expects `"Call Center"` (not `"office"`). Check with `syllable schema get AgentSttProvider` / `syllable schema get AgentWaitSound`. |
| Prompt text with backticks in shell | Generate prompt JSON with Python `json.dumps()`, not a shell heredoc or `echo` — zsh treats backtick characters as command substitution and strips them from the output. |
| Service `auth_type` for public APIs | Set to `null` (not omitted) for APIs with no authentication. |
| Session labels are immutable | Once applied, a session label can't be changed or removed via the API — apply carefully. |
| Day-of-week abbreviations differ by context | Message rules use two-letter codes (`mo`, `tu`, `we`, `th`, `fr`, `sa`, `su` — `DayOfWeek`); campaign `active_days` use three-letter codes (`mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun` — `DaysOfWeek`). Match the format to the context. |

## Campaign Behavior

| Topic | What to do |
|-------|-----------|
| Batch dedupes by target number | An outbound batch dedupes by `target` phone number — duplicate targets in one batch dial only the first and mark the rest `DUPLICATE`. To call the same number more than once, use sequential single-record batches. |
| Set `caller_id` (and `source`) on campaign create | Although `OutboundCampaignInput` types `caller_id` as optional (`anyOf [string, null]`), campaign create needs it: set both `caller_id` and `source` to the source phone number, and have a voice channel target binding that number to the campaign's agent already in place. The server resolves `agent_id` from `caller_id` → channel target, so that binding must exist first. (Without it: 400 "No agent with assign number None found".) |
| Channel target create takes channel-id as a positional | `syllable channels targets create <channel-id> --file payload.json` — the channel ID is a positional argument (the body also has a `channel_id` field). Without it: "accepts 1 arg(s), received 0". |

## Conversation Test API (`send-test-message`)

| Topic | What to do |
|-------|-----------|
| `--override-timestamp` expects a timezone-naive value | Pass `YYYY-MM-DDTHH:MM:SS` with no offset, no `Z`/UTC suffix, and no space separator (e.g. `2030-12-25T09:30:00`, read in the agent's timezone) — only this form is applied; other forms fall back to the real wall clock. CLI **v1.6.1** corrected the `--help` example to the naive form; **v1.7** adds a stderr warning on the known-ignored forms (trailing `Z`, `±HH:MM` offset, space separator), which also fires under `--dry-run`. Confirm it applied by checking the session transcript `actions[].tool_result` for `get_current_datetime` (or whether a time-based greeting changed), rather than trusting a green run. When applied, the override drives both `get_current_datetime` and time-based message/greeting selection. ([cli#76](https://github.com/asksyllable/syllable-cli/issues/76), [cli#78](https://github.com/asksyllable/syllable-cli/issues/78); a server-side strictness fix is pending.) |

## Bridge Phrases — two separate surfaces

There are **two** commands for bridge phrases. They share vocabulary, overlap in
effect, and have **separate storage**.

| Surface | What it is |
|---------|-----------|
| `bridge-phrases` (v2.1) | **Named, reusable** configs with full CRUD. Each has a default phrase set (`config.phrases.messages`), optional per-language variants (`config.phrases.localized`), and optional per-tool overrides (`config.tools[]`). Attached to an agent by the agent's `bridge_phrases_id`. At most one non-deleted config per suborg may be `is_default`. |
| `conversation-config bridges` | A **single** config read/written in place, scoped to the org (no flags), an agent (`--agent-id`), or a tool (`--tool-name`). Uses the legacy field shape (`first_slow_messages`, `very_slow_messages`, `tool_responses`) plus unified `messages`. **Slated for deprecation.** |

**Before any bridge-phrase write, ask the user which surface they mean — do not
guess, and do not default to one.** A write to the wrong one returns 200 and
changes nothing the agent actually uses, so there is no error to catch it.

**If the user doesn't know**, ask them to check the Console: *does the agent's
config page let them set a bridge phrase config per agent?* **Yes** → use
`bridge-phrases`. **No** → use `conversation-config bridges-update`.

Reads need no confirmation — reading both is usually the fastest way to find
where an org's phrases actually live. See SKILL.md § *Bridge Phrases — Confirm
Which Config Surface First*.

| Topic | What to do |
|-------|-----------|
| Writing the wrong surface fails silently | Covered above: confirm first. Symptom is a clean 200 with no behavior change in the agent — check whether the org's phrases live in `bridge-phrases list` or `conversation-config bridges` before concluding a field didn't persist. |
| `bridge-phrases update` is a full replacement | It is a PUT on the collection keyed by the body `id`, and replaces the fields you send — fetch first (`get <id> -o json`), modify, then push, rather than sending a partial body. Omitting `is_default` (or sending `null`) preserves the current flag. A positional id that conflicts with the body `id` is an error, not a silent override. |
| `bridge-phrases create` inline flags leave phrases empty | `--name`/`--description`/`--default` create a config with an **empty** phrase set. Use `--file` to set the phrases themselves (`syllable schema get BridgePhrasesCreateRequest`). |
| `bridge-phrases get` summarizes the nested config | The table view shows a phrase preview, language tags, and tool names — not the full lists. Use `--output json` when you need the actual phrases. |
| Persistence of the new `bridge-phrases` fields is only *partly* live-verified | Verified 2026-07-29 on the CI org (`TestBridgePhrasesCRUD`, PR #167): `config.phrases.messages` and `config.phrases.localized` **do** round-trip — a full create → update → read-back cycle returned both the default phrase list and an `es-US` override intact. **Still unverified:** `smart_turn_timeout_seconds` and `randomize_bridge_phrases` are sent by that test but not asserted on read-back, and `config.tools[]` is sent empty, so per-tool overrides are untested end to end. Read back after writing any of those three and file a `prod-drift` issue if one is silently dropped on a 200 — that is exactly what happened to the `conversation-config` equivalents (see the rows below and [cli#100](https://github.com/asksyllable/syllable-cli/issues/100)). |
| Unified `messages` / `randomize_messages` not yet active in prod | The v1.7.1 spec adds `messages` + `randomize_messages` to the bridges body and `bridges-update` sends them, but production doesn't persist them yet — until it does, only the legacy lists (`first_slow_messages`, `very_slow_messages`, `tool_responses`) take effect. Read back after a `bridges-update` to confirm what applied. |
| `smart_turn_timeout_seconds` not yet active in prod | Same for the top-level `smart_turn_timeout_seconds` (number `0.25`–`30`, or `null`): `bridges-update` passes it through via `--file`, but production doesn't persist it yet. Read back to confirm. (Tracked alongside [cli#100](https://github.com/asksyllable/syllable-cli/issues/100).) |

## Channels (`channels targets`)

| Topic | What to do |
|-------|-----------|
| `target_mode_list` search field not yet in prod | The v1.7.3 spec ([cli#106](https://github.com/asksyllable/syllable-cli/issues/106)) lists `target_mode_list` in the channel-target `search_fields` vocabulary — a filter meant to match a comma-separated list of modes (`channels targets list --search-field target_mode_list --search "voice,sms"`) — but production's `search_fields` enum doesn't accept it yet (422). Until it does, filter client-side: `channels targets list -o json \| jq '.items[] \| select(.target_mode == "voice" or .target_mode == "sms")'`. The singular `target_mode` and the default `target` search fields work normally. (Tracked in [cli#109](https://github.com/asksyllable/syllable-cli/issues/109).) The companion `order_by` change in the same sync has no CLI surface — `channels targets list` exposes no `--order-by` flag. |

## Deprecations

| Topic | What to do |
|-------|-----------|
| Language Groups renamed to Voice Groups | The `language-groups` commands still work, but the platform renamed them to Voice Groups — use `voice-groups` for new work. The Console shows "Voices," not "Language Groups." |
| Agent/campaign `label` (singular) → `labels` (plural) | Use `labels` (array of strings); the singular `label` field is deprecated and an old payload using it may not apply the value. The same applies to campaigns. |

## Known Gaps

| Topic | What to do |
|-------|-----------|
| Step tools — Console/SDK only | The schema defines `Step`, `StepTools`, `StepEventActions`, and `NextStep`, but the CLI has no `steps` or `step-tools` commands yet — manage step tools through the Console or SDK. |

## Users

| Topic | What to do |
|-------|-----------|
| `users create` for an email that already exists platform-wide | If the email already has a Syllable identity (the user exists in another org, or has a Google Workspace account), include `"skip_auth": true` **and** an explicit `"login_type"` in the create body — this matches what the Console sends (`skipAuth: true`). Without them the server tries to provision a fresh auth identity for an already-known email and returns a 500. See `payload-examples.md` § *User Create — Add to Org*. |
| Inline-create integer flags send as integers (v1.8) | `--role-id` (`users create`), `--prompt-id` (`agents create`), `--service-id` (`tools create`), and `--campaign-id` (`outbound batches create`) parse and send as JSON integers, with a clear error on a non-numeric value. On pre-1.8 CLIs they serialized as strings — use `--file` with the unquoted integer (`"role_id": 19`) instead. |
| `users get` is keyed by email | `users get <email>` takes the **email**, not the numeric ID — an integer returns "<id> is not a valid email address". |
| `users me` needs your email configured | The API has no key→identity endpoint, so `users me` does an exact lookup of the email set via `syllable setup`. Since v1.7 it fails clearly without one (`Error: no email configured — run \`syllable setup\` …`, exit 1). Pre-1.7 it returned the org's first user, so don't rely on older versions to confirm which account a key belongs to. |
| Choosing `login_type` | Use `"google"` for Google Workspace domains and `"username_and_password"` for everything else. The schema defaults to `"google"` only for `@gmail.com`, so set it explicitly for Workspace SSO orgs on other domains. |

## LLM Config

| Topic | What to do |
|-------|-----------|
| Use `syllable prompts supported-llms` for valid combos | `supported-llms` returns the current provider/model/version/api_version combinations. Prefer it over `syllable schema get PromptLlmProvider`, which may not list every supported provider (e.g. `anthropic`). |
| LLM config needs exact `version` and `api_version` | `provider` + `model` alone returns 400 — copy `version` and `api_version` verbatim from `supported-llms`. Examples: voice — `{ "provider": "openai", "model": "gpt-4.1", "version": "2025-04-14", "api_version": null, "temperature": 0.0 }`; chat — `{ "provider": "openai", "model": "gpt-5.1", "version": "2025-11-13", "api_version": null, "temperature": null }`. (gpt-4o is being discontinued — don't default to it.) |
| GPT-5 family uses `temperature: null` | gpt-5, gpt-5-mini, gpt-5.1, and gpt-5.2 don't take a temperature parameter — set `temperature: null`. (With `temperature: 0.0` the greeting fires but a later user turn returns 400.) |
