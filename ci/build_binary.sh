#!/bin/sh
################################################################################
# This script will build the necessary binaries for tegola.
################################################################################

OLDDIR=$(pwd)
LDFLAGS="-w -X github.com/terranodo/tegola/cmd/tegola/cmd.Version=${TRAVIS_TAG}"


if [ -z "$TRAVIS_BUILD_DIR" ]; then
	TRAVIS_BUILD_DIR=.
fi

echo $TRAVIS_BUILD_DIR
mkdir -p "${TRAVIS_BUILD_DIR}/releases"
cd "${TRAVIS_BUILD_DIR}/cmd/tegola"
for GOARCH in AMD64
do
	for GOOS in darwin linux windows
	do
		unset CGO_ENABLED
		FILENAME="${TRAVIS_BUILD_DIR}/releases/tegola_${GOOS}_${GOARCH}"
	
		go build -ldflags ${LDFLAGS} -o ${FILENAME}
		chmod +x ${FILENAME}
		CGO_ENABLED=0
		go build -ldflags ${LDFLAGS} -o "${FILENAME}_nocgo"
		chmod +x "${FILENAME}_nocgo"
	done
done
cd $OLDDIR




