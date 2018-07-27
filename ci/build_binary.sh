#!/bin/bash
################################################################################
# This script will build the necessary binaries for tegola.
################################################################################

CI_DIR=`dirname $0`

# xgo is used for cross compiling cgo. use the docker container and xgo wrapper tool
docker pull karalabe/xgo-latest
source $CI_DIR/install_go_bin.sh
go_install github.com/karalabe/xgo

OLDDIR=$(pwd)
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

		GOOS=${GOOS} GOARCH=${GOARCH} xgo build -ldflags "${LDFLAGS}" -o ${FILENAME} github.com/go-spatial/tegola/cmd/tegola
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
cd $OLDDIR
