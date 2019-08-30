#!/bin/bash


if [ "${CGO_ENABLED}" == "0" ]; then
	echo "skipping build of docker to avoid double build in TRAVIS."
	exit 0
fi

OLDDIR=$(pwd)
CI_DIR=`dirname $0`
PROJECT_DIR="${CI_DIR}/.."

${PROJECT_DIR}/docker/generate_dockerfile.sh

if [ -z "${VERSION_TAG}" ]; then 
	 VERSION_TAG=${TRAVIS_TAG}
	 if [ -z "${VERSION_TAG}" ]; then
		  VERSION_TAG=$(git rev-parse --short HEAD)
	 fi
fi



PUSH_TO_DOCKER="yes"
DOCKER_NAME="${DOCKERHUB_ORG}/${DOCKERHUB_REPO}"
if [ "${DOCKER_NAME}" == "/" ] ; then 
	DOCKER_NAME="tegola"
	PUSH_TO_DOCKER="no"
fi


# build the container
docker build -f ${PROJECT_DIR}/docker/Dockerfile -t "${DOCKER_NAME}:${VERSION_TAG}" .

# push to docker hub
if [ "${PUSH_TO_DOCKER}" == "yes" ] ; then
   docker tag ${DOCKERHUB_ORG}/${DOCKERHUB_REPO}:${VERSION_TAG} ${DOCKERHUB_ORG}/${DOCKERHUB_REPO}:latest
   echo ${DOCKER_PASSWORD} | docker login -u ${DOCKER_USERNAME} --password-stdin
   docker push ${DOCKERHUB_ORG}/${DOCKERHUB_REPO}:${VERSION_TAG}
   docker push ${DOCKERHUB_ORG}/${DOCKERHUB_REPO}:latest
fi

