#!/usr/bin/env python3

# Copyright 2026 Abera/Corteza contributors
# Licensed under the Apache License, Version 2.0.

"""Validate demo datasets, explicit layouts and enabled workflows."""

import csv
import json
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
TEMPLATE_IDS = [
    line.strip()
    for line in (ROOT / "demos" / "showroom-co" / "templates.list").read_text(encoding="utf-8").splitlines()
    if line.strip() and not line.lstrip().startswith("#")
]
MAIN_MODULES = {
    "inmobiliaria-co": {"leads", "inmuebles", "citas", "actividades", "negociaciones"},
    "automotriz-co": {"prospectos", "vehiculos", "pruebas-manejo", "oportunidades", "ordenes-servicio"},
    "admisiones-educativas-co": {"prospectos", "programas", "solicitudes", "citas-admision", "matriculas"},
    "servicios-tecnicos-co": {"clientes-prospectos", "activos", "solicitudes", "cotizaciones", "ordenes-trabajo"},
    "centro-contacto-co": {"contactos-leads", "campanas", "registros-campana", "llamadas", "oportunidades"},
    "soporte-renovaciones-co": {"clientes", "contratos", "casos", "interacciones", "renovaciones"},
}
EXPECTED_AUXILIARY_MODULES = {
    "inmobiliaria-co": set(),
    "automotriz-co": {"actividades-comerciales", "tareas-servicio"},
    "admisiones-educativas-co": {"requisitos-solicitud", "actividades-admision"},
    "servicios-tecnicos-co": {"items-cotizacion", "tareas-orden"},
    "centro-contacto-co": set(),
    "soporte-renovaciones-co": {"problemas-conocidos"},
}
EXPECTED_MAPS = {"inmobiliaria-co": 1, "servicios-tecnicos-co": 1}
EXPECTED_PERSONAL_QUEUES = {"centro-contacto-co", "soporte-renovaciones-co"}


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


def minimum_height(block):
    kind = block.get("kind")
    minimum = {
        "Metric": 18,
        "Chart": 30,
        "RecordList": 38,
        "Calendar": 44,
        "Geometry": 44,
        "RecordOrganizer": 44,
        "Report": 44,
        "Record": 30,
        "Content": 12,
        "Navigation": 12,
    }.get(kind, 24)
    if kind == "Record":
        minimum = max(minimum, 10 + len(block.get("options", {}).get("fields", [])) * 5)
    return minimum


