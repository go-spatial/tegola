"""Prepare owned migration data or check an already started ordinary CLI."""
import argparse
from contextlib import closing
import hashlib
import json
from pathlib import Path
import sqlite3
from urllib.parse import urlsplit
from urllib.request import build_opener, ProxyHandler, Request

from run_ets import bounded_json, local_target, NoRedirect


def exact_value(actual, expected):
    return json.dumps(actual, sort_keys=True, allow_nan=False) == json.dumps(
        expected, sort_keys=True, allow_nan=False)


def prepare(destination):
    destination = Path(destination).absolute()
    if destination.parent.resolve() != destination.parent:
        raise ValueError("Fixture parent must not traverse a symlink")
    destination.mkdir(parents=True, exist_ok=False)
    source = Path(__file__).resolve().parents[2] / "testdata/migration/jivan"
    sql = (source / "fixture.sql").read_bytes()
    database = destination / "fixture.gpkg"
    with closing(sqlite3.connect(database)) as connection:
        connection.executescript(sql.decode("utf-8-sig"))
        connection.commit()
    config = destination / "config.toml"
    config.write_text((source / "migrated.toml").read_text().replace(
        '"${OGC_GPKG}"', json.dumps(str(database))), encoding="utf-8")
    return str(config)


def check(origin, output):
    local_target(origin)
    parsed = urlsplit(origin)
    if (parsed.scheme != "http" or parsed.hostname not in ("127.0.0.1", "localhost")
            or not parsed.port or parsed.username or parsed.password
            or parsed.query or parsed.fragment):
        raise ValueError("An owned credential-free loopback origin is required")
    output = Path(output).absolute()
    output.mkdir(parents=True, exist_ok=False)
    expected = json.loads((Path(__file__).resolve().parents[2] /
                           "testdata/migration/jivan/expected.json").read_text(encoding="utf-8-sig"))
    opener = build_opener(ProxyHandler({}), NoRedirect())
    checks = []

    def fetch(path, method="GET", status=200):
        from urllib.error import HTTPError
        try:
            response = opener.open(Request(origin.rstrip("/") + path, method=method), timeout=30)
        except HTTPError as error:
            response = error
        with response:
            body = response.read(4 * 1024 * 1024 + 1)
            if len(body) > 4 * 1024 * 1024 or response.status != status:
                raise ValueError("Unexpected bounded migration response")
            if response.headers.get("Cache-Control") != "no-store":
                raise ValueError("Migration response lost no-store")
            if response.headers.get("ETag") or response.headers.get("Last-Modified"):
                raise ValueError("Feature validator must remain absent")
            if method == "HEAD" and body:
                raise ValueError("HEAD returned a body")
            if status == 200 and "/items" in path:
                identifier = "0/CRS84h" if "/xyz/" in path else "1.3/CRS84"
                if response.headers.get("Content-Crs") != "<http://www.opengis.net/def/crs/OGC/" + identifier + ">":
                    raise ValueError("Literal migrated Content-Crs differs")
            checks.append({"path": path, "method": method, "status": status,
                           "body_sha256": hashlib.sha256(body).hexdigest()})
            return bounded_json(body) if body and "json" in response.headers.get("Content-Type", "") else body

    for path in ("", "/api", "/conformance", "/collections", "/collections/xy",
                 "/collections/xy/items", "/collections/xy/items/10"):
        for form in ("json", "html"):
            fetch(path + "?f=" + form)
            fetch(path + "?f=" + form, "HEAD")
    for suffix, key in (("filter=n%3D1", "filter_n_equals_1"),
                        ("filter=s%3D%27Alpha%27", "filter_s_equals_Alpha"),
                        ("datetime=1970-01-01T00%3A00%3A00Z", "datetime_epoch"),
                        ("bbox=15,30,15,30", "bbox_15_30_point")):
        value = fetch("/collections/xy/items?f=json&limit=100&" + suffix)
        if [feature["id"] for feature in value["features"]] != expected[key]:
            raise ValueError("Literal migration membership differs")
    catalog = fetch("/collections?f=json")
    if sorted(value["id"] for value in catalog["collections"]) != sorted(expected["collections"]):
        raise ValueError("Explicit migration publication differs")
    xyz = fetch("/collections/xyz/items?f=json&limit=100")
    if [value["id"] for value in xyz["features"]] != expected["all_xyz_ids"]:
        raise ValueError("Literal XYZ membership differs")
    for collection, identifier, key in (("xy", 10, "item_10"),
                                        ("xyz", 10, "xyz_item_10"),
                                        ("xy", 40, "null_item_40")):
        value = fetch(f"/collections/{collection}/items/{identifier}?f=json")
        literal = expected[key]
        if any(name not in value or not exact_value(value[name], literal[name]) for name in literal):
            raise ValueError("Literal migrated feature differs")
    all_items = fetch("/collections/xy/items?f=json&limit=100")
    if ([item["id"] for item in all_items["features"]] != expected["all_xy_ids"]
            or all_items.get("numberMatched") != 6 or all_items.get("numberReturned") != 6):
        raise ValueError("Exhausted migration membership/count differs")
    for item in all_items["features"]:
        if "geometry" not in item or not exact_value(item["geometry"], expected["xy_geometries"][str(item["id"]) ]):
            raise ValueError("Literal full migrated geometry differs")
    first = fetch("/collections/xy/items?f=json&limit=2")
    if [item["id"] for item in first["features"]] != expected["first_page"]:
        raise ValueError("First migration page differs")
    next_links = [link["href"] for link in first["links"] if link["rel"] == "next"]
    if len(next_links) != 1 or not next_links[0].startswith(origin.rstrip("/") + "/"):
        raise ValueError("Unsafe or missing genuine next link")
    second = fetch(next_links[0][len(origin.rstrip("/")):])
    if [item["id"] for item in second["features"]] != expected["second_page"]:
        raise ValueError("Next-link migration page differs")
    untimed = fetch("/collections/untimed/items?f=json&limit=100&datetime=2099-01-01T00%3A00%3A00Z")
    if [item["id"] for item in untimed["features"]] != expected["all_xy_ids"]:
        raise ValueError("Unmapped temporal geometry must match all")
    fetch("/collections/untimed/items?f=json&datetime=bad", status=400)
    for suffix in expected["invalid_old_parameters"]:
        fetch("/collections/xy/items?" + suffix, status=400)
        fetch("/collections/xy/items?" + suffix, method="HEAD", status=400)
    receipt = {"scope": "Actual ordinary CLI loopback HTTP, no legacy or AWS deployment claim",
               "checks": checks, "result": "PASS"}
    (output / "receipt.json").write_text(json.dumps(receipt, indent=2), encoding="utf-8")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--prepare")
    parser.add_argument("--origin")
    parser.add_argument("--output")
    arguments = parser.parse_args()
    if arguments.prepare:
        print(prepare(arguments.prepare))
    elif arguments.origin and arguments.output:
        check(arguments.origin, arguments.output)
    else:
        parser.error("Use --prepare or --origin with --output")
