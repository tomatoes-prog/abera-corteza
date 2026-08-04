#!/usr/bin/env python3

# Copyright 2026 Abera/Corteza contributors
# Licensed under the Apache License, Version 2.0.

"""Normalize template pages for Corteza's 48-column grid."""

from pathlib import Path
from typing import Any

import yaml


ROOT = Path(__file__).resolve().parents[1]
GRID_COLUMNS = 48
LEGACY_GRID_COLUMNS = 12
CHART_HEIGHT = 30


def normalize_pages(pages: dict[str, dict[str, Any]]) -> None:
    for page in pages.values():
        blocks = page.get("blocks", [])
        legacy_grid = max(
            (int(value) for block in blocks for value in block.get("xywh", []) if value is not None),
            default=0,
        ) <= LEGACY_GRID_COLUMNS

        normalized_positions: list[list[int]] = []
        for block in blocks:
            position = list(block.get("xywh", [0, 0, GRID_COLUMNS, 15]))
            position += [0] * (4 - len(position))
            x, y, width, height = (int(value) for value in position[:4])
            if legacy_grid:
                x *= GRID_COLUMNS // LEGACY_GRID_COLUMNS
                width *= GRID_COLUMNS // LEGACY_GRID_COLUMNS
            if block.get("kind") == "Chart":
                height = max(height, CHART_HEIGHT)
            normalized_positions.append([x, y, width, height])

        # A chart that grows vertically must push down blocks that overlap it.
        # Blocks next to a chart keep their row, which preserves dashboard columns.
        for index, block in enumerate(blocks):
            x, y, width, height = normalized_positions[index]
            if block.get("kind") == "Chart":
                continue
            for chart_index, chart in enumerate(blocks):
                if chart.get("kind") != "Chart":
                    continue
                chart_x, chart_y, chart_width, chart_height = normalized_positions[chart_index]
                overlaps_horizontally = x < chart_x + chart_width and chart_x < x + width
                overlaps_vertically = y < chart_y + chart_height and chart_y < y + height
                if overlaps_horizontally and overlaps_vertically:
                    y = max(y, chart_y + chart_height)
            normalized_positions[index][1] = y

        layout_blocks: list[dict[str, Any]] = []
        for block_id, (block, position) in enumerate(zip(blocks, normalized_positions), start=1):
            block["blockID"] = block_id
            block["xywh"] = position
            layout_blocks.append({
                "blockID": block_id,
                "xywh": list(position),
            })

        config: dict[str, Any] = {"useTitle": True}
        if page.get("module"):
            config["buttons"] = {
                "new": {"enabled": True},
                "edit": {"enabled": True},
                "submit": {"enabled": True},
                "delete": {"enabled": True},
                "clone": {"enabled": True},
                "back": {"enabled": True},
            }

        page["page_layouts"] = {
            "principal": {
                "primary": True,
                "meta": {
                    "title": f"Diseño principal · {page.get('title', 'Página')}",
                },
                "config": config,
                "blocks": layout_blocks,
            },
        }

        children = page.get("children", {})
        if children:
            normalize_pages(children)


def main() -> None:
    for path in sorted((ROOT / "templates").glob("*/provision/pages.yaml")):
        document = yaml.safe_load(path.read_text(encoding="utf-8"))
        normalize_pages(document.get("pages", {}))
        encoded = yaml.safe_dump(
            document,
            allow_unicode=True,
            sort_keys=False,
            default_flow_style=False,
            width=120,
        )
        path.write_text(
            "# Copyright 2026 Abera/Corteza contributors\n"
            "# Licensed under the Apache License, Version 2.0.\n\n"
            + encoded,
            encoding="utf-8",
            newline="\n",
        )
        print(path.relative_to(ROOT))


if __name__ == "__main__":
    main()
