#!/bin/sh 


go_install(){
	if [ -z ${PROJECT_DIR+x} ]; then 
		PROJECT_DIR=`dirname $0`/..
	fi
	OLDPWD=`pwd`
	cd $PROJECT_DIR/vendor/"$@"
	go install .
	cd $OLDPWD 
}

