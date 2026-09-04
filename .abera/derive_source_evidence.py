#!/usr/bin/env python3
"""Derive fail-closed source-release evidence from the checked-out tree."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
from pathlib import Path, PurePosixPath


ROOT = Path(__file__).resolve().parents[1]
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
VARIABLE_RE = re.compile(r"^[A-Z][A-Z0-9_]{1,127}$")
MIGRATION_ID_RE = re.compile(r"^[a-z0-9][a-z0-9.-]{0,127}$")
SQL_IDENTIFIER_RE = re.compile(r"^[a-z][a-z0-9_]{0,62}$")
ADDITIVE_MANIFEST_RE = re.compile(
    r"^\.abera/migrations/([a-z0-9][a-z0-9.-]{0,127})\.json$"
)
ADDITIVE_COLUMN_TYPES = {
    "bigint",
    "boolean",
    "integer",
    "jsonb",
    "text",
    "timestamp",
    "uuid",
}
DATABASE_PATTERNS = (
    re.compile(r"(^|/)(migrations?|schema)(/|$)", re.IGNORECASE),
    re.compile(r"\.sql$", re.IGNORECASE),
    re.compile(r"^server/store/"),
    re.compile(r"^server/pkg/dal/"),
)
INFRASTRUCTURE_PATTERNS = (
    re.compile(r"^Dockerfile$"),
    re.compile(r"^docker-(entrypoint|template-select)\.sh$"),
    re.compile(r"^\.github/workflows/"),
    re.compile(r"^\.abera/"),
    re.compile(r"^templates/"),
)
ENVIRONMENT_PATTERNS = (
    re.compile(r"(?:os\.(?:Getenv|LookupEnv)|getenv)\(\s*['\"]([A-Z][A-Z0-9_]*)['\"]"),
    re.compile(r"\$env:([A-Z][A-Z0-9_]*)", re.IGNORECASE),
    re.compile(r"process\.env\.([A-Z][A-Z0-9_]*)"),
    re.compile(r"process\.env\[['\"]([A-Z][A-Z0-9_]*)['\"]\]"),
    re.compile(r"\benv:\"([A-Z][A-Z0-9_]*)\""),
    re.compile(r"\$\{([A-Z][A-Z0-9_]*)(?::[-+?=][^}]*)?\}"),
    re.compile(r"\b(ABERA_[A-Z0-9_]+)\b"),
)
RUNTIME_SUFFIXES = {".go", ".js", ".jsx", ".ps1", ".sh", ".ts", ".tsx"}
EXCLUDED_PARTS = {
    "build",
    "dist",
    "gen",
    "generated",
    "node_modules",
    "testdata",
    "tests",
    "vendor",
}


class EvidenceError(ValueError):
    pass


def _run_git(*args: str) -> str:
    result = subprocess.run(
        ["git", *args], cwd=ROOT, check=True, capture_output=True, text=True
    )
    return result.stdout


def _canonical_sha256(value: object) -> str:
    payload = json.dumps(value, sort_keys=True, separators=(",", ":")).encode()
    return hashlib.sha256(payload).hexdigest()


def _safe_paths(values: list[str]) -> list[str]:
    normalized = []
    for raw in values:
        value = raw
        path = PurePosixPath(value)
        if (
            not value
            or "\\" in value
            or any(ord(character) < 32 or ord(character) == 127 for character in value)
            or path.is_absolute()
            or ".." in path.parts
            or any(part in {"", "."} for part in path.parts)
        ):
            raise EvidenceError(f"Unsafe changed path: {raw!r}")
        normalized.append(value)
    return sorted(set(normalized))


def _nul_paths(value: str) -> list[str]:
    return [] if not value else value.rstrip("\0").split("\0")


def _is_runtime_file(path: str) -> bool:
    candidate = PurePosixPath(path)
    if any(part.lower() in EXCLUDED_PARTS for part in candidate.parts):
        return False
    if candidate.name.endswith((".gen.go", "_test.go")):
        return False
    return candidate.suffix.lower() in RUNTIME_SUFFIXES


def runtime_files() -> list[Path]:
    tracked = _safe_paths(_nul_paths(_run_git("ls-files", "-z")))
    return [ROOT / path for path in tracked if _is_runtime_file(path)]


def discover_environment_variables() -> list[str]:
    observed: set[str] = set()
    for path in runtime_files():
        try:
            text = path.read_text(encoding="utf-8")
        except UnicodeDecodeError as error:
            raise EvidenceError(f"Runtime source is not UTF-8: {path}") from error
        for pattern in ENVIRONMENT_PATTERNS:
            observed.update(pattern.findall(text))
    return sorted(observed)


def approved_environment_inventory(contract: dict) -> list[str]:
    runtime = contract.get("runtimeEnvironment")
    if not isinstance(runtime, dict) or set(runtime) != {
        "schemaVersion",
        "internalVariables",
    }:
        raise EvidenceError("runtimeEnvironment must declare schemaVersion/internalVariables")
    if runtime["schemaVersion"] != 1:
        raise EvidenceError("runtimeEnvironment.schemaVersion must be 1")
    internal = runtime["internalVariables"]
    if (
        not isinstance(internal, list)
        or internal != sorted(set(internal))
        or any(not isinstance(name, str) or not VARIABLE_RE.fullmatch(name) for name in internal)
    ):
        raise EvidenceError("runtimeEnvironment.internalVariables must be sorted and unique")
    variables = contract.get("environmentContract", {}).get("variables", [])
    if not isinstance(variables, list):
        raise EvidenceError("environmentContract.variables must be an array")
    declared = {
        item.get("name")
        for item in variables
        if isinstance(item, dict) and isinstance(item.get("name"), str)
    }
    if len(declared) != len(variables) or any(
        not VARIABLE_RE.fullmatch(name) for name in declared
    ):
        raise EvidenceError("environmentContract variables must have unique safe names")
    return sorted(declared | set(internal))


def validate_environment_inventory(contract: dict, observed: list[str]) -> None:
    approved = approved_environment_inventory(contract)
    missing = sorted(set(observed) - set(approved))
    if missing:
        raise EvidenceError(
            "Runtime variables need a platform or internal contract decision: "
            + ", ".join(missing)
        )


def classify_changed_files(changed_files: list[str]) -> tuple[list[str], list[str]]:
    safe = _safe_paths(changed_files)
    database = [
        path for path in safe if any(pattern.search(path) for pattern in DATABASE_PATTERNS)
    ]
    infrastructure = [
        path
        for path in safe
        if any(pattern.search(path) for pattern in INFRASTRUCTURE_PATTERNS)
    ]
    return database, infrastructure


def _normalize_additive_manifest(
    manifest_path: str,
    database_files: list[str],
) -> dict | None:
    match = ADDITIVE_MANIFEST_RE.fullmatch(manifest_path)
    if not match:
        return None
    try:
        document = json.loads((ROOT / manifest_path).read_text(encoding="utf-8"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError):
        return None
    if not isinstance(document, dict) or set(document) != {
        "schemaVersion",
        "migrationId",
        "databaseSensitiveFiles",
        "operations",
    }:
        return None
    migration_id = document.get("migrationId")
    if (
        type(document.get("schemaVersion")) is not int
        or document["schemaVersion"] != 1
        or not isinstance(migration_id, str)
        or not MIGRATION_ID_RE.fullmatch(migration_id)
        or migration_id != match.group(1)
    ):
        return None
    declared_files = document.get("databaseSensitiveFiles")
    if not isinstance(declared_files, list):
        return None
    try:
        normalized_files = _safe_paths(declared_files)
    except EvidenceError:
        return None
    expected_files = sorted(set(database_files) - {manifest_path})
    if (
        not expected_files
        or normalized_files != declared_files
        or normalized_files != expected_files
    ):
        return None
    raw_operations = document.get("operations")
    if not isinstance(raw_operations, list) or not raw_operations:
        return None
    operations = []
    for operation in raw_operations:
        if not isinstance(operation, dict) or set(operation) != {
            "type",
            "table",
            "column",
            "dataType",
            "nullable",
        }:
            return None
        if (
            operation.get("type") != "add_column"
            or not isinstance(operation.get("table"), str)
            or not SQL_IDENTIFIER_RE.fullmatch(operation["table"])
            or not isinstance(operation.get("column"), str)
            or not SQL_IDENTIFIER_RE.fullmatch(operation["column"])
            or operation.get("dataType") not in ADDITIVE_COLUMN_TYPES
            or operation.get("nullable") is not True
        ):
            return None
        operations.append(dict(operation))
    normalized_operations = sorted(
        operations,
        key=lambda item: (
            item["table"],
            item["column"],
            item["dataType"],
            item["type"],
        ),
    )
    identities = {(item["table"], item["column"]) for item in operations}
    if operations != normalized_operations or len(identities) != len(operations):
        return None
    normalized_manifest = {
        "schemaVersion": 1,
        "migrationId": migration_id,
        "databaseSensitiveFiles": normalized_files,
        "operations": normalized_operations,
    }
    return {
        **normalized_manifest,
        "manifestPath": manifest_path,
        "manifestSha256": _canonical_sha256(normalized_manifest),
    }


def derive_database_migration_evidence(
    changed_files: list[str], database_files: list[str]
) -> dict | None:
    candidates = [
        path for path in changed_files if ADDITIVE_MANIFEST_RE.fullmatch(path)
    ]
    if len(candidates) != 1:
        return None
    return _normalize_additive_manifest(candidates[0], database_files)


def classify_database_migration(
    database_files: list[str], migration_evidence: dict | None
) -> str:
    if not database_files:
        return "none"
    return "additive" if migration_evidence is not None else "incompatible"


def derive_evidence(base: str, head: str) -> dict:
    if not SHA_RE.fullmatch(head):
        raise EvidenceError("head must be a full lowercase commit SHA")
    history_complete = bool(SHA_RE.fullmatch(base)) and set(base) != {"0"}
    if not history_complete:
        base = _run_git("rev-parse", f"{head}^").strip()
    if not SHA_RE.fullmatch(base) or base == head:
        raise EvidenceError("base must resolve to a distinct full commit SHA")
    changed = _safe_paths(
        _nul_paths(
            _run_git(
                "diff", "--name-only", "--diff-filter=ACDMRTUXB", "-z", base, head
            )
        )
    )
    database, infrastructure = classify_changed_files(changed)
    observed = discover_environment_variables()
    contract = json.loads(
        (ROOT / ".abera" / "source-contract.json").read_text(encoding="utf-8")
    )
    validate_environment_inventory(contract, observed)
    approved_environment = approved_environment_inventory(contract)
    migration_evidence = derive_database_migration_evidence(changed, database)
    classification = {
        # Only an exact, attested allowlisted declaration may downgrade the
        # conservative incompatible classification to additive.
        "databaseMigration": classify_database_migration(
            database, migration_evidence
        ),
        "infrastructureChanged": bool(infrastructure),
    }
    return {
        "schemaVersion": 1,
        "baseCommit": base,
        "headCommit": head,
        "historyComplete": history_complete,
        "changedFiles": changed,
        "changedFilesSha256": _canonical_sha256(changed),
        "databaseSensitiveFiles": database,
        "databaseMigrationEvidence": migration_evidence,
        "infrastructureFiles": infrastructure,
        "runtimeEnvironmentVariables": approved_environment,
        "runtimeEnvironmentSha256": _canonical_sha256(approved_environment),
        "changeClassification": classification,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True)
    parser.add_argument("--head", required=True)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    evidence = derive_evidence(args.base, args.head)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(evidence, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
