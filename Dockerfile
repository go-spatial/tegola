# To build, run in root of tegola source tree:
#
#	$ git clone git@github.com:go-spatial/tegola.git or git clone https://github.com/go-spatial/tegola.git
#	$ cd tegola
#	$ docker build -t tegola .
#
# To use with local files, add file data sources (i.e. Geopackages) and config as config.toml to a
# local directory and mount that directory as a volume at /opt/tegola_config/.  Examples:
#
# To display command-line options available:
#  
#	$ docker run --rm tegola
#
# Example PostGIS use w/ http-based config:
#
#	$ docker run -p 8080 tegola --config http://my-domain.com/config serve
#
# Example PostGIS use w/ local config:
#	$ mkdir docker-config
#	$ cp my-config-file docker-config/config.toml
#	$ docker run -v /path/to/docker-config:/opt/tegola_config -p 8080 tegola serve
#
# Example gpkg use:
#  $ mkdir docker-config
#  $ cp my-config-file docker-config/config.toml
#  $ cp my-db.gpkg docker-config/
#  $ docker run -v /path/to/docker-config:/opt/tegola_config -p 8080 tegola serve

# Intermediary container for building
FROM golang:1.23.2-alpine3.20 AS build

ARG BUILDPKG="github.com/go-spatial/tegola/internal/build"
ARG VER="Version Not Set"
ARG BRANCH="not set"
ARG REVISION="not set"
ENV VERSION="${VER}"
ENV GIT_BRANCH="${BRANCH}"
ENV GIT_REVISION="${REVISION}"
ENV BUILD_PKG="${BUILDPKG}"

# Only needed for CGO support at time of build, results in no noticable change in binary size
# incurs approximately 1:30 extra build time (1:54 vs 0:27) to install packages.  Doesn't impact
# development as these layers are drawn from cache after the first build.
RUN apk update \
	&& apk add build-base

# Set up source for compilation
RUN mkdir -p /go/src/github.com/go-spatial/tegola
COPY . /go/src/github.com/go-spatial/tegola

RUN env

# Build binary
RUN cd /go/src/github.com/go-spatial/tegola/cmd/tegola \
	&& go build -v  \
	-ldflags "-w -X '${BUILD_PKG}.Version=${VERSION}' -X '${BUILD_PKG}.GitRevision=${GIT_REVISION}' -X '${BUILD_PKG}.GitBranch=${GIT_BRANCH}'" \
	-gcflags "-N -l" \
	-o /opt/tegola \
	&& chmod a+x /opt/tegola

# Create minimal deployment image, just alpine & the binary
FROM alpine:3.18

RUN apk update \
	&& apk add ca-certificates \
	&& rm -rf /var/cache/apk/*

COPY --from=build /opt/tegola /opt/
WORKDIR /opt
ENTRYPOINT ["/opt/tegola"]
