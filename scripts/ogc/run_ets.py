"""Run pinned official ETS against a credential-free owned local publication."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tomllib
import uuid
from urllib.error import HTTPError
from urllib.parse import quote, urlsplit, urlunsplit, urlencode
from urllib.request import HTTPRedirectHandler, ProxyHandler, Request, build_opener
import xml.etree.ElementTree as ET

from report import classify
from provenance import process_identity
from supplement import execute as execute_supplement

IMAGE = "ogccite/ets-ogcapi-features10@sha256:a1c1345acff1f671a6ca6aefc36a85551c855223973aff3d40c4e75611c7e55e"
SUITE = {"version": "1.9", "commit": "e7c8a96deff936f7ae9c4cbbafb900ce65371d2c",
         "jar_sha256": "9312ebd089c3e196bcc450ddbf38cad16a0439b1b164c6745fb46a0b13c54444"}
CORE = "http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/core"
GEOJSON = "http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/geojson"
CRS = "http://www.opengis.net/spec/ogcapi-features-2/1.0/conf/crs"
MAX_JSON_BYTES = 4 << 20
JAR = "/usr/local/tomcat/webapps/teamengine/WEB-INF/lib/ets-ogcapi-features10-1.9.jar"


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ValueError("Redirect forbidden for local conformance target")


def local_target(value):
    if len(value) > 8192 or any(ord(char) < 32 or ord(char) == 127 for char in value):
        raise ValueError("Invalid local URI bytes")
    uri = urlsplit(value)
    if uri.scheme != "http" or uri.hostname not in {"127.0.0.1", "localhost"} or \
            uri.username or uri.password or uri.query or uri.fragment or not uri.port:
        raise ValueError("Expected credential-free explicit-port loopback HTTP landing URI")
    return uri


def fetch(value, include_headers=False):
    request = Request(value, headers={"Accept": "application/json"})
    opener = build_opener(ProxyHandler({}), NoRedirect())
    try:
        response = opener.open(request, timeout=30)
    except HTTPError as error:
        response = error
    with response:
        raw = response.read(MAX_JSON_BYTES + 1)
        if len(raw) > MAX_JSON_BYTES:
            raise ValueError("Preflight response exceeds budget")
        if include_headers:
            return response.status, raw, response.headers
        return response.status, raw


def git(root, *arguments):
    return subprocess.check_output(["git", *arguments], cwd=root)


def invalid_bbox_uri(landing, collection_id):
    return landing + "/collections/" + quote(collection_id, safe="") + "/items?" + urlencode({"f": "json", "bbox": "0,0,1,1", "bbox-crs": "urn:invalid:unlisted"})


def run_owned_container(command, output, log):
    name = "tegola-ogc-ets-" + uuid.uuid4().hex
    cidfile = output / (name + ".cid")
    owned = [*command[:3], "--name", name, "--cidfile", str(cidfile), *command[3:]]
    try:
        return subprocess.run(owned, stdout=log, stderr=subprocess.STDOUT, timeout=1800)
    finally:
        # A client timeout does not terminate the daemon-owned container.
        # This unguessable run-specific name is never shared with other runs.
        for action in (["stop", "--time", "5", name], ["rm", "--force", name]):
            try:
                subprocess.run(["docker", *action], stdout=log, stderr=subprocess.STDOUT, timeout=20)
            except (OSError, subprocess.TimeoutExpired):
                log.write(b"Owned ETS container cleanup command failed; inspect this run name.\n")
                log.flush()
        inspection = subprocess.run(["docker", "container", "inspect", name], stdout=subprocess.PIPE,
                                    stderr=subprocess.PIPE, timeout=20)
        if inspection.returncode == 0 or b"No such" not in inspection.stderr:
            raise ValueError("Owned ETS container cleanup could not be verified")


def bounded_json(raw, limit=MAX_JSON_BYTES):
    if len(raw) > limit:
        raise ValueError("Preflight JSON exceeds budget")
    depth = 0
    quoted = False
    escaped = False
    for char in raw.decode("utf-8"):
        if quoted:
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif char == '"':
                quoted = False
        elif char == '"':
            quoted = True
        elif char in "[{":
            depth += 1
            if depth > 64:
                raise ValueError("Preflight JSON depth exceeds budget")
        elif char in "]}":
            depth -= 1
    def invalid_constant(value):
        raise ValueError("Nonfinite JSON constant forbidden")
    return json.loads(raw, parse_constant=invalid_constant)


def read_json(path, limit):
    with path.open("rb") as stream:
        raw = stream.read(limit + 1)
    return raw, bounded_json(raw, limit)


def source_manifest(root):
    names = git(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard").decode().split("\0")
    names = [n for n in names if n and n != "testdata/postgis/migration.sh" and
             not any(part in {".ai-factory", ".agents", ".codex", "AGENTS.md"} for part in n.split("/")) and
             Path(n).suffix.lower() not in {".gpkg", ".env", ".pem", ".key", ".exe"}]
    for name in names:
        path = root / name
        if path.is_symlink() or (path.exists() and not path.resolve().is_relative_to(root.resolve())):
            raise ValueError("Source manifest must not traverse external paths")
    return {n: hashlib.sha256((root / n).read_bytes()).hexdigest() for n in sorted(names) if (root / n).is_file()}


def verify_build(receipt, binary, sources, revision):
    settings = receipt.get("settings", {})
    if receipt.get("schema") != 1 or receipt.get("result") != "PASS" or \
            receipt.get("sources") != sources or receipt.get("revision") != revision or \
            receipt.get("binary") != str(binary) or \
            receipt.get("binary_sha256") != hashlib.sha256(binary.read_bytes()).hexdigest() or \
            "go1.26.9 " not in receipt.get("compiler", "") or \
            any(settings.get(key) != value for key, value in {"GOTOOLCHAIN": "go1.26.9", "GOWORK": "off", "GOFLAGS": "", "CGO_ENABLED": "1"}.items()) or \
            receipt.get("command") != ["go", "build", "-mod=vendor", "-trimpath", "-o", str(binary), "./cmd/tegola"]:
        raise ValueError("CLI build receipt does not bind the current candidate")


def verify_revision(root, verified_revision):
    if git(root, "rev-parse", "HEAD").decode().strip() != verified_revision:
        raise ValueError("Checkout revision changed during official run")


def run(args):
    target = local_target(args.iut)
    root = Path(__file__).resolve().parents[2]
    output = Path(args.output).absolute()
    if any(char in str(output) for char in (",", "\r", "\n")):
        raise ValueError("Invalid Docker mount path")
    if output.parent.resolve() != output.parent:
        raise ValueError("Report directory must not traverse a symlink")
    output.mkdir(parents=True, exist_ok=False)
    inputs = [Path(args.config).resolve(), Path(args.dataset).resolve(), Path(args.binary).resolve()]
    configuration = tomllib.loads(inputs[0].read_text(encoding="utf-8"))
    providers = configuration.get("providers", [])
    if not providers or any(p.get("type") != "gpkg" or
                            Path(p.get("filepath", "")).resolve() != inputs[1] or
                            any(key in p for key in ("password", "user", "uri", "host")) for p in providers):
        raise ValueError("Only the supplied owned GeoPackage may be configured")
    if configuration.get("features", {}).get("basepath", "/features").rstrip("/") != target.path.rstrip("/"):
        raise ValueError("Landing path differs from installed feature configuration")
    process_before = process_identity(args.pid, inputs[2], inputs[0], target.port)
    input_hashes = {str(p): hashlib.sha256(p.read_bytes()).hexdigest() for p in inputs}
    sources = source_manifest(root)
    build_path = Path(args.build_receipt).resolve()
    build_raw, build_receipt = read_json(build_path, 8 << 20)
    verified_revision = git(root, "rev-parse", "HEAD").decode().strip()
    verify_build(build_receipt, inputs[2], sources, verified_revision)
    landing = args.iut.rstrip("/")
    status, raw = fetch(landing + "/conformance?f=json")
    (output / "product-conformance.json").write_bytes(raw)
    if status != 200:
        raise ValueError("Conformance preflight failed")
    declared = bounded_json(raw).get("conformsTo")
    if not isinstance(declared, list) or any(not isinstance(x, str) for x in declared):
        raise ValueError("Invalid raw conformance list")
    if CORE not in declared or GEOJSON not in declared:
        raise ValueError("Actual publication must declare independently admitted Core and GeoJSON")
    docker_uri = urlunsplit((target.scheme, "host.docker.internal:" + str(target.port), target.path, "", ""))
    properties = ET.Element("properties", version="1.0")
    ET.SubElement(properties, "entry", key="iut").text = docker_uri
    ET.ElementTree(properties).write(output / "test-run-props.xml", encoding="utf-8", xml_declaration=True)
    image = json.loads(subprocess.check_output(["docker", "image", "inspect", IMAGE]))[0]
    if IMAGE.split("@", 1)[1] not in [x.split("@", 1)[-1] for x in image.get("RepoDigests", [])]:
        raise ValueError("Pinned suite digest not present in inspected image")
    with (output / "jar-check.log").open("wb") as jar_log:
        jar_process = run_owned_container(["docker", "run", "--rm", "--entrypoint", "sha256sum", IMAGE, JAR], output, jar_log)
    if jar_process.returncode != 0:
        raise ValueError("Official JAR verification infrastructure failed")
    actual_jar = (output / "jar-check.log").read_text().split()[0]
    if actual_jar != SUITE["jar_sha256"]:
        raise ValueError("Bundled official JAR hash mismatch")
    reports = output / "reports"; reports.mkdir()
    command = ["docker", "run", "--rm", "--mount", f"type=bind,source={output},target=/input,readonly",
               "--mount", f"type=bind,source={reports},target=/reports", "--entrypoint", "java", IMAGE,
               "-cp", "/usr/local/tomcat/webapps/teamengine/WEB-INF/lib/*:/usr/local/tomcat/lib/*",
               "org.opengis.cite.ogcapifeatures10.TestNGController", "-o", "/reports", "-h", "true", "/input/test-run-props.xml"]
    with (output / "runner.log").open("wb") as log:
        process = run_owned_container(command, output, log)
    xmls = list(reports.rglob("testng-results.xml"))
    if process.returncode != 0 or len(xmls) != 1:
        raise ValueError("Official runner infrastructure/report failure")
    supplement = False
    if CRS in declared:
        status, raw = fetch(landing + "/collections?f=json")
        if status != 200: raise ValueError("Collection preflight failed")
        collections = bounded_json(raw)["collections"]
        supplemental = []
        for collection in collections:
            # IDs are URL path segments, never caller SQL or shell text.
            uri = invalid_bbox_uri(landing, collection["id"])
            code, body = fetch(uri)
            supplemental.append({"id": collection["id"], "status": code, "body_sha256": hashlib.sha256(body).hexdigest()})
        supplement = bool(supplemental) and all(x["status"] == 400 for x in supplemental)
        (output / "invalid-bbox-crs-supplement.json").write_text(json.dumps(supplemental, indent=2))
    normative = None
    if CRS in declared:
        normative = execute_supplement(landing, lambda uri: fetch(uri, True), bounded_json, raw, inputs[1], inputs[0], root, output)
    result = classify(xmls[0], CRS in declared, supplement, normative)
    if source_manifest(root) != sources or any(hashlib.sha256(p.read_bytes()).hexdigest() != input_hashes[str(p)] for p in inputs):
        raise ValueError("Source/config/dataset/binary drift invalidates evidence")
    if process_identity(args.pid, inputs[2], inputs[0], target.port) != process_before:
        raise ValueError("Fixture process identity changed during official run")
    verify_revision(root, verified_revision)
    result.update({"commit": verified_revision, "sources": sources,
                   "build_receipt_sha256": hashlib.sha256(build_raw).hexdigest(), "build": build_receipt,
                   "inputs": input_hashes, "process": process_before, "image": IMAGE, "image_id": image["Id"], "suite": SUITE,
                   "declared": declared, "iut": args.iut, "docker_iut": docker_uri,
                   "reports": {str(p.relative_to(output)): hashlib.sha256(p.read_bytes()).hexdigest()
                               for p in reports.rglob("*") if p.is_file()},
                   "scope_note": "No declaration mutation; Part3/CQL2 not verified by this official suite"})
    (output / "receipt.json").write_text(json.dumps(result, indent=2))
    print(json.dumps({"result": result["result"], "counts": result["counts"], "output": str(output)}))
    return 0 if result["result"] in {"PASS", "PASS_WITH_SUPPLEMENTS"} else 1


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    for name in ("iut", "output", "config", "dataset", "binary", "build-receipt"):
        parser.add_argument("--" + name, required=True)
    parser.add_argument("--pid", required=True, type=int)
    arguments = parser.parse_args()
    try:
        sys.exit(run(arguments))
    except Exception as error:
        print("INVALID: " + str(error), file=sys.stderr)
        sys.exit(2)
