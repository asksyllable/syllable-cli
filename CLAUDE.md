# Syllable CLI

This repo contains the Syllable CLI — a Go tool for managing the Syllable AI platform.

## Binary

Run the CLI by building locally or from a release:

**From source (no pre-built binary checked in):**
```bash
cd scripts/syllable-cli && go build -o syllable . && ./syllable --help
```

Or install a release: `curl -fsSL https://github.com/asksyllable/syllable-cli/releases/latest/download/install.sh | sh`

To run tests:
```bash
cd scripts/syllable-cli && go test ./...
```

## Configuration

Run `syllable setup` to open the browser-based config UI. This manages orgs, API keys, and environments in `~/.syllable/config.yaml`. Never edit that file directly or ask the user to paste keys into the terminal.

## Key Flags

| Flag | Purpose |
|------|---------|
| `--org NAME` | Select org (looks up API key from config) |
| `--env NAME` | Select environment: `prod`, `staging`, `dev` |
| `--output json` | Machine-readable output |
| `--fields id,name` | Filter table columns |
| `--dry-run` | Show request without sending it |
| `--debug` | Print full HTTP exchange to stderr |
| `--file -` | Read JSON body from stdin |

## Common Patterns

```bash
# List with filtered columns
syllable agents list --fields id,name

# Get full JSON for scripting
syllable agents get 42 --output json

# Pipe to update (fetch → modify → push)
syllable agents get 42 --output json | jq '.name = "New"' | syllable agents update 42 --file -

# Preview a destructive action
syllable agents delete 42 --dry-run

# Discover schema for a create/update body
syllable schema list --filter agent
syllable schema get AgentCreate
```

## Error Hints

With `--output json`, errors include a `hint` field with the next action to take. HTTP status codes follow standard conventions — 401 (auth), 403 (permissions), 404 (not found), 422 (validation).

## Full Reference

See `AGENTS.md` for the complete command reference.
