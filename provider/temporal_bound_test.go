package provider

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTemporalExactRemainderValidation(t *testing.T) {
	instant := time.Unix(-1, 999999999).UTC()
	for _, test := range []struct {
		start, end string
		invalid    bool
	}{
		{"1", "2", false}, {"2", "1", true}, {"10", "1", false}, {"", strings.Repeat("0", 10000), false},
		{strings.Repeat("0", 9999) + "1", "", true}, {"1x", "2", true}, {"+1", "2", true},
	} {
		q := TemporalConstraint{Start: &instant, End: &instant, StartSubNanosecond: test.start, EndSubNanosecond: test.end}
		err := q.Validate()
		var invalid InvalidFeatureQueryError
		if test.invalid != errors.As(err, &invalid) {
			t.Fatalf("fraction case invalid=%v error=%v", test.invalid, err)
		}
	}
	if err := (TemporalConstraint{Start: &instant, EndSubNanosecond: "0"}).Validate(); err == nil {
		t.Fatal("orphan remainder accepted")
	}
}

func TestTemporalLeapOrdering(t *testing.T) {
	previous := time.Date(2016, 12, 31, 23, 59, 59, 900000000, time.UTC)
	leap := time.Date(2016, 12, 31, 23, 59, 59, 100000000, time.UTC)
	next := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, q := range []TemporalConstraint{
		{Start: &previous, End: &leap, EndLeapSecond: true},
		{Start: &leap, StartLeapSecond: true, End: &next},
		{Start: &leap, StartLeapSecond: true, StartSubNanosecond: "1", End: &leap, EndLeapSecond: true, EndSubNanosecond: "2"},
	} {
		if err := q.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []TemporalConstraint{
		{Start: &leap, StartLeapSecond: true, End: &previous},
		{Start: &next, End: &leap, EndLeapSecond: true},
		{Start: &leap, StartLeapSecond: true, StartSubNanosecond: "2", End: &leap, EndLeapSecond: true, EndSubNanosecond: "1"},
		{Start: &next, StartLeapSecond: true, End: &next},
	} {
		if err := q.Validate(); err == nil {
			t.Fatal("invalid leap constraint accepted")
		}
	}
	if !IsPositiveLeapSecondPredecessor(leap.In(time.FixedZone("offset", 3600))) {
		t.Fatal("timezone changed insertion")
	}
	for _, date := range []time.Time{
		time.Date(1971, 12, 31, 23, 59, 59, 0, time.UTC),
		time.Date(2017, 12, 31, 23, 59, 59, 0, time.UTC),
		time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
	} {
		if IsPositiveLeapSecondPredecessor(date) {
			t.Fatal("unannounced insertion accepted")
		}
	}
}
