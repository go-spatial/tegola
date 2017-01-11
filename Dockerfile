FROM golang
MAINTAINER terranodo
RUN mkdir -p /tegola/
WORKDIR /tegola/
COPY . /tegola

#Requirements to compile 
RUN mkdir -p /usr/local/go/src/github.com/terranodo/
RUN cp -r /tegola/ /usr/local/go/src/github.com/terranodo/
RUN mkdir -p /usr/local/go/src/github.com/BurntSushi/
RUN cd /usr/local/go/src/github.com/BurntSushi/ && git clone https://github.com/BurntSushi/toml.git

#Compile
RUN cd /tegola/cmd/tegola && go build -o tegola *.go
EXPOSE 8080


## In your Dockerfile you would have: 
#
# FROM terranodo/tegola
# COPY config.toml /tegola/
# CMD ["./cmd/tegola/tegola", "--config=/tegola/config.toml"]

