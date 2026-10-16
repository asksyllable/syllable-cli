#!/usr/bin/env python3
"""Classify an upstream OpenAPI change for the spec-sync bot.

Verdicts:
  ship       Safe to sync and release automatically: new optional fields,
             schemas, enum values, and query parameters; deprecations; loosened
             or widened rules; response-only changes; and server-side tightening
             of a request body the CLI only passes through from --file.
  decision   Breaking for CLI users, so a human decides: removed fields, enum
             values, or schemas; tightening of a body the CLI builds from inline
             flags; changed or removed parameters; security and server changes;
             anything unrecognized.
  reconcile  Commands must change before the spec can land (the CLI's coverage
             tests enforce this): an operation was removed, or an added one
             couldn't get a generated command.

An added operation ships when a build of the CLI with the new spec (and
`go generate ./cmd`) gave it a generated command; pass that build's
`syllable __operations` dump as the candidate. Without one, every added
operation needs reconciling.

The rules are the ones documented in CONTRIBUTING (Public repo hygiene aside):
"Breaking changes" in .claude/skills/syllable-sync/routines/full-cycle.md.

Usage:
  classify_drift.py OLD.json NEW.json --operations OPS.json [--candidate CANDIDATE.json] [--format json|text]
OPS.json is the output of `syllable __operations` built from the current code;
CANDIDATE.json is the same, built with the new spec after `go generate ./cmd`.
"""
import argparse
import hashlib
import importlib.util
import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
DIFF_SPECS = os.path.join(HERE, "..", "..", ".claude", "skills", "syllable-sync", "scripts", "diff_specs.py")

CONSTRAINTS = {
    "minLength", "maxLength", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum",
    "pattern", "minItems", "maxItems", "multipleOf", "uniqueItems", "minProperties",
    "maxProperties", "format", "const",
}
UNION_KEYS = {"anyOf", "oneOf", "type"}
WRITE_METHODS = {"post", "put", "patch", "delete"}
HTTP_METHODS = {"get", "post", "put", "patch", "delete", "head", "options", "trace"}
UNSAFE_TEXT = re.compile("[\x00-\x1f\x7f-\x9f\u061c\u200e\u200f\u202a-\u202e\u2066-\u2069]")


def plain(text, limit=120):
    """Upstream text, safe for one line of a public release note."""
    text = " ".join(UNSAFE_TEXT.sub(" ", text or "").split()).replace("`", "'")
    return text if len(text) <= limit else text[:limit - 1].rstrip() + "…"


def load_diff_specs():
    spec = importlib.util.spec_from_file_location("diff_specs", DIFF_SPECS)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def refs_in(node):
    """Schema names referenced anywhere inside node."""
    if isinstance(node, dict):
        ref = node.get("$ref")
        if isinstance(ref, str) and ref.startswith("#/components/schemas/"):
            yield ref.rsplit("/", 1)[-1]
        for value in node.values():
            yield from refs_in(value)
    elif isinstance(node, list):
        for value in node:
            yield from refs_in(value)


def closure(names, schemas):
    seen, todo = set(), list(names)
    while todo:
        name = todo.pop()
        if name in seen or name not in schemas:
            continue
        seen.add(name)
        todo.extend(refs_in(schemas[name]))
    return seen


def request_users(spec):
    """schema name -> set of (METHOD, path) whose request body reaches it."""
    schemas = spec.get("components", {}).get("schemas", {})
    users = {}
    for path, item in spec.get("paths", {}).items():
        for method, op in item.items():
            if method not in WRITE_METHODS or not isinstance(op, dict):
                continue
            body = op.get("requestBody")
            if not body:
                continue
            for name in closure(set(refs_in(body)), schemas):
                users.setdefault(name, set()).add((method.upper(), path))
    return users


def tokens(path):
    return [t.replace("~1", "/").replace("~0", "~") for t in path.strip("/").split("/") if t]


