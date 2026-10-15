package gml

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/gml-corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ID    string          `json:"id"`
		XML   string          `json:"xml"`
		Want  json.RawMessage `json:"want"`
		Error bool            `json:"error"`
		Swap  bool            `json:"swap"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		hint := ""
		if tc.Swap {
			hint = "urn:ogc:def:crs:EPSG::4326"
		}
		g, err := ParseGeometry(tc.XML, hint)
		if tc.Error {
			if err == nil {
				t.Errorf("%s: expected error, got %T", tc.ID, g)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.ID, err)
			continue
		}
		raw, err := json.Marshal(g)
		if err != nil {
			t.Fatal(err)
		}
		var got, want interface{}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(tc.Want, &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %s want %s", tc.ID, raw, tc.Want)
		}
	}
}
