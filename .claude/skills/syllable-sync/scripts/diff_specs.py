#!/usr/bin/env python3
"""
diff_specs.py — Structured diff of two Syllable OpenAPI specs.

Usage:
    python diff_specs.py [--exit-code] [--format text|json] <old_spec.json> <new_spec.json>

Outputs a human+Claude-readable summary of what changed:
  - New paths (endpoints)
  - Removed paths
  - Changed paths (new/removed methods, changed request/response schemas)
  - New schemas (components.schemas)
  - Removed schemas
  - Changed schemas (new/removed/changed fields)

Flags:
  --exit-code         Exit 1 if any structural differences are found, else 0.
                      Without this flag the script always exits 0, preserving
                      the original behaviour relied on by the syllable-sync skill.
  --format text|json  Output format. "text" (default) is the human-readable
                      report; "json" emits a machine-readable object carrying
                      the six diff buckets, their counts, and a has_changes flag
                      (for CI / scripted consumption).

Exit codes:
  0  success — no differences, or --exit-code not set
  1  differences found (only when --exit-code is set)
  2  usage error — bad arguments or an unreadable/invalid spec file
"""

import json
import sys

METHODS = ("get", "post", "put", "patch", "delete")


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
        if method in METHODS:
            summary = op.get("summary", "")
            tags = op.get("tags", [])
            ops.append(f"  {method.upper()} — {summary} {tags}")
    return ops


def compute_diff(old, new):
    """Compute the six diff buckets. Pure — no I/O."""
    old_paths = old.get("paths", {})
    new_paths = new.get("paths", {})
    old_schemas = old.get("components", {}).get("schemas", {})
    new_schemas = new.get("components", {}).get("schemas", {})

    added_paths = sorted(set(new_paths) - set(old_paths))
    removed_paths = sorted(set(old_paths) - set(new_paths))
    common_paths = sorted(set(old_paths) & set(new_paths))

    changed_paths = []
    for p in common_paths:
        old_methods = {m for m in old_paths[p] if m in METHODS}
        new_methods = {m for m in new_paths[p] if m in METHODS}
        added_m = new_methods - old_methods
        removed_m = old_methods - new_methods
        if added_m or removed_m:
            changed_paths.append((p, added_m, removed_m))

    added_schemas = sorted(set(new_schemas) - set(old_schemas))
    removed_schemas = sorted(set(old_schemas) - set(new_schemas))

    changed_schema_summary = []
    for s in sorted(set(old_schemas) & set(new_schemas)):
        added_f, removed_f, changed_f = diff_schemas(
            old_schemas[s], new_schemas[s], old_schemas, new_schemas
        )
        if added_f or removed_f or changed_f:
            changed_schema_summary.append((s, added_f, removed_f, changed_f))

    return {
        "old_paths": old_paths,
        "new_paths": new_paths,
        "new_schemas": new_schemas,
        "added_paths": added_paths,
        "removed_paths": removed_paths,
        "changed_paths": changed_paths,
        "added_schemas": added_schemas,
        "removed_schemas": removed_schemas,
        "changed_schema_summary": changed_schema_summary,
    }


def has_changes(d):
    return any(
        [
            d["added_paths"],
            d["removed_paths"],
            d["changed_paths"],
            d["added_schemas"],
            d["removed_schemas"],
            d["changed_schema_summary"],
        ]
    )


def render_text(d):
    added_paths = d["added_paths"]
    removed_paths = d["removed_paths"]
    changed_paths = d["changed_paths"]
    added_schemas = d["added_schemas"]
    removed_schemas = d["removed_schemas"]
    changed_schema_summary = d["changed_schema_summary"]
    new_paths = d["new_paths"]
    old_paths = d["old_paths"]
    new_schemas = d["new_schemas"]

    if added_paths:
        print(f"\n## NEW PATHS ({len(added_paths)})")
        for p in added_paths:
            print(f"\n+ {p}")
            for line in summarize_path(p, new_paths[p]):
                print(line)

    if removed_paths:
        print(f"\n## REMOVED PATHS ({len(removed_paths)})")
        for p in removed_paths:
            print(f"\n- {p}")
            for line in summarize_path(p, old_paths[p]):
                print(line)

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

    print(f"""
## SUMMARY
  Paths:   +{len(added_paths)} added, -{len(removed_paths)} removed, ~{len(changed_paths)} changed
  Schemas: +{len(added_schemas)} added, -{len(removed_schemas)} removed, ~{len(changed_schema_summary)} changed
""")

    if not has_changes(d):
        print("\nNo differences found — specs are equivalent.")


def render_json(d):
    obj = {
        "added_paths": d["added_paths"],
        "removed_paths": d["removed_paths"],
        "changed_paths": [
            {"path": p, "added_methods": sorted(am), "removed_methods": sorted(rm)}
            for p, am, rm in d["changed_paths"]
        ],
        "added_schemas": d["added_schemas"],
        "removed_schemas": d["removed_schemas"],
        "changed_schemas": [
            {
                "schema": s,
                "added_fields": af,
                "removed_fields": rf,
                "changed_fields": cf,
            }
            for s, af, rf, cf in d["changed_schema_summary"]
        ],
        "summary": {
            "paths": {
                "added": len(d["added_paths"]),
                "removed": len(d["removed_paths"]),
                "changed": len(d["changed_paths"]),
            },
            "schemas": {
                "added": len(d["added_schemas"]),
                "removed": len(d["removed_schemas"]),
                "changed": len(d["changed_schema_summary"]),
            },
        },
        "has_changes": has_changes(d),
    }
    print(json.dumps(obj, indent=2))


def _usage(msg):
    print(f"error: {msg}", file=sys.stderr)
    print(
        "Usage: diff_specs.py [--exit-code] [--format text|json] <old.json> <new.json>",
        file=sys.stderr,
    )
    sys.exit(2)


def main():
    argv = sys.argv[1:]
    exit_code = False
    fmt = "text"
    positional = []

    i = 0
    while i < len(argv):
        a = argv[i]
        if a in ("-h", "--help"):
            print(__doc__)
            sys.exit(0)
        elif a == "--exit-code":
            exit_code = True
        elif a == "--format":
            i += 1
            if i >= len(argv):
                _usage("--format requires an argument (text|json)")
            fmt = argv[i]
        elif a.startswith("--format="):
            fmt = a.split("=", 1)[1]
        elif a.startswith("-"):
            _usage(f"unknown option: {a}")
        else:
            positional.append(a)
        i += 1

    if fmt not in ("text", "json"):
        _usage(f"invalid --format value: {fmt!r} (expected text|json)")
    if len(positional) != 2:
        _usage("expected exactly two spec files: <old.json> <new.json>")

    try:
        old = load(positional[0])
        new = load(positional[1])
    except (OSError, json.JSONDecodeError) as e:
        _usage(f"could not read spec: {e}")

    d = compute_diff(old, new)

    if fmt == "json":
        render_json(d)
    else:
        render_text(d)

    if exit_code and has_changes(d):
        sys.exit(1)
    sys.exit(0)


if __name__ == "__main__":
    main()
