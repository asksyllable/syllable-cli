---
name: syllable-sync
description: >
  Reconciles the Syllable CLI Go codebase with a new OpenAPI spec file provided by
  the backend team. Use this skill whenever the user says the API has changed, they
  have a new openapi.json, endpoints were added or removed, or they want to sync/update
  the CLI. Also trigger when the user mentions "openapi", "spec diff", "new endpoints",
  "sync the CLI", or "update the CLI to match the API". The spec is sourced from the
  TypeScript SDK repo (asksyllable/syllable-sdk-typescript, main branch, openapi.json).
  This skill handles the full workflow: diff the specs, identify what changed, update
  Go cmd files, update docs, run tests.
---

# syllable-sync

Reconcile the Syllable CLI with a new version of the OpenAPI spec. This is a
guided, step-by-step workflow — follow each phase in order.

## Key paths

| What | Where |
|------|-------|
| CLI source | `scripts/syllable-cli/` |
| Embedded spec | `scripts/syllable-cli/internal/spec/openapi.json` |
| Cmd files | `scripts/syllable-cli/cmd/` (one `.go` file per resource) |
| HTTP client | `scripts/syllable-cli/internal/client/client.go` |
| Diff script | `~/.claude/skills/syllable-sync/scripts/diff_specs.py` |

---

## Phase 1 — Get the new spec

The latest spec is published automatically when the TypeScript SDK is deployed:

```
https://raw.githubusercontent.com/asksyllable/syllable-sdk-typescript/main/openapi.json
```

Download it and copy the current embedded spec for comparison:

```bash
python3 -c "
import urllib.request
url = 'https://raw.githubusercontent.com/asksyllable/syllable-sdk-typescript/main/openapi.json'
data = urllib.request.urlopen(url, timeout=15).read()
open('/tmp/openapi_new.json', 'wb').write(data)
import json; d = json.loads(data)
print('Version:', d['info']['version'], '| Paths:', len(d['paths']), '| Schemas:', len(d['components']['schemas']))
"

cp scripts/syllable-cli/internal/spec/openapi.json /tmp/openapi_old.json
```

Compare the version in the new spec against the embedded one:
```bash
python3 -c "
import json
old = json.load(open('/tmp/openapi_old.json'))['info']['version']
new = json.load(open('/tmp/openapi_new.json'))['info']['version']
print(f'Embedded: {old}  →  SDK repo: {new}')
"
```

If the versions match, run the diff anyway — schema changes can land without a version bump
(as seen in practice). Only skip if the diff also shows no changes.

---

## Phase 2 — Diff the specs

Run the diff script and capture the output:

```bash
python3 ~/.claude/skills/syllable-sync/scripts/diff_specs.py \
  /tmp/openapi_old.json /tmp/openapi_new.json
```

The script reports:
- **NEW PATHS** — endpoints that don't exist in the current CLI
- **REMOVED PATHS** — endpoints the current CLI calls that are gone
- **CHANGED PATHS** — existing endpoints with new/removed HTTP methods
- **NEW SCHEMAS** — new request/response types (with fields)
- **REMOVED SCHEMAS** — types that no longer exist
- **CHANGED SCHEMAS** — field-level diffs on existing types

Present the summary to the user and ask them to confirm before making any changes:
> "Here's what changed. Want me to proceed with updating the CLI?"

---

## Phase 3 — Plan the changes

Based on the diff output, categorize the work into a checklist:

**For each new path group (e.g., `/api/v1/widgets/`):**
- [ ] Create `scripts/syllable-cli/cmd/widgets.go` following the standard pattern
- [ ] Register the new command in `scripts/syllable-cli/cmd/root.go`

**For each removed path group:**
- [ ] Remove or stub the corresponding cmd file
- [ ] Remove the registration from `root.go`

**For each changed schema (field added/removed on a Create/Update type):**
- [ ] Update flags in the relevant `create` or `update` subcommand

**Doc updates:**
- [ ] README.md — add/remove/update the resource's section under Commands
- [ ] AGENTS.md — update the Resource Overview table and any relevant constraints

Share the plan with the user before writing any code. This is especially important
for removals — confirm the user wants to remove them, not just deprecate them.

