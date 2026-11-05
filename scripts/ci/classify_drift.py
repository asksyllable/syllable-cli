#!/usr/bin/env python3
"""Classify an upstream OpenAPI change for the spec-sync bot.

The API is the source of truth, and the server enforces its own rules whatever
CLI version sends a request. Holding the CLI back from a change protects nobody,
so no change waits for a person: either syncing the spec is enough, or the
CLI's code has to change first.

Verdicts:
  ship       Syncing is enough: new optional fields, schemas, enum values, and
             query parameters; deprecations; loosened or widened rules;
             response-only changes; and every request rule the server
             enforces, new or removed (new required fields and constraints,
             narrowed types, removed fields, enum values, schemas, and query or
             path parameters). Anything a user can notice gets a release-notes
             line. Where a flag can no longer do its job (no flag sets a field
             the API now requires, or a flag sets a field the API removed), the
             change still ships and a follow-up asks the local spec-sync
             routine to fix the flags.
  reconcile  The CLI's code must change before the spec can land, and the
             local spec-sync routine changes it: an operation was removed, an
             added one couldn't get a generated command, a generated command
             can no longer be generated, a success response or a flag-built
             request body changed shape, a header or cookie parameter became
             required, or the auth, servers, or something unrecognized changed.

An added operation ships when a build of the CLI with the new spec (and
`go generate ./cmd`) gave it a generated command; pass that build's
`syllable __operations` dump as the candidate. Without one, every added
operation needs reconciling.

CONTRIBUTING's "Spec sync" section documents these rules.

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


def body_users(spec):
    """Two maps, schema name -> set of (METHOD, path): operations whose request
    body reaches the schema, and operations whose success response does."""
    schemas = spec.get("components", {}).get("schemas", {})
    requests, responses = {}, {}
    for path, item in spec.get("paths", {}).items():
        for method, op in item.items():
            if method not in HTTP_METHODS or not isinstance(op, dict):
                continue
            route = (method.upper(), path)
            body = op.get("requestBody")
            if body and method in WRITE_METHODS:
                for name in closure(set(refs_in(body)), schemas):
                    requests.setdefault(name, set()).add(route)
            for code, response in (op.get("responses") or {}).items():
                if str(code).startswith("2"):
                    for name in closure(set(refs_in(response)), schemas):
                        responses.setdefault(name, set()).add(route)
    return requests, responses


IDENT_UNSAFE = re.compile(r"[^A-Za-z0-9_.\-\[\]{}/:~$]")
VALUE_UNSAFE = re.compile(r"[^A-Za-z0-9_.\-/:,+() ]")


def ident(name, limit=64):
    """A name from the upstream spec (a field, parameter, or JSON pointer),
    reduced to the characters a name needs. The local routine acts on follow-up
    text, so spec-controlled strings can't carry prose or markup into it."""
    name = IDENT_UNSAFE.sub("", str(name))
    return name if len(name) <= limit else name[:limit - 1] + "…"


def value(text, limit=64):
    """A value from the upstream spec (an enum member or a constraint), which
    may contain spaces, reduced the same way."""
    text = " ".join(VALUE_UNSAFE.sub("", str(text)).split())
    return text if len(text) <= limit else text[:limit - 1] + "…"


