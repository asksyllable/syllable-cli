# Syllable CLI: Common Patterns

Proven CLI command recipes for common platform operations.

### Recipe: Wiring a Data Source to an Agent

1. Create the data source (`data-sources create`) — name must have no whitespace
2. The `data_source_search` tool already exists in the default service — no need to create it
3. Add `data_source_search` to the prompt's `tools` list when creating the prompt
4. On the agent, set `prompt_tool_defaults` to point the tool at the data source:
   ```json
   "prompt_tool_defaults": [{
     "tool_name": "data_source_search",
     "default_values": [{
       "field_name": "doc",
       "default_value": ["your_data_source_name"]
     }]
   }]
   ```
   **IMPORTANT:** `default_value` for `doc` must be a JSON array (e.g., `["my_faq"]`), not a plain string.

### Recipe: Creating Tools for a Public API (No Auth)

1. Create a service with `"auth_type": null` and `"auth_values": null`
2. Create tools referencing that service's ID
3. For APIs using path parameters (e.g., `/resource/{name}`), use `"argument_location": "path"` in the tool endpoint

### Recipe: Updating a Prompt's Tool List

Prompt updates require the full payload — you cannot send just the changed fields.

1. Fetch the current prompt: `syllable prompts get <prompt-id> --org <org> -o json > /tmp/prompt.json`
2. Modify the `tools` array in the JSON (add/remove tool names)
3. Ensure the JSON includes at minimum: `id`, `name`, `type`, `llm_config`, and `tools`
4. Resubmit: `syllable prompts update <prompt-id> --file /tmp/prompt.json --org <org>`

**Common use case:** Adding a system tool you forgot during initial build (e.g., `get_current_datetime`).

### Recipe: Deciding Whether to Reuse or Create Tools

When an existing tool has a similar name or endpoint to what you need:

1. Fetch the existing tool: `syllable tools get <tool_name> --org <org> -o json`
2. Compare: same endpoint URL **AND** same parameters (both dynamic and static)?
   - **Yes** → Reuse the existing tool. Do not create a duplicate.
   - **No** → Create a new tool with a distinct name, even if the base API is the same.
3. Common case: same API, different static params → create new tool.

### Recipe: Clone Agent from Another Org

Clone an existing agent from a source org to a target org. Always use `--env <name>` — it sets both the base URL and the API key.

**Step 1: Find the agent in source org**

```bash
syllable agents list --org <source-org> --env <source-env> --search "<keyword>" -o json
```

If the name contains `[` or `]`, search with a substring (e.g., `Downtown` instead of `[TEST] Downtown Clinic`).

Note the agent's `prompt_id` and `custom_message_id`.

**Step 2: Fetch prompt and greeting in parallel**

```bash
syllable prompts get <prompt_id> --org <source-org> --env <source-env> -o json
syllable custom-messages get <custom_message_id> --org <source-org> --env <source-env> -o json
```

**Step 3: Resolve tools in target org**

```bash
syllable tools list --org <target-org> --env <target-env> --limit 200 -o json \
  | python3 -c "import json,sys; d=json.load(sys.stdin); [print(i['name']) for i in d['items']]"
```

For tools missing in the target: substitute with a target-org equivalent, or copy from the source (fetch with `tools get`, resolve the correct `service_id` in the target — must be an integer, not the name).

**Step 4: Create resources in order**

1. Tools (if copying from source)
2. Prompt: `syllable prompts create --file /tmp/prompt.json --org <target-org> --env <target-env> -o json`
3. Greeting: `syllable custom-messages create --file /tmp/greeting.json --org <target-org> --env <target-env> -o json`
4. Agent (with new prompt/greeting IDs): `syllable agents create --file /tmp/agent.json --org <target-org> --env <target-env> -o json`

Set `language_group_id: null` — the source org's ID won't be valid in the target.

**Known gotchas:**

| Issue | Fix |
|-------|-----|
| 401 Unauthorized | Use `--env <name>` — sets both URL and API key |
| Search 400 with `[` in name | Use a substring without special chars |
| `Tool not found` on prompt create | Tool missing in target — substitute or copy |
| `service_id` required (422) | Must be an integer; `service_name` alone not accepted |
| `language_group_id` from source | Always set to `null` unless verified in target |
| `tools get` uses name, not ID | `syllable tools get <name>`, not `<id>` |

### Recipe: Configuring Agent Variables for Greeting + Prompt Rendering

Greetings and prompts resolve variables from **different keys** on the agent's `variables` dict. Store each custom variable with both keys:

```json
{
  "variables": {
    "agent_name": "Anna",
    "vars.agent_name": "Anna",
    "callback_number": "+18005551234",
    "vars.callback_number": "+18005551234"
  }
}
```

- **Greeting** uses bare keys: `{{ agent_name }}` → matches key `agent_name`
- **Prompt** uses `vars.` prefix: `{{vars.agent_name}}` → matches key `vars.agent_name`

