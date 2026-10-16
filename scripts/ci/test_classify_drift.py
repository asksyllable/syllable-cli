"""Regression tests for classify_drift.py on small synthetic specs.

Run: python3 -m unittest discover -s scripts/ci -p 'test_*.py'
"""
import copy
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from classify_drift import classify  # noqa: E402

BASE = {
    "openapi": "3.1.0",
    "paths": {
        "/api/v1/widgets/": {
            "get": {"operationId": "widgets_list", "parameters": [
                {"name": "page", "in": "query", "required": False, "schema": {"type": "integer"}}],
                "responses": {"200": {"description": "ok"}}},
            "post": {"operationId": "widgets_create",
                     "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/WidgetCreate"}}}},
                     "responses": {"200": {"description": "ok"}}},
        },
        "/api/v1/gadgets/": {
            "post": {"operationId": "gadgets_create",
                     "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/GadgetCreate"}}}},
                     "responses": {"200": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Gadget"}}}}}},
        },
    },
    "components": {"schemas": {
        "WidgetCreate": {"type": "object", "required": ["name"], "properties": {
            "name": {"type": "string"},
            "size": {"type": "integer"},
            "color": {"type": "string", "enum": ["red", "blue"]},
        }},
        "GadgetCreate": {"type": "object", "required": ["name"], "properties": {
            "name": {"type": "string"}, "parts": {"type": "array", "items": {"$ref": "#/components/schemas/Part"}}}},
        "Part": {"type": "object", "properties": {"weight": {"type": "integer"}}},
        "Gadget": {"type": "object", "properties": {"id": {"type": "integer"}, "label": {"type": "string"}}},
    }},
}

# widgets create builds its body from inline flags; gadgets create is --file only.
OPS = {
    "widgets_list": {"command": "widgets list", "method": "GET", "path": "/api/v1/widgets/", "flags": ["search"]},
    "widgets_create": {"command": "widgets create", "method": "POST", "path": "/api/v1/widgets/", "flags": ["name", "size"]},
    "gadgets_create": {"command": "gadgets create", "method": "POST", "path": "/api/v1/gadgets/", "flags": []},
}


def changed(mutate):
    new = copy.deepcopy(BASE)
    mutate(new)
    return classify(BASE, new, OPS)


S = lambda spec, name: spec["components"]["schemas"][name]


class Ship(unittest.TestCase):
    def test_no_change(self):
        self.assertFalse(classify(BASE, copy.deepcopy(BASE), OPS)["has_changes"])

    def test_new_optional_field(self):
        r = changed(lambda s: S(s, "WidgetCreate")["properties"].update(shape={"type": "string"}))
        self.assertEqual(r["verdict"], "ship")
        self.assertIn("`WidgetCreate.shape` is a new field.", r["release_notes"])

    def test_deprecation(self):
        r = changed(lambda s: S(s, "WidgetCreate")["properties"]["size"].update(deprecated=True))
        self.assertEqual(r["verdict"], "ship")
        self.assertIn("`WidgetCreate.size` is now deprecated.", r["release_notes"])

    def test_new_enum_value(self):
        r = changed(lambda s: S(s, "WidgetCreate")["properties"]["color"]["enum"].append("green"))
        self.assertEqual(r["verdict"], "ship")

    def test_tightening_pass_through_body(self):  # like #194
        r = changed(lambda s: S(s, "GadgetCreate")["required"].append("parts"))
        self.assertEqual(r["verdict"], "ship", r)
        self.assertTrue(any("gadgets create" in n and "required" in n for n in r["release_notes"]))

    def test_tightening_nested_pass_through_schema(self):  # like DaoExperimentVariantWrite
        r = changed(lambda s: S(s, "Part")["properties"]["weight"].update(minimum=1))
        self.assertEqual(r["verdict"], "ship", r)

    def test_response_only_tightening(self):
        r = changed(lambda s: S(s, "Gadget")["properties"]["label"].update(minLength=1))
        self.assertEqual(r["verdict"], "ship", r)

    def test_widened_type(self):  # like #190
        r = changed(lambda s: S(s, "GadgetCreate")["properties"]["name"].update(type=["string", "null"]))
        self.assertEqual(r["verdict"], "ship", r)

    def test_new_optional_query_parameter(self):
        r = changed(lambda s: s["paths"]["/api/v1/widgets/"]["get"]["parameters"].append(
            {"name": "sort", "in": "query", "required": False, "schema": {"type": "string"}}))
        self.assertEqual(r["verdict"], "ship", r)

    def test_new_schema(self):
        r = changed(lambda s: s["components"]["schemas"].update(Doohickey={"type": "object"}))
        self.assertEqual(r["verdict"], "ship", r)


