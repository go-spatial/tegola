package wkb_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/terranodo/tegola/wkb"
)

type TestCase struct {
	bytes    []byte
	bom      binary.ByteOrder
	expected wkb.Geometry
}

func (tc *TestCase) Reader() io.Reader {
	return bytes.NewReader(tc.bytes)
}

type TestCases []TestCase

func (tcs TestCases) RunTests(t *testing.T, tester func(num int, test *TestCase)) {
	for i, test := range tcs {
		tester(i, &test)
	}
}
