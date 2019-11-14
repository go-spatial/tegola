set -x

# read the hash of the geom repo
GEOM_HASH=`git -C $GOPATH/src/github.com/go-spatial/geom rev-parse --short HEAD`

TEGOLA_HASH=`git rev-parse --short HEAD`

TEGOLA_BRANCH=`git rev-parse --abbrev-ref HEAD`

VERSION_TAG="nonrelease_branch_${TEGOLA_BRANCH}_hash_${TEGOLA_HASH}_geom_${GEOM_HASH}"

LDFLAGS="-w -X github.com/go-spatial/tegola/cmd/tegola/cmd.Version=${VERSION_TAG}"

go build -ldflags "${LDFLAGS}" -o "tegola_${TEGOLA_BRANCH}" github.com/go-spatial/tegola/cmd/tegola
