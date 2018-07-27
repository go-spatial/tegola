#!/bin/bash
################################################################################
# This script will build the necessary binaries for tegola.
################################################################################

set -ex

OLD_DIR=$(pwd)
CI_DIR=`dirname $0`


VERSION_TAG=$TRAVIS_TAG
if [ -z "$VERSION_TAG" ]; then 
	VERSION_TAG=$(git rev-parse --short HEAD)
fi

LDFLAGS="-w -X github.com/go-spatial/tegola/cmd/tegola/cmd.Version=${VERSION_TAG}"
if [[ "$CGO_ENABLED" == "0" ]]; then
	echo "Building binaries without CGO."
	LDFLAGS="${LDFLAGS} -s"
else 
	# xgo is used for cross compiling cgo. use the docker container and xgo wrapper tool
	docker pull karalabe/xgo-latest
	source $CI_DIR/install_go_bin.sh
	go_install github.com/karalabe/xgo	
fi

if [ -z "$TRAVIS_BUILD_DIR" ]; then
	TRAVIS_BUILD_DIR=.
fi

mkdir -p "${TRAVIS_BUILD_DIR}/releases"
echo "building bins into ${TRAVIS_BUILD_DIR}/releases"

for GOARCH in amd64
do
	for GOOS in darwin linux windows
	do
		FILENAME="${TRAVIS_BUILD_DIR}/releases/tegola_${GOOS}_${GOARCH}"
		if [[ "$CGO_ENABLED" != "0" ]]; then
			echo "CGO_ENABLED: $CGO_ENABLED"
			FILENAME="${FILENAME}_cgo"
		fi

		EXT=""
		if [[ $GOOS == windows ]]; then 
			EXT=".exe"
		fi

		# use xgo for CGO builds and the normal Go toolchain for non CGO builds
		if [[ "$CGO_ENABLED" != "0" ]]; then
			echo "CGO_ENABLED: $CGO_ENABLED"
			xgo -go 1.10.x --targets="${GOOS}/${GOARCH}" -ldflags "${LDFLAGS}" -dest "${TRAVIS_BUILD_DIR}/releases" github.com/go-spatial/tegola/cmd/tegola
			mv ${TRAVIS_BUILD_DIR}/releases/tegola-${GOOS}* "${TRAVIS_BUILD_DIR}/releases/tegola${EXT}"
		else
			GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags "${LDFLAGS}" -o "${TRAVIS_BUILD_DIR}/releases/tegola${EXT}" github.com/go-spatial/tegola/cmd/tegola
			chmod a+x "${TRAVIS_BUILD_DIR}/releases/tegola${EXT}"
		fi

		dir=$(dirname $FILENAME)
		fn=$(basename $FILENAME)
		cdir=$(pwd)
		cd $dir
		zip -9 -D ${fn}.zip tegola${EXT}
		rm tegola${EXT}
		cd ${cdir}
	done
done
cd $OLD_DIR