def field_of(path):
    """The dotted property chain a change path points into, e.g. 'tool_name'."""
    parts, out = tokens(path), []
    for i, t in enumerate(parts):
        if i > 0 and parts[i - 1] == "properties":
            out.append(t)
    return ".".join(out)


def as_list(v):
    if isinstance(v, list):
        return v
    return [] if v is None else [v]


class Classifier:
    def __init__(self, old, new, operations, candidate=None):
        self.old, self.new = old, new
        self.ops_by_route = {(e["method"].upper(), e["path"]): e for e in operations.values()}
        self.candidate = None if candidate is None else {
            (e["method"].upper(), e["path"]): e for e in candidate.values()}
        self.new_commands = []
        users = request_users(old)
        for name, routes in request_users(new).items():
            users.setdefault(name, set()).update(routes)
        self.request_users = users
        self.decisions, self.reconcile, self.notes, self.ship_reasons = [], [], [], []

    def commands_for(self, routes):
        return sorted({self.ops_by_route[r]["command"] for r in routes if r in self.ops_by_route})

    def tightening(self, schema, what):
        """A rule became stricter on `schema`. Ship it only if every request that
        uses the schema is a pass-through body (the command has no inline flags)."""
        routes = self.request_users.get(schema, set())
        if not routes:
            self.ship_reasons.append(f"{schema}: {what} (response-only)")
            return
        inline = sorted(
            f"{self.ops_by_route[r]['command']} (--{', --'.join(self.ops_by_route[r]['flags'])})"
            for r in routes if r in self.ops_by_route and self.ops_by_route[r]["flags"])
        unmapped = sorted(f"{m} {p}" for m, p in routes if (m, p) not in self.ops_by_route)
        if inline or unmapped:
            self.decisions.append(
                f"{schema}: {what}; the CLI builds this body from flags in "
                f"{', '.join(inline + unmapped)}, so existing invocations may stop working")
            return
        cmds = self.commands_for(routes)
        self.notes.append(f"`{'`, `'.join(cmds)}`: {what} (enforced by the server for `--file` bodies).")
        self.ship_reasons.append(f"{schema}: {what} (pass-through body)")

    def schema_change(self, schema, change):
        path, kind = change["path"], change["kind"]
        parts = tokens(path)
        last = parts[-1] if parts else ""
        field = field_of(path)
        label = f"`{schema}.{field}`" if field else f"`{schema}`"

        if last == "deprecated":
            if kind != "removed" and change.get("new"):
                self.notes.append(f"{label} is now deprecated.")
            return
        if len(parts) >= 2 and parts[-2] == "properties":
            if kind == "added":
                self.notes.append(f"{label} is a new field.")
                return
            if kind == "removed":
                self.decisions.append(f"{label} was removed")
                return
        if last == "required":
            before, after = set(as_list(change.get("old"))), set(as_list(change.get("new")))
            for name in sorted(after - before):
                full = f"{field}.{name}" if field else name
                self.tightening(schema, f"`{full}` is now required")
            return
        if last in CONSTRAINTS:
            if kind == "removed":
                return
            value = json.dumps(change.get("new"))
            self.tightening(schema, f"{label} now has `{last}: {value}`")
            return
        if last in UNION_KEYS or last == "enum":
            before = [json.dumps(v, sort_keys=True) for v in as_list(change.get("old"))]
            after = [json.dumps(v, sort_keys=True) for v in as_list(change.get("new"))]
            if kind == "added" and last == "enum":
                self.tightening(schema, f"{label} now only accepts listed values")
            elif set(before) <= set(after):
                if last != "enum":
                    self.notes.append(f"{label} now accepts more types.")
            elif last == "enum":
                pass  # removed enum values are reported once, from changed_enums
            else:
                self.tightening(schema, f"{label} accepts fewer types")
            return
        if last in {"default", "additionalProperties", "nullable", "items"}:
            self.tightening(schema, f"{label} changed `{last}`")
            return
        self.decisions.append(f"{label}: unrecognized change {kind} at `{path}`")

    def operation_change(self, method, path, change):
        route = (method.upper(), path)
        cmd = self.ops_by_route.get(route, {}).get("command", f"{method.upper()} {path}")
        parts = tokens(change["path"])
        head = parts[0] if parts else ""
        if head == "deprecated" and change["kind"] != "removed":
            return
        if head == "responses":
            # Error responses and additions inside an existing success body are
            # safe. A success response that is removed or changed, or gains a
            # media type (which also changes the CLI's Accept header), changes
            # what users get back, so a human decides.
            code = parts[1] if len(parts) > 1 else ""
            new_media = len(parts) == 4 and parts[2] == "content" and change["kind"] == "added"
            if len(parts) == 1 or (code.startswith("2") and (change["kind"] != "added" or new_media)):
                self.decisions.append(f"`{cmd}`: success response changed ({change['kind']} at `{change['path']}`)")
            return
        if head in {"parameters", "path_parameters"}:
            if len(parts) == 1 and change["kind"] == "changed":
                old_p = {(p.get("in"), p.get("name")): p for p in as_list(change.get("old")) if isinstance(p, dict)}
                new_p = {(p.get("in"), p.get("name")): p for p in as_list(change.get("new")) if isinstance(p, dict)}
                for key in sorted(set(new_p) - set(old_p)):
                    if new_p[key].get("required"):
                        self.decisions.append(f"`{cmd}`: new required {key[0]} parameter `{key[1]}`")
                    else:
                        self.notes.append(f"`{cmd}`: new optional {key[0]} parameter `{key[1]}`.")
                for key in sorted(set(old_p) - set(new_p)):
                    self.decisions.append(f"`{cmd}`: {key[0]} parameter `{key[1]}` was removed")
                for key in sorted(set(old_p) & set(new_p)):
                    if old_p[key] != new_p[key]:
                        a = {k: v for k, v in old_p[key].items() if k != "deprecated"}
                        b = {k: v for k, v in new_p[key].items() if k != "deprecated"}
                        if a == b:
                            self.notes.append(f"`{cmd}`: parameter `{key[1]}` is now deprecated.")
                        else:
                            self.decisions.append(f"`{cmd}`: {key[0]} parameter `{key[1]}` changed")
                return
            self.decisions.append(f"`{cmd}`: parameter change at `{change['path']}`")
            return
        if head == "requestBody":
            if self.ops_by_route.get(route, {}).get("flags"):
                self.decisions.append(f"`{cmd}`: request body contract changed at `{change['path']}` and the CLI builds it from flags")
            else:
                self.ship_reasons.append(f"{cmd}: pass-through request body changed at {change['path']}")
            return
        self.decisions.append(f"`{cmd}`: {head or 'operation'} changed ({change['kind']} at `{change['path']}`)")

    def added_operation(self, method, path):
        route = (method.upper(), path)
        entry = (self.candidate or {}).get(route)
        if self.candidate is None:
            self.reconcile.append(f"new operation {route[0]} {path}")
        elif not entry:
            self.reconcile.append(f"new operation {route[0]} {path} can't get a generated command, so it needs a hand-written one")
        else:
            op = self.new["paths"][path][method.lower()]
            summary = plain(op.get("summary"))
            self.new_commands.append(entry["command"])
            self.notes.append(f"New command `syllable {entry['command']}`" + (f": {summary}" if summary else "")
                              + ". It's generated from the API spec; run it with `--help` for its arguments and request fields.")
            self.ship_reasons.append(f"new operation {route[0]} {path} got a command (`{entry['command']}`)")

    def lost_commands(self):
        """A command main has that the candidate build doesn't, for an operation
        that still exists: a generated command that can no longer be generated."""
        if self.candidate is None:
            return
        paths = self.new.get("paths", {})
        for (method, path), entry in sorted(self.ops_by_route.items()):
            if method.lower() in paths.get(path, {}) and (method, path) not in self.candidate:
                self.reconcile.append(f"`{entry['command']}` ({method} {path}) can no longer be generated, so it needs a hand-written command")

    def run(self, d):
        for p in d["added_paths"]:
            item = self.new["paths"][p]
            if "$ref" in item or "servers" in item:
                self.reconcile.append(f"new path {p} uses a path-level `$ref` or `servers`, which generation doesn't support")
                continue
            for m in sorted(k for k in item if k in HTTP_METHODS):
                self.added_operation(m, p)
        for p in d["removed_paths"]:
            self.reconcile.append(f"removed path {p}")
        for entry in d["changed_paths"]:
            for m in entry["added_methods"]:
                self.added_operation(m, entry["path"])
            for m in entry["removed_methods"]:
                self.reconcile.append(f"removed operation {m.upper()} {entry['path']}")
        self.lost_commands()
        for change in d["changed_global_contracts"]:
            self.decisions.append(f"global spec change ({change['kind']} at `{change['path']}`)")
        for name in d["removed_schemas"]:
            self.decisions.append(f"schema `{name}` was removed")
        if d["added_schemas"]:
            self.ship_reasons.append(f"{len(d['added_schemas'])} new schema(s)")
        for e in d["changed_enums"]:
            if e["removed"]:
                self.decisions.append(f"`{e['enum']}` lost values {e['removed']}")
            elif e["added"]:
                self.notes.append(f"`{e['enum']}` accepts new values: {', '.join(e['added'])}.")
        for entry in d["changed_schema_contracts"]:
            for change in entry["changes"]:
                self.schema_change(entry["schema"], change)
        for entry in d["changed_operation_contracts"]:
            for change in entry["changes"]:
                self.operation_change(entry["method"], entry["path"], change)


