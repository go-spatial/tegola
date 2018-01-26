#!/bin/bash

set -ex

CI_DIR=`dirname $0`
PROJECT_DIR="$CI_DIR/.."
source $CI_DIR/install_go_bin.sh

#	uses go-bindata & go-bindata-assetfs to convert the tegola internal viewer static assets into binary 
#	so they can be compiled into the tegola binary
build_bindata() {
	#	fetch our bindata tooling
	go_install github.com/jteeuwen/go-bindata/go-bindata
	go_install github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs
	ls -l $GOPATH/bin

	#	change directory to the location of this script
	cd $CI_DIR
	#	move to our server directory
	cd ../server

	#	build bindata
	go-bindata-assetfs -pkg=server -ignore=.DS_Store static/...
}

build_bindata
