"""Static owned-fixture ATS checks, distinct from official method outcomes."""
import hashlib
import json
import math
from pathlib import Path
import sqlite3
import tempfile
from contextlib import closing
from urllib.parse import urlencode

MISSING = {
    ("bboxcrs.BBoxCrsParameter", "verifyBboxCrsParameter"),
    ("bboxcrs.BBoxCrsParameter", "verifyBboxCrsParameterWithDefaultCrs"),
    ("bboxcrs.BBoxCrsParameterDefault", "verifyBboxCrsParameterDefault"),
    ("crs.features.FeaturesCrsParameterTransform", "verifyFeaturesCrsParameterTransformWithCrsParameter"),
}
PREFIX = "org.opengis.cite.ogcapifeatures10.conformance.crs.query."
CRS84 = "http://www.opengis.net/def/crs/OGC/1.3/CRS84"
EPSG = "http://www.opengis.net/def/crs/EPSG/0/"
_VALIDATED = object()


class Evidence(dict):
    def __init__(self, value, authority):
        if authority is not _VALIDATED:
            raise ValueError("Only executed supplementary checks produce evidence")
        super().__init__(value)


def verify_dataset(dataset, sql):
    if dataset.stat().st_size > 1 << 20:
        raise ValueError("Owned literal fixture exceeds size budget")
    def schema(connection):
        return connection.execute("SELECT type,name,tbl_name,sql FROM sqlite_master ORDER BY type,name").fetchmany(32)
    with tempfile.TemporaryDirectory(prefix="tegola-ets-oracle-") as temporary:
        with closing(sqlite3.connect(str(Path(temporary) / "expected.gpkg"))) as expected:
            expected.executescript(sql.read_bytes().decode("utf-8"))
            with closing(sqlite3.connect(dataset.as_uri() + "?mode=ro", uri=True)) as actual:
                if schema(expected) != schema(actual):
                    raise ValueError("Dataset differs from independently reconstructed literal SQL")
                budget = [0]
                def bounded_work():
                    budget[0] += 1
                    return int(budget[0] > 100)
                actual.set_progress_handler(bounded_work, 1000)
                for name in ("gpkg_spatial_ref_sys", "xy", "xyz", "bare"):
                    wanted = expected.execute('SELECT * FROM "' + name + '" ORDER BY rowid').fetchall()
                    rows = actual.execute('SELECT * FROM "' + name + '" ORDER BY rowid LIMIT ?', (len(wanted) + 1,)).fetchall()
                    if wanted != rows:
                        raise ValueError("Dataset differs from independently reconstructed literal SQL")


def geometry_equal(actual, expected):
    if expected is None:
        return actual is None
    if isinstance(expected, dict):
        return isinstance(actual, dict) and actual.keys() == expected.keys() and all(geometry_equal(actual[key], value) for key, value in expected.items())
    if isinstance(expected, list):
        return isinstance(actual, list) and len(actual) == len(expected) and all(geometry_equal(a, b) for a, b in zip(actual, expected))
    if isinstance(expected, (int, float)):
        return type(actual) in (int, float) and math.isfinite(actual) and abs(actual - expected) <= 1e-7
    return actual == expected


