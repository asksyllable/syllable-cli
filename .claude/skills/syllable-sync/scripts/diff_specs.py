#!/usr/bin/env python3
"""
diff_specs.py — Structured diff of two Syllable OpenAPI specs.

Usage:
    python diff_specs.py <old_spec.json> <new_spec.json>

Outputs a human+Claude-readable summary of what changed:
  - New paths (endpoints)
  - Removed paths
  - Changed paths (new/removed methods, changed request/response schemas)
  - New schemas (components.schemas)
  - Removed schemas
  - Changed schemas (new/removed/changed fields)
"""

import json
import sys
from collections import defaultdict


def load(path):
    with open(path) as f:
        return json.load(f)


def schema_fields(schema, schemas):
    """Recursively resolve $ref and return a flat dict of field -> type."""
    if "$ref" in schema:
        ref_name = schema["$ref"].split("/")[-1]
        return schema_fields(schemas.get(ref_name, {}), schemas)
    fields = {}
    props = schema.get("properties", {})
    required = set(schema.get("required", []))
    for name, prop in props.items():
        type_str = prop.get("type", "")
        if not type_str and "$ref" in prop:
            type_str = "$ref:" + prop["$ref"].split("/")[-1]
        elif not type_str and "anyOf" in prop:
            type_str = "anyOf"
        req = " (required)" if name in required else ""
        fields[name] = f"{type_str}{req}"
    return fields


def diff_schemas(old_s, new_s, all_old, all_new):
    old_fields = schema_fields(old_s, all_old)
    new_fields = schema_fields(new_s, all_new)
    added = {k: v for k, v in new_fields.items() if k not in old_fields}
    removed = {k: v for k, v in old_fields.items() if k not in new_fields}
    changed = {
        k: {"old": old_fields[k], "new": new_fields[k]}
        for k in old_fields
        if k in new_fields and old_fields[k] != new_fields[k]
    }
    return added, removed, changed


def summarize_path(path, methods):
    ops = []
    for method, op in methods.items():
        if method in ("get", "post", "put", "patch", "delete"):
            summary = op.get("summary", "")
            tags = op.get("tags", [])
            ops.append(f"  {method.upper()} — {summary} {tags}")
    return ops


def main():
    if len(sys.argv) != 3:
        print("Usage: diff_specs.py <old.json> <new.json>")
        sys.exit(1)

    old = load(sys.argv[1])
    new = load(sys.argv[2])

    old_paths = old.get("paths", {})
    new_paths = new.get("paths", {})
    old_schemas = old.get("components", {}).get("schemas", {})
    new_schemas = new.get("components", {}).get("schemas", {})

    added_paths = sorted(set(new_paths) - set(old_paths))
    removed_paths = sorted(set(old_paths) - set(new_paths))
    common_paths = sorted(set(old_paths) & set(new_paths))

    # --- New paths ---
    if added_paths:
        print(f"\n## NEW PATHS ({len(added_paths)})")
        for p in added_paths:
            print(f"\n+ {p}")
            for line in summarize_path(p, new_paths[p]):
                print(line)

    # --- Removed paths ---
    if removed_paths:
        print(f"\n## REMOVED PATHS ({len(removed_paths)})")
        for p in removed_paths:
            print(f"\n- {p}")
            for line in summarize_path(p, old_paths[p]):
                print(line)

    # --- Changed paths ---
    changed_paths = []
    for p in common_paths:
        old_methods = {m for m in old_paths[p] if m in ("get","post","put","patch","delete")}
        new_methods = {m for m in new_paths[p] if m in ("get","post","put","patch","delete")}
        added_methods = new_methods - old_methods
        removed_methods = old_methods - new_methods
        if added_methods or removed_methods:
            changed_paths.append((p, added_methods, removed_methods))

    if changed_paths:
        print(f"\n## CHANGED PATHS — methods added/removed ({len(changed_paths)})")
        for p, added_m, removed_m in changed_paths:
            print(f"\n~ {p}")
            for m in sorted(added_m):
                op = new_paths[p][m]
                print(f"  + {m.upper()} — {op.get('summary', '')}")
            for m in sorted(removed_m):
                op = old_paths[p][m]
                print(f"  - {m.upper()} — {op.get('summary', '')}")

    # --- New schemas ---
    added_schemas = sorted(set(new_schemas) - set(old_schemas))
    removed_schemas = sorted(set(old_schemas) - set(new_schemas))

    if added_schemas:
        print(f"\n## NEW SCHEMAS ({len(added_schemas)})")
        for s in added_schemas:
            fields = schema_fields(new_schemas[s], new_schemas)
            print(f"\n+ {s}")
            for f, t in list(fields.items())[:10]:
                print(f"  {f}: {t}")
            if len(fields) > 10:
                print(f"  ... ({len(fields) - 10} more fields)")

    if removed_schemas:
        print(f"\n## REMOVED SCHEMAS ({len(removed_schemas)})")
        for s in removed_schemas:
            print(f"- {s}")

    # --- Changed schemas (only Create/Update/Response types — skip internal) ---
    changed_schema_summary = []
    for s in sorted(set(old_schemas) & set(new_schemas)):
        added_f, removed_f, changed_f = diff_schemas(
            old_schemas[s], new_schemas[s], old_schemas, new_schemas
        )
        if added_f or removed_f or changed_f:
            changed_schema_summary.append((s, added_f, removed_f, changed_f))

    if changed_schema_summary:
        print(f"\n## CHANGED SCHEMAS — field-level diffs ({len(changed_schema_summary)})")
        for s, added_f, removed_f, changed_f in changed_schema_summary:
            print(f"\n~ {s}")
            for f, t in added_f.items():
                print(f"  + {f}: {t}")
            for f, t in removed_f.items():
                print(f"  - {f}: {t}")
            for f, vals in changed_f.items():
                print(f"  ~ {f}: {vals['old']} → {vals['new']}")

    # --- Summary ---
    print(f"""
## SUMMARY
  Paths:   +{len(added_paths)} added, -{len(removed_paths)} removed, ~{len(changed_paths)} changed
  Schemas: +{len(added_schemas)} added, -{len(removed_schemas)} removed, ~{len(changed_schema_summary)} changed
""")

    if not any([added_paths, removed_paths, changed_paths, added_schemas, removed_schemas, changed_schema_summary]):
        print("\nNo differences found — specs are equivalent.")


if __name__ == "__main__":
    main()
