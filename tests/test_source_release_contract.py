import importlib.util
import json
import re
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
DERIVER_SPEC = importlib.util.spec_from_file_location(
    "derive_source_evidence", ROOT / ".abera" / "derive_source_evidence.py"
)
DERIVER = importlib.util.module_from_spec(DERIVER_SPEC)
DERIVER_SPEC.loader.exec_module(DERIVER)


class SourceReleaseContractTest(unittest.TestCase):
    def setUp(self):
        self.contract = json.loads(
            (ROOT / ".abera" / "source-contract.json").read_text(encoding="utf-8")
        )

    def test_contract_is_explicit_and_has_unique_variables(self):
        self.assertEqual(self.contract["schemaVersion"], 1)
        self.assertEqual(self.contract["productId"], "abera-corteza-essential")
        variables = self.contract["environmentContract"]["variables"]
        names = [item["name"] for item in variables]
        self.assertEqual(names, sorted(set(names)))
        for variable in variables:
            self.assertRegex(variable["name"], r"^[A-Z][A-Z0-9_]+$")
            self.assertIsInstance(variable["required"], bool)
            self.assertIsInstance(variable["sensitive"], bool)
            self.assertIn(variable["valueSource"], {"platform", "secret", "image-default"})
            if variable["sensitive"]:
                self.assertEqual(variable["valueSource"], "secret")

    def test_runtime_injection_is_covered_by_contract(self):
        template = (ROOT / "tests" / "docker-postgres-smoke.sh").read_text(
            encoding="utf-8"
        )
        declared = {
            item["name"]
            for item in self.contract["environmentContract"]["variables"]
        }
        application_environment = template.split(
            'docker run --detach --name "$application"', 1
        )[1].split('"$image"', 1)[0]
        smoke_variables = set(
            re.findall(
                r"(?:-e|--env)\s+['\"]?([A-Z][A-Z0-9_]*)=",
                application_environment,
            )
        )
        critical = {
            "ABERA_AI_ENABLED",
            "ABERA_BOOTSTRAP_OUTPUT_FILE",
            "ABERA_BOOTSTRAP_PUBLIC_KEY_FILE",
            "ABERA_DEMO_BUNDLE",
            "ABERA_INITIAL_ADMIN_EMAIL",
            "ABERA_INITIAL_ADMIN_HANDLE",
            "ABERA_INITIAL_ADMIN_NAME",
            "ABERA_MCP_MODE",
            "ABERA_MODE",
            "AUTH_JWT_SECRET",
            "DB_DSN",
            "DOMAIN",
            "DOMAIN_WEBAPP",
            "HTTP_SSL_TERMINATED",
        }
        self.assertFalse(critical - declared)
        self.assertFalse(critical - smoke_variables)
        self.assertFalse(
            smoke_variables - declared,
            "Every variable injected into the application smoke test must be declared",
        )

    def test_every_abera_runtime_variable_is_declared_or_internal(self):
        declared = {
            item["name"]
            for item in self.contract["environmentContract"]["variables"]
        }
        runtime_paths = [
            ROOT / "docker-template-select.sh",
            ROOT / "server" / "abera" / "assistant" / "config.go",
            ROOT / "server" / "pkg" / "provision" / "abera_bootstrap.go",
        ]
        runtime_paths.extend(sorted((ROOT / "templates").rglob("*.ps1")))
        observed = set()
        for path in runtime_paths:
            observed.update(
                re.findall(
                    r"\bABERA_[A-Z0-9_]+\b",
                    path.read_text(encoding="utf-8"),
                )
            )
        internal_only = {
            # Image layout defaults, never supplied by the platform.
            "ABERA_DEMO_ROOT",
            "ABERA_TEMPLATE_ROOT",
            # Ephemeral values passed from bootstrap to an in-container seed script.
            "ABERA_ADMIN_EMAIL",
            "ABERA_ADMIN_PASSWORD",
            "ABERA_CLIENT_SECRET",
        }
        self.assertFalse(
            observed - declared - internal_only,
            "A new ABERA_* runtime variable requires a source-contract decision",
        )

    def test_complete_runtime_inventory_requires_an_explicit_decision(self):
        observed = DERIVER.discover_environment_variables()
        declared = {
            item["name"]
            for item in self.contract["environmentContract"]["variables"]
        }
        internal = self.contract["runtimeEnvironment"]["internalVariables"]
        DERIVER.validate_environment_inventory(self.contract, observed)
        self.assertFalse(set(observed) - declared - set(internal))
        self.assertEqual(
            DERIVER.approved_environment_inventory(self.contract),
            sorted(declared | set(internal)),
        )
        self.assertTrue(
            {
                "AUTH_DEFAULT_USER_GROUP",
                "AWS_REGION",
                "CORTEZA_DEFAULT_LOCALE",
                "DOMAIN",
                "SMTP_HOST",
            }.issubset(observed),
            "Inventory must include non-ABERA runtime variables",
        )
        runtime_paths = {
            path.relative_to(ROOT).as_posix() for path in DERIVER.runtime_files()
        }
        self.assertFalse(any("/vendor/" in f"/{path}/" for path in runtime_paths))
        self.assertFalse(any(path.endswith(".gen.go") for path in runtime_paths))
        self.assertFalse(any("/tests/" in f"/{path}/" for path in runtime_paths))

        unsafe = json.loads(json.dumps(self.contract))
        unsafe["runtimeEnvironment"]["internalVariables"].remove("AWS_REGION")
        with self.assertRaisesRegex(DERIVER.EvidenceError, "AWS_REGION"):
            DERIVER.validate_environment_inventory(unsafe, observed)

    def test_change_classification_is_derived_from_changed_paths(self):
        database, infrastructure = DERIVER.classify_changed_files(
            [
                "README.md",
                "Dockerfile",
                "server/store/adapters/rdbms/upgrade.go",
            ]
        )
        self.assertEqual(database, ["server/store/adapters/rdbms/upgrade.go"])
        self.assertEqual(infrastructure, ["Dockerfile"])
        self.assertNotIn("changeClassification", self.contract)

    def test_add_column_manifest_is_explicit_and_reproducible(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            manifest_path = ".abera/migrations/corteza-schema-2.json"
            migration_path = "server/store/migrations/20260831_add_column.sql"
            document = {
                "schemaVersion": 1,
                "migrationId": "corteza-schema-2",
                "databaseSensitiveFiles": [migration_path],
                "operations": [
                    {
                        "type": "add_column",
                        "table": "compose_record",
                        "column": "abera_external_id",
                        "dataType": "text",
                        "nullable": True,
                    }
                ],
            }
            target = root / manifest_path
            target.parent.mkdir(parents=True)
            target.write_text(json.dumps(document), encoding="utf-8")
            changed = sorted([manifest_path, migration_path])
            database, _infrastructure = DERIVER.classify_changed_files(changed)
            original_root = DERIVER.ROOT
            DERIVER.ROOT = root
            try:
                evidence = DERIVER.derive_database_migration_evidence(
                    changed, database
                )
            finally:
                DERIVER.ROOT = original_root

        self.assertIsNotNone(evidence)
        self.assertEqual(evidence["migrationId"], "corteza-schema-2")
        self.assertEqual(evidence["manifestPath"], manifest_path)
        self.assertEqual(
            evidence["manifestSha256"], DERIVER._canonical_sha256(document)
        )
        self.assertEqual(
            DERIVER.classify_database_migration(database, evidence),
            "additive",
        )

    def test_unknown_or_incomplete_database_change_remains_incompatible(self):
        cases = [
            {
                "schemaVersion": 1,
                "migrationId": "corteza-schema-2",
                "databaseSensitiveFiles": [
                    "server/store/migrations/20260831_add_column.sql"
                ],
                "operations": [
                    {
                        "type": "drop_column",
                        "table": "compose_record",
                        "column": "obsolete",
                        "dataType": "text",
                        "nullable": True,
                    }
                ],
            },
            {
                "schemaVersion": 1,
                "migrationId": "corteza-schema-2",
                "databaseSensitiveFiles": [],
                "operations": [
                    {
                        "type": "add_column",
                        "table": "compose_record",
                        "column": "abera_external_id",
                        "dataType": "text",
                        "nullable": True,
                    }
                ],
            },
        ]
        for document in cases:
            with self.subTest(document=document), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                manifest_path = ".abera/migrations/corteza-schema-2.json"
                migration_path = "server/store/migrations/20260831_add_column.sql"
                target = root / manifest_path
                target.parent.mkdir(parents=True)
                target.write_text(json.dumps(document), encoding="utf-8")
                changed = sorted([manifest_path, migration_path])
                database, _infrastructure = DERIVER.classify_changed_files(changed)
                original_root = DERIVER.ROOT
                DERIVER.ROOT = root
                try:
                    evidence = DERIVER.derive_database_migration_evidence(
                        changed, database
                    )
                finally:
                    DERIVER.ROOT = original_root
                self.assertIsNone(evidence)
                self.assertEqual(
                    DERIVER.classify_database_migration(database, evidence),
                    "incompatible",
                )

    def test_new_ai_contract_is_safe_by_default(self):
        variables = {
            item["name"]: item
            for item in self.contract["environmentContract"]["variables"]
        }
        self.assertFalse(variables["ABERA_AI_ENABLED"]["required"])
        self.assertEqual(
            variables["ABERA_AI_ENABLED"]["valueSource"], "image-default"
        )
        for name in {
            "ABERA_AI_AGENT_URL",
            "ABERA_AI_API_KEY_FILE",
            "ABERA_AI_AUTH_MODE",
            "ABERA_AI_AWS_REGION",
            "ABERA_AI_LOCAL_TOKEN_FILE",
            "ABERA_AI_REQUIRED_ROLE",
            "ABERA_AI_TENANT_ID",
            "ABERA_AI_TIMEOUT",
        }:
            self.assertFalse(variables[name]["required"])

    def test_source_workflow_has_no_aws_trust_and_emits_sha_release(self):
        workflow = (
            ROOT / ".github" / "workflows" / "abera-image.yml"
        ).read_text(encoding="utf-8")
        lowered = workflow.lower()
        self.assertNotIn("configure-aws-credentials", lowered)
        self.assertNotIn("aws_access_key", lowered)
        self.assertIn('version="sha-${GITHUB_SHA}"', workflow)
        self.assertIn("Refusing to overwrite immutable image", workflow)
        self.assertIn("local_image_id", workflow)
        self.assertIn("published_image_id", workflow)
        self.assertNotIn("actions/attest-sbom", workflow)
        self.assertNotIn("actions/attest-build-provenance", workflow)
        self.assertEqual(workflow.count("uses: actions/attest@"), 4)
        exact_image = (
            "docker.io/${{ env.DOCKERHUB_REPOSITORY }}@"
            "${{ needs.build-and-test-image.outputs.digest }}"
        )
        self.assertIn(f"image-ref: {exact_image}", workflow)
        self.assertIn(f"image: {exact_image}", workflow)
        self.assertNotIn(
            "image: docker.io/${{ env.DOCKERHUB_REPOSITORY }}:"
            "${{ env.BUILD_VERSION }}",
            workflow,
        )
        self.assertIn("subject-path: abera-corteza.spdx.json", workflow)
        self.assertIn(
            "subject-path: dist/source-release/source-release.json", workflow
        )
        self.assertIn("SBOM_ARTIFACT_ATTESTATION_URL", workflow)
        self.assertIn('"runtimeEnvironment": contract["runtimeEnvironment"]', workflow)
        self.assertIn('"changeEvidence": change_evidence', workflow)
        self.assertIn("derive_source_evidence.py", workflow)
        self.assertIn("source-release.json", workflow)
        self.assertIn("abera-corteza-source-release-", workflow)


if __name__ == "__main__":
    unittest.main()
