set -x

# cd to geom package location
# gopath or vendor
GEOM_PACKAGE_NAME=`go list -f "$(printf '{{ range .Imports }}{{ . }}\n{{ end }}\n')" ./.. | grep geom`
cd `go list -f '{{.Dir}}' $GEOM_PACKAGE_NAME`
GEOM_HASH=`git rev-parse --short HEAD`
cd -

TEGOLA_HASH=`git rev-parse --short HEAD`

TEGOLA_BRANCH=`git rev-parse --abbrev-ref HEAD`

VERSION_TAG="nonrelease_branch_${TEGOLA_BRANCH}_hash_${TEGOLA_HASH}_geom_${GEOM_HASH}"

LDFLAGS="-w -X github.com/go-spatial/tegola/cmd/tegola/cmd.Version=${VERSION_TAG}"

go build -ldflags "${LDFLAGS}" -o "tegola_${TEGOLA_BRANCH}" github.com/go-spatial/tegola/cmd/tegola
