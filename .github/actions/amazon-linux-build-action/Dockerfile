# Amazon Linux is used to build tegola_linux so the CGO requirements are linked correctly
FROM amazonlinux:latest

# install build deps
RUN yum install -y tar gzip gcc

# install Go
ENV GOLANG_VERSION=1.26.9
ENV GOLANG_VERSION_SHA256=42d158b4d8f7b61ac0a830567c940a86098fb7aac52e467a5ebec03ef5cc2f8d

RUN curl -o golang.tar.gz https://dl.google.com/go/go$GOLANG_VERSION.linux-amd64.tar.gz \
	&& echo "$GOLANG_VERSION_SHA256 golang.tar.gz" | sha256sum --strict --check \
	&& tar -C /usr/local -xzf golang.tar.gz \
	&& rm golang.tar.gz

ENV PATH=/usr/local/go/bin:$PATH

# entrypoint.sh holds the build instructions for tegola_lambda
COPY entrypoint.sh /entrypoint.sh

# run the build script when this container starts up
ENTRYPOINT ["/entrypoint.sh"]
