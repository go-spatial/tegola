package filecache_test

import (
	"bytes"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/terranodo/tegola/cache/filecache"
)

func TestNew(t *testing.T) {
	testcases := []struct {
		config   map[string]interface{}
		expected *filecache.Filecache
	}{
		{
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
			},
			expected: &filecache.Filecache{
				Basepath: "testfiles/tegola-cache",
			},
		},
	}

	for i, tc := range testcases {
		output, err := filecache.New(tc.config)
		if err != nil {
			t.Errorf("testcase (%v) failed. err: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(tc.expected, output) {
			t.Errorf("testcase (%v) failed. expected (%+v) does not match output (%+v)", i, tc.expected, output)
			continue
		}
	}
}

func TestWriteReadPurge(t *testing.T) {
	testcases := []struct {
		config   map[string]interface{}
		key      string
		expected []byte
	}{
		{
			config: map[string]interface{}{
				"basepath": "testfiles/tegola-cache",
			},
			key:      "/osm/0/1/2.pbf",
			expected: []byte("\x53\x69\x6c\x61\x73"),
		},
	}

	for i, tc := range testcases {
		fc, err := filecache.New(tc.config)
		if err != nil {
			t.Errorf("testcase (%v) failed. err: %v", i, err)
			continue
		}

		//	wrap our data in a reader
		input := bytes.NewReader(tc.expected)

		//	test write
		if err = fc.Set(tc.key, input); err != nil {
			t.Errorf("testcase (%v) write failed. err: %v", i, err)
			continue
		}

		r, err := fc.Get(tc.key)
		if err != nil {
			t.Errorf("testcase (%v) read failed. err: %v", i, err)
			continue
		}

		//	test read
		output, err := ioutil.ReadAll(r)
		if err != nil {
			t.Errorf("testcase (%v) readAll failed. err: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("testcase (%v) failed. output (%v) does not match expected (%v)", i, output, tc.expected)
			continue
		}

		//	test purge
		if err = fc.Purge(tc.key); err != nil {
			t.Errorf("testcase (%v) failed. purge failed. err: %v", i, err)
			continue
		}
	}
}
