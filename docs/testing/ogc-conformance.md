# Reproducing OGC verification

This runner uses the actual publication's conformance document. It does not add
classes, enable unsupported providers, or certify the product. Part 3/CQL2
endpoints remain implemented; their global classes are withheld until an
appropriate approved executable validator is available.

## Requirements

- Go 1.26.9, CGO and a C compiler for the owned GeoPackage fixture.
- Python 3.11 or later with its standard library, and Docker.
- Official immutable image:
  `ogccite/ets-ogcapi-features10@sha256:a1c1345acff1f671a6ca6aefc36a85551c855223973aff3d40c4e75611c7e55e`.

The bundled ETS is version 1.9 at upstream commit
`e7c8a96deff936f7ae9c4cbbafb900ce65371d2c`, using TEAM Engine 6.0.0-RC2
and Java Temurin 17.0.13+11. It tests Parts 1 and 2. Its original-edition tests
do not establish all corrigendum, CRS84h, application-URI or dimensional behavior.
Those require the independently reviewed supplementary runtime tests.

Use the `core` XY profile for official execution. The retained `crs` profile
exposes genuine XYZ capabilities for independent literal ATS checks. ETS 1.9's
Proj4J transformer cannot resolve EPSG:4979; its raw failed diagnostic remains
tool evidence and receives no official pass credit.

The suite initializes default CRS data only when optional collection extent
metadata exists. This publication does not supply extent metadata. Four exact
parameterized methods therefore have no official executions. The runner checks
their requirements independently on the committed literal XY fixture, preserving
the required inventory and original official counts. It reports
`SUPPLEMENTED_TOOL_LIMITATION` for these missing methods and
`PASS_WITH_SUPPLEMENTS` for successful combined evidence. Additional missing
methods or ordinary assertion failures still reject the run. These results are
distinct from full official CRS execution or certification.

Sources: [official ETS](https://github.com/opengeospatial/ets-ogcapi-features10),
[OGC Features standards and validator coverage](https://github.com/opengeospatial/ogcapi-features),
[Part 3 normative ATS](https://docs.ogc.org/is/19-079r2/19-079r2.html),
[CQL2 normative ATS](https://docs.ogc.org/is/21-065r2/21-065r2.html).

## Owned fixture and real server

Use a new directory. The fixture generator refuses existing directories before
connecting to SQLite; it never uses a user database. Configurations and literal
SQL are committed under `testdata/ogc/conformance/`.

```sh
python3 scripts/ogc/build_fixture.py --output /tmp/tegola-ogc-build
python3 scripts/ogc/prepare_fixture.py /tmp/tegola-ogc-fixture --profile core
/tmp/tegola-ogc-build/tegola serve --config /tmp/tegola-ogc-fixture/config.toml
```

On Windows invoke `python` and the generated executable directly. The Python
runner is the same implementation; the shell launcher is only a convenience.
The configured server port is 19100. Docker must reach it through
`host.docker.internal:19100`; this isolated test server must not contain private
data. A container on Linux may require the documented host-gateway mapping.

Profiles:

| Profile | Purpose |
|---|---|
| `core` | Explicit XY raw geometry, scalar properties and mapped time |
| `crs` | Explicit XY and height-preserving XYZ source profiles |
| `filter` | Scalar filtering regression fixture; no official Part3 claim |
| `mixed` | Proven XY plus a Core-only source without CRS proof or scalar properties; verifies global extension suppression |

Conformance classes are a global intersection over the frozen published
collections and independently admitted implementation profiles. Optional
endpoints and collection links can exist without a global extension declaration.
A valid empty Queryables catalog is distinct from a missing capability, but does
not satisfy the Basic CQL2 ATS nonempty scalar dataset precondition.

## Official run

```sh
docker pull ogccite/ets-ogcapi-features10@sha256:a1c1345acff1f671a6ca6aefc36a85551c855223973aff3d40c4e75611c7e55e
sh scripts/ogc/run-ets.sh \
  --iut http://127.0.0.1:19100/features \
  --output /tmp/tegola-ogc-report \
  --build-receipt /tmp/tegola-ogc-build/build-receipt.json \
  --config /tmp/tegola-ogc-fixture/config.toml \
  --dataset /tmp/tegola-ogc-fixture/fixture.gpkg \
  --binary /tmp/tegola-ogc-build/tegola --pid SERVER_PID
```

The output directory must be new. The supplied binary/config/dataset provenance
is recorded and checked for drift. The runner verifies the supplied running PID
has the exact executable path and explicit `--config` argument, before and after
the suite. Process verification supports Windows and Linux; command lines are
not retained in reports. Passing paths without a matching process is rejected.
Only credential-free loopback HTTP targets with explicit ports are accepted;
redirects are rejected. The Docker URI is derived with the same port and path.

The runner requires actual independently admitted Core and GeoJSON declarations.
An empty declaration is an invalid final conformance target, rather than a
passing no-test run. Part2 assertions are required only when its global class is
actually declared. Part3/CQL2 is not evaluated by this official suite.

## Evidence and outcomes

The run preserves the raw product declaration, official properties XML,
runner log, TestNG reports, exact Git commit and product-file hashes, effective
config/dataset/binary hashes, pinned image ID/digest and suite provenance.
`receipt.json` contains explicit PASS, FAIL and SKIP counts and every
parameterized assertion. Docker exit zero alone does not mean verification PASS.

Exit status 0 means selected official assertions pass with separately recorded
optional skips; 1 means a selected semantic failure; 2 means invalid inputs,
infrastructure, missing required coverage or source drift. No previous report is
overwritten. DTD/entities and oversized reports are rejected.

Optional `numberMatched` and `timeStamp` skips remain skips. Unselected extension
tests remain unselected. A required skipped assertion blocks its claim unless
an explicitly named, independently reviewed supplement covers that requirement.
The invalid/unlisted bbox-crs HTTP400 supplement is recorded separately when
Part2 is selected; it never increases the official PASS count. Reports do not
authorize OGC certification or Reference Implementation status.

Runner negative controls:

```sh
python3 -m unittest discover -s scripts/ogc -p 'test_*.py' -v
```

## See also

- [API](../api.md)
- [Filtering](../filtering.md)
- [CRS](../crs.md)
- [Development](../development.md)
