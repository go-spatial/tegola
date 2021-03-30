set -x

# read the hash of the geom repo
GEOM_HASH=`git -C $GOPATH/src/github.com/go-spatial/geom rev-parse --short HEAD`

TEGOLA_HASH=`git rev-parse --short HEAD`

TEGOLA_BRANCH=`git rev-parse --abbrev-ref HEAD`

VERSION_TAG="nonrelease_branch_${TEGOLA_BRANCH}_hash_${TEGOLA_HASH}_geom_${GEOM_HASH}"

LDFLAGS_VERSION="-X github.com/go-spatial/tegola/internal/build.Version=${VERSION_TAG}"
LDFLAGS_BRANCH="-X github.com/go-spatial/tegola/internal/build.GitBranch=${TEGOLA_BRANCH}"
LDFLAGS_REVISION="-X github.com/go-spatial/tegola/internal/build.GitRevision=${TEGOLA_HASH}"

LDFLAGS="-w ${LDFLAGS_VERSION} ${LDFLAGS_BRANCH} ${LDFLAGS_REVISION}"

go build -ldflags "${LDFLAGS}" -o "tegola_${TEGOLA_BRANCH}" github.com/go-spatial/tegola/cmd/tegola
