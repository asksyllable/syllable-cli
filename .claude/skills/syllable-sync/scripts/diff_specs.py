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
  - Changed enums (added/removed enum members — REMOVED values are breaking)

Flags:
  --exit-code         Exit 1 if any structural differences are found, else 0.
                      Without this flag the script always exits 0, preserving
                      the original behaviour relied on by the syllable-sync skill.
  --format text|json  Output format. "text" (default) is the human-readable
                      report; "json" emits a machine-readable object carrying
                      the diff buckets, their counts, and a has_changes flag
                      (for CI / scripted consumption).

Exit codes:
  0  success — no differences, or --exit-code not set
  1  differences found (only when --exit-code is set)
  2  usage error — bad arguments or an unreadable/invalid spec file
"""

import json
import re
import sys
from collections import Counter

METHODS = ("get", "post", "put", "patch", "delete", "head", "options", "trace")

# These annotate a contract without changing accepted input or returned data.
# Defaults, deprecated flags, constraints, and $refs deliberately remain.
ANNOTATIONS = {"description", "summary", "title", "example", "examples", "externalDocs", "x-codeSamples"}
UNORDERED_ARRAYS = {"required", "enum", "anyOf", "oneOf", "allOf", "parameters", "type", "security"}
NAMED_MAPS = {"properties", "schemas", "paths", "responses", "content", "headers", "$defs", "definitions", "parameters", "requestBodies", "securitySchemes", "webhooks"}
UNIONS = {"anyOf", "oneOf", "allOf"}


def contract(node, key=""):
    """Normalize structural API data, ignoring prose and irrelevant ordering."""
    # Default/const/enum values are data, not schema annotations. An object
    # value may itself have a key called "description" that must be preserved.
    if key in {"default", "const", "enum"}:
        return sorted(node, key=lambda v: json.dumps(v, sort_keys=True)) if key == "enum" and isinstance(node, list) else node
    if isinstance(node, dict):
        named = key in NAMED_MAPS
        return {k: contract(v, "" if named else k) for k, v in node.items() if named or k not in ANNOTATIONS}
    if isinstance(node, list):
        values = [contract(v) for v in node]
        if key in UNORDERED_ARRAYS:
            values = sorted(values, key=lambda v: json.dumps(v, sort_keys=True))
        return values
    return node


def contract_changes(old, new, path=""):
    """Describe deep differences, including constraints inside union/array types."""
    if old == new:
        return []
    if isinstance(old, dict) and isinstance(new, dict):
        changes = []
        for key in sorted(old.keys() | new.keys()):
            location = path + "/" + key.replace("~", "~0").replace("/", "~1")
            if key not in old:
                changes.append({"path": location, "kind": "added", "new": new[key]})
            elif key not in new:
                changes.append({"path": location, "kind": "removed", "old": old[key]})
            else:
                changes.extend(contract_changes(old[key], new[key], location))
        return changes
    return [{"path": path or "/", "kind": "changed", "old": old, "new": new}]


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


def enum_members(spec):
    """Walk the entire spec and return {json_path: set(enum_values)} for every
    `enum` array found, at any depth (schemas, properties, parameters, etc.).
    Catches enum-member changes that the field-level schema diff cannot see."""
    out = {}

    def walk(node, path):
        if isinstance(node, dict):
            e = node.get("enum")
            if isinstance(e, list):
                out[path] = set(map(str, e))
            for k, v in node.items():
                walk(v, f"{path}/{k}")
        elif isinstance(node, list):
            for i, v in enumerate(node):
                # Parameter indices shift when an unrelated query is inserted.
                # Identify them by name/location so that doesn't invent enum removals.
                part = f"{v['in']}:{v['name']}" if path.endswith("/parameters") and isinstance(v, dict) and "name" in v and "in" in v else str(i)
                walk(v, f"{path}/{part}")

    walk(spec, "")
    return out


def enum_shape(node, omit_nested_unions=False, key=""):
    """Branch identity without mutable enum values; keep all other constraints.

    The outer shape is a fallback for a branch whose nested union gained/lost
    members. Defaults and constants remain data, including keys named 'enum'.
    """
    if key in {"default", "const"}:
        return node
    if isinstance(node, dict):
        named = key in NAMED_MAPS
        return {
            k: [] if not named and omit_nested_unions and k in UNIONS else
            enum_shape(v, omit_nested_unions, "" if named else k)
            for k, v in node.items() if named or k != "enum"
        }
    if isinstance(node, list):
        return [enum_shape(v, omit_nested_unions) for v in node]
    return node


def enum_profile(branch):
    """Enum overlap guides one-to-one matching, never the removal report.

    Ignore union positions but retain field paths and multiplicity. Actual
    changes are still compared separately in each aligned branch below.
    """
    profile = Counter()
    for path, values in enum_members(branch).items():
        path = re.sub(r"/(anyOf|oneOf|allOf)/[0-9]+", r"/\1/branch", path)
        profile.update((path, value) for value in values)
    return profile


def match_union_branches(old, new):
    """Match unchanged branches first, then enum-free structural identities.

    Within duplicate shapes, augmenting paths find an additive matching when
    one exists (greedy overlap alone can invent a removal). Remaining changed
    pairs prefer enum overlap. Distinct constraints/discriminators stay separate,
    so a value retained in another branch cannot hide a meaningful loss.
    """
    pending_old, pending_new = set(range(len(old))), set(range(len(new)))
    pairs = {}
    profiles_old, profiles_new = list(map(enum_profile, old)), list(map(enum_profile, new))

    def distance(i, j):
        before, after = profiles_old[i], profiles_new[j]
        return sum((before - after).values()), sum((after - before).values())

    for shape in (lambda node: node, enum_shape, lambda node: enum_shape(node, True)):
        before, after = {}, {}
        for indexes, branches, groups in ((pending_old, old, before), (pending_new, new, after)):
            for i in sorted(indexes):
                identity = json.dumps(contract(shape(branches[i])), sort_keys=True)
                groups.setdefault(identity, []).append(i)
        for identity in sorted(before.keys() & after.keys()):
            candidates = {
                i: sorted((j for j in after[identity] if distance(i, j)[0] == 0),
                          key=lambda j: (distance(i, j)[1], j))
                for i in before[identity]
            }
            matched_new = {}

            def augment(i, visited):
                for j in candidates[i]:
                    if j in visited:
                        continue
                    visited.add(j)
                    if j not in matched_new or augment(matched_new[j], visited):
                        matched_new[j] = i
                        return True
                return False

            for i in sorted(before[identity], key=lambda i: (len(candidates[i]), i)):
                augment(i, set())
            matched = {i: j for j, i in matched_new.items()}
            unused_old = set(before[identity]) - matched.keys()
            unused_new = set(after[identity]) - set(matched.values())
            while unused_old and unused_new:
                i, j = min(((i, j) for i in unused_old for j in unused_new),
                           key=lambda pair: (*distance(*pair), *pair))
                matched[i] = j
                unused_old.remove(i)
                unused_new.remove(j)
            pairs.update(matched)
            pending_old.difference_update(matched)
            pending_new.difference_update(matched.values())
    return pairs


def align_enum_branches(old, new, key=""):
    """Align only the enum walk; the structural diff keeps its canonical order."""
    if key in {"default", "const", "enum"}:
        return new
    if isinstance(new, dict):
        old = old if isinstance(old, dict) else {}
        named = key in NAMED_MAPS
        return {k: align_enum_branches(old.get(k), v, "" if named else k) for k, v in new.items()}
    if isinstance(new, list):
        old = old if isinstance(old, list) else []
        if key in UNIONS:
            pairs = match_union_branches(old, new)
            # Empty placeholders retain paths for removed branches; additions
            # get fresh paths after the old slots, rather than shifting them.
            aligned = [align_enum_branches(branch, new[pairs[i]]) if i in pairs else {}
                       for i, branch in enumerate(old)]
            aligned.extend(branch for j, branch in enumerate(new) if j not in pairs.values())
            return aligned
        if key == "parameters":
            def identity(parameter):
                if isinstance(parameter, dict) and "in" in parameter and "name" in parameter:
                    return parameter["in"], parameter["name"]
                return json.dumps(parameter, sort_keys=True)
            previous = {identity(parameter): parameter for parameter in old}
            return [align_enum_branches(previous.get(identity(parameter)), parameter) for parameter in new]
        return [align_enum_branches(old[i] if i < len(old) else None, v) for i, v in enumerate(new)]
    return new


def enum_label(path):
    """Render a readable label for an enum's JSON path."""
    parts = [p for p in path.split("/") if p]
    if "schemas" in parts:
        i = parts.index("schemas")
        if i + 1 < len(parts):
            name = parts[i + 1]
            if "properties" in parts[i + 1:]:
                j = parts.index("properties", i + 1)
                if j + 1 < len(parts):
                    return f"{name}.{parts[j + 1]}"
            return name
    if parts and parts[-1] == "enum":
        parts = parts[:-1]
    return "/".join(parts)


