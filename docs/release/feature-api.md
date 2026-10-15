[Documentation index](../README.md) · [Migration guide](../migration/jivan-to-tegola.md)

# Feature API release

Tegola replaces Jivan's feature-serving role through its existing server and
provider packages. Use the [compatibility matrix](../migration/jivan-compatibility-matrix.md)
for each legacy endpoint, parameter and deployment disposition. This statement
does not imply identical historical behavior or an OGC certification.

## Historical source release

The annotated source tag `v0.21.0-fork.2` was pushed on 2026-10-02 and binds
`db4e8ee73a3ddfe3b3c8e054c59e833b697dce85`. OGC implementation and the
MOS provider/diagnostic integration are included in this revision. The MOS
source branch ancestry was recorded while retaining the reviewed integration
tree from `8bc78cd4`. This is a source tag; no GitHub Release assets or AWS
deployment are asserted. See [Windows builds](../windows-release.md).

## Current source capabilities

Opt-in publication exposes landing, API definition, conformance, collections,
collection metadata, items and individual items. The implemented profile includes
JSON and HTML, HEAD, link-driven paging, bbox and datetime, Queryables with the
documented CQL2 text subset, and supported public CRS transformations. See the
[API reference](../api.md), [configuration](../configuration.md),
[filtering contract](../filtering.md) and [CRS contract](../crs.md).

GeoPackage, PostGIS, MySQL/MariaDB and HANA publication requires each provider's
documented source identity, metadata and query guarantees. Explicit publication
and admitted capabilities determine the exposed API; configuring a provider alone
does not publish every layer. Existing tile endpoints remain available.

The optional Lambda binary uses the same assembled HTTP router. Its transport
contract covers buffered REST v1, HTTP v2, Function URL v2 and ALB with multi-value
headers enabled. Origin selection, stage mapping and transport limits are defined
in [ADR-0014](../architecture/decisions/ADR-0014-lambda-router-adapter.md).
The stock Lambda entry point binds tiles and read-only Features; WFS, feature writes and tile-maintenance/HTTP-cache settings are not wired there. See the [Lambda guide](../../cmd/tegola_lambda/README.md). Local invocation parity does not establish a deployed AWS integration.

Current master additionally includes WFS reads and transactions and the opt-in Part 4 draft mutation profile for admitted GeoPackage, PostGIS and MySQL/MariaDB sources. These changes postdate `v0.21.0-fork.2`. Review the [WFS API](../api.md#wfs-and-feature-mutations) and [scope limitations](../wfs-scope-limitations.md) before enabling writes. Writable tile caching uses generation invalidation, with an editor-active request header for ordinary cache bypass.

## Candidate verification

Build from the reviewed revision using the [development instructions](../development.md).
Retain the revision, compiler version, binary hash and configuration hash with
the operator's acceptance record. Run the deterministic migration fixture and
checks described in the [migration guide](../migration/jivan-to-tegola.md), then
repeat representative requests with your own publication configuration.

The [conformance workflow](../testing/ogc-conformance.md) distinguishes official
suite results, supplemental checks and tool limitations. The
[performance guide](../testing/feature-performance-observability.md) describes
measured dataset-specific budgets; page limits do not guarantee cheap candidate
scans. Review nullable attributes, geometry dimensions, filters, actual next links,
public origins, HEAD, error responses and retained tile clients before cutover.

The historical source tag identifies its included read implementation. Current master extends it with WFS and mutation support. Acceptance results remain
bound to their exact revision, fixtures and environment. A tag does not establish
OGC certification, live-provider acceptance for another installation, or deployment.

## Cutover and rollback

1. Preserve the previous working binary, configuration and proxy routing, with
   their hashes. Back up data through the normal database process.
2. Start the candidate separately against read-only publication sources. Verify
   its process path, configuration, revision and direct responses.
3. Migrate clients according to the matrix, including datetime, CQL2 and paging
   differences. Switch proxy routing only after operator acceptance.
4. If acceptance fails, restore the previous routing, binary and configuration;
   verify direct legacy requests and tiles again. Keep the failed candidate's
   evidence for diagnosis. Read publication requires no destructive source migration. Writable profiles may require explicitly provisioned revision/audit metadata; preserve it during rollback and account for any data already committed.

Jivan is an independently managed upstream project. This migration changes
Tegola documentation and operator deployments; it makes no claim that upstream
Jivan has been modified or archived.

## See also

- [Migration guide](../migration/jivan-to-tegola.md)
- [Compatibility matrix](../migration/jivan-compatibility-matrix.md)
- [License provenance](../migration/jivan-provenance.md)
