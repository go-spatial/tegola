// Copyright 2016 Gautam Dey. All rights reserved.
// Use of this source code is governed by FreeBDS License (2-clause Simplified BSD.)
// that can be found in the LICENSE file.

package tbltest

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

var runorder = flag.String("tblTest.RunOrder", "", "List of comma separated index of the test cases to run.")

// Test holds the testcases.
type Test struct {
	cases []reflect.Value
	vType reflect.Type
	// InOrder defines weather to run the test case in the order defined or randomly.
	// This option is overridden by the tblTest.RunOrder command line flag.
	InOrder bool

	// The order in which to run these tests. This will be overridden by the Command line flag.
	RunOrder string
}

// TestFunc describes a function that will do the actual testing. It must take one of four forms.
//
//    *  `func (tc $testcase)`
//
//    *  `func (tc $testcase) bool`
//
//    *  `func (idx int, tc $testcase)`
//
//    *  `func (idx int, tc $testcase) bool`
//
type TestFunc interface{}

// TestCase is a custom type that describes a test case.
type TestCase interface{}

func panicf(format string, vals ...interface{}) {
	pc, _, _, ok := runtime.Caller(2)
	details := runtime.FuncForPC(pc)
	var callSite string
	if ok && details != nil {
		callSite = fmt.Sprintf("Called from %v: ", details.Name())
	}
	panic(fmt.Sprintf(callSite+format, vals...))
}

func logf(format string, vals ...interface{}) {
	pc, _, _, ok := runtime.Caller(2)
	details := runtime.FuncForPC(pc)
	var callSite string
	if ok && details != nil {
		callSite = fmt.Sprintf("Called from %v: ", details.Name())
	}
	log.Printf(callSite+format, vals...)
}

func runOrder(runorder string) (idx []int, ok bool) {

	for _, s := range strings.Split(runorder, ",") {
		// Only care about the good values.
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			idx = append(idx, int(i))
		}
	}
	return idx, len(idx) > 0
}

// Cases takes a list of test cases to use for the table driven tests.
//   The test cases can be any type, as long as they are all the same.
func Cases(testcases ...TestCase) *Test {
	tc := Test{}
	for i, tcase := range testcases {
		val := reflect.ValueOf(tcase)
		if val.Kind() == reflect.Invalid {
			panicf("Testcase %v is not a valid test case.", i)
		}
		// The first element determines that type of the rest of the elements.
		if tc.vType == nil {
			tc.vType = val.Type()
		} else {
			if val.Type() != tc.vType {
				panicf("Testcases should be of type %v, but element %v is of type %v.", tc.vType, i, val.Type())
			}
		}
		tc.cases = append(tc.cases, val)
	}
	return &tc
}

func runTest(fn reflect.Value, idx int, testcase reflect.Value, tp bool, r bool) bool {
	var params []reflect.Value
	if tp {
		params = append(params, reflect.ValueOf(idx))
	}
	params = append(params, testcase)
	res := fn.Call(params)
	if r {
		return res[0].Bool()
	}
	return true
}

func runTests(list []int, fn reflect.Value, cases []reflect.Value, tp bool, r bool) int {
	count := 0
	for _, idx := range list {
		if idx < 0 || idx >= len(cases) {
			logf("Encountered invalid index %v, skipping.", idx)
			continue
		}
		count++
		if !runTest(fn, idx, cases[idx], tp, r) {
			break
		}
	}
	return count
}

func seq(n int) (idxs []int) {
	for i := 0; i < n; i++ {
		idxs = append(idxs, i)
	}
	return idxs
}

// Run calls the given function for each test case. (Note the function may be called again with the same testcase, if the tblTest.RunOrder option is specified.)
// The function must take one of four forms.
//
//    *  `func (tc $testcase)`
//
//    *  `func (tc $testcase) bool`
//
//    *  `func (idx int, tc $testcase)`
//
//    *  `func (idx int, tc $testcase) bool`
//
func (tc *Test) Run(function TestFunc) int {

	if function == nil {
		fmt.Fprintf(os.Stderr, "WARNING: on %v : Run called with nil function, skipping", MyCallerFileLine())
		return 0
	}

	fn := reflect.ValueOf(function)
	fnType := fn.Type()

	if fnType.Kind() != reflect.Func {
		panicf("Was not provided a function.")
	}
	// Check the parameters.
	var twoInParams bool
	var hasOutParam bool
	switch fnType.NumIn() {
	// If there is only one parameter then it should of the test case type.
	case 1:
		if fnType.In(0) != tc.vType {
			panicf("Incorrect parameter for test function given. Was given %v, expected it to be %v", fnType.In(0), tc.vType)
		}
	case 2:
		if fnType.In(0) != reflect.TypeOf(int(1)) {
			panicf("Incorrect parameter one for test function given. Was given %v, expected it to be int", fnType.In(0))
		}
		if fnType.In(1) != tc.vType {
			panicf("Incorrect parameter two for test function given. Was given %v, expected it to be %v", fnType.In(0), tc.vType)
		}
		twoInParams = true
	default:
		panicf("Incorrect number of parameters given. Expect function to take one of two forms. func(idx int, testcase $T) or func(testcase $T)")
	}
	switch fnType.NumOut() {
	case 0:
	// Nothing to do.
	case 1:
		if fnType.Out(0) != reflect.TypeOf(true) {
			panicf("Expected out parameter of test function to be a boolean. Was given %v", fnType.Out(0))
		}
		hasOutParam = true
	default:
		panicf("Expected there to be not out parameters or a boolean out parameter to test function.")
	}
	if len(tc.cases) == 0 {
		return 0
	}
	// Now loop through the test cases and call the test function, check to see if we should stop or keep going.
	return runTests(tc.runOrder(), fn, tc.cases, twoInParams, hasOutParam)
}

// AddCases takes a list of test cases to use for the table driven tests. It is added to the current list of tests.
//   The test cases can be any type, as long as they are ALL the tests are of the same type, this included any tests declared
// in the Cases methods to create the test object.
func (tc *Test) AddCases(testcases ...TestCase) {
	for i, tcase := range testcases {
		val := reflect.ValueOf(tcase)
		if val.Kind() == reflect.Invalid {
			panicf("Testcase %v is not a valid test case.", i)
		}
		// The first element determines that type of the rest of the elements.
		if tc.vType == nil {
			tc.vType = val.Type()
		} else {
			if val.Type() != tc.vType {
				panicf("Testcases should be of type %v, but element %v is of type %v.", tc.vType, i, val.Type())
			}
		}
		tc.cases = append(tc.cases, val)
	}
}

func (tc *Test) runOrder() []int {

	if runorder != nil && *runorder != "" {
		if idxs, ok := runOrder(*runorder); ok {
			return idxs
		}
	}
	if tc.RunOrder != "" {
		if idxs, ok := runOrder(tc.RunOrder); ok {
			return idxs
		}
	}
	if tc.InOrder {
		return seq(len(tc.cases))
	}
	return rand.Perm(len(tc.cases))
}
