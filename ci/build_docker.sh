#!/bin/bash


if [ -z "${CGO_ENABLED}" == "0" ]; then
	echo "skipping build of docker to avoid double build in TRAVIS."
	exit 0
fi


OLDDIR=$(pwd)
CI_DIR=`dirname $0`
PROJECT_DIR="$CI_DIR/.."
source $CI_DIR/install_go_bin.sh


go_install github.com/gdey/bastet

VERSION_TAG=$TRAVIS_TAG
if [ -z "$VERSION_TAG" ]; then 
	VERSION_TAG=$(git rev-parse --short HEAD)
fi

PUSH_TO_DOCKER="yes"
DOCKER_NAME="${DOCKERHUB_ORG}/${DOCKERHUB_REPO}"
if [ "${DOCKER_NAME}" == "/" ] ; then 
	DOCKER_NAME="tegola"
	PUSH_TO_DOCKER="no"
fi


LDFLAGS="-w -X github.com/terranodo/tegola/cmd/tegola/cmd.Version=${VERSION_TAG}"
CONTAINER_MAINTAINER="Development@JivanAmara.net"

bastet -o docker/Dockerfile1 docker/Dockerfile.tpl "flags=-ldflags \"${LDFLAGS}\"" "version=${VERSION_TAG}" "maintainer=${CONTAINER_MAINTAINER}"
docker build -f docker/Dockerfile1 -t ${DOCKER_NAME}:${VERSION_TAG} .
if [ "${PUSH_TO_DOCKER}" == "yes" ] ; then
   docker tag $DOCKERHUB_ORG/$DOCKERHUB_REPO:${VERSION_TAG} $DOCKERHUB_ORG/$DOCKERHUB_REPO:latest
   docker login -u $DOCKER_USER -p $DOCKER_PASSWORD
   docker push $DOCKERHUB_ORG/$DOCKERHUB_REPO:${VERSION_TAG}
   docker push $DOCKERHUB_ORG/$DOCKERHUB_REPO:latest
fi