class Responses(unittest.TestCase):
    def route(self, s):
        return s["paths"]["/api/v1/gadgets/"]["post"]["responses"]

    def test_new_error_response_ships(self):
        r = changed(lambda s: self.route(s).update({"502": {"description": "bad gateway"}}))
        self.assertEqual(r["verdict"], "ship", r)

    def test_json_to_binary_needs_decision(self):  # the case Codex found
        def mutate(s):
            self.route(s)["200"]["content"] = {"application/octet-stream": {"schema": {"type": "string", "format": "binary"}}}
        self.assertEqual(changed(mutate)["verdict"], "decision")

    def test_extra_success_media_type_needs_decision(self):
        def mutate(s):
            self.route(s)["200"]["content"]["text/csv"] = {"schema": {"type": "string"}}
        self.assertEqual(changed(mutate)["verdict"], "decision")

    def test_removed_success_response_needs_decision(self):
        self.assertEqual(changed(lambda s: self.route(s).pop("200"))["verdict"], "decision")


class Decision(unittest.TestCase):
    def test_tightening_inline_body(self):
        r = changed(lambda s: S(s, "WidgetCreate")["required"].append("size"))
        self.assertEqual(r["verdict"], "decision", r)
        self.assertTrue(any("widgets create" in d for d in r["decisions"]))

    def test_removed_field(self):
        r = changed(lambda s: S(s, "Gadget")["properties"].pop("label"))
        self.assertEqual(r["verdict"], "decision")

    def test_removed_enum_value(self):  # like AgentSttProvider in #177
        r = changed(lambda s: S(s, "WidgetCreate")["properties"]["color"]["enum"].remove("blue"))
        self.assertEqual(r["verdict"], "decision")
        self.assertEqual(len(r["decisions"]), 1, r["decisions"])

    def test_removed_schema(self):
        r = changed(lambda s: s["components"]["schemas"].pop("Gadget"))
        self.assertEqual(r["verdict"], "decision")

    def test_new_required_query_parameter(self):
        r = changed(lambda s: s["paths"]["/api/v1/widgets/"]["get"]["parameters"].append(
            {"name": "tenant", "in": "query", "required": True, "schema": {"type": "string"}}))
        self.assertEqual(r["verdict"], "decision", r)

    def test_removed_query_parameter(self):
        r = changed(lambda s: s["paths"]["/api/v1/widgets/"]["get"]["parameters"].clear())
        self.assertEqual(r["verdict"], "decision", r)

    def test_security_change(self):
        r = changed(lambda s: s.update(security=[{"ApiKey": []}]))
        self.assertEqual(r["verdict"], "decision", r)


class Reconcile(unittest.TestCase):
    def test_new_path(self):
        r = changed(lambda s: s["paths"].update({"/api/v1/sprockets/": {"get": {"operationId": "sprockets_list", "responses": {}}}}))
        self.assertEqual(r["verdict"], "reconcile")

    def test_new_method(self):
        r = changed(lambda s: s["paths"]["/api/v1/gadgets/"].update(get={"operationId": "gadgets_list", "responses": {}}))
        self.assertEqual(r["verdict"], "reconcile")

    def test_removed_path_outranks_decisions(self):
        def mutate(s):
            s["paths"].pop("/api/v1/gadgets/")
            S(s, "Gadget")["properties"].pop("label")
        self.assertEqual(changed(mutate)["verdict"], "reconcile")


class DeltaBinding(unittest.TestCase):
    def test_delta_identifies_the_change(self):
        a = changed(lambda s: S(s, "WidgetCreate")["required"].append("size"))
        b = changed(lambda s: S(s, "WidgetCreate")["required"].append("size"))
        c = changed(lambda s: S(s, "WidgetCreate")["required"].append("color"))
        self.assertEqual(a["delta"], b["delta"])
        self.assertNotEqual(a["delta"], c["delta"])


if __name__ == "__main__":
    unittest.main()
