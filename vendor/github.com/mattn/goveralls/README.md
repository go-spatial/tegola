goveralls
=========

[Go](http://golang.org) integration for [Coveralls.io](http://coveralls.io)
continuous code coverage tracking system.

# Installation

`goveralls` requires a working Go installation (Go-1.2 or higher).

```bash
$ go get github.com/mattn/goveralls
```


# Usage

First you will need an API token.  It is found at the bottom of your
repository's page when you are logged in to Coveralls.io.  Each repo has its
own token.

```bash
$ cd $GOPATH/src/github.com/yourusername/yourpackage
$ goveralls -repotoken your_repos_coveralls_token
```

You can set the environment variable `$COVERALLS_TOKEN` to your token so you do
not have to specify it at each invocation.


You can also run this reporter for multiple passes with the flag `-parallel` or
by setting the environment variable `COVERALLS_PARALLEL=true` (see [coveralls
docs](https://docs.coveralls.io/parallel-build-webhook) for more details.


# Continuous Integration

There is no need to run `go test` separately, as `goveralls` runs the entire
test suite.

## Github Actions

[shogo82148/actions-goveralls](https://github.com/marketplace/actions/actions-goveralls) is available on GitHub Marketplace.
It provides the shorthand of the GitHub Actions YAML configure.

```yaml
name: Quality
on: [push, pull_request]
jobs:
  test:
    name: Test with Coverage
    runs-on: ubuntu-latest
    steps:
    - name: Set up Go
      uses: actions/setup-go@v1
      with:
        go-version: '1.13'
    - name: Check out code
      uses: actions/checkout@v2
    - name: Install dependencies
      run: |
        go mod download
    - name: Run Unit tests
      run: |
        go test -race -covermode atomic -coverprofile=profile.cov ./...
    - name: Send coverage
      env:
        COVERALLS_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      run: |
        GO111MODULE=off go get github.com/mattn/goveralls
        $(go env GOPATH)/bin/goveralls -coverprofile=profile.cov -service=github
    # or use shogo82148/actions-goveralls
    # - name: Send coverage
    #   uses: shogo82148/actions-goveralls@v1
    #   with:
    #     path-to-profile: profile.cov
```

### Test with Legacy GOPATH mode

If you want to use Go 1.10 or earlier, you have to set `GOPATH` environment value and the working directory.
See <https://github.com/golang/go/wiki/GOPATH> for more detail.

Here is an example for testing `example.com/owner/repo` package.

```yaml
name: Quality
on: [push, pull_request]
jobs:
  test:
    name: Test with Coverage
    runs-on: ubuntu-latest
    steps:
    - name: Set up Go
      uses: actions/setup-go@v1
      with:
        go-version: '1.10'

    # add this step
    - name: Set up GOPATH
      run: |
        echo "::set-env name=GOPATH::${{ github.workspace }}"
        echo "::add-path::${{ github.workspace }}/bin"

    - name: Check out code
      uses: actions/checkout@v2
      with:
        path: src/example.com/owner/repo # add this
    - name: Run Unit tests
      run: |
        go test -race -covermode atomic -coverprofile=profile.cov ./...
      working-directory: src/example.com/owner/repo # add this
    - name: Send coverage
      env:
        COVERALLS_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      run: |
        GO111MODULE=off go get github.com/mattn/goveralls
        $(go env GOPATH)/bin/goveralls -coverprofile=profile.cov -service=github
      working-directory: src/example.com/owner/repo # add this
```

## Travis CI

### GitHub Integration

Enable Travis-CI on your github repository settings.

For a **public** github repository put bellow's `.travis.yml`.

```yml
language: go
go:
  - tip
before_install:
  - go get github.com/mattn/goveralls
script:
  - $GOPATH/bin/goveralls -service=travis-ci
```

For a **public** github repository, it is not necessary to define your repository key (`COVERALLS_TOKEN`).

For a **private** github repository put bellow's `.travis.yml`. If you use **travis pro**, you need to specify `-service=travis-pro` instead of `-service=travis-ci`.

```yml
language: go
go:
  - tip
before_install:
  - go get github.com/mattn/goveralls
script:
  - $GOPATH/bin/goveralls -service=travis-pro
```

Store your Coveralls API token in `Environment variables`.

```
COVERALLS_TOKEN = your_token_goes_here
```

or you can store token using [travis encryption keys](https://docs.travis-ci.com/user/encryption-keys/). Note that this is the token provided in the page for that specific repository on Coveralls. This is *not* one that was created from the "Personal API Tokens" area under your Coveralls account settings.

```
$ gem install travis
$ travis encrypt COVERALLS_TOKEN=your_token_goes_here --add env.global
```

travis will add `env` block as following example:

```yml
env:
  global:
    secure: xxxxxxxxxxxxx
```

### For others:

```
$ go get github.com/mattn/goveralls
$ go test -covermode=count -coverprofile=profile.cov
$ goveralls -coverprofile=profile.cov -service=travis-ci
```

## Drone.io

Store your Coveralls API token in `Environment Variables`:

```
COVERALLS_TOKEN=your_token_goes_here
```

Replace the `go test` line in your `Commands` with these lines:

```
$ go get github.com/mattn/goveralls
$ goveralls -service drone.io
```

`goveralls` automatically use the environment variable `COVERALLS_TOKEN` as the
default value for `-repotoken`.

You can use the `-v` flag to see verbose output from the test suite:

```
$ goveralls -v -service drone.io
```

## CircleCI

Store your Coveralls API token as an [Environment Variable](https://circleci.com/docs/environment-variables).

In your `circle.yml` add the following commands under the `test` section.

```yml
test:
  pre:
    - go get github.com/mattn/goveralls
  override:
    - go test -v -cover -race -coverprofile=/home/ubuntu/coverage.out
  post:
    - /home/ubuntu/.go_workspace/bin/goveralls -coverprofile=/home/ubuntu/coverage.out -service=circle-ci -repotoken=$COVERALLS_TOKEN
```

For more information, See https://coveralls.zendesk.com/hc/en-us/articles/201342809-Go

## Semaphore

Store your Coveralls API token in `Environment Variables`:

```
COVERALLS_TOKEN=your_token_goes_here
```

More instructions on how to do this can be found in the [Semaphore documentation](https://semaphoreci.com/docs/exporting-environment-variables.html).

Replace the `go test` line in your `Commands` with these lines:

```
$ go get github.com/mattn/goveralls
$ goveralls -service semaphore
```

`goveralls` automatically use the environment variable `COVERALLS_TOKEN` as the
default value for `-repotoken`.

You can use the `-v` flag to see verbose output from the test suite:

```
$ goveralls -v -service semaphore
```

## Coveralls Enterprise

If you are using Coveralls Enterprise and have a self-signed certificate, you need to skip certificate verification:

```shell
$ goveralls -insecure
```

# Authors

* Yasuhiro Matsumoto (a.k.a. mattn)
* haya14busa

# License

under the MIT License: http://mattn.mit-license.org/2016
