import copy
import contextlib
import io
import itertools
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

    def test_additive_enum_update_does_not_reidentify_union_branches(self):
        for keyword in ("anyOf", "oneOf", "allOf"):
            with self.subTest(keyword=keyword):
                old = document()
                old["components"]["schemas"]["Item"]["properties"]["value"] = {keyword: [
                    {"type": "string", "pattern": "^m$", "enum": ["m"]},
                    {"type": "string", "pattern": "^[az]$", "enum": ["z"]},
                ]}
                new = copy.deepcopy(old)
                new["components"]["schemas"]["Item"]["properties"]["value"][keyword][1]["enum"].append("a")
                result = diff_specs.compute_diff(old, new)
                self.assertTrue(diff_specs.has_changes(result))
                self.assertEqual(diff_specs.enum_removals(result), [])
                self.assertEqual([added for _, added, _ in result["enum_changes"]], [["a"]])

    def test_union_branch_insertions_and_reordering_are_additive(self):
        for keyword in ("anyOf", "oneOf", "allOf"):
            with self.subTest(keyword=keyword):
                old = document()
                old["components"]["schemas"]["Item"]["properties"]["value"] = {keyword: [
                    {"type": "string", "pattern": "^z$", "enum": ["z"]},
                    {"type": "null"},
                ]}
                new = copy.deepcopy(old)
                branches = new["components"]["schemas"]["Item"]["properties"]["value"][keyword]
                branches.reverse()
                branches.insert(0, {"type": "string", "pattern": "^a$", "enum": ["a"]})
                result = diff_specs.compute_diff(old, new)
                self.assertEqual(diff_specs.enum_removals(result), [])
                self.assertEqual([added for _, added, _ in result["enum_changes"]], [["a"]])

    def test_nested_anyof_oneof_enum_additions_survive_reordering(self):
        for outer, inner in (("anyOf", "oneOf"), ("oneOf", "anyOf")):
            with self.subTest(outer=outer, inner=inner):
                old = document()
                old["components"]["schemas"]["Item"]["properties"]["value"] = {outer: [
                    {"type": "array", "items": {inner: [
                        {"type": "string", "pattern": "^m$", "enum": ["m"]},
                        {"type": "string", "pattern": "^[az]$", "enum": ["z"]},
                    ]}},
                    {"type": "null"},
                ]}
                new = copy.deepcopy(old)
                branches = new["components"]["schemas"]["Item"]["properties"]["value"][outer]
                branches[0]["items"][inner][1]["enum"].append("a")
                branches[0]["items"][inner].reverse()
                branches.reverse()
                result = diff_specs.compute_diff(old, new)
                self.assertEqual(diff_specs.enum_removals(result), [])
                self.assertEqual([added for _, added, _ in result["enum_changes"]], [["a"]])

    def test_nested_union_branch_addition_keeps_outer_branch_identity(self):
        old = document()
        old["components"]["schemas"]["Item"]["properties"]["value"] = {"oneOf": [
            {"type": "object", "properties": {"status": {"anyOf": [
                {"type": "string", "pattern": "^z$", "enum": ["z"]},
            ]}}},
            {"type": "null"},
        ]}
        new = copy.deepcopy(old)
        new["components"]["schemas"]["Item"]["properties"]["value"]["oneOf"][0]["properties"]["status"]["anyOf"].append(
            {"type": "string", "pattern": "^a$", "enum": ["a"]})
        result = diff_specs.compute_diff(old, new)
        self.assertEqual(diff_specs.enum_removals(result), [])
        self.assertEqual([added for _, added, _ in result["enum_changes"]], [["a"]])

    def test_optional_property_addition_preserves_union_branch_enums(self):
        old = document()
        old["components"]["schemas"]["Item"] = {"anyOf": [
            {"type": "object", "properties": {"mode": {"type": "string", "enum": ["a"]}}},
            {"type": "null"},
        ]}
        new = copy.deepcopy(old)
        new["components"]["schemas"]["Item"]["anyOf"][0]["properties"]["extra"] = {"type": "string"}
        result = diff_specs.compute_diff(old, new)
        self.assertTrue(result["changed_schema_contracts"])
        self.assertEqual(result["enum_changes"], [])

    def test_new_optional_const_and_enum_properties_are_additive(self):
        for extra in ({"type": "string", "const": "fixed"}, {"type": "string", "enum": ["new"]}):
            with self.subTest(extra=extra):
                old = document()
                old["components"]["schemas"]["Item"] = {"oneOf": [
                    {"type": "object", "properties": {"mode": {"type": "string", "enum": ["a"]}}},
                    {"type": "null"},
                ]}
                new = copy.deepcopy(old)
                new["components"]["schemas"]["Item"]["oneOf"][0]["properties"]["extra"] = extra
                result = diff_specs.compute_diff(old, new)
                self.assertEqual(diff_specs.enum_removals(result), [])
                self.assertEqual([added for _, added, _ in result["enum_changes"]], [["new"]] if "enum" in extra else [])

    def test_constraint_and_default_changes_keep_enum_association(self):
        for change in ({"minLength": 1}, {"pattern": "^a$"}, {"default": "a"}):
            with self.subTest(change=change):
                old = document()
                old["components"]["schemas"]["Item"] = {"anyOf": [
                    {"type": "object", "properties": {"mode": {"type": "string", "enum": ["a"]}}},
                    {"type": "null"},
                ]}
                new = copy.deepcopy(old)
                new["components"]["schemas"]["Item"]["anyOf"][0]["properties"]["mode"].update(change)
                result = diff_specs.compute_diff(old, new)
                self.assertTrue(result["changed_schema_contracts"])
                self.assertEqual(result["enum_changes"], [])

    def test_removed_discriminator_branch_is_not_matched_to_changed_survivor(self):
        old = document()
        old["components"]["schemas"]["Item"] = {"oneOf": [
            {"type": "object", "properties": {"kind": {"const": "left"}, "mode": {"enum": ["a"]}}},
            {"type": "object", "properties": {"kind": {"const": "right"}, "mode": {"enum": ["a"]}}},
        ]}
        new = copy.deepcopy(old)
        branches = new["components"]["schemas"]["Item"]["oneOf"]
        del branches[0]
        branches[0]["properties"]["extra"] = {"type": "string"}
        result = diff_specs.compute_diff(old, new)
        self.assertEqual(result["enum_changes"], [("Item.mode", [], ["a"])])

    def test_removed_constraint_branch_is_not_hidden_by_changed_survivor(self):
        old = document()
        old["components"]["schemas"]["Item"] = {"anyOf": [
            {"type": "object", "properties": {"mode": {"pattern": "^red$", "enum": ["red"]}}},
            {"type": "object", "properties": {"mode": {"pattern": "^(red|green)$", "enum": ["red", "green"]}}},
        ]}
        new = copy.deepcopy(old)
        branches = new["components"]["schemas"]["Item"]["anyOf"]
        del branches[0]
        branches[0]["properties"]["extra"] = {"type": "string"}
        result = diff_specs.compute_diff(old, new)
        self.assertEqual(result["enum_changes"], [("Item.mode", [], ["red"])])

    def test_changed_shapes_with_shared_profiles_stay_one_to_one(self):
        old = document()
        old["components"]["schemas"]["Item"] = {"anyOf": [
            {"type": "object", "properties": {"mode": {"pattern": "^a$", "enum": ["a"]}}},
            {"type": "object", "properties": {"mode": {"pattern": "^[ab]$", "enum": ["a"]}}},
        ]}
        new = copy.deepcopy(old)
        for branch in new["components"]["schemas"]["Item"]["anyOf"]:
            branch["properties"]["extra"] = {"type": "string"}
        result = diff_specs.compute_diff(old, new)
        self.assertTrue(result["changed_schema_contracts"])
        self.assertEqual(result["enum_changes"], [])

    def test_reference_identity_survives_shape_changes_and_shared_enum_values(self):
        old = document()
        old["components"]["schemas"]["Item"] = {"anyOf": [
            {"$ref": "#/components/schemas/Left", "type": "string", "enum": ["a"]},
            {"$ref": "#/components/schemas/Right", "type": "string", "enum": ["a"]},
        ]}
        new = copy.deepcopy(old)
        branches = new["components"]["schemas"]["Item"]["anyOf"]
        del branches[0]
        branches[0]["minLength"] = 1
        result = diff_specs.compute_diff(old, new)
        self.assertEqual(result["enum_changes"], [("Item", [], ["a"])])

    def test_enum_location_move_is_reported_despite_shared_values(self):
        old = document()
        old["components"]["schemas"]["Item"] = {"anyOf": [
            {"type": "object", "properties": {"mode": {"enum": ["a"]}}},
            {"type": "null"},
        ]}
        new = copy.deepcopy(old)
        properties = new["components"]["schemas"]["Item"]["anyOf"][0]["properties"]
        properties["other"] = properties.pop("mode")
        result = diff_specs.compute_diff(old, new)
        self.assertEqual(diff_specs.enum_removals(result), [("Item.mode", [], ["a"])])
        self.assertIn(("Item.other", ["a"], []), result["enum_changes"])

    def test_duplicate_shapes_use_one_to_one_additive_matching(self):
        before = [{"type": "string", "enum": ["a", "b"]}, {"type": "string", "enum": ["b", "c"]}]
        after = [{"type": "string", "enum": ["a", "b", "c"]}, {"type": "string", "enum": ["a", "b", "d"]}]
        for keyword, old_order, new_order in itertools.product(
                ("anyOf", "oneOf", "allOf"), itertools.permutations(before), itertools.permutations(after)):
            with self.subTest(keyword=keyword, old=old_order, new=new_order):
                old, new = document(), document()
                old["components"]["schemas"]["Item"]["properties"]["value"] = {keyword: list(old_order)}
                new["components"]["schemas"]["Item"]["properties"]["value"] = {keyword: list(new_order)}
                result = diff_specs.compute_diff(old, new)
                self.assertTrue(result["enum_changes"])
                self.assertEqual(diff_specs.enum_removals(result), [])

    def test_value_retained_elsewhere_does_not_hide_branch_removal(self):
        for keyword in ("anyOf", "oneOf", "allOf"):
            with self.subTest(keyword=keyword):
                old = document()
                old["components"]["schemas"]["Item"]["properties"]["value"] = {keyword: [
                    {"type": "string", "pattern": "^(blue|red)$", "enum": ["blue", "red"]},
                    {"type": "string", "pattern": "^(amber|green|red)$", "enum": ["green", "red"]},
                ]}
                new = copy.deepcopy(old)
                branches = new["components"]["schemas"]["Item"]["properties"]["value"][keyword]
                branches[0]["enum"].remove("red")
                branches[1]["enum"].append("amber")
                branches.reverse()
                result = diff_specs.compute_diff(old, new)
                self.assertEqual([removed for _, _, removed in diff_specs.enum_removals(result)], [["red"]])
                self.assertEqual([added for _, added, _ in result["enum_changes"] if added], [["amber"]])

    def test_removing_distinct_branch_is_reported_with_shared_enum_values(self):
        old = document()
        old["components"]["schemas"]["Item"]["properties"]["value"] = {"oneOf": [
            {"type": "object", "properties": {"kind": {"const": "left"}, "status": {"enum": ["red"]}}},
            {"type": "object", "properties": {"kind": {"const": "right"}, "status": {"enum": ["red"]}}},
        ]}
        new = copy.deepcopy(old)
        del new["components"]["schemas"]["Item"]["properties"]["value"]["oneOf"][0]
        result = diff_specs.compute_diff(old, new)
        self.assertEqual([removed for _, _, removed in diff_specs.enum_removals(result)], [["red"]])

    def test_query_insertion_and_nested_enum_addition_keep_parameter_identity(self):
        old = document()
        old["paths"]["/items"]["get"]["parameters"] = [{"name": "status", "in": "query", "schema": {"oneOf": [
            {"type": "string", "pattern": "^m$", "enum": ["m"]},
            {"type": "string", "pattern": "^[az]$", "enum": ["z"]},
        ]}}]
        new = copy.deepcopy(old)
        parameters = new["paths"]["/items"]["get"]["parameters"]
        parameters[0]["schema"]["oneOf"][1]["enum"].append("a")
        parameters.insert(0, {"name": "earlier", "in": "query", "schema": {"type": "integer"}})
        result = diff_specs.compute_diff(old, new)
        self.assertEqual(diff_specs.enum_removals(result), [])
        self.assertEqual([added for _, added, _ in result["enum_changes"]], [["a"]])

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
