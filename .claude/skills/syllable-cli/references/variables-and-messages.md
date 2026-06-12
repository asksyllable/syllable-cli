# Variables & Messages Reference

Variable substitution syntax, system variables, and message (greeting/email template) configuration. Sources: `syllable schema get` and [docs.syllable.ai/resources/Variables](https://docs.syllable.ai/resources/Variables), [docs.syllable.ai/resources/Messages](https://docs.syllable.ai/resources/Messages).

## Variable Syntax Formats

The platform supports three substitution formats with different missing-value behaviors:

| Format | Missing-Value Behavior | Spacing | Use In |
|--------|----------------------|---------|--------|
| `{{ variable }}` | Renders empty string | Spaces around name are ignored | Greetings, tool descriptions, static parameters |
| `${variable=fallback}` | Renders the fallback value | No space trimming — `${ name }` fails to resolve | Prompts, anywhere a default value is needed |
| `{variable}` | Preserves literal text (e.g., `{variable}` stays as-is) | — | Prompts where the model should see the placeholder |

**Choose syntax intentionally:**
- `{{ }}` for values that should silently disappear when missing
- `${variable=fallback}` when you need a sensible default (e.g., `${timezone=America/Chicago}`)
- `{variable}` when the LLM should see and adapt to the placeholder

### Valid Variable Names

Permitted characters: letters (A–Z, a–z), numbers (0–9), underscores (`_`), and periods (`.`). No spaces or special characters.

## System Variables

Platform-managed values accessed via the `vars.session.*` path. These are read-only and always available.

| Variable | Description | Example Value |
|----------|-------------|---------------|
| `{{ vars.session.id }}` | Session identifier | `12345` |
| `{{ vars.session.agent.name }}` | Current agent name | `Anna` |
| `{{ vars.session.datetime }}` | Timestamp | `2025-01-02 12:00` |
| `{{ vars.session.date }}` | Date only | `2025-01-02` |
| `{{ vars.session.day_of_week }}` | Day name | `Tuesday` |
| `{{ vars.session.timezone }}` | Agent's configured timezone | `America/New_York` |
| `{{ vars.session.language }}` | Language code | `en-US` |
| `{{ vars.session.source }}` | ANI (caller's phone number) | `+15125551234` |
| `{{ vars.session.target }}` | DNIS (number the caller dialed) | `+18005559999` |

**Important:** Shorter forms like `{{ vars.agent_name }}` do not resolve — always use the full `vars.session.*` path for system variables.

## Custom Variables

Custom variables live on the agent as a flat `"variables": {}` dictionary. Reference a variable by the **exact same key** wherever it appears — greetings, prompt context, tool descriptions, and tool parameter values all resolve a variable as a literal key lookup. `{{ foo }}` resolves the key `foo`; `{{ vars.foo }}` resolves the key `vars.foo`. The three syntaxes (see "Variable Syntax Formats" above) differ only in missing-value behavior, not in how the key is matched.

### The `vars.` prefix has no special meaning

`vars.` is just part of the key. Reserve it for two cases — do **not** add it to new variables you define yourself:

1. **Platform-provided variables** — system values exposed under the `vars.session.*` namespace (`vars.session.id`, `vars.session.datetime`, `vars.session.source`, …). See "System Variables" above.
2. **Outbound campaign variables** — the outbound campaign system prepends `vars.` to every variable you supply (CSV upload or batch `request_variables`). So a campaign variable you call `patientId` is referenced in the prompt as `{{vars.patientId}}`. This is automatic and unavoidable for campaign-sourced variables.

For any **other** variable you define (e.g. a static value on the agent's `variables` map, or a tool parameter), pick a clean name **without** the `vars.` prefix and reference it by that exact name. One key per value — do **not** store both a bare and a `vars.`-prefixed copy of the same variable.

### Testing outbound agents in the conversation-test API

`send-test-message` sessions have no campaign batch, so campaign variables (`vars.*`) resolve to empty strings. Two implications:

- Write outbound prompts defensively: "If the value is blank, empty, missing, or anything other than exactly 'true', treat it as false."
- To exercise a persona without a live campaign, set the campaign variables **directly on the agent's `variables` map, using the exact `vars.`-prefixed keys the prompt reads** (e.g. `vars.firstName`, `vars.appointments_json`). This reproduces the session state a real campaign would produce — a useful pattern for dedicated `[TEST]` copies of outbound agents.

## Messages (Greetings & Email Templates)

Messages are configurable greetings or email templates delivered at the start of a conversation.

### Message Types

| Type | Value | Description |
|------|-------|-------------|
| Greeting | `greeting` | Voice/chat greeting delivered at conversation start (default) |
| Email Template | `email_template` | Branded email with subject line and HTML body |

### Core Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Console reference identifier |
| `text` | string | yes | Default message text (fallback when no rules match). For email templates, this is the HTML body |
| `type` | enum | no | `greeting` (default) or `email_template` |
| `label` | string | no | Label for filtering in the Console |
| `preamble` | string | no | Uninterruptible prefix delivered before the main message (e.g., legal disclaimers). Cannot contain `{{ language.mode }}` |
| `subject` | string | no | Email subject line. **Only accepted on `email_template`** — including it on a `greeting` is rejected with a 422 (`subject is only allowed when type is email_template`) |
| `rules` | array | no | Time-based rules for conditional message variants (default: `[]`) |

### Language Menu Tag

Insert `{{ language.mode }}` in the greeting text to trigger the language selection menu (DTMF-based). This tag is **not** allowed in the `preamble` field.

### Time-Based Rules

Rules allow different messages based on day-of-week, specific dates, or time ranges. Each rule is an object in the `rules` array.

#### Rule Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `description` | string | yes | Context about the rule (e.g., "Closed on New Year's Day") |
| `invert` | boolean | yes | If `true`, the rule matches when the condition is NOT met |
| `text` | string | yes | Message text to deliver when this rule matches |
| `days_of_week` | string[] | no | Day abbreviations: `mo`, `tu`, `we`, `th`, `fr`, `sa`, `su` |
| `date` | string | no | Specific date in `YYYY-MM-DD` format |
| `time_range_start` | string | no | Start of time window in 24-hour `HH:MM` format. Set to `null` for all-day rules |
| `time_range_end` | string | no | End of time window in 24-hour `HH:MM` format. Set to `null` for all-day rules |

**Day-of-week abbreviation warning:** Message rules use two-letter abbreviations (`mo`, `tu`, `we`... from the `DayOfWeek` enum). Campaign `active_days` use three-letter abbreviations (`mon`, `tue`, `wed`... from the `DaysOfWeek` enum). Don't mix them up.

#### Priority Ordering

When multiple rules match the current timestamp, the most specific rule wins:

1. **Specific date** rules (highest priority)
2. **Day-of-week** rules
3. **Inverted** rules (lowest priority)

Within the same specificity level, the rule matching the shortest time range takes precedence.

#### Example: Office Hours with Holiday Override

```json
{
  "name": "Main Office Greeting",
  "text": "Hello, thank you for calling. How can I help you today?",
  "preamble": "This call may be recorded for quality assurance.",
  "type": "greeting",
  "rules": [
    {
      "description": "Closed on New Year's Day",
      "invert": false,
      "date": "2025-01-01",
      "time_range_start": null,
      "time_range_end": null,
      "text": "Thank you for calling. Our office is closed today for New Year's Day. Please call back on our next business day."
    },
    {
      "description": "Weekend hours",
      "invert": false,
      "days_of_week": ["sa", "su"],
      "time_range_start": null,
      "time_range_end": null,
      "text": "Thank you for calling. Our office is currently closed for the weekend. Please call back Monday through Friday between 8 AM and 5 PM."
    },
    {
      "description": "After hours on weekdays",
      "invert": true,
      "days_of_week": ["mo", "tu", "we", "th", "fr"],
      "time_range_start": "08:00",
      "time_range_end": "17:00",
      "text": "Thank you for calling. Our office is currently closed. Our hours are Monday through Friday, 8 AM to 5 PM."
    }
  ]
}
```
