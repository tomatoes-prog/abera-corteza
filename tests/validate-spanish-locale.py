#!/usr/bin/env python3
"""Validate Spanish webapp locales without changing backend translations."""

from __future__ import annotations

import re
from pathlib import Path
from typing import Any, Iterator

import yaml


ROOT = Path(__file__).resolve().parents[1]
EN_ROOT = ROOT / "locale" / "en"
ES_ROOT = ROOT / "locale" / "es"

ALLOWED_IDENTICAL_VALUES = {
    "Agenda",
    "Alias",
    "AND",
    "Avatar",
    "CC",
    "CSV",
    "Color",
    "Compose",
    "Editor",
    "Error",
    "Experimental",
    "Extras",
    "General",
    "HTML",
    "HTTP POST",
    "Horizontal",
    "ID",
    "IFrame",
    "ID:",
    "JSON",
    "Logo",
    "OpenID Connect",
    "OR",
    "Proxy",
    "Roles",
    "SAML",
    "Total",
    "URI",
    "URL",
    "Vertical",
    "Visible",
    "YAML",
    "Zoom",
    "px",
    "roles",
    "script",
    " (ID)",
    " - ERROR: ",
    "AVG",
    "MAX",
    "MIN",
    "STD",
    "SUM",
}

TECHNICAL_KEY_PARTS = {
    "example",
    "formatting",
    "multivaluedelimiter",
    "placeholder",
}

PLACEHOLDER_RE = re.compile(
    r"{{\s*[^{}]+\s*}}"
    r"|\$\{[^{}]+}"
    r"|(?<!{){[A-Za-z0-9_.-]+}(?!})"
    r"|%[-+#0-9.]*[a-zA-Z]"
)
HTML_TAG_RE = re.compile(r"</?([A-Za-z][A-Za-z0-9-]*)\b[^>]*>")
MOJIBAKE_MARKERS = ("Ã", "Â", "â€", "ï¿½", "�")


def load_yaml(path: Path) -> Any:
    with path.open("r", encoding="utf-8-sig") as source:
        return yaml.safe_load(source) or {}


def flatten(value: Any, prefix: str = "") -> Iterator[tuple[str, Any]]:
    if isinstance(value, dict):
        for key, item in value.items():
            child = f"{prefix}.{key}" if prefix else str(key)
            yield from flatten(item, child)
    elif isinstance(value, list):
        for index, item in enumerate(value):
            yield from flatten(item, f"{prefix}[{index}]")
    else:
        yield prefix, value


def is_technical_identity(key: str, value: str) -> bool:
    if value in ALLOWED_IDENTICAL_VALUES:
        return True

    normalized_key = key.lower()
    if any(part in normalized_key for part in TECHNICAL_KEY_PARTS):
        return True

    if re.match(r"^(?:https?://|postgres://|corteza::)", value):
        return True

    without_tokens = PLACEHOLDER_RE.sub("", value)
    without_tags = HTML_TAG_RE.sub("", without_tokens)
    return not re.search(r"[A-Za-z]{2}", without_tags)


def main() -> int:
    errors: list[str] = []
    checked_files = 0
    checked_strings = 0

    # Deliberately validate only browser applications. The backend locale is
    # upstream-owned and remains unchanged by this presentation-layer audit.
    for english_path in sorted(EN_ROOT.glob("corteza-webapp-*/*.yaml")):
        relative = english_path.relative_to(EN_ROOT)
        spanish_path = ES_ROOT / relative
        checked_files += 1

        if not spanish_path.exists():
            errors.append(f"{relative}: missing Spanish locale file")
            continue

        english = dict(flatten(load_yaml(english_path)))
        spanish = dict(flatten(load_yaml(spanish_path)))

        for key, english_value in english.items():
            if key not in spanish:
                errors.append(f"{relative}:{key}: missing Spanish key")
                continue

            spanish_value = spanish[key]
            if not isinstance(english_value, str) or not isinstance(spanish_value, str):
                continue

            checked_strings += 1
            english_tokens = sorted(PLACEHOLDER_RE.findall(english_value))
            spanish_tokens = sorted(PLACEHOLDER_RE.findall(spanish_value))
            if english_tokens != spanish_tokens:
                errors.append(
                    f"{relative}:{key}: placeholders changed "
                    f"{english_tokens!r} -> {spanish_tokens!r}"
                )

            english_tags = sorted(HTML_TAG_RE.findall(english_value))
            spanish_tags = sorted(HTML_TAG_RE.findall(spanish_value))
            if english_tags != spanish_tags:
                errors.append(
                    f"{relative}:{key}: HTML tags changed "
                    f"{english_tags!r} -> {spanish_tags!r}"
                )

            if any(marker in spanish_value for marker in MOJIBAKE_MARKERS):
                errors.append(f"{relative}:{key}: possible mojibake in {spanish_value!r}")

            if (
                english_value == spanish_value
                and re.search(r"[A-Za-z]{2}", english_value)
                and not is_technical_identity(key, english_value)
            ):
                errors.append(
                    f"{relative}:{key}: unreviewed English text {english_value!r}"
                )

    if errors:
        print("Spanish webapp locale validation failed:")
        for error in errors:
            print(f" - {error}")
        return 1

    print(
        f"Spanish webapp locales valid: {checked_files} files, "
        f"{checked_strings} translated strings"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