---

## Phase 4 — Update the embedded spec

Replace the embedded spec first, before changing any code:

```bash
cp /tmp/openapi_new.json scripts/syllable-cli/internal/spec/openapi.json
```

Run the spec test to confirm it's valid:
```bash
cd scripts/syllable-cli && go test ./internal/spec/... -v
```

---

## Phase 5 — Implement code changes

### Adding a new resource

Read an existing cmd file for a similar resource first (e.g., `cmd/agents.go` for
a full CRUD resource, `cmd/events.go` for a read-only resource, `cmd/outbound.go`
for a nested resource). Then follow this pattern exactly:

```go
package cmd

import (
    "github.com/asksyllable/syllable-cli/internal/output"
    "github.com/spf13/cobra"
)

func widgetsCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "widgets",
        Short: "Manage widgets",
        Example: `  syllable widgets list
  syllable widgets get 42
  syllable widgets create --file widget.json`,
    }
    cmd.AddCommand(widgetsListCmd())
    cmd.AddCommand(widgetsGetCmd())
    cmd.AddCommand(widgetsCreateCmd())
    cmd.AddCommand(widgetsUpdateCmd())
    cmd.AddCommand(widgetsDeleteCmd())
    return cmd
}
```

Key conventions:
- API path: derive from the OpenAPI path, e.g., `/api/v1/widgets/` → `"/api/v1/widgets/"`
- Table headers and row fields: derive from the response schema's properties
- Use `printTable()` (not `output.PrintTable()`) — the wrapper applies `--fields` filtering
- Use `readFile(file)` (not `os.ReadFile()`) — the wrapper supports `--file -` for stdin
- IDs in URLs: use `fmt.Sprintf("/api/v1/widgets/%d", id)` for integer IDs, `%s` for string IDs
- Truncate long fields: `output.Truncate(val, 40)`
- For list commands, always support `--page`, `--limit`, `--search`

Register in `root.go` `init()`:
```go
rootCmd.AddCommand(widgetsCmd())
```

Also add a hint in `hintForError()` if the resource has a common 409/422 pattern.

### Updating an existing resource (field changes)

For added fields on a Create/Update schema:
- Add a flag: `cmd.Flags().StringVar(&newField, "new-field", "", "Description")`
- Include in the JSON body map when non-empty

For removed fields:
- Remove the flag and its usage from the body map
- Don't remove quietly — if the field was required, add a comment explaining the change

### Removing a resource

Before deleting, check `cmd_test.go` for any tests referencing the command and
remove those too. Then delete the cmd file and remove the `rootCmd.AddCommand()` line.

---

## Phase 6 — Update documentation

### README.md

- **New resource**: add a section under `## Commands` following the same format as
  existing sections (brief description, then the command signatures)
- **Removed resource**: remove its section; update the "Other Resources" table if applicable
- **Changed fields**: update flag descriptions or command signatures as needed

### AGENTS.md

- **New resource**: add a row to the Resource Overview table (Name | Purpose | Key Fields | Constraints)
- **Removed resource**: remove its row; note any deprecation if it was replaced
- **Dependency changes**: update the Resource Dependencies section if creation order changed

---

## Phase 7 — Test and verify

```bash
# Run all tests
cd scripts/syllable-cli && go test ./...

# Build to confirm compilation
go build -o /tmp/syllable-test .

# Smoke test the new command
/tmp/syllable-test widgets --help
```

Fix any compilation errors or test failures before proceeding.

---

## Phase 8 — Commit

Stage and commit in logical groups:

```bash
# 1. Spec update
git add scripts/syllable-cli/internal/spec/openapi.json
git commit -m "chore: update embedded OpenAPI spec to vX.Y.Z"

# 2. Code changes
git add scripts/syllable-cli/cmd/
git commit -m "feat: add widgets command / fix: update agents for renamed fields"

# 3. Docs
git add README.md AGENTS.md
git commit -m "docs: sync README and AGENTS with spec vX.Y.Z"
```

If there are breaking changes (removed commands, renamed flags), note them explicitly
in the commit message body and flag them to the user — they may need a version bump.
