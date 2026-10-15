With the upcoming release of 1.10 being release, and 1.7 introducing subtests, I think it is better to no longer use this package and instead do the following:

```go
package test

import "testing"

func TestFoo(t *testing.T) {
	/*

		With this style of writing test we break our tests into three sections.

		The first defines the test case; which takes the inputs and the expected outputs and wraps them
		up into a single unit.

		I like to match the names of the function parameters to the field names. Say you are testing a
		function that take two parameters a, and b and returns a^b.
		```go
			func raise(a, b int) float {
				...
			}
		```
		The test case should look like:
	*/
	type testCase struct {
		a int // The a parameter to raise
		b int // the b parameter to raise

		expected float // a^b
	}
	/*

		The next section is the test func, which we will call fn. (This will be a curried function.)
		This function should take a test case, and return a function that will actually run the test.
		For the most part you can ignore this, and just copy the template below; filling the the necessary
		parts.

		By having the test defined right after the test case, it is easy to see what the test is doing,
		and what each field in the test case means.

		So, to test our fiticious raise function we would do the following:
	*/
	fn := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {

			/* Here is where we would add our test code, replace as needed for your own functions. */
			got := raise(tc.a, tc.b)
			if got != tc.expected {
				t.Errorf("invalid answer from raise, expected %v got %v", tc.expected, got)
			}

			/* rest of the function follows */
		}
	}

	/*
		Next we define our actual test cases. Each test case has a unique name, which is the key
		in the map, followed by the values for the test case.

		By using a map, we get two benificates:
		1. Each test will have a unique name, that we can use to run just that test.
		2. The test are run an a random order everytime insuring that there aren't an
			dependencies between test runs.

	*/
	tests := map[string]testCase{
		"2^5": testCase{
			a:        2,
			b:        5,
			expected: 32,
		},
		"5^2": testcase{
			a:        5,
			b:        2,
			expected: 25,
		},
	}

	/*
		The last section is the boilerplate that actually runs the tests.
	*/
	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
```
[In Play](https://goplay.space/#nPwUJ2M78pv)
In order only run a pictular test use: `-run "TestFoo/foo"`

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

