import copy
import contextlib
import io
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import diff_specs


def document():
    return {
        "paths": {"/items": {"get": {
            "parameters": [],
            "responses": {"200": {"content": {"application/json": {
                "schema": {"$ref": "#/components/schemas/Item"}
            }}}},
        }}},
        "components": {"schemas": {"Item": {
            "type": "object", "properties": {
                "value": {"anyOf": [{"type": "string"}, {"type": "null"}]},
                "mode": {"type": "string", "enum": ["a", "b"]},
            }, "required": ["value", "mode"],
        }}},
    }


class ContractDiffTests(unittest.TestCase):
    def assert_detected(self, mutate):
        old = document()
        new = copy.deepcopy(old)
        mutate(new)
        result = diff_specs.compute_diff(old, new)
        self.assertTrue(diff_specs.has_changes(result))
        self.assertTrue(result["changed_operation_contracts"] or result["changed_schema_contracts"])
        return result

    def test_new_query_parameter_without_method_change(self):
        result = self.assert_detected(lambda d: d["paths"]["/items"]["get"]["parameters"].append({
            "name": "selected", "in": "query", "schema": {"type": "string"}
        }))
        self.assertEqual(result["changed_paths"], [])
        self.assertEqual(len(result["changed_operation_contracts"]), 1)

    def test_server_change(self):
        old=document()
        new=copy.deepcopy(old)
        new["servers"]=[{"url":"https://api.example.invalid"}]
        result=diff_specs.compute_diff(old,new)
        self.assertTrue(diff_specs.has_changes(result))
        self.assertTrue(result["changed_global_contracts"])

    def test_security_header_change(self):
        old=document()
        old["components"]["securitySchemes"]={"APIKey":{"type":"apiKey","in":"header","name":"Old-Key"}}
        new=copy.deepcopy(old)
        new["components"]["securitySchemes"]["APIKey"]["name"]="New-Key"
        result=diff_specs.compute_diff(old,new)
        self.assertTrue(diff_specs.has_changes(result))
        self.assertTrue(result["changed_global_contracts"])

    def test_response_schema_reference_change(self):
        def mutate(d):
            d["paths"]["/items"]["get"]["responses"]["200"]["content"]["application/json"]["schema"]["$ref"] = "#/components/schemas/Other"
        self.assert_detected(mutate)

    def test_request_body_change(self):
        def mutate(d):
            d["paths"]["/items"]["get"]["requestBody"] = {"required": True, "content": {
                "application/json": {"schema": {"$ref": "#/components/schemas/Item"}}
            }}
        self.assert_detected(mutate)

    def test_inherited_parameter_change(self):
        self.assert_detected(lambda d: d["paths"]["/items"].update(parameters=[{
            "name": "status", "in": "query", "schema": {"type": "string"}
        }]))

    def test_constraints_inside_union(self):
        for constraint, value in (("pattern", "^[0-9]+$"), ("minLength", 1), ("format", "date")):
            with self.subTest(constraint=constraint):
                result = self.assert_detected(lambda d: d["components"]["schemas"]["Item"]["properties"]["value"]["anyOf"][0].update({constraint: value}))
                # The shallow field view is unchanged; the deep view must detect it.
                self.assertEqual(result["changed_schema_summary"], [])

    def test_default_change(self):
        self.assert_detected(lambda d: d["components"]["schemas"]["Item"]["properties"]["mode"].update(default="a"))

    def test_required_field_change(self):
        self.assert_detected(lambda d: d["components"]["schemas"]["Item"]["required"].append("extra"))

    def test_nested_array_item_change(self):
        old = document()
        old["components"]["schemas"]["Item"]["properties"]["items"] = {"type": "array", "items": {"type": "integer"}}
        new = copy.deepcopy(old)
        new["components"]["schemas"]["Item"]["properties"]["items"]["items"]["minimum"] = 1
        self.assertTrue(diff_specs.has_changes(diff_specs.compute_diff(old, new)))

    def test_nullable_type_change(self):
        self.assert_detected(lambda d: d["components"]["schemas"]["Item"]["properties"]["value"].update(anyOf=[{"type": "string"}]))

    def test_prose_and_semantic_reordering_are_ignored(self):
        old = document()
        new = copy.deepcopy(old)
        schema = new["components"]["schemas"]["Item"]
        schema.update(description="Updated docs", title="Renamed display", examples=[{"mode": "a"}])
        schema["required"].reverse()
        schema["properties"]["mode"]["enum"].reverse()
        schema["properties"]["value"]["anyOf"].reverse()
        new["paths"]["/items"]["get"].update(summary="New summary", description="New prose")
        self.assertFalse(diff_specs.has_changes(diff_specs.compute_diff(old, new)))

    def test_default_array_order_is_meaningful(self):
        self.assertNotEqual(diff_specs.contract({"default": [1, 2]}), diff_specs.contract({"default": [2, 1]}))

    def test_property_named_description_is_not_an_annotation(self):
        old = document()
        old["components"]["schemas"]["Item"]["properties"]["description"] = {"type":"string","minLength":1}
        new = copy.deepcopy(old)
        new["components"]["schemas"]["Item"]["properties"]["description"]["minLength"] = 2
        result = diff_specs.compute_diff(old,new)
        self.assertTrue(result["changed_schema_contracts"])

    def test_default_object_fields_are_data(self):
        before = {"default":{"description":"before"}}
        after = {"default":{"description":"after"}}
        self.assertNotEqual(diff_specs.contract(before),diff_specs.contract(after))
        self.assertNotEqual(diff_specs.contract({"enum":[before]}),diff_specs.contract({"enum":[after]}))

    def test_union_reordering_with_enum_is_ignored(self):
        old = document()
        old["components"]["schemas"]["Item"]["properties"]["value"]["anyOf"][0]["enum"] = ["a", "b"]
        new = copy.deepcopy(old)
        new["components"]["schemas"]["Item"]["properties"]["value"]["anyOf"].reverse()
        self.assertFalse(diff_specs.has_changes(diff_specs.compute_diff(old, new)))

    def test_query_insertion_does_not_invent_enum_removal(self):
        old = document()
        old["paths"]["/items"]["get"]["parameters"] = [{"name": "status", "in": "query", "schema": {"enum": ["active", "deleted"]}}]
        new = copy.deepcopy(old)
        new["paths"]["/items"]["get"]["parameters"].insert(0, {"name": "earlier", "in": "query", "schema": {"type": "string"}})
        result = diff_specs.compute_diff(old, new)
        self.assertTrue(diff_specs.has_changes(result))
        self.assertEqual(result["enum_changes"], [])

    def test_json_report_includes_deep_changes(self):
        result = self.assert_detected(lambda d: d["components"]["schemas"]["Item"]["properties"]["mode"].update(default="a"))
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            diff_specs.render_json(result)
        report = json.loads(out.getvalue())
        self.assertEqual(report["summary"]["schemas"]["contract_changed"], 1)
        self.assertTrue(report["changed_schema_contracts"])

    def test_exit_code_for_previously_missed_change(self):
        old = document()
        new = copy.deepcopy(old)
        new["components"]["schemas"]["Item"]["properties"]["value"]["anyOf"][0]["pattern"] = "^x$"
        with tempfile.TemporaryDirectory() as directory:
            paths = [Path(directory) / name for name in ("old.json", "new.json")]
            for path, value in zip(paths, (old, new)):
                path.write_text(json.dumps(value))
            run = subprocess.run([sys.executable, str(Path(diff_specs.__file__)), "--exit-code", "--format", "json", *map(str, paths)], capture_output=True, text=True)
            self.assertEqual(run.returncode, 1, run.stderr)
            self.assertTrue(json.loads(run.stdout)["has_changes"])


if __name__ == "__main__":
    unittest.main()
