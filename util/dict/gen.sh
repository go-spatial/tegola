#!/bin/sh

./gen.pl > map.go && goimports -w map.go