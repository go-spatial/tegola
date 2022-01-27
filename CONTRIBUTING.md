# Welcome

Thank you for even thinking about contributing! We are excited to have you. This document is intended as a guide to help your through the contribution process. This guide assumes a you have a basic understanding of Git and Go.

For sensitive security-related issue please start a conversation with a core contributor on the [#go-spatial](https://invite.slack.golangbridge.org/) channel in the [gophers slack](https://invite.slack.golangbridge.org/) organization.

This project and everyone who is participating in it is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).
By participating, you are expected to uphold this code. Please report unacceptable behavior to the [#go-spatial](https://invite.slack.golangbridge.org/) channel in the [gophers slack](https://invite.slack.golangbridge.org/) organization or to one of the Core Contributors.

## There are several places where you can contribute. 

### Found a bug or something does not feel right?

Everything we do is done through [issues](https://github.com/go-spatial/tegola/issues). The first thing to do is to search the current issues to see if it is something that has been reported or requested already. If you are unable to find an issue that is similar or are unsure just file a new issue. If you find one that is similar, you can add a comment to add additional details, or if you have nothing new to add you can “+1” the issue.

* If you are unable to find an issue that is similar or are unsure go ahead and file a new one. 
* If it is a bug your can use the following [template](https://github.com/go-spatial/tegola/issues/new?template=bug.md). 
* If this is a rendering bug, please include the relevant data set and configuration file. 
* If it is a feature request use the following [template](https://github.com/go-spatial/tegola/issues/new?template=feature.md).
* If this is a feature request, please include a description of what the feature is, and the use case for the feature.

Once you have filed an issue, we will discuss it in the issue. If we need more information or you have further questions about that issue, this is the place to ask. This is the place where we will discuss the design of the fix or feature. Any pull request that adds a feature or fixes an issue should reference the issue number.

If you have changes to for the Tegola.io website — documentation on the website, tutorials, translation or anything else — the process is similar but on a different [repository](https://github.com/go-spatial/tegola-docs) (https://github.com/go-spatial/tegola-docs).

Don’t be afraid to reach out if you have any questions.  You can reach us on the gophers Slack on the channel #tegola or #go-spatial. You can get an invite into the gophers Slack via (https://invite.slack.golangbridge.org/)

## Making a Contribution to the code base.

For the Tegola project our master branch is always the most recent stable version of the code base. The current release candidate will be in a branch name for the next version of the software. For example if the current release is v0.6.1 the next release will be v0.7.0, the release candidate branch will be called “v0.7.0”. Please, base all of your pull requests on the release candidate branch.

### Discuss your design

All contributions are welcome, but please let everyone know what you are working on. The way to do this is to first file an issue (or claim an existing issue). In this issue, please, discuss what your plan is to fix or add the feature. Also, all design discussions should happen on the issue. If design discussions happen in a channel, reconcile the decisions to the relevant issue(s). Once, your contribution is ready, create a pull request referencing the issue. Once, a pull request is created one or more of the Core Contributors will review the pull request and may request changes. Once the changes are approved, it will be merged into the current release candidate branch.

Be sure to keep the pull request updated as merge conflicts may occur as other things get merged into the release branch before yours.

Please, note that we may push your pull request to the next release candidate at which point you will have to resolve any conflicts that occur.

### Not sure where to contribute?

Want to contribute but not sure where? Not a problem, the best thing to do is look through the issues and find one that interests you. If the issue has the label `good first issue`, it means that one of the core contributors thinks this is a good issue to start with. But, this doesn’t mean that you have to start with these issues. Go through the issues and see if someone is already working on it. If no one is, state that you will be working on the issue to claim it. If you are unsure where to start on the issue, ask in the issue and one of the Core Contributors will help you out.

## How to build from source

If you just need to play with the binary the easy way is to get the binary from our [releases page](https://github.com/go-spatial/tegola/releases), or if a binary for your OS is not there, use `go get -u github.com/go-spatial/tegola/cmd/tegola`.

If however you want to build the latest release candidate you will have to build from source. The first thing to do is to clone the `go-spatial/tegola` repo to your `GOPATH`. The simplest way to do this is to use `go get -u github.com/go-spatial/tegola`, navigate to the repository root then: 

* Checkout the current release candidate branch, (i.e. v0.7.0)
	
    (`git checkout v0.7.0`)
	
* Create a new feature branch. 
	
    (`git checkout -b issue-XXX-new_feature`)
	
* Work on the fix, and run all the tests. We need to run tests with CGO enabled and disabled.

  (`go generate ./...`) # to regenerate any autogenerated assets
  (`go test ./…`)
    (`CGO_ENABLED=0 go test ./…`)

* Make sure tegola can be built and run:

    (`cd cmd/tegola`)
    (`go build && ./tegola serve --config=path/to/config.toml`)
	
* Commit your changes (`git commit -am ‘Add some feature #XXX\n\nExtened description.'`)

### Contribute upstream:

* On github, fork the repo to into your account.
* Add a new remote pointing to your fork. 

	(`git remote add fork  git@github.com:yourname/rep.git`)
	
* Push to the branch 
	
	(`git push fork issue-XXX-new_feature`)
	
* Create a new Pull Request on GitHub against the Release Candidate branch.

For more information about this work flow, please refer to this [great explanation by Katrina Owen](https://splice.com/blog/contributing-open-source-git-repositories-go/).

## Conventions

* All code should be formatted using:
	
	(`gofmt -s ./…`).

	- if you find that running `gofmt` produces changes across parts of the code base you're not working on, submit the formatting change in a separate Pull Request. This helps decouple engineering changes from formatting changes and focused the code review efforts. 
	
* When declaring errors variables they should take the form of:
	
	(`var ErrErrorName  = errors.New("provider: canceled")`)
	
* The text should be all lowercase, and no punctuation at the end.

## Testing

For tests we use go 1.7 sub tests. Please, look at the [cmp_test.go](https://github.com/go-spatial/tegola/blob/master/geom/cmp/cmp_test.go).

