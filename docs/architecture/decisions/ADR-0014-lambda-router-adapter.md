# Read ADR-0014: Lambda transport over the existing router

Status: Accepted after independent architecture review on 2026-10-02.

## Decision

The optional `cmd/tegola_lambda` binary wraps the assembled Tegola HTTP router
through `server/lambda`. FeatureService, representations, validation, recovery
and provider execution remain shared with the ordinary server. Reuse the existing
vendored algnhsa and aws-lambda-go dependencies.

Support buffered API Gateway REST v1, HTTP API v2, Function URL v2 and ALB with
target-group multi-value headers enabled. ALB requires an operator-configured
public URL. Other modes use a validated gateway request-context domain and HTTPS
unless the operator supplies a complete public URL. Client Host and forwarded
headers cannot select the public origin. A configured URL includes its mapping
path; inferred stages are appended only to gateway-derived origins.
Operator mapping paths containing encoded slash or backslash separators are
rejected because the shared public-link builder cannot preserve their semantics.
This restriction does not decode or clean incoming escaped request paths.

Preserve escaped paths and request URIs without path cleaning. Preserve repeated
query values and cookies. Decode ALB query keys and values once, retaining
collisions as repeated values. Reject contradictory event shapes, malformed
encoding and invalid headers before invoking the router. Request context carries
a detached public root and cancellation; no per-request global mutation.

Bound events before decoding, decoded bodies before routing, and response bodies,
headers and serialized envelopes before return. Maximum event envelope is 6 MiB,
decoded request body 1 MiB, buffered response body 4 MiB and response headers
64 KiB. Serialized responses are capped at 1 MiB for ALB and 6 MiB otherwise.
Lower configured bounds must still accommodate fixed transport errors. Limit
failures return a fixed, nonrecursive error without logging request payloads.
Known MVT responses retain binary base64 encoding; text representations must
remain valid UTF-8. HEAD preserves headers and suppresses the response body.

## Entry-point scope

The shared adapter can wrap an assembled router, but the stock cmd/tegola_lambda binary currently binds tiles and read-only Features. It does not wire WFS, write authentication/configuration or webserver tile-maintenance/HTTP-cache settings. Shared router code does not imply parity of executable composition. See the [Lambda guide](../../../cmd/tegola_lambda/README.md).

## Verification and limits

Compare actual router responses with local invocation in every supported mode,
including landing, collections, items, HEAD, OPTIONS, errors, public links,
escaped paths, duplicate parameters, cookies, MVT, cancellation and limits.
Local invocation establishes adapter parity; it does not establish a deployed
AWS integration. Streaming responses and ALB single-value mode are outside this
adapter contract.

## Sources

- [API Gateway HTTP integrations](https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-develop-integrations-lambda.html)
- [REST proxy integrations](https://docs.aws.amazon.com/apigateway/latest/developerguide/set-up-lambda-proxy-integrations.html)
- [Lambda quotas](https://docs.aws.amazon.com/lambda/latest/dg/gettingstarted-limits.html)
- [ALB Lambda targets](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/lambda-functions.html)
- [Function URL invocation](https://docs.aws.amazon.com/lambda/latest/dg/urls-invocation.html)
