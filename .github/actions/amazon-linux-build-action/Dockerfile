# Amazon Linux is used to build tegola_linux so the CGO requirements are linked correctly
FROM amazonlinux:latest

# install build deps
RUN yum install -y tar gzip gcc

# install Go
ENV GOLANG_VERSION=1.26.2
ENV GOLANG_VERSION_SHA256=990e6b4bbba816dc3ee129eaeaf4b42f17c2800b88a2166c265ac1a200262282

RUN curl -o golang.tar.gz https://dl.google.com/go/go$GOLANG_VERSION.linux-amd64.tar.gz \
	&& echo "$GOLANG_VERSION_SHA256 golang.tar.gz" | sha256sum --strict --check \
	&& tar -C /usr/local -xzf golang.tar.gz \
	&& rm golang.tar.gz

ENV PATH=/usr/local/go/bin:$PATH

# entrypoint.sh holds the build instructions for tegola_lambda
COPY entrypoint.sh /entrypoint.sh

# run the build script when this container starts up
ENTRYPOINT ["/entrypoint.sh"]
