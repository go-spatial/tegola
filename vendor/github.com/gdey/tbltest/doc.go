// Copyright 2016 Gautam Dey. All rights reserved.
// Use of this source code is governed by FreeBDS License (2-clause Simplified BSD.)
// that can be found in the LICENSE file.

// Package tbltest implements helper functions to help write table driven tests.
//
// The most common way to use this package is in a test function.
//
//   func TestFoo(t testing.T){
//      type testcase struct {
//          first    string
//          last     string
//          expected string
//      }
//      test := tbltest.Cases(
//          testcase{
//              first    : "Gautam",
//              last     : "Dey",
//              expected : "Gautam Dey"
//          },
//      )
//      test.Run(func(tc testcase){
//          if tc.expected != join(tc.first," ",tc.last) {
//              t.Errorf("Failed test.")
//          }
//      })
//   }
//
//  The function provided to run is called for each testcase in the cases.
//  If the function provided returns a boolean, the return value will be used to
//  determine weather or not to continue onto the next testcase.
package tbltest
