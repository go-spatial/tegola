# Tegola on AWS Lambda

`cmd/tegola_lambda` initializes configuration, providers, pools, maps and the HTTP
router once per execution environment. Warm invocations reuse them; separate
execution environments have independent state. `server/lambda` adapts buffered
AWS events to that router, using the vendored algnhsa implementation internally.

## Supported entry-point configuration

The stock Lambda binary loads `config.toml`, or the location in `TEGOLA_CONFIG`.
It binds map tiles, configured cache backends, generic response headers, public
hostname/URI prefix and explicitly published read-only Feature API collections.
Viewer availability depends on embedded assets and gateway routing.

It does **not** wire WFS handlers, feature-write configuration/authentication,
`tile_http_max_age` or tile-maintenance configuration from the TOML into the
router. Do not assume a configuration accepted by `tegola serve` enables these
features in the Lambda executable. Embedders can assemble a different router,
but must implement and verify that composition explicitly. In particular,
process-local writable cache generations are not distributed invalidation across
Lambda execution environments.

## Buffered event modes

| Event | Adapter support |
| --- | --- |
| API Gateway REST proxy v1 | Supported |
| API Gateway HTTP API v2 | Supported |
| Function URL v2 | Supported |
| ALB | Requires target-group multi-value headers |

HEAD suppresses the body while preserving response headers. MVT payloads use
binary/base64 transport; textual representations must be valid UTF-8. Streaming
responses and ALB single-value mode are unsupported.

The adapter defaults cap event envelopes at 6 MiB, decoded request bodies at
1 MiB, response bodies at 4 MiB and headers at 64 KiB. Serialized responses are
also capped: 1 MiB for ALB and 6 MiB for other supported events. These are Tegola's
transport bounds, not a general statement of AWS account quotas.

Configured `webserver.hostname` supplies the complete public URL and mapping
path. Otherwise origin/stage handling comes from the validated event profile;
forwarded headers are not an arbitrary origin override. See the
[Lambda transport decision](../../docs/architecture/decisions/ADR-0014-lambda-router-adapter.md).

## Build and package

From the repository root in a POSIX shell:

```sh
CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go build -mod=vendor \
  -tags 'lambda.norpc noViewer' -o bootstrap ./cmd/tegola_lambda
zip deployment.zip bootstrap config.toml
```

Use `GOARCH=amd64` for x86-64. This build excludes the viewer and GeoPackage
because CGO is disabled. If those are required, build embedded assets and a
compatible CGO environment deliberately; see [development](../../docs/development.md).
Inject release version/revision metadata when packaging a release.

Deploy the archive through the supported AWS runtime/infrastructure process,
configure database/cache access and gateway binary transport, then test real
responses at the public endpoint. Keep credentials out of the archive where
possible and use the deployment's secret mechanism. Configuration changes take
effect in newly initialized execution environments.

## Verification

```sh
go test -mod=vendor ./server/lambda
```

Local adapter tests compare buffered events and actual router responses,
including JSON/HTML discovery, links, errors, HEAD, MVT and transport limits.
They do not establish a deployed AWS integration. Verify public URLs, stage
mapping, binary payloads, headers and database connectivity in the actual
installation before cutover.

## See Also

- [Feature API release scope](../../docs/release/feature-api.md)
- [Configuration](../../docs/configuration.md)
- [Write operating limits](../../docs/wfs-scope-limitations.md)