def rectangles_overlap(left, right):
    lx, ly, lw, lh = left
    rx, ry, rw, rh = right
    return lx < rx + rw and rx < lx + lw and ly < ry + rh and ry < ly + lh


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
        assert set(modules) - MAIN_MODULES[template_id] == EXPECTED_AUXILIARY_MODULES[template_id], (
            template_id,
            "unexpected auxiliary module set",
        )

        rows_by_module = {}
        for module_handle, source in sources_by_module.items():
            csv_path = root / "demo" / "provision" / source["source"]
            with csv_path.open(encoding="utf-8", newline="") as stream:
                reader = csv.DictReader(stream)
                rows = list(reader)
                unknown_columns = set(reader.fieldnames or []) - {"ID"} - set(modules[module_handle]["fields"])
            assert not unknown_columns, (template_id, module_handle, "unknown CSV columns", unknown_columns)
            if module_handle in MAIN_MODULES[template_id]:
                assert len(rows) == 100, (template_id, module_handle, "principal module must have 100 rows")
            else:
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
                if field["kind"] == "Geometry":
                    for record in rows_by_module[module_handle]:
                        value = record.get(field_handle, "")
                        if not value:
                            continue
                        point = json.loads(value)
                        assert set(point) == {"coordinates"}, (
                            template_id, module_handle, record["ID"], field_handle, "invalid Geometry object"
                        )
                        assert len(point["coordinates"]) == 2 and all(
                            isinstance(coordinate, (int, float)) for coordinate in point["coordinates"]
                        ), (template_id, module_handle, record["ID"], field_handle, "invalid Geometry point")
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
        metric_count = 0
        map_count = 0
        organizer_count = 0
        page_handles = set()
        for handle, page in walk_pages(pages):
            page_count += 1
            page_handles.add(handle)
            layouts = page.get("page_layouts", {})
            assert len(layouts) == 1, (template_id, handle, "missing explicit layout")
            layout = next(iter(layouts.values()))
            block_ids = [block["blockID"] for block in page.get("blocks", [])]
            layout_ids = [block["blockID"] for block in layout.get("blocks", [])]
            assert block_ids == layout_ids, (template_id, handle, "layout/block mismatch")
            layout_positions = {block["blockID"]: block["xywh"] for block in layout.get("blocks", [])}
            rectangles = []
            for block in page.get("blocks", []):
                position = layout_positions[block["blockID"]]
                assert len(position) == 4 and all(isinstance(value, int) for value in position), (
                    template_id,
                    handle,
                    block["blockID"],
                    "invalid block position",
                )
                x, y, width, height = position
                assert x >= 0 and y >= 0 and width > 0 and x + width <= 48, (
                    template_id,
                    handle,
                    block["blockID"],
                    position,
                    "block outside 48-column grid",
                )
                assert height >= minimum_height(block), (
                    template_id, handle, block["blockID"], position, "block below its minimum height"
                )
                for other in rectangles:
                    assert not rectangles_overlap(position, other), (
                        template_id, handle, block["blockID"], position, other, "overlapping blocks"
                    )
                rectangles.append(position)
                kind = block.get("kind")
                if handle == "inicio" and kind == "Metric":
                    assert height >= 18, (
                        template_id, handle, block["blockID"], position, "home metric is clipped"
                    )
                    assert not block.get("title", "").startswith("Indicador ·"), (
                        template_id, handle, block["blockID"], "redundant metric title prefix"
                    )
                    assert "Indicador accionable calculado" not in block.get("description", ""), (
                        template_id, handle, block["blockID"], "generic metric description"
                    )
                if handle == "inicio" and kind == "RecordList":
                    assert height >= 48, (
                        template_id, handle, block["blockID"], position, "home list is too short"
                    )
                metric_count += int(kind == "Metric")
                map_count += int(kind == "Geometry")
                organizer_count += int(kind == "RecordOrganizer")
                if kind == "Geometry":
                    feeds = block.get("options", {}).get("feeds", [])
                    assert len(feeds) == 1, (template_id, handle, "Geometry requires one record feed")
                    feed = feeds[0]
                    feed_options = feed.get("options", {})
                    module_handle = feed_options.get("module")
                    geometry_field = feed.get("geometryField")
                    title_field = feed.get("titleField")
                    assert module_handle in modules, (template_id, handle, "unknown Geometry module")
                    assert geometry_field in modules[module_handle]["fields"], (
                        template_id, handle, geometry_field, "unknown Geometry field"
                    )
                    assert modules[module_handle]["fields"][geometry_field]["kind"] == "Geometry", (
                        template_id, handle, geometry_field, "feed field is not Geometry"
                    )
                    assert title_field in modules[module_handle]["fields"], (
                        template_id, handle, title_field, "unknown marker title field"
                    )
                    located = [row for row in rows_by_module[module_handle] if row.get(geometry_field)]
                    assert located, (template_id, handle, "map has no records with coordinates")
                    if template_id == "inmobiliaria-co":
                        assert module_handle == "inmuebles"
                        assert geometry_field == "ubicacion"
                        assert title_field == "nombre"
                        assert feed_options.get("prefilter") == "estado = 'Disponible'"
                        visible = [row for row in located if row.get("estado") == "Disponible"]
                        assert visible, (template_id, handle, "map filter has no visible properties")
                        bounds = block.get("options", {}).get("bounds")
                        assert bounds and len(bounds) == 2, (template_id, handle, "Colombia bounds missing")
                        north_east, south_west = bounds
                        for row in visible:
                            latitude, longitude = json.loads(row[geometry_field])["coordinates"]
                            assert south_west[0] <= latitude <= north_east[0], (
                                template_id, row["ID"], "latitude outside map bounds"
                            )
                            assert south_west[1] <= longitude <= north_east[1], (
                                template_id, row["ID"], "longitude outside map bounds"
                            )
        assert page_count >= len(modules), (template_id, "not enough pages")
        assert metric_count == 4, (template_id, metric_count, "expected four actionable metrics")
        assert map_count == EXPECTED_MAPS.get(template_id, 0), (template_id, map_count, "unexpected map design")
        assert organizer_count >= 1, (template_id, "missing pipeline organizer")
        assert ("mi-cola" in page_handles) == (template_id in EXPECTED_PERSONAL_QUEUES), (
            template_id, "personal queue must remain sector-specific"
        )

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
