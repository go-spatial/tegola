package cql2

import (
	"strings"
	"testing"
)

func TestCanReferenceProperty(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"n", true}, {"AND", true}, {"TRUE", true}, {"LIKE", true},
		{"地物", true}, {"falſe", true}, {"LIKE", true},
		{"_source.value7", true}, {":name", true}, {"e\u0301", true},
		{"\U000EFFFF", true}, {strings.Repeat("a", maxPropertyBytes), true},
		{strings.Repeat("é", maxPropertyBytes/2), true},
		{"", false}, {"has space", false}, {"7name", false},
		{"a-b", false}, {"a\"b", false}, {"\"AND\"", false},
		{"a'b", false}, {"a\\b", false}, {"a\x00b", false},
		{"\u0301a", false}, {"\u1FFF", false}, {"\U000F0000", false},
		{string([]byte{0xff}), false}, {strings.Repeat("a", maxPropertyBytes+1), false},
		{strings.Repeat("é", maxPropertyBytes/2) + "a", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanReferenceProperty(tc.name); got != tc.want {
				t.Fatalf("addressable=%v want %v", got, tc.want)
			}
			if tc.want {
				expression, err := Parse(`"` + tc.name + `" IS NULL`)
				if err != nil || expression.Root().Property != tc.name {
					t.Fatal("addressable alias not preserved by quoted parser", err)
				}
			}
		})
	}
}
