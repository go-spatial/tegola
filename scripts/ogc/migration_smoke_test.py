"""Boundary controls for the owned migration fixture tooling."""
import tempfile
from pathlib import Path
import sqlite3
import unittest

import migration_smoke


class MigrationFixtureTests(unittest.TestCase):
    def test_strict_json_types_null_and_nonpoint_geometry(self):
        self.assertFalse(migration_smoke.exact_value({"n": True}, {"n": 1}))
        self.assertFalse(migration_smoke.exact_value({}, {"n": None}))
        line = {"type": "LineString", "coordinates": [[14, 30], [16, 30]]}
        self.assertTrue(migration_smoke.exact_value(line, line))
        self.assertFalse(migration_smoke.exact_value(
            {"type": "LineString", "coordinates": [[14, 30], [16, 31]]}, line))
        with self.assertRaises(ValueError):
            migration_smoke.exact_value(float("nan"), 15)

    def test_fresh_fixture_and_static_nulls(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "owned"
            config = Path(migration_smoke.prepare(path))
            self.assertIn('id = "untimed"', config.read_text())
            connection = sqlite3.connect(path / "fixture.gpkg")
            try:
                self.assertEqual(connection.execute("SELECT id FROM xy ORDER BY id").fetchall(),
                                 [(10,), (20,), (30,), (40,), (50,), (60,)])
                self.assertEqual(connection.execute("SELECT geom,n,s,b,at FROM xy WHERE id=40").fetchone(),
                                 (None, None, None, None, None))
            finally:
                connection.close()

    def test_existing_directory_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(FileExistsError):
                migration_smoke.prepare(directory)

    def test_remote_credential_redirect_targets_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            for target in ("https://example.org/features", "http://user:pass@localhost:9000/features",
                           "http://127.0.0.1/features", "http://localhost:9000/features?x=1"):
                with self.subTest(target=target), self.assertRaises(ValueError):
                    migration_smoke.check(target, Path(directory) / "unused")
            self.assertFalse((Path(directory) / "unused").exists())


if __name__ == "__main__":
    unittest.main()
