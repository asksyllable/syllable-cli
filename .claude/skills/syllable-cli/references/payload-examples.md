# Syllable CLI: Payload Examples

JSON templates for common create and update operations. Write payloads to `/tmp/` and pass with `--file`.

## Default LLM Config

Include this block in every prompt creation unless the user specifies otherwise:

```json
"llm_config": {
  "version": "2025-04-14",
  "model": "gpt-4.1",
  "provider": "openai",
  "temperature": 0
}
```

Use OpenAI (not Azure OpenAI) — Azure is slower. Temperature 0 for deterministic, safe responses. Always verify current valid combos with `syllable prompts supported-llms`.

## Tool Create — Query Parameters

```json
{
  "name": "get_weather",
  "service_id": 40,
  "definition": {
    "type": "endpoint",
    "endpoint": {
      "url": "https://api.open-meteo.com/v1/forecast",
      "method": "get",
      "argument_location": "query"
    },
    "tool": {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get the weather for a city",
        "parameters": {
          "type": "object",
          "properties": {
            "latitude": { "type": "number", "description": "Latitude of the city" },
            "longitude": { "type": "number", "description": "Longitude of the city" }
          },
          "required": ["latitude", "longitude"]
        }
      }
    },
    "static_parameters": [
      {
        "name": "current",
        "default": "temperature_2m,relative_humidity_2m,precipitation",
        "description": "Information to retrieve from the API, comma-separated",
        "type": "string",
        "required": true
      }
    ]
  }
}
```

**Required fields:** `name`, `service_id`, `definition.type` (`"endpoint"`), `definition.endpoint`, `definition.tool`. The `tool.function.name` must match the top-level `name`.

## Tool Create — Path Parameters

```json
{
  "name": "get_pokemon",
  "service_id": 62,
  "definition": {
    "type": "endpoint",
    "endpoint": {
      "url": "https://pokeapi.co/api/v2/pokemon/{name}",
      "method": "get",
      "argument_location": "path"
    },
    "tool": {
      "type": "function",
      "function": {
        "name": "get_pokemon",
        "description": "Look up a Pokemon by name",
        "parameters": {
          "type": "object",
          "properties": {
            "name": {
              "type": "string",
              "description": "The Pokemon name (e.g. pikachu)"
            }
          },
          "required": ["name"]
        }
      }
    }
  }
}
```

**`argument_location` options:** `query`, `body`, `path`, `form`.

## Data Source Search — Agent Wiring

After creating a data source and adding `data_source_search` to the prompt's tools list, set `prompt_tool_defaults` on the agent:

```json
"prompt_tool_defaults": [{
  "tool_name": "data_source_search",
  "default_values": [{
    "field_name": "doc",
    "default_value": ["your_data_source_name"]
  }]
}]
```

**`default_value` must be a JSON array** (e.g., `["my_faq"]`), not a plain string.

## Agent Variables — Dual-Key Format

Store each custom variable with both bare and `vars.`-prefixed keys so it renders in both greetings and prompts:

```json
{
  "variables": {
    "agent_name": "Anna",
    "vars.agent_name": "Anna",
    "callback_number": "+18005551234",
    "vars.callback_number": "+18005551234",
    "scheduling_transfer": "+18005559999",
    "vars.scheduling_transfer": "+18005559999"
  }
}
```

## Parallel Tool Creation

Tools only depend on `service_id`, not on each other. Create them in parallel:

```bash
# Write all tool JSON files first (e.g., with a Python script)
# Then create them all in parallel:
syllable tools create --file /tmp/tool-1.json --org <org> -o json &
syllable tools create --file /tmp/tool-2.json --org <org> -o json &
syllable tools create --file /tmp/tool-3.json --org <org> -o json &
wait  # Wait for all parallel creates to finish
```

This reduces N sequential CLI calls (~20s each) to a single parallel batch (~20s total).

## Static Parameters Format

The API format differs from some shorthand formats:

```json
"static_parameters": [
  {
    "name": "temperature_unit",
    "default": "fahrenheit",
    "description": "Temperature unit",
    "type": "string",
    "required": false
  }
]
```

**Translation from shorthand:** `value` → `default`, add `"type": "string"` and `"required": false`.

## Custom Message with Time-Based Rules

Greeting with holiday closure, weekend hours, and after-hours messages:

```json
{
  "name": "Main Office Greeting",
  "text": "Hello, thank you for calling. How can I help you today?",
  "type": "greeting",
  "preamble": "This call may be recorded for quality assurance.",
  "rules": [
    {
      "description": "Closed on New Year's Day",
      "invert": false,
      "date": "2025-01-01",
      "time_range_start": null,
      "time_range_end": null,
      "text": "Thank you for calling. Our office is closed today for New Year's Day."
    },
    {
      "description": "Weekend closure",
      "invert": false,
      "days_of_week": ["sa", "su"],
      "time_range_start": null,
      "time_range_end": null,
      "text": "Thank you for calling. Our office is closed on weekends."
    },
    {
      "description": "After hours on weekdays",
      "invert": true,
      "days_of_week": ["mo", "tu", "we", "th", "fr"],
      "time_range_start": "08:00",
      "time_range_end": "17:00",
      "text": "Thank you for calling. Our office is currently closed. Hours are Monday through Friday, 8 AM to 5 PM."
    }
  ]
}
```

