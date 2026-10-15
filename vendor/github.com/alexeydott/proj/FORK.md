# Tegola dependency fork

Published module: github.com/alexeydott/proj.

Based on go-spatial/proj v0.3.0, with the audited
Tegola dependency snapshot from alexeydott/tegola commit f1111b316ba50387c47b924daced47d641beb1e5.
Upstream copyright and license notices are retained. The module path and all
self-imports use the alexeydott namespace; callers must migrate imports.

Includes cache invalidation, thread-safe registration, projected and geographic three/seven-parameter datum shifts, validation and regression tests.

Source of truth: this repository. Tegola consumes tagged versions and vendors
them for offline builds. Change this repository, test it, publish a new tag,
then update Tegola requirements and vendor; do not edit Tegola vendor alone.

Run go test -mod=readonly ./... before publishing.