def compute_diff(old, new):
    """Compute the diff buckets (paths, schemas, enums). Pure — no I/O."""
    old_paths = old.get("paths", {})
    new_paths = new.get("paths", {})
    old_schemas = old.get("components", {}).get("schemas", {})
    new_schemas = new.get("components", {}).get("schemas", {})

    def global_contract(spec):
        value = {k: spec.get(k) for k in ("openapi", "jsonSchemaDialect", "security", "servers", "webhooks")}
        value["components"] = {k: v for k, v in spec.get("components", {}).items() if k != "schemas"}
        return contract(value)

    changed_global_contracts = contract_changes(global_contract(old), global_contract(new))

    added_paths = sorted(set(new_paths) - set(old_paths))
    removed_paths = sorted(set(old_paths) - set(new_paths))
    common_paths = sorted(set(old_paths) & set(new_paths))

    changed_paths = []
    changed_operation_contracts = []
    for p in common_paths:
        old_methods = {m for m in old_paths[p] if m in METHODS}
        new_methods = {m for m in new_paths[p] if m in METHODS}
        added_m = new_methods - old_methods
        removed_m = old_methods - new_methods
        if added_m or removed_m:
            changed_paths.append((p, added_m, removed_m))
        for method in sorted(old_methods & new_methods):
            # Include inherited path parameters, not just operation parameters.
            before = {k: old_paths[p][method].get(k) for k in
                      ("parameters", "requestBody", "responses", "security", "servers", "deprecated")}
            after = {k: new_paths[p][method].get(k) for k in before}
            before["path_parameters"] = old_paths[p].get("parameters")
            after["path_parameters"] = new_paths[p].get("parameters")
            before["path_servers"] = old_paths[p].get("servers")
            after["path_servers"] = new_paths[p].get("servers")
            changes = contract_changes(contract(before), contract(after))
            if changes:
                changed_operation_contracts.append({"path": p, "method": method, "changes": changes})

    added_schemas = sorted(set(new_schemas) - set(old_schemas))
    removed_schemas = sorted(set(old_schemas) - set(new_schemas))

    changed_schema_summary = []
    changed_schema_contracts = []
    for s in sorted(set(old_schemas) & set(new_schemas)):
        added_f, removed_f, changed_f = diff_schemas(
            old_schemas[s], new_schemas[s], old_schemas, new_schemas
        )
        if added_f or removed_f or changed_f:
            changed_schema_summary.append((s, added_f, removed_f, changed_f))
        changes = contract_changes(contract(old_schemas[s]), contract(new_schemas[s]))
        if changes:
            changed_schema_contracts.append({"schema": s, "changes": changes})

    # Enum-member diff (walks the whole spec). Removed values are breaking and
    # are invisible to the field-level schema diff above.
    old_contract = contract(old)
    old_enums = enum_members(old_contract)
    new_enums = enum_members(align_enum_branches(old_contract, contract(new)))
    enum_changes = []  # (label, added_values, removed_values)
    for p in sorted(set(old_enums) | set(new_enums)):
        a = old_enums.get(p, set())
        b = new_enums.get(p, set())
        if a != b:
            enum_changes.append((enum_label(p), sorted(b - a), sorted(a - b)))

    return {
        "old_paths": old_paths,
        "new_paths": new_paths,
        "new_schemas": new_schemas,
        "added_paths": added_paths,
        "removed_paths": removed_paths,
        "changed_paths": changed_paths,
        "changed_operation_contracts": changed_operation_contracts,
        "added_schemas": added_schemas,
        "removed_schemas": removed_schemas,
        "changed_schema_summary": changed_schema_summary,
        "changed_schema_contracts": changed_schema_contracts,
        "enum_changes": enum_changes,
        "changed_global_contracts": changed_global_contracts,
    }


