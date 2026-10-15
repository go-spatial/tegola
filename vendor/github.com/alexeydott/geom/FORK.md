# Tegola dependency fork

Published module: github.com/alexeydott/geom.

Based on go-spatial/geom v0.1.0, with the audited
Tegola dependency snapshot from alexeydott/tegola commit f1111b316ba50387c47b924daced47d641beb1e5.
Upstream copyright and license notices are retained. The module path and all
self-imports use the alexeydott namespace; callers must migrate imports.

Includes bounded WKB decoding, degenerate MVT geometry handling, protobuf APIv2 code generation and regression tests. Depends directly on the versioned alexeydott/proj fork.

Source of truth: this repository. Tegola consumes tagged versions and vendors
them for offline builds. Change this repository, test it, publish a new tag,
then update Tegola requirements and vendor; do not edit Tegola vendor alone.

Run go test -mod=readonly ./... before publishing.

Standalone upstream tests were restored and adapted to the current slippy and
Go JSON APIs. Sweep events now have deterministic ordering at equal endpoints.
The native SpatiaLite recorder test skips explicitly when that optional
extension is absent; this does not qualify native recorder integration.
