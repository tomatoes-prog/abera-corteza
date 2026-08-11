#!/usr/bin/env python3

# Copyright 2026 Abera/Corteza contributors
# Licensed under the Apache License, Version 2.0.

"""Validate demo datasets, explicit layouts and enabled workflows."""

import csv
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
TEMPLATE_IDS = [
    line.strip()
    for line in (ROOT / "demos" / "showroom-co" / "templates.list").read_text(encoding="utf-8").splitlines()
    if line.strip() and not line.lstrip().startswith("#")
]


def walk_pages(pages):
    for handle, page in pages.items():
        yield handle, page
        yield from walk_pages(page.get("children", {}))


def parse_multi_value(value):
    if not value:
        return []
    if value.startswith("[") and value.endswith("]"):
        return [item.strip() for item in value[1:-1].split(",") if item.strip()]
    return [value]


def main() -> None:
    for template_id in TEMPLATE_IDS:
        root = ROOT / "templates" / template_id
        modules = yaml.safe_load((root / "provision" / "modules.yaml").read_text(encoding="utf-8"))["modules"]
        pages = yaml.safe_load((root / "provision" / "pages.yaml").read_text(encoding="utf-8"))["pages"]
        workflows = yaml.safe_load((root / "provision" / "workflows.yaml").read_text(encoding="utf-8"))["workflows"]
        identities = yaml.safe_load((root / "provision" / "roles.yaml").read_text(encoding="utf-8"))
        permissions = yaml.safe_load((root / "provision" / "permissions.yaml").read_text(encoding="utf-8"))["allow"]
        manifest = yaml.safe_load((root / "manifest.yaml").read_text(encoding="utf-8"))
        records = yaml.safe_load((root / "demo" / "provision" / "records.yaml").read_text(encoding="utf-8"))[
            "record_sources"
        ]

        automation_users = {
            handle: user
            for handle, user in identities.get("users", {}).items()
            if user.get("kind") == "sys"
        }
        assert len(automation_users) == 1, (template_id, "expected one system automation user")
        automation_handle = next(iter(automation_users))
        assert automation_handle in identities["roles"], (template_id, "automation role missing")
        assert automation_handle in permissions, (template_id, "automation permissions missing")
        assert automation_handle in manifest["resources"].get("users", []), (
            template_id,
            "automation user missing from manifest",
        )

        sources_by_module = {source["references"]["module"]: source for source in records}
        assert set(sources_by_module) == set(modules), (
            template_id,
            "every module must have exactly one demo datasource",
        )

        rows_by_module = {}
        for module_handle, source in sources_by_module.items():
            csv_path = root / "demo" / "provision" / source["source"]
            with csv_path.open(encoding="utf-8", newline="") as stream:
                reader = csv.DictReader(stream)
                rows = list(reader)
                unknown_columns = set(reader.fieldnames or []) - {"ID"} - set(modules[module_handle]["fields"])
            assert not unknown_columns, (template_id, module_handle, "unknown CSV columns", unknown_columns)
            assert 1 <= len(rows) <= 100, (template_id, module_handle, len(rows))
            assert len({row["ID"] for row in rows}) == len(rows), (
                template_id,
                module_handle,
                "duplicate synthetic IDs",
            )
            rows_by_module[module_handle] = rows

        ids_by_module = {
            module_handle: {row["ID"] for row in rows}
            for module_handle, rows in rows_by_module.items()
        }

        for module_handle, module in modules.items():
            for field_handle, field in module["fields"].items():
                if field["kind"] != "Select":
                    continue
                allowed = {
                    str(option["value"])
                    for option in field.get("options", {}).get("options", [])
                }
                for row in rows_by_module[module_handle]:
                    for value in parse_multi_value(row.get(field_handle, "")):
                        assert value in allowed, (
                            template_id,
                            module_handle,
                            row["ID"],
                            field_handle,
                            value,
                            "invalid Select value",
                        )

        relationship_count = 0
        for module_handle, source in sources_by_module.items():
            for reference, target_module in source["references"].items():
                if not reference.endswith(".module"):
                    continue
                field_handle = reference.removesuffix(".module")
                populated = 0
                for row in rows_by_module[module_handle]:
                    values = parse_multi_value(row.get(field_handle, ""))
                    populated += len(values)
                    for value in values:
                        assert value in ids_by_module[target_module], (
                            template_id,
                            module_handle,
                            row["ID"],
                            field_handle,
                            value,
                            "broken record reference",
                        )
                assert populated > 0, (
                    template_id,
                    module_handle,
                    field_handle,
                    "relationship has no demo values",
                )
                relationship_count += populated
        assert relationship_count > 0, (template_id, "demo has no populated relationships")

        page_count = 0
        for handle, page in walk_pages(pages):
            page_count += 1
            layouts = page.get("page_layouts", {})
            assert len(layouts) == 1, (template_id, handle, "missing explicit layout")
            layout = next(iter(layouts.values()))
            block_ids = [block["blockID"] for block in page.get("blocks", [])]
            layout_ids = [block["blockID"] for block in layout.get("blocks", [])]
            assert block_ids == layout_ids, (template_id, handle, "layout/block mismatch")
            layout_positions = {block["blockID"]: block["xywh"] for block in layout.get("blocks", [])}
            for block in page.get("blocks", []):
                position = layout_positions[block["blockID"]]
                assert len(position) == 4 and all(isinstance(value, int) for value in position), (
                    template_id,
                    handle,
                    block["blockID"],
                    "invalid block position",
                )
                x, y, width, height = position
                assert x >= 0 and y >= 0 and width > 0 and height >= 5 and x + width <= 48, (
                    template_id,
                    handle,
                    block["blockID"],
                    position,
                    "block outside 48-column grid",
                )
                if block.get("kind") == "Chart":
                    assert height >= 30, (template_id, handle, block["blockID"], position, "chart too short")
        assert page_count >= len(modules), (template_id, "not enough pages")

        assert workflows, (template_id, "missing workflows")
        for handle, workflow in workflows.items():
            assert workflow.get("enabled") is True, (template_id, handle, "workflow disabled")
            assert workflow.get("steps"), (template_id, handle, "workflow without steps")
            assert workflow.get("paths"), (template_id, handle, "workflow without paths")
            step_refs = {step.get("ref") for step in workflow["steps"]}
            event_types = {trigger.get("eventType") for trigger in workflow.get("triggers", [])}
            if "rolesEachMember" in step_refs or event_types & {"onInterval", "onTimestamp"}:
                assert workflow.get("runAs") == automation_handle, (
                    template_id,
                    handle,
                    "privileged or scheduled workflow must use the template automation identity",
                )

        if template_id == "soporte-renovaciones-co":
            renewal_workflow = workflows["preparar-renovacion-contrato"]
            assert {trigger["eventType"] for trigger in renewal_workflow["triggers"]} == {"onInterval"}
            assert any(
                step.get("ref") == "composeRecordsSearch"
                for step in renewal_workflow["steps"]
            ), (template_id, "renewal scheduler must prevent duplicate renewals")
            sla_workflow = workflows["actualizar-estado-sla"]
            sla_paths = " ".join(path.get("expr", "") for path in sla_workflow["paths"])
            assert "umbralResolucionSLA" in sla_paths and "vencimientoResolucion" in sla_paths

        print(
            f"{template_id}: {len(modules)} módulos con datos, "
            f"{relationship_count} relaciones, {page_count} páginas con layout "
            f"y {len(workflows)} workflows habilitados"
        )


if __name__ == "__main__":
    main()
