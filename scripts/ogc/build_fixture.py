"""Build the owned CLI and bind its bytes to the exact source inventory."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess

from run_ets import git, source_manifest


def build(output):
    root = Path(__file__).resolve().parents[2]
    output = Path(output).absolute()
    if output.parent.resolve() != output.parent:
        raise ValueError("Build directory must not traverse a symlink")
    output.mkdir(parents=True, exist_ok=False)
    environment = os.environ.copy()
    environment.update(GOTOOLCHAIN="go1.26.9", GOWORK="off", GOFLAGS="", CGO_ENABLED="1")
    sources = source_manifest(root)
    revision = git(root, "rev-parse", "HEAD").decode().strip()
    compiler = subprocess.check_output(["go", "version"], env=environment, cwd=root).decode().strip()
    if "go1.26.9 " not in compiler:
        raise ValueError("Unexpected Go compiler")
    settings = json.loads(subprocess.check_output(["go", "env", "-json", "GOOS", "GOARCH", "CC", "CXX", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS", "GOTOOLCHAIN", "GOWORK", "GOFLAGS", "CGO_ENABLED"], env=environment, cwd=root))
    binary = output / ("tegola.exe" if os.name == "nt" else "tegola")
    command = ["go", "build", "-mod=vendor", "-trimpath", "-o", str(binary), "./cmd/tegola"]
    with (output / "build.log").open("wb") as log:
        subprocess.run(command, cwd=root, env=environment, stdout=log, stderr=subprocess.STDOUT, check=True)
    if sources != source_manifest(root) or revision != git(root, "rev-parse", "HEAD").decode().strip():
        raise ValueError("Source changed during CLI build")
    receipt = {"schema": 1, "result": "PASS", "revision": revision, "sources": sources,
               "binary": str(binary), "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
               "compiler": compiler, "settings": settings, "command": command}
    (output / "build-receipt.json").write_text(json.dumps(receipt, indent=2), encoding="utf-8")
    return receipt


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True)
    build(parser.parse_args().output)