**Day-of-week abbreviations:** `mo`, `tu`, `we`, `th`, `fr`, `sa`, `su` (two-letter, from `DayOfWeek` enum). Do NOT use the three-letter campaign format here.

**Required fields:** `description`, `invert`, `text` on each rule. Time fields (`time_range_start`/`time_range_end`) are null for all-day rules. Either `date` or `days_of_week` should be set (not both).

## Campaign Create

```json
{
  "campaign_name": "Appointment Reminders Q2",
  "description": "Automated appointment reminder calls",
  "caller_id": "+18005551234",
  "source": "+18005551234",
  "mode": "voice",
  "active_days": ["mon", "tue", "wed", "thu", "fri"],
  "daily_start_time": "09:00:00",
  "daily_end_time": "17:00:00",
  "hourly_rate": 50,
  "max_daily_calls": 500,
  "retry_count": 1,
  "retry_interval": "30m",
  "labels": ["reminders", "q2"],
  "campaign_variables": {
    "campaign_type": "appointment_reminder",
    "vars.campaign_type": "appointment_reminder"
  },
  "voicemail_detection": {
    "voicemail_detection_overall_timeout": 30,
    "voicemail_detection_post_speech_timeout": 1.75,
    "voicemail_detection_pre_speech_timeout": 3.5
  }
}
```

**Required fields:** `campaign_name`, `caller_id`, `campaign_variables`, `active_days`.

**Day-of-week abbreviations:** `mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun` (three-letter, from `DaysOfWeek` enum). Do NOT use the two-letter message-rule format here.

**Voicemail detection:** Set to `null` to disable. The defaults shown above are the platform defaults.

## Batch Requests

Add contacts to a batch. Each request needs a `reference_id` (unique within the batch), `target` phone number, and `request_variables` (custom fields rendered as `{{vars.*}}` in prompts):

```json
[
  {
    "reference_id": "appt-001",
    "target": "+15125551234",
    "request_variables": {
      "patient_name": "Jane Doe",
      "appointment_date": "2025-04-15",
      "appointment_time": "2:30 PM",
      "provider_name": "Dr. Smith"
    }
  },
  {
    "reference_id": "appt-002",
    "target": "+15125555678",
    "request_variables": {
      "patient_name": "John Smith",
      "appointment_date": "2025-04-15",
      "appointment_time": "3:00 PM",
      "provider_name": "Dr. Jones"
    }
  }
]
```

**Required fields:** `reference_id`, `target`, `request_variables` per request.

**Gotcha:** Batches deduplicate by `target` — if multiple requests share the same phone number in a single batch, only the first is dialed and the rest are marked `DUPLICATE`. Use sequential batches for repeated calls to the same number.

## Channel Target with Telephony Config

Create a channel target linking a phone number to an agent. Telephony config is set on the parent **channel**, not the target:

```json
{
  "agent_id": 42,
  "channel_id": 5,
  "target": "+18005559999",
  "target_mode": "voice",
  "fallback_target": "+18005550000",
  "is_test": false
}
```

**Required fields:** `agent_id`, `channel_id`, `target`, `target_mode`.

To update telephony config on the channel itself:

```json
{
  "name": "Main Voice Channel",
  "channel_service": "twilio",
  "config": {
    "telephony": {
      "async_enabled": true,
      "interruptibility": "all",
      "pre_input_timeout": 1.5,
      "post_speech_input_timeout": 1.0,
      "post_dtmf_input_timeout": 2.0,
      "overall_input_timeout": 30,
      "output_padding": 0.0,
      "passive_speech_input_enabled": true,
      "passive_input_start": 0.5
    }
  }
}
```

## Insight Tool Configuration

Insight tool configs reference a tool definition (template) and provide parameters. The `tool_result_set` on the definition determines output types.

```json
{
  "name": "call-summary-tool",
  "description": "Summarizes the call transcript",
  "version": 1,
  "insight_tool_definition_id": 1,
  "tool_arguments": {
    "prompt": "Provide a concise summary of the conversation, focusing on the caller's goal and how the agent responded."
  }
}
```

**Output value fields** on `InsightsOutput` responses:

| Field | Type | Use |
|-------|------|-----|
| `string_value` | string or null | Text outputs (summaries, classifications) |
| `numeric_value` | number or null | Numeric scores, counts |
| `json_value` | object | Structured data (always present, keyed by result set names) |

The `tool_result_set` on the definition maps result keys to types (e.g., `{"summary": "string"}`, `{"score": "number"}`, `{"is_resolved": "boolean"}`).

## Service Create — Auth Type Patterns

