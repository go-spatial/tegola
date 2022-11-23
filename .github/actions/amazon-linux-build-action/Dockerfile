# Amazon Linux is used to build tegola_linux so the CGO requirements are linked correctly
FROM amazonlinux:latest

# install build deps
RUN yum install -y tar gzip gcc

# install Go
ENV GOLANG_VERSION 1.19.3
ENV GOLANG_VERSION_SHA256 74b9640724fd4e6bb0ed2a1bc44ae813a03f1e72a4c76253e2d5c015494430ba

RUN curl -o golang.tar.gz https://dl.google.com/go/go$GOLANG_VERSION.linux-amd64.tar.gz \
	&& echo "$GOLANG_VERSION_SHA256 golang.tar.gz" | sha256sum --strict --check \
	&& tar -C /usr/local -xzf golang.tar.gz \
	&& rm golang.tar.gz

ENV PATH /usr/local/go/bin:$PATH

# entrypoint.sh holds the build instructions for tegola_lambda
COPY entrypoint.sh /entrypoint.sh

# run the build script when this container starts up
ENTRYPOINT ["/entrypoint.sh"]