def enum_removals(d):
    """Enum changes that drop a value (breaking)."""
    return [(label, add, rem) for label, add, rem in d["enum_changes"] if rem]


def has_changes(d):
    return any(
        [
            d["added_paths"],
            d["removed_paths"],
            d["changed_paths"],
            d["changed_operation_contracts"],
            d["added_schemas"],
            d["removed_schemas"],
            d["changed_schema_summary"],
            d["changed_schema_contracts"],
            d["changed_global_contracts"],
            d["enum_changes"],
        ]
    )


def render_text(d):
    added_paths = d["added_paths"]
    removed_paths = d["removed_paths"]
    changed_paths = d["changed_paths"]
    added_schemas = d["added_schemas"]
    removed_schemas = d["removed_schemas"]
    changed_schema_summary = d["changed_schema_summary"]
    enum_changes = d["enum_changes"]
    new_paths = d["new_paths"]
    old_paths = d["old_paths"]
    new_schemas = d["new_schemas"]

    if d["changed_global_contracts"]:
        print("\n## CHANGED GLOBAL CONTRACT — servers, security, shared components")
        for change in d["changed_global_contracts"]:
            print("  " + json.dumps(change, sort_keys=True))

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

    if enum_changes:
        print(f"\n## CHANGED ENUMS — member diffs ({len(enum_changes)})")
        for label, added_v, removed_v in enum_changes:
            print(f"\n~ {label}")
            for v in added_v:
                print(f"  + {v}")
            for v in removed_v:
                print(f"  - {v}  (REMOVED — breaking)")
        breaking = enum_removals(d)
        if breaking:
            print(
                f"\n  ⚠ {len(breaking)} enum(s) lost values — BREAKING for clients "
                f"relying on them; do not treat as additive."
            )

    for label, entries, identifier in (
        ("OPERATION CONTRACTS", d["changed_operation_contracts"], "path"),
        ("SCHEMA CONTRACTS", d["changed_schema_contracts"], "schema"),
    ):
        if not entries:
            continue
        print(f"\n## CHANGED {label} — deep structural diffs ({len(entries)})")
        for entry in entries:
            prefix = entry.get("method", "").upper()
            print(f"\n~ {prefix} {entry[identifier]}".rstrip())
            for change in entry["changes"]:
                values = {k: v for k, v in change.items() if k in ("old", "new")}
                detail = json.dumps(values, sort_keys=True)
                if len(detail) > 240:
                    detail = detail[:237] + "..."
                print(f"  {change['kind']} {change['path']}: {detail}")

    print(f"""
## SUMMARY
  Paths:   +{len(added_paths)} added, -{len(removed_paths)} removed, ~{len(changed_paths)} changed
  Schemas: +{len(added_schemas)} added, -{len(removed_schemas)} removed, ~{len(changed_schema_summary)} changed
  Enums:   ~{len(enum_changes)} changed ({len(enum_removals(d))} with removed values — breaking)
  Contracts: {len(d['changed_operation_contracts'])} operations, {len(d['changed_schema_contracts'])} schemas, {len(d['changed_global_contracts'])} global changes
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
        "changed_operation_contracts": d["changed_operation_contracts"],
        "changed_schema_contracts": d["changed_schema_contracts"],
        "changed_global_contracts": d["changed_global_contracts"],
        "changed_enums": [
            {"enum": label, "added": added_v, "removed": removed_v}
            for label, added_v, removed_v in d["enum_changes"]
        ],
        "summary": {
            "paths": {
                "added": len(d["added_paths"]),
                "removed": len(d["removed_paths"]),
                "changed": len(d["changed_paths"]),
                "contract_changed": len(d["changed_operation_contracts"]),
            },
            "schemas": {
                "added": len(d["added_schemas"]),
                "removed": len(d["removed_schemas"]),
                "changed": len(d["changed_schema_summary"]),
                "contract_changed": len(d["changed_schema_contracts"]),
            },
            "enums": {
                "changed": len(d["enum_changes"]),
                "with_removed_values": len(enum_removals(d)),
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
