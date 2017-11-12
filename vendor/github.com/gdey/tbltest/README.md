# tbltest

[![GoDoc](https://godoc.org/github.com/gdey/tbl?status.svg)](https://godoc.org/github.com/gdey/tbl) [![Build Status](https://travis-ci.org/gdey/tbltest.svg?branch=master)](https://travis-ci.org/gdey/tbltest)
[![Go Report Card](https://goreportcard.com/badge/github.com/gdey/tbltest)](https://goreportcard.com/report/github.com/gdey/tbltest)

This is a simple package to help with table driven tests. Given a set
of test cases, it will call the provided function each test case.

This helps remove boiler plate that comes with writing table driven code.

## Example

```go
package main

import (
  "github.com/gdey/tbltest"
  "testing"
)

func Foo(foo string) bool {
  if foo == "foo" {
    return true
  }
  return false
}

// tlb version
func TestFoo(t *testing.T) {
  type testcase struct {
    foo      string
    expected bool
  }

  tests := tlbtest.Cases(
    testcase{
      foo:      "foo",
      expected: true,
    },
    testcase{
      foo:      "bar",
      expected: false,
    },
  )

  tests.Run(func(tc testcase) {
    if Foo(tc.foo) != tc.expected {
      t.FailNow()
    }
  })

}

// standard version
func TestFoo(t *testing.T) {
  type testCase struct {
    foo      string
    expected bool
  }

  tests := []testCase{
    testCase{
      foo:      "foo",
      expected: true,
    },
    testcase{
      foo:      "bar",
      expected: false,
    },
  }

  for _, tc := range tests {
    if Foo(tc.foo) != tc.expected {
      t.FailNow()
    }
  }
}

```

# command line flags

In addition, the tool adds a new command line flag to help with debugging.

`--tblTest.RunOrder` : Allows one to specify the testcases's and the order they should run in.
This is usually helpful, when you are trying to fix one failing test, that you want to keep running
over and over again.

# Why

The biggest benefits provided by this library are:

1. By default randomizes the order in which the test cases are tested. Ensuring that there are no hidden dependencies between the tests caused by to call order.
2. Make it easy to run a single test case using the `--tblTest.runOrder` command line parameter.

