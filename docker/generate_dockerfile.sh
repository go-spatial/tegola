#!/bin/bash

OLDDIR=$(pwd)
DOCKER_DIR=`dirname $0`
PROJECT_DIR="${DOCKER_DIR}/.."
CI_DIR="${PROJECT_DIR}/ci"
source ${CI_DIR}/install_go_bin.sh

# a cat like program for filling in template vars
go_install github.com/gdey/bastet

if [ -z "${VERSION_TAG}" ]; then 
	 VERSION_TAG=${TRAVIS_TAG}
	 if [ -z "${VERSION_TAG}" ]; then
		  VERSION_TAG=$(git rev-parse --short HEAD)
	 fi
fi

LDFLAGS="-w -X github.com/go-spatial/tegola/cmd/tegola/cmd.Version=${VERSION_TAG}"
if [ -z "$CONTAINER_MAINTAINER" ]; then
	 CONTAINER_MAINTAINER="a.rolek@gmail.com"
fi

# update our docker template with the provided variables
bastet -o ${DOCKER_DIR}/Dockerfile ${DOCKER_DIR}/Dockerfile.tpl "flags=-ldflags \"${LDFLAGS}\"" "version=${VERSION_TAG}" "maintainer=${CONTAINER_MAINTAINER}"

