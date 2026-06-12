# syllable-cli skill

A [Claude Code skill](https://docs.anthropic.com/en/docs/claude-code/skills) for working with the `syllable` CLI. It teaches Claude the full command surface, safety rules (org/env confirmation, never reading the config file, get → edit → put for updates), and the operational knowledge that isn't in `--help`: API gotchas, proven multi-step recipes, and copy-paste payload templates.

## Use it in your own project

Copy this directory into your repo:

```bash
mkdir -p .claude/skills
cp -R /path/to/syllable-cli/.claude/skills/syllable-cli .claude/skills/
```

Claude Code picks it up automatically the next time you start a session in that repo. Then just ask in plain language — "list the agents in my org", "update the greeting for agent 42", "what fields does AgentCreate expect?" — and the skill loads the right reference before running commands.

## Layout

| File | Purpose |
|------|---------|
| `SKILL.md` | Entry point: setup, global flags, resource/verb summary, safety rules |
| `references/commands.md` | Per-resource command syntax, table columns, inline-create flags |
| `references/gotchas.md` | Known CLI/API pitfalls with fixes (payload formats, validation, enums) |
| `references/common-patterns.md` | Multi-step recipes: cross-org cloning, bulk updates, data source wiring |
| `references/payload-examples.md` | Copy-paste JSON templates for create/update bodies |
| `references/telephony-and-channels.md` | Telephony config fields, channel/target enums, bridge phrases |
| `references/variables-and-messages.md` | Variable substitution syntax, system variables, greeting rules |
| `references/sessions-and-debugging.md` | Session fields, transcripts, latency analysis, debug commands |
| `references/insights.md` | Insights workflows, tool configs, folders, outputs |
| `references/outbound-campaigns.md` | Campaign/batch lifecycle, request statuses, variable scoping |

The reference files load on demand, not every session — `SKILL.md` stays lean and points to them.

## Keeping it current

The skill docs carry a version marker (`Docs synced to CLI v…`) at the top of `SKILL.md`. When CLI behavior changes in a release, update the affected reference files and bump the marker in the same PR.
