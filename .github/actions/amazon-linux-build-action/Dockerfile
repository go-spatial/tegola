# Amazon Linux is used to build tegola_linux so the CGO requirements are linked correctly
FROM amazonlinux:latest

# install build deps
RUN yum install -y tar gzip gcc

# install Go
ENV GOLANG_VERSION=1.26.5
ENV GOLANG_VERSION_SHA256=5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053

RUN curl -o golang.tar.gz https://dl.google.com/go/go$GOLANG_VERSION.linux-amd64.tar.gz \
	&& echo "$GOLANG_VERSION_SHA256 golang.tar.gz" | sha256sum --strict --check \
	&& tar -C /usr/local -xzf golang.tar.gz \
	&& rm golang.tar.gz

ENV PATH=/usr/local/go/bin:$PATH

# entrypoint.sh holds the build instructions for tegola_lambda
COPY entrypoint.sh /entrypoint.sh

# run the build script when this container starts up
ENTRYPOINT ["/entrypoint.sh"]
