#!/bin/bash
################################################################################
# This script will build the necessary binaries for tegola.
################################################################################

set -ex

OLD_DIR=$(pwd)
CI_DIR=`dirname $0`

# xgo is used for cross compiling cgo. use the docker container and xgo wrapper tool
docker pull karalabe/xgo-latest
source $CI_DIR/install_go_bin.sh
go_install github.com/karalabe/xgo

VERSION_TAG=$TRAVIS_TAG
if [ -z "$VERSION_TAG" ]; then 
	VERSION_TAG=$(git rev-parse --short HEAD)
fi

LDFLAGS="-w -X github.com/go-spatial/tegola/cmd/tegola/cmd.Version=${VERSION_TAG}"
if [[ "$CGO_ENABLED" == "0" ]]; then
	echo "Building binaries without CGO."
	LDFLAGS="${LDFLAGS} -s"
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
		if [[ $GOOS == windows ]]; then 
			FILENAME="${FILENAME}.exe"
		fi

		xgo -go 1.10.x --targets="${GOOS}/${GOARCH}" -ldflags "${LDFLAGS}" -dest "${TRAVIS_BUILD_DIR}/releases" github.com/go-spatial/tegola/cmd/tegola
		mv releases/tegola-${GOOS}*${GOARCH} ${FILENAME}
		chmod a+x ${FILENAME}
		dir=$(dirname $FILENAME)
		fn=$(basename $FILENAME)
		cdir=$(pwd)
		cd $dir
		zip -9 -D ${fn}.zip ${fn}
		rm ${fn}
		cd ${cdir}
	done
done
cd $OLD_DIR