Services provide authentication for tools. The `auth_type` determines the `auth_values` format.

### Bearer Token

```json
{
  "name": "Patient Scheduling API",
  "description": "Bearer-authenticated scheduling service",
  "auth_type": "bearer",
  "auth_values": {
    "token": "your-api-token-here"
  }
}
```

### Basic Auth

```json
{
  "name": "Legacy EHR API",
  "description": "Basic auth for EHR integration",
  "auth_type": "basic",
  "auth_values": {
    "username": "api-user",
    "password": "api-password"
  }
}
```

### Custom Headers

```json
{
  "name": "Internal API",
  "description": "Custom header authentication",
  "auth_type": "custom_headers",
  "auth_values": {
    "X-API-Key": "your-key-here",
    "X-Client-ID": "your-client-id"
  }
}
```

### OAuth2

```json
{
  "name": "OAuth2 Service",
  "description": "OAuth2 client credentials flow",
  "auth_type": "oauth2",
  "auth_values": {
    "client_id": "your-client-id",
    "client_secret": "your-client-secret",
    "auth_url": "https://auth.example.com/oauth2/token"
  }
}
```

### No Authentication (Public API)

```json
{
  "name": "Public Weather API",
  "description": "No auth required",
  "auth_type": null,
  "auth_values": null
}
```

**Auth type enum values:** `basic`, `bearer`, `custom_headers`, `oauth2`, or `null`.

**Finding existing services:** Before creating a new service, check if one already exists for the same API:
```bash
syllable services list --org <org> -o json
```

**Required fields:** `name`, `description`. The `auth_type` and `auth_values` are optional (both default to `null` for public APIs).

## User Create — Add to Org

Adds a user to an org with a given role. Always pass via `--file`, never with the `--email`/`--role-id` flags (those serialize `role_id` as a string and fail the integer schema check).

```json
{
  "email": "person@yourcompany.com",
  "first_name": "First",
  "last_name": "Last",
  "role_id": 19,
  "login_type": "google",
  "skip_auth": true
}
```

```bash
syllable --org <org> users create --file /tmp/user.json -o json
```

**Required fields:** `email`, `role_id`.

**Critical fields (omit at your peril):**

| Field | Why |
|-------|-----|
| `skip_auth: true` | Required when the email already has an identity in Syllable (e.g., user exists in another org). Without it, the API returns an opaque `500 "An error occurred while creating the user"` because it tries to provision a fresh auth record for an already-known email. The Console always sends `skipAuth: true` for this reason — match that. |
| `login_type` | Set explicitly. Use `"google"` for Google Workspace domains. Use `"username_and_password"` otherwise. The schema default of `"google"` only triggers for `@gmail.com`, which is wrong for Workspace SSO orgs. |
| `role_id` (integer) | Look up via `syllable --org <org> roles list -o json`. Role IDs differ per org. Common roles: `Admin`, `Syllable Admin` (cross-org elevated), `Editor`, `Analyst`, `Viewer`. |

**On success:** the API returns the created user with `email_sent: true` and `activity_status: "invited"` — the activation email is dispatched automatically.

**Where to look up role IDs:**

```bash
syllable --org <org> roles list -o json | jq '.items[] | {id, name}'
```

## Voice Group Create

Voice groups map languages to voice providers and DTMF codes for language selection menus.

```json
{
  "name": "English-Spanish Voices",
  "language_configs": [
    {
      "language_code": "en-US",
      "voice_provider": "OpenAI",
      "voice_display_name": "Alloy",
      "dtmf_code": 1,
      "voice_speed": 1.0
    },
    {
      "language_code": "es-US",
      "voice_provider": "Google",
      "voice_display_name": "es-US-Neural2-B",
      "dtmf_code": 2,
      "voice_speed": 1.0
    }
  ],
  "skip_current_language_in_message": true,
  "description": "English and Spanish with DTMF selection"
}
```

**Required fields:** `name`, `language_configs`, `skip_current_language_in_message`.

**Language config fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `language_code` | enum | yes | BCP 47 code (e.g., `en-US`, `es-US`, `yue-HK`, `zh-CN`, `ko-KR`, `vi-VN`, `ru-RU`, `hi-IN`) |
| `voice_provider` | enum | yes | `OpenAI`, `ElevenLabs`, or `Google` |
| `voice_display_name` | string | yes | Voice name (e.g., `Alloy`, `es-US-Neural2-B`) |
| `dtmf_code` | integer | yes | Key press for language selection menu |
| `voice_speed` | number | no | Speed multiplier. Range: 0.25–4.0 (OpenAI/Google), 0.7–1.2 (ElevenLabs). Default: 1.0 |
| `voice_pitch` | number | no | Pitch adjustment in semitones, -20.0 to 20.0. Google only. Default: 0 |

**`skip_current_language_in_message`:** When `true`, the language selection menu in greetings (via `{{ language.mode }}` tag) omits the agent's current language. If the agent is already speaking English, callers only hear options for other languages.
