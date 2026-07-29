# Telephony & Channel Configuration Reference

Field-level reference for channels, telephony config, targets, and related features. Sources: `syllable schema get` and [docs.syllable.ai/resources/Channels](https://docs.syllable.ai/resources/Channels).

## Channel Services

The `channel_service` field determines the communication provider. Enum values:

| Value | Description |
|-------|-------------|
| `sip` | Custom SIP trunking (forward calls via SIP Invite to Syllable's SIP servers) |
| `twilio` | Twilio voice (your own account or Syllable's Twilio) |
| `email` | Asynchronous email via SendGrid |
| `webchat` | Embedded web chat widget |
| `africastalking` | Africa's Talking voice/SMS provider |
| `whatsapp` | WhatsApp Business messaging |

System channels (built-in, not customizable): `Freeswitch Twilio` (voice), `Syllable Webchat` (chat).

## Target Modes

The `target_mode` field on channel targets determines the communication method. Enum values:

| Value | Description |
|-------|-------------|
| `voice` | Phone call |
| `chat` | Webchat |
| `sms` | Text message |
| `email` | Email |
| `whatsapp` | WhatsApp message |

A target's mode must match one of the parent channel's `supported_modes`.

> **Filtering targets by mode (v1.7.3, not yet usable on prod):** the v1.7.3 spec added a `target_mode_list` filter field to `channels targets list` (`--search-field target_mode_list --search "voice,sms"`), intended to match a comma-separated list of modes. **Prod currently rejects it with 422** — until the backend deploys it, filter client-side from `-o json`. See `gotchas.md` § Channels and cli#109.

## Channel Config

Channel configuration lives under `config` and contains two optional sub-objects:

```json
{
  "config": {
    "telephony": { ... },
    "email": { ... }
  }
}
```

- `telephony` — applies to voice-capable channels only (see below)
- `email` — applies to email channels only (see below)

WhatsApp is a valid `channel_service` and `target_mode` but has no documented config schema.

## Telephony Configuration Fields

All fields are nullable. When `null`, the platform default applies. Set on the channel and inherited by all targets belonging to that channel.

| API Field | Type | Range | Console Label | Description |
|-----------|------|-------|---------------|-------------|
| `async_enabled` | boolean | — | Responsive Dialogue | Whether asynchronous (ReDi) mode is enabled for the conversation |
| `interruptibility` | string | `none`, `dtmf_only`, `speech_only`, `all` | Interruptibility | Controls whether and how the user can interrupt agent speech |
| `passive_speech_input_enabled` | boolean | — | Passive Speech Input | Enables input capture while the agent is speaking |
| `passive_input_start` | number | 0–5 sec | Passive Input Start | Delay (seconds) before passive input activates after agent speech starts |
| `transfer_leg_voicemail_detection_enabled` | boolean | — | Transfer Leg Voicemail Detection | Whether voicemail detection is enabled on the transfer leg |
| `pre_input_timeout` | number | 0–10 sec | Pre-input Timeout | Silence threshold (seconds) after agent stops speaking before the system stops waiting for input |
| `post_speech_input_timeout` | number | 0–10 sec | Post-speech Input Timeout | Silence duration (seconds) after user speech to determine input has ended |
| `post_dtmf_input_timeout` | number | 0–10 sec | Post-DTMF Input Timeout | Silence duration (seconds) after DTMF input to determine input has ended |
| `overall_input_timeout` | number | 0–300 sec | Overall Input Timeout | Maximum total wait time (seconds) for any input |
| `output_padding` | number | -5 to 5 sec | Output Padding | Seconds to start listening for user input before agent speech ends (negative = start early) |

**Safety rule:** Do not infer telephony settings from a channel's name (e.g., a channel named "ReDi Enabled" does not prove `async_enabled` is true). Always check the actual `config.telephony` field values.

## Email Configuration

Applies to `email` service channels only.

| API Field | Type | Required | Description |
|-----------|------|----------|-------------|
| `sending_domain` | string | yes | Domain for sending emails (must be authenticated in SendGrid) |

Example:
```json
{
  "config": {
    "email": {
      "sending_domain": "mail.example.com"
    }
  }
}
```

## Twilio Channel Configuration

When creating a Twilio channel, provide credentials:

| API Field | Type | Required | Description |
|-----------|------|----------|-------------|
| `account_sid` | string | yes | Twilio Account SID (e.g., `AC123...`) |
| `auth_token` | string | yes | Twilio auth token |

These are write-only — the API response only returns `account_sid_last_four` in `config.credentials`.

## Channel Targets

A target links a channel to an agent, allowing users to reach the agent through that channel.

### Create Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent_id` | integer | yes | Agent to associate with this target |
| `channel_id` | integer | yes | Channel this target belongs to |
| `target` | string | yes | Phone number, email address, or identifier. Must be an organization-level available target (check with `syllable channels available-targets`) |
| `target_mode` | string | yes | Communication mode (`voice`, `chat`, `sms`, `email`, `whatsapp`) |
| `fallback_target` | string | no | Alternate number if primary unavailable (voice mode only) |
| `is_test` | boolean | no | If true, sessions from this target are flagged as test (excluded from dashboards). Default: `false` |

### Response Fields (additional)

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | Target ID |
| `channel_name` | string | Name of the parent channel |
| `updated_at` | datetime | Last update timestamp |
| `last_updated_by` | string | Email of last updater |

## Responsive Dialogue (ReDi) — Bridge Phrases

When `async_enabled` is true, the agent uses holding phrases to fill silence during LLM processing delays. Bridge phrases are managed via `conversation-config bridges` and can be scoped at **three levels** (not the channel level):

- **Org default** — plain `bridges` / `bridges-update` with no scope flags.
- **Per-agent** — add `--agent-id <id>` to either command.
- **Per-tool** — add `--tool-name <name>` (may be combined with `--agent-id`).

Both `--agent-id` and `--tool-name` are optional query parameters (CLI v2.1+). A scoped GET falls back to the org default when no override exists for that agent/tool; a scoped PUT creates/updates the override without touching the org default. The API does not validate that the agent/tool exists — an unknown value simply returns the org default.

> **A second surface exists, and this one is slated for deprecation.** The `bridge-phrases` command (CLI v2.1+) manages **named, reusable** configs attached to an agent via the agent's `bridge_phrases_id` — separate endpoints, separate storage, and a different field shape (`config.phrases.messages` + `config.tools[]` rather than the three legacy `*_messages` lists). Everything in this section describes `conversation-config bridges` only.
>
> **Confirm which surface the org uses before writing either** — a write to the wrong one returns 200 and changes nothing. If the user doesn't know, ask whether the Console's agent config page lets them set a bridge phrase config per agent: yes → `bridge-phrases`, no → `conversation-config`. See SKILL.md § *Bridge Phrases — Confirm Which Config Surface First*, `commands.md` § *Bridge Phrases*, and `gotchas.md` § *Bridge Phrases — two separate surfaces*.

Bridge phrase schema (`schemas__cortex__v1__bridge_phrases__BridgePhraseMessages`) per language:

| Field | Type | Description |
|-------|------|-------------|
| `messages` | string[] | **Unified ordered list** (CLI v1.7.1+ spec) — when non-empty, this single list is used for all bridge phrases and the three legacy fields below are ignored |
| `randomize_messages` | boolean | Default `false`. When `true`, the unified `messages` play in randomized no-repeat cycles. Ignored when `messages` is empty |
| `first_slow_messages` | string[] | *Legacy* — messages when the agent is first delayed (e.g., "One moment please"). Applies only when `messages` is empty |
| `very_slow_messages` | string[] | *Legacy* — messages for continued/significant delays. Applies only when `messages` is empty |
| `tool_responses` | string[] | *Legacy* — messages while a tool call is in progress. Applies only when `messages` is empty |

The top-level `BridgePhrasesConfig` carries the same fields plus `localized` (per-language `BridgePhraseMessages` maps) and `smart_turn_timeout_seconds`. All fields are optional — existing configs without `messages` keep working unchanged.

| Field (top-level only) | Type | Description |
|-------|------|-------------|
| `smart_turn_timeout_seconds` | number \| null | **CLI v1.7.2+ spec.** Seconds of caller silence before the agent injects the *first* bridge phrase. Range `0.25`–`30`. Subsequent sleep intervals scale to 2x / 3x / 4x this base. When unset (`null`), the service-wide default applies. |

> **Persistence — agent-scoped writes now stick** (verified 2026-07-09 on the `cli-test` org): an `--agent-id`-scoped `bridges-update` persisted both `messages` and `smart_turn_timeout_seconds`, confirmed by read-back. This supersedes the earlier 2026-06-04 note that these fields were silently dropped on 200. Org-default (unscoped) persistence was not separately re-verified in that run — still read back to confirm. See `gotchas.md` § *Bridge Phrases*.

Manage via CLI:
```bash
# View the org-default bridge phrases
syllable conversation-config bridges

# View an agent- or tool-scoped config (falls back to org default if no override)
syllable conversation-config bridges --agent-id 768
syllable conversation-config bridges --tool-name transfer_call

# Update the org default
syllable conversation-config bridges-update --file bridges.json

# Create/update an agent-scoped override (leaves the org default untouched)
syllable conversation-config bridges-update --agent-id 768 --file bridges.json
```

## Campaign Voicemail Detection

Outbound voice campaigns have a separate `voicemail_detection` config object (distinct from the channel-level `transfer_leg_voicemail_detection_enabled`).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `voicemail_detection_overall_timeout` | number | 30 | Maximum seconds to wait for voicemail detection |
| `voicemail_detection_post_speech_timeout` | number | 1.75 | Seconds of silence after speech to classify as voicemail |
| `voicemail_detection_pre_speech_timeout` | number | 3.5 | Seconds of silence before speech to classify as voicemail |

Set `voicemail_detection` to `null` on the campaign to disable voicemail detection entirely.
