# To build, run in root of tegola source tree:
#
# Fork note: clone this fork (alexeydott/tegola), not upstream go-spatial/tegola.
# The gpkg provider requires CGO (sqlite3), so the build stage installs build-base.
#
#	$ git clone git@github.com:alexeydott/tegola.git or git clone https://github.com/alexeydott/tegola.git
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
#
# The container runs as the non-root "tegola" user and /opt is owned by root.
# A writable directory is provided at /opt/cache (owned by tegola) for file
# caches; log files written to the working directory will fail unless the
# directory is mounted from the host with appropriate permissions.

# Intermediary container for building
FROM golang:1.26.9-alpine3.23 AS build

ARG BUILDPKG="github.com/alexeydott/tegola/internal/build"
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
RUN mkdir -p /go/src/github.com/alexeydott/tegola
COPY . /go/src/github.com/alexeydott/tegola

# Debug image stage: builds with compiler optimizations disabled.
# Prefer the default (production) target unless you need to debug the binary.
FROM build AS debug
RUN cd /go/src/github.com/alexeydott/tegola/cmd/tegola \
	&& go build -v  \
	-ldflags "-w -X '${BUILD_PKG}.Version=${VERSION}' -X '${BUILD_PKG}.GitRevision=${GIT_REVISION}' -X '${BUILD_PKG}.GitBranch=${GIT_BRANCH}'" \
	-gcflags "-N -l" \
	-o /opt/tegola \
	&& chmod a+x /opt/tegola

# Release stage: builds the production binary with optimizations enabled.
# Each stage builds its own /opt/tegola so the final image copies the
# production binary (the debug stage keeps the -gcflags build).
FROM build AS release
RUN cd /go/src/github.com/alexeydott/tegola/cmd/tegola \
	&& go build -v  \
	-ldflags "-w -X '${BUILD_PKG}.Version=${VERSION}' -X '${BUILD_PKG}.GitRevision=${GIT_REVISION}' -X '${BUILD_PKG}.GitBranch=${GIT_BRANCH}'" \
	-o /opt/tegola \
	&& chmod a+x /opt/tegola

# Create minimal deployment image, just alpine & the binary.
# Runtime alpine version matches the build stage (musl compatibility for CGO/sqlite).
FROM alpine:3.23

RUN apk update \
	&& apk add ca-certificates \
	&& rm -rf /var/cache/apk/* \
	&& addgroup -S tegola \
	&& adduser -S -G tegola tegola \
	&& mkdir -p /opt/cache \
	&& chown tegola:tegola /opt/cache

COPY --from=release /opt/tegola /opt/
WORKDIR /opt
USER tegola
ENTRYPOINT ["/opt/tegola"]
