"""Create only a new owned conformance database/config directory."""
import argparse
from contextlib import closing
import hashlib
import json
from pathlib import Path
import sqlite3


def prepare(destination, profile):
    destination = Path(destination).absolute()
    if destination.parent.resolve() != destination.parent:
        raise ValueError("Fixture directory must not traverse a symlink")
    destination.mkdir(parents=True, exist_ok=False)
    root = Path(__file__).resolve().parents[2]
    source = root / "testdata/ogc/conformance"
    database = destination / "fixture.gpkg"
    sql = (source / "fixture.sql").read_bytes()
    with closing(sqlite3.connect(database)) as connection:
        connection.executescript(sql.decode("utf-8"))
        connection.commit()
    template = (source / (profile + ".toml")).read_text(encoding="utf-8")
    # JSON quoting supplies TOML basic-string escaping; all input paths are owned.
    template = template.replace('"${OGC_GPKG}"', json.dumps(str(database)))
    # This is an identifier, not a fetch target or credential URI.
    safe = template.replace("http://www.opengis.net/def/crs/OGC/0/CRS84h", "")
    if "://" in safe:
        raise ValueError("External URI in owned fixture config")
    config = destination / "config.toml"
    config.write_text(template, encoding="utf-8")
    receipt = {"profile": profile, "sql_sha256": hashlib.sha256(sql).hexdigest(),
               "config_sha256": hashlib.sha256(config.read_bytes()).hexdigest(),
               "database_sha256": hashlib.sha256(database.read_bytes()).hexdigest(),
               "scope": "New owned fixture only; no existing database or credentials used"}
    (destination / "fixture-manifest.json").write_text(json.dumps(receipt, indent=2))
    return config


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("directory")
    parser.add_argument("--profile", choices=["core", "crs", "filter", "mixed"], default="core")
    args = parser.parse_args()
    print(prepare(args.directory, args.profile))
