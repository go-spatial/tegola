import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import subprocess
import io
import hashlib
import xml.etree.ElementTree as ET

from prepare_fixture import prepare
from report import classify
from run_ets import bounded_json, fetch, invalid_bbox_uri, local_target, read_json, run_owned_container, verify_build, verify_revision
from supplement import Evidence, MISSING, PREFIX, execute as execute_supplement, geometry_equal, verify_dataset
from contextlib import closing
import sqlite3
from urllib.parse import parse_qs, urlsplit


class RunnerTests(unittest.TestCase):
    def test_invalid_bbox_request_selects_actual_json_representation(self):
        uri = invalid_bbox_uri("http://localhost:19100/features", "xy/slash")
        self.assertIn("xy%2Fslash/items?", uri)
        query = parse_qs(urlsplit(uri).query)
        self.assertEqual(query["f"], ["json"])
        self.assertEqual(query["bbox-crs"], ["urn:invalid:unlisted"])

    def test_supplement_oracle_and_negative_controls(self):
        directory = self.root / "fixture"
        prepare(directory, "core")
        dataset, config = directory / "fixture.gpkg", directory / "config.toml"
        metadata = b'{"collections":[{"id":"xy"}]}'
        project = Path(__file__).resolve().parents[2]
        literal = json.loads(Path(__file__).with_name("supplement-oracles.json").read_text())
        def response(uri):
            query = parse_qs(urlsplit(uri).query)
            code = query.get("crs", ["default"])[0]
            coordinates = [30, 15] if code.endswith("4326") else [1669792.3618991037, 3503549.8435043753] if code.endswith("3857") else [15, 30]
            mode = "4326" if code.endswith("4326") else "3857" if code.endswith("3857") else "default"
            point = {"id": 10, "geometry": literal[mode]["10"]}
            value = point if "/items/10?" in uri else {"features": [{"id": id, "geometry": literal[mode][str(id)]} for id in (10, 30, 40, 50, 60)], "numberMatched": 5}
            if "offset" in query:
                value = {"features": [{"id": 60, "geometry": literal[mode]["60"]}], "numberMatched": 5}
            header = "<" + query.get("crs", ["http://www.opengis.net/def/crs/OGC/1.3/CRS84"])[0] + ">"
            return 200, json.dumps(value).encode(), {"Content-Crs": header}
        evidence = execute_supplement("http://localhost:1/features", response, bounded_json, metadata, dataset, config, project, self.root)
        self.assertIsInstance(evidence, Evidence)
        allowed = {(PREFIX + cls, name) for cls, name in MISSING}
        def omit(root):
            for cls in list(root):
                for method in list(cls):
                    if (cls.get("name"), method.get("name")) in allowed:
                        cls.remove(method)
        result = classify(self.report(omit), True, True, evidence)
        self.assertEqual(result["result"], "PASS_WITH_SUPPLEMENTS")
        self.assertEqual(len(result["official_missing"]), 4)
        self.assertEqual(result["counts"]["FAIL"], 0)
        self.assertEqual(result["counts"]["PASS"], len(result["cases"]))
        def extra(root):
            omit(root)
            root.remove(root[0])
        with self.assertRaises(ValueError):
            classify(self.report(extra), True, True, evidence)
        for metadata_value in (b'{"collections":[{"id":"other"}]}', b'{"collections":[{"id":"xy","extent":{}}]}'):
            with self.assertRaises(ValueError):
                execute_supplement("http://localhost:1/features", response, bounded_json, metadata_value, dataset, config, project, self.root)
        for status, body, headers in ((500, b'{}', {}), (200, b'{"features":[],"numberMatched":0}', {"Content-Crs": "<wrong>"})):
            with self.assertRaises(ValueError):
                execute_supplement("http://localhost:1/features", lambda uri: (status, body, headers), bounded_json, metadata, dataset, config, project, self.root)
        for damaged in (literal["default"]["30"], None, {"type": "LineString", "coordinates": [[0, 0], [1, 1]]}):
            def corrupt(uri):
                status, body, headers = response(uri)
                value = json.loads(body)
                if "features" in value and "crs=" in uri and "3857" in uri:
                    value["features"][1]["geometry"] = damaged
                return status, json.dumps(value).encode(), headers
            with self.assertRaises(ValueError):
                execute_supplement("http://localhost:1/features", corrupt, bounded_json, metadata, dataset, config, project, self.root)
        with closing(sqlite3.connect(dataset)) as connection:
            connection.execute("UPDATE xy SET geom='POINT (0 0)' WHERE id=10")
            connection.commit()
        with self.assertRaises(ValueError):
            verify_dataset(dataset, project / "testdata/ogc/conformance/fixture.sql")
        with closing(sqlite3.connect(dataset)) as connection:
            connection.executescript("DROP TABLE xy; CREATE VIEW xy AS WITH RECURSIVE x(n) AS (VALUES(1) UNION ALL SELECT n+1 FROM x) SELECT zeroblob(999999999) FROM x;")
        with self.assertRaises(ValueError):
            verify_dataset(dataset, project / "testdata/ogc/conformance/fixture.sql")

    def test_full_geometry_finite_and_structure_controls(self):
        literal = json.loads(Path(__file__).with_name("supplement-oracles.json").read_text())
        clockwise = {"type": "Polygon", "coordinates": [[[29,14],[29,16],[31,16],[31,14],[29,14]]]}
        self.assertFalse(geometry_equal(clockwise, literal["4326"]["50"]))
        self.assertEqual(literal["4326"]["50"]["coordinates"], [[[29,14],[31,14],[31,16],[29,16],[29,14]]])
        line = {"type": "LineString", "coordinates": [[14, 30], [16, 30]]}
        for actual in (None, {}, {"type": "LineString", "coordinates": [[14, 30], [16, 31]]},
                       {"type": "LineString", "coordinates": [[float("nan"), 30], [16, 30]]},
                       {"type": "LineString", "coordinates": [[float("inf"), 30], [16, 30]]},
                       {"type": "LineString", "coordinates": [[True, 30], [16, 30]]}):
            self.assertFalse(geometry_equal(actual, line))
        self.assertFalse(geometry_equal({"type": "Point", "coordinates": [30, 15]}, {"type": "Point", "coordinates": [30, 15, 23]}))
        for raw in (b'{"n":NaN}', b'{"n":Infinity}'):
            with self.assertRaises(ValueError):
                bounded_json(raw)
        self.assertFalse(geometry_equal(bounded_json(b"1e999"), 1))

    def test_forged_supplement_cannot_authorize_missing_method(self):
        def remove(root):
            for cls in list(root):
                if cls.get("name") == PREFIX + "bboxcrs.BBoxCrsParameter":
                    root.remove(cls)
        forged = {"result": "PASS", "profile": "core", "missing_methods": [{"class": PREFIX + cls, "name": name} for cls, name in MISSING]}
        with self.assertRaises(ValueError):
            classify(self.report(remove), True, True, forged)
        with self.assertRaises(ValueError):
            Evidence(forged, object())

    def test_revision_drift_rejected(self):
        with patch("run_ets.git", return_value=b"new-byte-identical-commit\n"):
            with self.assertRaises(ValueError):
                verify_revision(self.root, "built-commit")

    def test_stale_build_receipt_rejected(self):
        binary = self.root / "tegola"
        binary.write_bytes(b"owned test bytes")
        sources = {"source.go": "fixed"}
        receipt = {"schema": 1, "result": "PASS", "sources": sources, "revision": "revision",
                   "binary": str(binary), "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
                   "compiler": "go version go1.26.9 windows/amd64",
                   "settings": {"GOTOOLCHAIN": "go1.26.9", "GOWORK": "off", "GOFLAGS": "", "CGO_ENABLED": "1"},
                   "command": ["go", "build", "-mod=vendor", "-trimpath", "-o", str(binary), "./cmd/tegola"]}
        verify_build(receipt, binary, sources, "revision")
        for key, value in (("revision", "older"), ("sources", {}), ("binary_sha256", "wrong")):
            with self.assertRaises(ValueError):
                verify_build({**receipt, key: value}, binary, sources, "revision")

    def test_preflight_disables_environment_proxy(self):
        with patch("run_ets.build_opener") as opener:
            response = opener.return_value.open.return_value.__enter__.return_value
            response.read.return_value = b"{}"
            response.status = 200
            fetch("http://127.0.0.1:19100/features")
            self.assertEqual(opener.call_args.args[0].proxies, {})

    def test_timeout_cleans_only_owned_container(self):
        commands = []
        def execute(command, **kwargs):
            commands.append(command)
            if len(commands) == 1:
                raise subprocess.TimeoutExpired(command, 1800)
            if command[1:3] == ["container", "inspect"]:
                return subprocess.CompletedProcess(command, 1, b"", b"No such container")
            return subprocess.CompletedProcess(command, 0)
        with patch("run_ets.subprocess.run", side_effect=execute):
            with self.assertRaises(subprocess.TimeoutExpired):
                run_owned_container(["docker", "run", "--rm", "image"], self.root, io.BytesIO())
        name = commands[0][commands[0].index("--name") + 1]
        self.assertTrue(name.startswith("tegola-ogc-ets-"))
        self.assertEqual(commands[1], ["docker", "stop", "--time", "5", name])
        self.assertEqual(commands[2], ["docker", "rm", "--force", name])
        self.assertEqual(commands[3], ["docker", "container", "inspect", name])

    def test_retained_container_invalidates_result(self):
        with patch("run_ets.subprocess.run", return_value=subprocess.CompletedProcess([], 0, b"[]", b"")):
            with self.assertRaises(ValueError):
                run_owned_container(["docker", "run", "--rm", "image"], self.root, io.BytesIO())

    def test_receipt_bounded_read(self):
        path = self.root / "receipt.json"
        path.write_bytes(b" " * 33)
        with self.assertRaises(ValueError):
            read_json(path, 32)

    def test_json_limits_before_parse(self):
        self.assertEqual(bounded_json(b'{"value":"[quoted]"}'), {"value": "[quoted]"})
        for raw in (b"[" * 65 + b"0" + b"]" * 65, b" " * ((4 << 20) + 1), b'"\xff"'):
            with self.assertRaises(ValueError):
                bounded_json(raw)

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)

    def report(self, change=None):
        root = ET.Element("testng-results")
        inventory = json.loads(Path(__file__).with_name("required-ets-1.9.json").read_text())
        for entry in inventory["methods"]:
            cls = ET.SubElement(root, "class", name=entry["class"])
            ET.SubElement(cls, "test-method", name=entry["name"], status="PASS")
        if change:
            change(root)
        path = self.root / "report.xml"
        ET.ElementTree(root).write(path, encoding="utf-8")
        return path

    def test_explicit_outcomes(self):
        result = classify(self.report(), crs_selected=True)
        self.assertEqual(result["result"], "PASS")
        self.assertEqual(result["counts"]["FAIL"], 0)

    def test_fail_is_not_process_success(self):
        path = self.report(lambda r: next(r.iter("test-method")).set("status", "FAIL"))
        self.assertEqual(classify(path)["result"], "FAIL")

    def test_required_skip_blocks(self):
        path = self.report(lambda r: next(r.iter("test-method")).set("status", "SKIP"))
        self.assertEqual(classify(path)["result"], "FAIL")

    def test_missing_method_and_config_only_rejected(self):
        path = self.report(lambda r: r.remove(list(r)[0]))
        with self.assertRaises(ValueError):
            classify(path)
        path.write_text('<testng-results><class><test-method status="PASS" is-config="true"/></class></testng-results>')
        with self.assertRaises(ValueError):
            classify(path)

    def test_optional_skip_remains_skip(self):
        def change(root):
            next(x for x in root.iter("test-method") if x.get("name") ==
                 "validateFeaturesResponse_NumberMatched").set("status", "SKIP")
        result = classify(self.report(change))
        self.assertEqual(result["result"], "PASS")
        self.assertEqual(result["counts"]["SKIP"], 1)

    def test_no_xml_entities_or_utf16(self):
        path = self.root / "bad.xml"
        for raw in [b'<!DOCTYPE x [<!ENTITY secret SYSTEM "file:///untrusted">]><x/>',
                    '<!DOCTYPE x><x/>'.encode("utf-16")]:
            path.write_bytes(raw)
            with self.assertRaises(ValueError):
                classify(path)

    def test_local_uri_boundaries(self):
        self.assertEqual(local_target("http://127.0.0.1:19100/features").port, 19100)
        for uri in ["https://127.0.0.1:19100/features", "http://example.invalid:19100/features",
                    "http://user:fictional@localhost:19100/features", "http://localhost/features",
                    "http://localhost:19100/features?key=value", "http://localhost:19100/\nfeatures"]:
            with self.assertRaises(ValueError):
                local_target(uri)

    def test_fixture_only_new_directory(self):
        directory = self.root / "fixture"
        config = prepare(directory, "core")
        before = (directory / "fixture.gpkg").read_bytes()
        self.assertTrue(config.is_file())
        with self.assertRaises(FileExistsError):
            prepare(directory, "mixed")
        self.assertEqual(before, (directory / "fixture.gpkg").read_bytes())

    def test_every_profile_materializes(self):
        for profile in ("core", "crs", "filter", "mixed"):
            config = prepare(self.root / profile, profile)
            self.assertNotIn("${OGC_GPKG}", config.read_text())


if __name__ == "__main__":
    unittest.main()
