# Security Policy

## Supported versions

This repository is a fork of go-spatial/tegola. Only the latest revision of
the default branch is supported with security fixes; please make sure you
are running an up-to-date build before reporting.

## Reporting a vulnerability

Please do **not** open a public issue for security-sensitive reports.

**Preferred channel:** use [GitHub Private Vulnerability
Reporting](https://github.com/alexeydott/tegola/security/advisories/new) —
reports filed this way stay private until a fix is published, and the
maintainer receives a notification without needing a public email address.

If private reporting is unavailable, contact the fork maintainer directly
via GitHub: [@alexeydott](https://github.com/alexeydott).

Include a description of the issue, the affected version/commit, and — if
possible — a minimal reproducer (configuration file and data set). You can
expect an initial response within a few days.

## Scope notes

* The tile server exposes HTTP endpoints and reads local files (GeoPackage)
  and databases (PostGIS, MySQL, HANA). Reports involving unauthenticated
  access to administrative endpoints, path traversal via config values, or
  SQL injection through config tokens are especially welcome.
* Dependency vulnerabilities are tracked with `govulncheck ./...` in CI
  (see `.github/workflows/`).