def execute(landing, fetch, parse, collection_metadata, dataset, config, root, output):
    dataset = dataset.resolve()
    verify_dataset(dataset, root / "testdata/ogc/conformance/fixture.sql")
    oracles = json.loads(Path(__file__).with_name("supplement-oracles.json").read_text())
    with dataset.with_name("fixture-manifest.json").open("rb") as stream:
        fixture_raw = stream.read((4 << 20) + 1)
    fixture = parse(fixture_raw)
    if fixture.get("profile") != "core" or fixture.get("database_sha256") != hashlib.sha256(dataset.read_bytes()).hexdigest() or fixture.get("sql_sha256") != hashlib.sha256((root / "testdata/ogc/conformance/fixture.sql").read_bytes()).hexdigest():
        raise ValueError("CRS supplementation requires the exact owned XY fixture")
    template = (root / "testdata/ogc/conformance/core.toml").read_text().replace('"${OGC_GPKG}"', json.dumps(str(dataset)))
    if config.read_text() != template:
        raise ValueError("Supplement fixture configuration differs from literal profile")
    collections = parse(collection_metadata)["collections"]
    if len(collections) != 1 or collections[0]["id"] != "xy" or "extent" in collections[0]:
        raise ValueError("Optional extent limitation does not apply to this metadata")
    checks = []
    def request(path, parameters):
        uri = landing + "/collections/xy/" + path + "?" + urlencode({"f": "json", **parameters})
        status, raw, headers = fetch(uri)
        if status != 200:
            raise ValueError("Supplement request failed")
        value = parse(raw)
        body_name = "supplement-response-" + str(len(checks)) + ".json"
        (output / body_name).write_bytes(raw)
        checks.append({"uri": uri, "status": status, "body": body_name, "body_sha256": hashlib.sha256(raw).hexdigest(), "content_crs": headers.get("Content-Crs")})
        expected_crs = parameters.get("crs", CRS84)
        if headers.get("Content-Crs") != "<" + expected_crs + ">":
            raise ValueError("Supplement Content-Crs differs")
        return value
    expected = [10, 30, 40, 50, 60]
    def check_geometry(features, mode):
        for feature in features:
            if "geometry" not in feature or not geometry_equal(feature["geometry"], oracles[mode][str(feature["id"])]):
                raise ValueError("Full literal supplementary geometry differs")
    queries = [
        {"bbox": "14.5,29.5,15.5,30.5"},
        {"bbox": "14.5,29.5,15.5,30.5", "bbox-crs": CRS84},
        {"bbox": "29.5,14.5,30.5,15.5", "bbox-crs": EPSG + "4326"},
        {"bbox": "1614132,3439440,1725452,3567983", "bbox-crs": EPSG + "3857"},
    ]
    results = []
    for query in queries:
        value = request("items", {"limit": "10000", **query})
        if [f["id"] for f in value["features"]] != expected or value.get("numberMatched") != 5:
            raise ValueError("Literal query-frame supplementary membership differs")
        results.append(value["features"])
        check_geometry(value["features"], "default")
    if results[0] != results[1]:
        raise ValueError("Absent and explicit default bbox differ")
    for code, coordinates in (("4326", [30, 15]), ("3857", [1669792.3618991037, 3503549.8435043753])):
        value = request("items/10", {"crs": EPSG + code})
        geometry = value.get("geometry", {})
        actual = geometry.get("coordinates", [])
        if geometry.get("type") != "Point" or len(actual) != 2 or any(type(a) not in (int, float) or not math.isfinite(a) or abs(a - b) > 1e-7 for a, b in zip(actual, coordinates)):
            raise ValueError("Literal output axis/coordinate supplementary oracle differs")
        collection = request("items", {"limit": "10000", "bbox": "14.5,29.5,15.5,30.5", "crs": EPSG + code})
        if [f["id"] for f in collection["features"]] != expected or collection.get("numberMatched") != 5 or collection["features"][0]["geometry"] != geometry:
            raise ValueError("Requested output changed literal collection membership")
        check_geometry(collection["features"], code)
    page = request("items", {"limit": "2", "offset": "4", "bbox": "1614132,3439440,1725452,3567983", "bbox-crs": EPSG + "3857"})
    if [f["id"] for f in page["features"]] != [60] or page.get("numberMatched") != 5:
        raise ValueError("Exact query-frame filtering must precede paging")
    check_geometry(page["features"], "default")
    result = {"result": "PASS", "profile": "core", "metadata_sha256": hashlib.sha256(collection_metadata).hexdigest(),
              "dataset_sha256": hashlib.sha256(dataset.read_bytes()).hexdigest(), "checks": checks,
              "missing_methods": [{"class": PREFIX + cls, "name": name, "outcome": "SUPPLEMENTED_TOOL_LIMITATION"}
                                  for cls, name in sorted(MISSING)],
              "reason": "ETS1.9 default-map initialization requires absent optional extent; official counts unchanged"}
    (output / "normative-supplement.json").write_text(json.dumps(result, indent=2))
    return Evidence(result, _VALIDATED)
