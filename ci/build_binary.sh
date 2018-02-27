#!/bin/sh
################################################################################
# This script will build the necessary binaries for tegola.
################################################################################

OLDDIR=$(pwd)
VERSION_TAG=$TRAVIS_TAG
if [ -z "$VERSION_TAG" ]; then 
	VERSION_TAG=$(git rev-parse --short HEAD)
fi

LDFLAGS="-w -X github.com/terranodo/tegola/cmd/tegola/cmd.Version=${VERSION_TAG}"
if [[ $CGO_ENABLED == 0 ]]; then
	LDFLAGS="${LDFLAGS} -s"
fi

if [ -z "$TRAVIS_BUILD_DIR" ]; then
	TRAVIS_BUILD_DIR=.
fi

mkdir -p "${TRAVIS_BUILD_DIR}/releases"
echo "building bins into ${TRAVIS_BUILD_DIR}/releases"

for GOARCH in AMD64
do
	for GOOS in darwin linux windows
	do
		FILENAME="${TRAVIS_BUILD_DIR}/releases/tegola_${GOOS}_${GOARCH}"
		if [[ $CGO_ENABLED != 0 ]]; then
			FILENAME="${FILENAME}_cgo"
		fi
		if [[ $GOOS == windows ]]; then 
			FILENAME="${FILENAME}.exe"
		fi

		go build -ldflags "${LDFLAGS}" -o ${FILENAME} github.com/terranodo/tegola/cmd/tegola
		chmod a+x ${FILENAME}
	done
done
cd $OLDDIR




