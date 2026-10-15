"""Classify the unmodified official TestNG report, including every skip."""
import collections
import json
from pathlib import Path
import xml.etree.ElementTree as ET
from supplement import Evidence, MISSING, PREFIX

MAX_REPORT_BYTES = 32 << 20
OPTIONAL = {
    "validateFeaturesResponse_NumberMatched", "validateFeaturesWithLimitResponse_NumberMatched",
    "validateFeaturesResponse_TimeStamp", "validateFeaturesWithLimitResponse_TimeStamp",
}


def classify(path, crs_selected=False, invalid_bbox_supplement=False, normative=None):
    path = Path(path)
    if path.stat().st_size > MAX_REPORT_BYTES:
        raise ValueError("Official XML exceeds report budget")
    raw = path.read_bytes()
    if b"\0" in raw or b"<!DOCTYPE" in raw.upper() or b"<!ENTITY" in raw.upper():
        raise ValueError("XML declarations/entities are forbidden")
    root = ET.fromstring(raw.decode("utf-8-sig"))
    cases = []
    for cls in root.iter("class"):
        for case in cls.findall("test-method"):
            if case.get("is-config") == "true":
                continue
            status = case.get("status")
            if status not in {"PASS", "FAIL", "SKIP"}:
                raise ValueError("Unknown official outcome")
            cases.append({"class": cls.get("name", ""), "name": case.get("name", ""),
                          "status": status, "parameters": ["".join(v.itertext()).strip()
                          for v in case.findall("params/param")],
                          "message": (case.findtext("exception/message") or "").strip()})
    if not cases:
        raise ValueError("No test outcomes in official report")
    counts = {name: sum(c["status"] == name for c in cases) for name in ("PASS", "FAIL", "SKIP")}
    selected = [c for c in cases if ".conformance.core." in c["class"] or
                (crs_selected and ".conformance.crs." in c["class"])]
    if not any(c["status"] == "PASS" and ".conformance.core." in c["class"] for c in selected):
        raise ValueError("No successful selected Core assertions")
    if crs_selected and not any(c["status"] == "PASS" and ".conformance.crs." in c["class"] for c in selected):
        raise ValueError("No successful selected CRS assertions")
    inventory = json.loads(Path(__file__).with_name("required-ets-1.9.json").read_text())
    expected = {(x["class"], x["name"]) for x in inventory["methods"]
                if ".conformance.core." in x["class"] or
                (crs_selected and ".conformance.crs." in x["class"])}
    observed = {(x["class"], x["name"]) for x in selected}
    missing = expected - observed
    allowed = {(PREFIX + cls, name) for cls, name in MISSING}
    supplemented = []
    if missing:
        if not isinstance(normative, Evidence) or normative.get("result") != "PASS" or normative.get("profile") != "core" or not missing.issubset(allowed) or \
                {(x["class"], x["name"]) for x in normative.get("missing_methods", [])} != allowed:
            raise ValueError("Official report lacks required selected methods")
        supplemented = [{"class": cls, "name": name, "outcome": "SUPPLEMENTED_TOOL_LIMITATION"} for cls, name in sorted(missing)]
    blocking = []
    for case in selected:
        if case["status"] == "FAIL":
            blocking.append(case)
        elif case["status"] == "SKIP" and case["name"] not in OPTIONAL:
            if case["name"] == "verifyBboxCrsParameterInvalid" and invalid_bbox_supplement and \
                    case["class"] == "org.opengis.cite.ogcapifeatures10.conformance.crs.query.bboxcrs.BBoxCrsParameterInvalid":
                continue
            blocking.append(case)
    by_class = {name: dict(collections.Counter(c["status"] for c in cases if c["class"] == name))
                for name in sorted({c["class"] for c in cases})}
    return {"counts": counts, "classes": by_class, "cases": cases, "blocking": blocking,
            "result": "FAIL" if blocking else ("PASS_WITH_SUPPLEMENTS" if supplemented else "PASS"),
            "official_missing": supplemented, "normative_supplement": normative,
            "scope": "Official selected Core/GeoJSON/HTML/OAS and advertised Part2 assertions; skips remain distinct"}
