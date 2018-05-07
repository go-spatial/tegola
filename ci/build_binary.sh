#!/bin/bash
################################################################################
# This script will build the necessary binaries for tegola.
################################################################################

OLDDIR=$(pwd)
VERSION_TAG=$TRAVIS_TAG
if [ -z "$VERSION_TAG" ]; then 
	VERSION_TAG=$(git rev-parse --short HEAD)
fi

LDFLAGS="-w -X github.com/go-spatial/tegola/cmd/tegola/cmd.Version=${VERSION_TAG}"
if [[ $CGO_ENABLED == 0 ]]; then
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
		if [[ $CGO_ENABLED != 0 ]]; then
			FILENAME="${FILENAME}_cgo"
		fi
		if [[ $GOOS == windows ]]; then 
			FILENAME="${FILENAME}.exe"
		fi

		GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags "${LDFLAGS}" -o ${FILENAME} github.com/go-spatial/tegola/cmd/tegola
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

# build tegola_lambda
FILENAME="${TRAVIS_BUILD_DIR}/releases/tegola_lambda"
GOOS=linux go build -ldflags "-w -X github.com/go-spatial/tegola/cmd/tegola-lambda.Version=${VERSION_TAG}" -o ${FILENAME} github.com/go-spatial/tegola/cmd/tegola_lambda
chmod a+x ${FILENAME}
dir=$(dirname $FILENAME)
fn=$(basename $FILENAME)
cdir=$(pwd)
cd $dir
zip -9 -D ${fn}.zip ${fn}
rm ${fn}
cd ${cdir}

cd $OLDDIR




