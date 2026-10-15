[← Maintenance Status](maintenance-status.md) · [Back to README](../README.md)

# Windows release builds

Build from the committed source with Go matching `go.mod`, Node/npm and an
amd64 C compiler on PATH. CGO must be enabled to include GeoPackage support.
Historical read-feature release tag: `v0.21.0-fork.2` at
`db4e8ee73a3ddfe3b3c8e054c59e833b697dce85`. Fetch tags and select the intended
release in an isolated checkout before building. Current master includes later WFS, mutation and writable-cache changes; build its exact revision when those features are required. A Git tag is not a GitHub Release
asset or proof of deployment.

Run these PowerShell commands from the repository root; stop on any failed step.

```powershell
Push-Location ui
try {
    npm ci --ignore-scripts --no-audit --no-fund
    if ($LASTEXITCODE -ne 0) { throw 'npm ci failed' }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'viewer build failed' }
} finally {
    Pop-Location
}
# Vite cleans dist; retain the tracked placeholder.
git restore -- ui/dist/.keep

$env:CGO_ENABLED = '1'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$revision = git rev-parse HEAD
$branch = git branch --show-current
$version = git describe --tags --exact-match HEAD
if ($LASTEXITCODE -ne 0) { $version = "v0.21.0-fork.2+git.$($revision.Substring(0, 8))" }
$flags = "-s -w -X github.com/alexeydott/tegola/internal/build.Version=$version -X github.com/alexeydott/tegola/internal/build.GitRevision=$revision -X github.com/alexeydott/tegola/internal/build.GitBranch=$branch"
go build -mod=vendor -trimpath -ldflags $flags -o tegola.exe ./cmd/tegola
if ($LASTEXITCODE -ne 0) { throw 'release build failed' }
.\tegola.exe version
Get-FileHash .\tegola.exe -Algorithm SHA256
```

`-trimpath` is a Go build flag; `-s -w` are linker flags that omit debug
symbols. The version's `+git` suffix identifies a commit build without claiming
a new numbered fork release. For a tagged release, supply that tag as the
version instead. `tegola.exe version` must report the expected source commit,
CGO/GeoPackage support, and a built viewer rather than `viewer not built`.

Run the CGO-off/on test matrix and lint before publishing; distinguish gated
database/cloud tests from actual live-service checks. The fork's current source scope and remaining limitations are listed in [UPSTREAM.md](../UPSTREAM.md).
Geographic definitions support WGS84 identity and the Go fork's three- and
seven-parameter datum transformations; see [the CRS contract](crs.md#geographic-proj-definitions)
for the remaining grid, unit, axis and prime-meridian restrictions and the
two-dimensional height convention. Geographic scale denominators use the source
ellipsoid's local parallel-arc factor `N(phi) * cos(phi) * pi / 180` at the
transformed tile center, multiplied by horizontal degrees per pixel and divided
by 0.00028 m; see [scale tokens](crs.md#scale-tokens). SQL token scanning assumes
the documented default backend string modes.

Commit messages in this fork must not contain `Co-authored-by` trailers.
Fetch and reconcile `origin/master`, inspect the outgoing commits, and push
`master` without force. Keep generated binaries, checksums, logs and local
datasets out of source commits. Building or pushing source does not create
a GitHub Release or a tag.

## See Also

- [Development and builds](development.md) — source build and debugging guidance
- [Contributing](../CONTRIBUTING.md) — repository build and test workflow
- [Security policy](../SECURITY.md) — report a vulnerability