def type_set(node):
    """The types a schema node accepts: names, `ref:` targets, or "any"."""
    if not isinstance(node, dict):
        return {"any"}
    if isinstance(node.get("$ref"), str):
        return {"ref:" + node["$ref"]}
    out = {str(t) for t in as_list(node.get("type"))}
    for key in ("anyOf", "oneOf"):
        for member in as_list(node.get(key)):
            out |= type_set(member)
    return out or {"any"}


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
        self.request_users, self.response_users = {}, {}
        for spec in (old, new):
            requests, responses = body_users(spec)
            for name, routes in requests.items():
                self.request_users.setdefault(name, set()).update(routes)
            for name, routes in responses.items():
                self.response_users.setdefault(name, set()).update(routes)
        self.reconcile, self.notes, self.ship_reasons = [], [], []
        self.new_optional_headers = {}  # (in, name) -> commands
        self.flag_checks = {}  # command that builds its body from flags -> what changed
        self.typed_fields = set()  # (schema, pointer) whose type change was judged
        self.output_checks = {}  # hand-written command that reads a response -> what changed

    def commands_for(self, routes):
        return sorted({self.ops_by_route[r]["command"] for r in routes if r in self.ops_by_route})

    def node(self, spec, schema, pointer):
        node = spec.get("components", {}).get("schemas", {}).get(schema)
        for t in pointer:
            if isinstance(node, dict):
                node = node.get(t)
            elif isinstance(node, list) and t.isdigit() and int(t) < len(node):
                node = node[int(t)]
            else:
                return None
        return node

    def check(self, bucket, cmd, item):
        items = bucket.setdefault(cmd, [])
        if item not in items:
            items.append(item)

    def changed(self, schema, what, field=None, removed=False, shape=False):
        """A rule changed on `schema`. Every rule the server enforces ships.
        Users get a release note, and commands that build the body from flags
        or parse the response get a follow-up for the routine to check.

        field: the top-level field that became required (or, with removed, was
        removed). shape: the change can break a reader of the schema (a removed
        field or a changed type), not only a writer."""
        requests = self.request_users.get(schema, set())
        responses = self.response_users.get(schema, set()) if shape else set()
        if requests:
            cmds = self.commands_for(requests)
            where = f"`{'`, `'.join(cmds)}`: " if cmds else f"`{ident(schema)}`: "
            self.notes.append(f"{where}{what} (enforced by the API).")
            self.ship_reasons.append(f"{ident(schema)}: {what} (server-enforced)")
            flag = field.replace("_", "-") if field and "." not in field else None
            for route in sorted(requests):
                entry = self.ops_by_route.get(route)
                if not entry or not entry["flags"]:
                    continue
                if flag and removed and flag in entry["flags"]:
                    item = f"`--{ident(flag)}` sets `{ident(field)}`, which the API removed"
                elif flag and not removed and flag not in entry["flags"]:
                    item = f"`{ident(field)}` is now required, and no flag sets it"
                else:
                    item = what
                self.check(self.flag_checks, entry["command"], item)
        readers = [self.ops_by_route[r] for r in sorted(responses) if r in self.ops_by_route]
        if readers:
            cmds = sorted({e["command"] for e in readers})
            self.notes.append(f"`{'`, `'.join(cmds)}`: responses changed: {what}.")
            for e in readers:
                if not e.get("generated"):
                    self.check(self.output_checks, e["command"], what)
        if not requests and not readers:
            self.ship_reasons.append(f"{ident(schema)}: {what} (response-only)")

    def schema_change(self, schema, change):
        path, kind = change["path"], change["kind"]
        parts = tokens(path)
        last = parts[-1] if parts else ""
        field = field_of(path)
        label = f"`{ident(schema)}.{ident(field)}`" if field else f"`{ident(schema)}`"

        if len(parts) >= 2 and parts[-2] == "properties":
            if kind == "added":
                self.notes.append(f"{label} is a new field.")
            elif kind == "removed":
                self.changed(schema, f"the API removed {label}", field=field, removed=True, shape=True)
            else:
                self.changed(schema, f"{label} changed shape", shape=True)
            return
        if last == "deprecated":
            if kind != "removed" and change.get("new"):
                self.notes.append(f"{label} is now deprecated.")
            return
        if last == "required":
            before, after = set(as_list(change.get("old"))), set(as_list(change.get("new")))
            for name in sorted(after - before):
                full = f"{field}.{name}" if field else name
                self.changed(schema, f"`{ident(full)}` is now required", field=full)
            return
        if last in CONSTRAINTS:
            if kind == "removed":
                return
            self.changed(schema, f"{label} now has `{last}: {value(json.dumps(change.get('new')))}`")
            return
        if last == "enum":
            if kind == "added":
                self.changed(schema, f"{label} now only accepts listed values")
            return  # removed enum values are reported once, from changed_enums
        if last in UNION_KEYS:
            # `type: X` becoming `anyOf: [X, null]` shows up as two changes, so
            # judge the field's whole set of types, once.
            pointer = tuple(parts[:-1])
            if (schema, pointer) in self.typed_fields:
                return
            self.typed_fields.add((schema, pointer))
            before = type_set(self.node(self.old, schema, pointer))
            after = type_set(self.node(self.new, schema, pointer))
            if before == after:
                return
            if "any" in after or after >= before:
                self.notes.append(f"{label} now accepts more types.")
            else:
                self.changed(schema, f"{label} accepts fewer types", shape=True)
            return
        if last in {"default", "additionalProperties", "nullable", "items"}:
            self.changed(schema, f"{label} changed `{last}`", shape=last != "default")
            return
        self.reconcile.append(f"{label}: unrecognized change ({kind} at `{ident(path, 200)}`); check whether the CLI needs to change")

    def enum_removed(self, label, values):
        """Values an enum no longer accepts: for request bodies the API rejects
        them, and a flag-built body may send one by default."""
        schema = label.split(".", 1)[0]
        shown = ", ".join(value(v) for v in values)
        self.notes.append(f"`{ident(label)}` no longer accepts: {shown} (enforced by the API).")
        for route in sorted(self.request_users.get(schema, set())):
            entry = self.ops_by_route.get(route)
            if entry and entry["flags"]:
                self.check(self.flag_checks, entry["command"], f"`{ident(label)}` no longer accepts {shown}")

    def resolve_parameter(self, p, spec):
        ref = p.get("$ref") if isinstance(p, dict) else None
        if isinstance(ref, str) and ref.startswith("#/components/parameters/"):
            p = spec.get("components", {}).get("parameters", {}).get(ref.rsplit("/", 1)[-1])
        return p if isinstance(p, dict) and p.get("in") and p.get("name") else None

    def parameter_note(self, cmd, key, old, new):
        """One release note for a query or path parameter that changed."""
        where, name = key[0], ident(key[1])
        if old is None:
            req = "required" if new.get("required") else "optional"
            return f"`{cmd}`: new {req} {where} parameter `{name}`."
        if new is None:
            return f"`{cmd}`: the API removed the {where} parameter `{name}`."
        a = {k: v for k, v in old.items() if k != "deprecated"}
        b = {k: v for k, v in new.items() if k != "deprecated"}
        if a == b:
            return f"`{cmd}`: {where} parameter `{name}` is now deprecated."
        details = []
        if not old.get("required") and new.get("required"):
            details.append("is now required")
        sa, sb = old.get("schema") or {}, new.get("schema") or {}
        for k in sorted(set(sa) | set(sb)):
            if sa.get(k) != sb.get(k) and k not in {"title", "description"}:
                details.append(f"now has `{ident(k)}: {value(json.dumps(sb.get(k)))}`" if k in sb else f"no longer has `{ident(k)}`")
        return f"`{cmd}`: {where} parameter `{name}` " + (", ".join(details) or "changed") + " (enforced by the API)."

    def operation_change(self, method, path, change):
        route = (method.upper(), path)
        entry = self.ops_by_route.get(route, {})
        cmd = entry.get("command", f"{method.upper()} {ident(path, 200)}")
        parts = tokens(change["path"])
        head = parts[0] if parts else ""
        if head == "deprecated" and change["kind"] != "removed":
            return
        if head == "responses":
            # Error responses are safe. A success response that is removed, or
            # gains or loses a media type (which also changes the CLI's Accept
            # header), may need the command's output handling to change. A
            # change inside a success body's schema ships, and a hand-written
            # command that parses that body gets a follow-up.
            code = parts[1] if len(parts) > 1 else ""
            in_schema = len(parts) >= 5 and parts[2] == "content" and parts[4] == "schema"
            new_media = len(parts) == 4 and parts[2] == "content" and change["kind"] == "added"
            if len(parts) == 1 or (code.startswith("2") and not in_schema and (change["kind"] != "added" or new_media)):
                self.reconcile.append(f"`{cmd}`: success response changed ({change['kind']} at `{ident(change['path'], 200)}`); "
                                      f"check the command's output and Accept header")
            elif in_schema and code.startswith("2"):
                self.ship_reasons.append(f"{cmd}: success response schema changed at {ident(change['path'], 200)}")
                loosened = change["kind"] == "removed" and parts[-1] in CONSTRAINTS | UNION_KEYS
                if entry and not entry.get("generated") and not loosened:
                    self.notes.append(f"`{cmd}`: the API changed its response.")
                    self.check(self.output_checks, cmd, f"its success response schema changed at `{ident(change['path'], 200)}`")
            return
        if head in {"parameters", "path_parameters"}:
            if len(parts) == 1 and change["kind"] == "changed":
                old_p, new_p = {}, {}
                for raw, spec, out in ((change.get("old"), self.old, old_p), (change.get("new"), self.new, new_p)):
                    for p in as_list(raw):
                        resolved = self.resolve_parameter(p, spec)
                        if resolved is None:
                            self.reconcile.append(f"`{cmd}`: a parameter the classifier can't resolve changed; check it by hand")
                            return
                        out[(resolved["in"], resolved["name"])] = resolved
                for key in sorted(set(old_p) | set(new_p)):
                    a, b = old_p.get(key), new_p.get(key)
                    if a == b:
                        continue
                    if key[0] in {"query", "path"}:
                        self.notes.append(self.parameter_note(cmd, key, a, b))
                    elif b is not None and b.get("required") and not (a or {}).get("required"):
                        self.reconcile.append(f"`{cmd}`: new required {key[0]} parameter `{ident(key[1])}`; only "
                                              f"hand-written code can send it")
                    elif b is None:
                        self.notes.append(f"`{cmd}`: the API removed the {key[0]} parameter `{ident(key[1])}`.")
                    else:
                        # Optional: the CLI doesn't send it, so the API uses its default.
                        self.ship_reasons.append(f"{cmd}: optional {key[0]} parameter {ident(key[1])} added or changed")
                        if a is None:
                            self.new_optional_headers.setdefault(key, []).append(cmd)
                return
            self.reconcile.append(f"`{cmd}`: unrecognized parameter change at `{ident(change['path'], 200)}`")
            return
        if head == "requestBody":
            if entry.get("flags"):
                self.reconcile.append(f"`{cmd}`: request body contract changed at `{ident(change['path'], 200)}`, and the CLI "
                                      f"builds it from flags")
            else:
                self.ship_reasons.append(f"{cmd}: pass-through request body changed at {ident(change['path'], 200)}")
            return
        self.reconcile.append(f"`{cmd}`: {head or 'operation'} changed ({change['kind']} at `{ident(change['path'], 200)}`)")

    def added_operation(self, method, path):
        route = (method.upper(), path)
        entry = (self.candidate or {}).get(route)
        if self.candidate is None:
            self.reconcile.append(f"new operation {route[0]} {ident(path, 200)}")
        elif not entry:
            self.reconcile.append(f"new operation {route[0]} {ident(path, 200)} can't get a generated command, so it needs a hand-written one")
        else:
            op = self.new["paths"][path][method.lower()]
            summary = value(op.get("summary") or "", 60)
            self.new_commands.append(entry["command"])
            self.notes.append(f"New command `syllable {entry['command']}`" + (f": {summary}" if summary else "")
                              + ". It's generated from the API spec; run it with `--help` for its arguments and request fields.")
            self.ship_reasons.append(f"new operation {route[0]} {ident(path, 200)} got a command (`{entry['command']}`)")

    def lost_commands(self):
        """A command main has that the candidate build doesn't, for an operation
        that still exists: a generated command that can no longer be generated."""
        if self.candidate is None:
            return
        paths = self.new.get("paths", {})
        for (method, path), entry in sorted(self.ops_by_route.items()):
            if method.lower() in paths.get(path, {}) and (method, path) not in self.candidate:
                self.reconcile.append(f"`{entry['command']}` ({method} {ident(path, 200)}) can no longer be generated, so it needs a hand-written command")

    def followups(self):
        """One follow-up per command whose flags or output may no longer fit
        the API, plus one per new optional header."""
        out = []
        for cmd, items in sorted(self.flag_checks.items()):
            out.append(f"`{cmd}` builds its request body from flags, and the API changed what it accepts: "
                       f"{'; '.join(items)}. Check that the flags and their defaults still produce a body the API "
                       f"accepts (`--dry-run` shows it), and fix them if not.")
        for cmd, items in sorted(self.output_checks.items()):
            out.append(f"`{cmd}` parses the API's response for its output, and the response changed: "
                       f"{'; '.join(items)}. Check its table and `get` output against the new schema, and fix the "
                       f"parsing if needed.")
        for (where, name), cmds in sorted(self.new_optional_headers.items()):
            out.append(f"The API accepts a new optional {where} parameter `{ident(name)}` on `{'`, `'.join(sorted(cmds))}`, "
                       f"which the CLI doesn't send. Add a flag for it if users need it, or close this.")
        return out

    def run(self, d):
        for p in d["added_paths"]:
            item = self.new["paths"][p]
            if "$ref" in item or "servers" in item:
                self.reconcile.append(f"new path {ident(p, 200)} uses a path-level `$ref` or `servers`, which generation doesn't support")
                continue
            for m in sorted(k for k in item if k in HTTP_METHODS):
                self.added_operation(m, p)
        for p in d["removed_paths"]:
            self.reconcile.append(f"removed path {ident(p, 200)}")
        for entry in d["changed_paths"]:
            for m in entry["added_methods"]:
                self.added_operation(m, entry["path"])
            for m in entry["removed_methods"]:
                self.reconcile.append(f"removed operation {m.upper()} {entry['path']}")
        self.lost_commands()
        for change in d["changed_global_contracts"]:
            self.reconcile.append(f"global spec change ({change['kind']} at `{ident(change['path'], 200)}`): check the CLI's "
                                  f"auth and server handling")
        for name in d["removed_schemas"]:
            self.notes.append(f"The API removed schema `{ident(name)}`, so `syllable schema get {ident(name)}` no longer finds it.")
        if d["added_schemas"]:
            self.ship_reasons.append(f"{len(d['added_schemas'])} new schema(s)")
        for e in d["changed_enums"]:
            if e["removed"]:
                self.enum_removed(e["enum"], e["removed"])
            if e["added"]:
                self.notes.append(f"`{ident(e['enum'])}` accepts new values: {', '.join(value(v) for v in e['added'])}.")
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
    verdict = "reconcile" if c.reconcile else "ship"
    delta = hashlib.sha256(json.dumps(d, sort_keys=True).encode()).hexdigest()[:16]
    notes = list(dict.fromkeys(c.notes))  # de-duplicate, keep order
    return {
        "has_changes": ds.has_changes(raw),
        "verdict": verdict,
        "reconcile": c.reconcile,
        "release_notes": notes,
        "followups": c.followups(),
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
    for key in ("reconcile", "release_notes", "followups", "ship_reasons"):
        for line in result[key]:
            print(f"  {key}: {line}")


if __name__ == "__main__":
    main()
