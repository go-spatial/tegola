#!/bin/sh
# Generate test coverage statistics for Go packages.
#
# Works around the fact that `go test -coverprofile` currently does not work
# with multiple packages, see https://code.google.com/p/go/issues/detail?id=6909
#
# Usage: go_test_multi_package_coverprofile --coverprofilename=<coverprofile base filename> [--html] [--coveralls]
#
#     --coverprofilename  basefilename for coverprofile data
#     --html              Additionally create HTML report <coverprofile base filename>.html
#     --coveralls         Push coverage statistics to coveralls.io
#
#
# S.C. - borrowed/adapted from https://github.com/mlafeldt/chef-runner/blob/v0.7.0/script/coverage; see also https://mlafeldt.github.io/blog/test-coverage-in-go/

set -e

CI_DIR=`dirname $0`
PROJECT_DIR="$CI_DIR/.."
source $CI_DIR/install_go_bin.sh

workdir=.cover
coverprofilename=default


while [ $# -gt 0 ]; do
  case "$1" in
    --coverprofilename=*)
      coverprofilename="${1#*=}"
      ;;
    --html)
      genhtml=true
      ;;
    --coveralls)
      pushtocoveralls=true
      ;;
    *)
      printf "\tError: Invalid argument \"$1\"\n"
      exit 1
  esac
  shift
done

coverprofile="$workdir/$coverprofilename.coverprofile"

# from official go test docs:
#           -covermode set,count,atomic
#            Set the mode for coverage analysis for the package[s]
#            being tested. The default is "set" unless -race is enabled,
#            in which case it is "atomic".
#            The values:
#                set: bool: does this statement run?
#                count: int: how many times does this statement run?
#                atomic: int: count, but correct in multithreaded tests;
#                        significantly more expensive.
#            Sets -cover.
mode=count



# functions section
generate_cover_data() {
    rm -rf "$workdir"
    mkdir "$workdir"

    for pkg in "$@"; do
        f="$workdir/$(echo $pkg | tr / -).pkgcoverprofile"
        go test -covermode="$mode" -coverprofile="$f" "$pkg"
    done

    echo "mode: $mode" > "$coverprofile"
    grep -h -v "^mode:" "$workdir"/*.pkgcoverprofile >> "$coverprofile"
}

generate_cover_report() {
    case ${1} in
      html)
        go tool cover -html="$coverprofile" -o "$coverprofilename".coverprofile.html
        ;;
      func)
        go tool cover -func="$coverprofile"
        ;;
      *)
        echo >&2 "error: generate_cover_report invalid arg: $1"
        exit 1
        ;;
    esac
}

push_to_coveralls() {
    echo "Pushing coverage statistics to coveralls.io"
    goveralls -coverprofile="$coverprofile"
}



# body of script
  # first get go cover tool in case it does not exist locally
go_install golang.org/x/tools/cmd/cover

# generate coverage data, but skip the vendor directory.
# skipping the vendor directory is necessary for go versions less than Go 1.9.x
generate_cover_data $(go list ./... | grep -v vendor)
generate_cover_report func
if [ "$genhtml" = true ] ; then
    generate_cover_report html
fi
if [ "$pushtocoveralls" = true ] ; then
    go_install github.com/mattn/goveralls
    push_to_coveralls
fi