If you only store `vars.agent_name`, the greeting renders blank. If you only store `agent_name`, the prompt renders blank.

**For outbound agents:** Campaign batch variables (from CSV `request_variables`) do NOT render in greetings — only in prompts via `{{vars.*}}`. Keep greetings minimal (agent name + recorded line notice). Have the prompt's opening step speak the recipient name and location name.

### Recipe: Bulk Prompt Text Update Across Agents

Update the same prompt text change across multiple agents (e.g., adding a holiday closure, changing a phone number, updating a workflow section).

**Step 1: List agents and identify which share the same prompt**

```bash
syllable agents list --org <org> --limit 100 -o json \
  | python3 -c "
import json, sys
data = json.load(sys.stdin)
for a in data['items']:
    print(f\"{a['id']}\t{a['name']}\t{a.get('prompt_id', 'N/A')}\")
"
```

Group agents by `prompt_id`. Agents sharing a prompt only need one update.

**Step 2: For each unique prompt, get → edit → put**

```bash
# Fetch the full prompt
syllable prompts get <prompt_id> --org <org> -o json > /tmp/prompt-<prompt_id>.json

# Edit the JSON (modify the 'context' field with your change)
# IMPORTANT: use Python json.dumps() to write the file, not shell heredoc,
# because backticks in prompt text are interpreted as command substitution by zsh

# Update with the full payload
syllable prompts update <prompt_id> --file /tmp/prompt-<prompt_id>.json --org <org>
```

**Safety tips:**
- Use `--dry-run` on the first update to verify the payload before executing
- Back up the original: `cp /tmp/prompt-<id>.json /tmp/prompt-<id>.backup.json`
- Prompts are versioned — you can view history with `syllable prompts history <id>` and compare versions in the Console

### Recipe: Bulk Variable Update Across Agents

Change a variable value (e.g., a transfer phone number) across multiple agents.

**Step 1: List agents and find those with the target variable**

```bash
syllable agents list --org <org> --limit 100 -o json \
  | python3 -c "
import json, sys
data = json.load(sys.stdin)
for a in data['items']:
    vars = a.get('variables', {})
    if 'scheduling_transfer' in vars:
        print(f\"{a['id']}\t{a['name']}\t{vars['scheduling_transfer']}\")
"
```

**Step 2: For each agent, get → update variables → put**

```bash
# Fetch agent
syllable agents get <agent_id> --org <org> -o json > /tmp/agent-<agent_id>.json

# Edit the variables dict (update both bare and vars. keys)
# Then update
syllable agents update <agent_id> --file /tmp/agent-<agent_id>.json --org <org>
```

**Remember:** Agent updates are full PUT. Include the entire agent payload, not just the changed fields.

### Recipe: Bulk Tool Swap Across Prompts

Replace an old tool with a new one across all prompts that reference it.

```bash
# Find all prompts using the old tool
syllable prompts list --org <org> --limit 100 -o json \
  | python3 -c "
import json, sys
data = json.load(sys.stdin)
for p in data['items']:
    tools = [t['name'] for t in p.get('tools', [])]
    if 'old_tool_name' in tools:
        print(f\"{p['id']}\t{p['name']}\t{tools}\")
"

# For each prompt: get, swap tool name in the tools array, update
syllable prompts get <prompt_id> --org <org> -o json > /tmp/prompt.json
# Edit: replace 'old_tool_name' with 'new_tool_name' in the tools array
syllable prompts update <prompt_id> --file /tmp/prompt.json --org <org>
```

**Gotcha:** The new tool must already exist in the org before updating the prompt.

### Recipe: Text Testing Outbound Agents with Batch Variables

The text API cannot inject campaign batch variables. Workaround: temporarily set agent-level variables to simulate a batch record.

1. **Create fixture files** — one JSON per test persona, containing the full agent payload with all batch columns added to the `variables` dict (both bare and `vars.` prefixed):
   ```json
   {
     "variables": {
       "agent_name": "Anna", "vars.agent_name": "Anna",
       "first_name": "Margaret", "vars.first_name": "Margaret",
       "appointment_date": "Wednesday, April ninth", "vars.appointment_date": "Wednesday, April ninth",
       "requires_transportation": "false", "vars.requires_transportation": "false"
     }
   }
   ```

2. **Save original variables** — `syllable agents get <id> -o json > original.json`

3. **For each fixture:**
   ```bash
   syllable agents update <id> --file fixture.json --org <org>
   # Run text tests — variables now render in both greeting and prompt
   ```

4. **Restore original variables** — `syllable agents update <id> --file original.json --org <org>`

This validates prompt logic with real values before committing to voice tests. Catches issues like:
- Conditional branching on "true"/"false" strings (vs blank)
- DOB comparison with actual mismatched values
- Variable rendering in greeting and prompt