def classify(old, new, operations, candidate=None):
    ds = load_diff_specs()
    raw = ds.compute_diff(old, new)
    d = {
        "added_paths": raw["added_paths"],
        "removed_paths": raw["removed_paths"],
        "changed_paths": [{"path": p, "added_methods": sorted(a), "removed_methods": sorted(r)}
                          for p, a, r in raw["changed_paths"]],
        "added_schemas": raw["added_schemas"],
        "removed_schemas": raw["removed_schemas"],
        "changed_operation_contracts": raw["changed_operation_contracts"],
        "changed_schema_contracts": raw["changed_schema_contracts"],
        "changed_global_contracts": raw["changed_global_contracts"],
        "changed_enums": [{"enum": l, "added": a, "removed": r} for l, a, r in raw["enum_changes"]],
    }
    c = Classifier(old, new, operations, candidate)
    c.run(d)
    verdict = "reconcile" if c.reconcile else "decision" if c.decisions else "ship"
    delta = hashlib.sha256(json.dumps(d, sort_keys=True).encode()).hexdigest()[:16]
    notes = list(dict.fromkeys(c.notes))  # de-duplicate, keep order
    return {
        "has_changes": ds.has_changes(raw),
        "verdict": verdict,
        "reconcile": c.reconcile,
        "decisions": c.decisions,
        "release_notes": notes,
        "ship_reasons": c.ship_reasons,
        "new_commands": c.new_commands,
        "delta": delta,
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("old")
    ap.add_argument("new")
    ap.add_argument("--operations", required=True)
    ap.add_argument("--candidate")
    ap.add_argument("--format", choices=("json", "text"), default="text")
    args = ap.parse_args()
    load = lambda p: json.load(open(p))
    result = classify(load(args.old), load(args.new), load(args.operations),
                      load(args.candidate) if args.candidate else None)
    if args.format == "json":
        print(json.dumps(result, indent=2))
        return
    print(f"verdict: {result['verdict']}  (delta {result['delta']})")
    for key in ("reconcile", "decisions", "release_notes", "ship_reasons"):
        for line in result[key]:
            print(f"  {key}: {line}")


if __name__ == "__main__":
    main()
