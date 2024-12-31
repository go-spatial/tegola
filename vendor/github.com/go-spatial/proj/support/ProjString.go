// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package support

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-spatial/proj/merror"
)

// Pair is a simple key-value pair
// Pairs use copy semantics (pass-by-value).
type Pair struct {
	Key   string
	Value string
}

//---------------------------------------------------------------------

// ProjString represents a "projection string", such as "+proj=utm +zone=11 +datum=WGS84"
//
// It is just an array of Pair objects.
// (We can't use a map because order of the items is important and
// because we might have duplicate keys.)
//
// TODO: we don't support the "pipeline" or "step" keywords
type ProjString struct {
	Pairs []Pair
}

// NewProjString returns a new ProjString from a string
// of the form "+proj=utm +zone=11 +datum=WGS84",
// with the leading "+" is optional and ignoring extra whitespace
func NewProjString(source string) (*ProjString, error) {

	ret := &ProjString{
		Pairs: []Pair{},
	}

	source = collapse(source)

	words := strings.Fields(source)
	for _, w := range words {

		var pair Pair

		if w[0:1] == "+" {
			w = w[1:]
		}

		v := strings.Split(w, "=")

		if v[0] == "" {
			return nil, merror.New(merror.InvalidProjectionSyntax, source)
		}

		switch len(v) {
		case 0:
			pair.Key = w
			pair.Value = ""
		case 1:
			// "proj=" is okay
			pair.Key = v[0]
			pair.Value = ""
		case 2:
			pair.Key = v[0]
			pair.Value = v[1]

		default:
			// "proj=utm=bzzt"
			return nil, merror.New(merror.InvalidProjectionSyntax, v)
		}

		ret.Add(pair)
	}

	return ret, nil
}

// handle extra whitespace in lines like "  +proj = merc   x = 1.2  "
func collapse(s string) string {
	re_leadclose_whtsp := regexp.MustCompile(`^[\s\p{Zs}]+|[\s\p{Zs}]+$`)
	re_inside_whtsp := regexp.MustCompile(`[\s\p{Zs}]{2,}`)
	re_equal_whtsp := regexp.MustCompile(`\s?[=]\s?`)
	s = re_leadclose_whtsp.ReplaceAllString(s, "")
	s = re_inside_whtsp.ReplaceAllString(s, " ")
	s = re_equal_whtsp.ReplaceAllString(s, "=")

	return s
}

// DeepCopy returns a detached deep copy of the ProjString
func (pl *ProjString) DeepCopy() *ProjString {

	copy := &ProjString{
		Pairs: []Pair{},
	}

	for _, pair := range pl.Pairs {
		pair2 := pair
		copy.Pairs = append(copy.Pairs, pair2)
	}

	return copy
}

func (pl *ProjString) String() string {
	b, err := json.MarshalIndent(pl, "", "    ")
	if err != nil {
		panic(err)
	}

	return string(b)
}

// Len returns the number of pairs in the list
func (pl *ProjString) Len() int {
	return len(pl.Pairs)
}

// Get returns the ith pair in the list
func (pl *ProjString) Get(i int) Pair {
	return pl.Pairs[i]
}

// Add adds a Pair to the end of the list
func (pl *ProjString) Add(pair Pair) {
	pl.Pairs = append(pl.Pairs, pair)
}

// AddList adds a ProjString's items to the end of the list
func (pl *ProjString) AddList(list *ProjString) {
	pl.Pairs = append(pl.Pairs, list.Pairs...)
}

// ContainsKey returns true iff the key is present in the list
func (pl *ProjString) ContainsKey(key string) bool {

	for _, pair := range pl.Pairs {
		if pair.Key == key {
			return true
		}
	}

	return false
}

// CountKey returns the number of times the key is in the list
func (pl *ProjString) CountKey(key string) int {

	count := 0
	for _, pair := range pl.Pairs {
		if pair.Key == key {
			count++
		}
	}

	return count
}

// get returns the (string) value of the first occurrence of the key
func (pl *ProjString) get(key string) (string, bool) {

	for _, pair := range pl.Pairs {
		if pair.Key == key {
			return pair.Value, true
		}
	}

	return "", false
}

// GetAsString returns the value of the first occurrence of the key, as a string
func (pl *ProjString) GetAsString(key string) (string, bool) {

	return pl.get(key)
}

// GetAsInt returns the value of the first occurrence of the key, as an int
func (pl *ProjString) GetAsInt(key string) (int, bool) {
	value, ok := pl.get(key)
	if !ok {
		return 0, false
	}
	i64, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, false
	}

	return int(i64), true
}

// GetAsFloat returns the value of the first occurrence of the key, as a float64
func (pl *ProjString) GetAsFloat(key string) (float64, bool) {

	value, ok := pl.get(key)
	if !ok {
		return 0.0, false
	}

	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0.0, false
	}

	return f, true
}

// GetAsFloats returns the value of the first occurrence of the key,
// interpreted as comma-separated floats
func (pl *ProjString) GetAsFloats(key string) ([]float64, bool) {

	value, ok := pl.get(key)
	if !ok {
		return nil, false
	}

	nums := strings.Split(value, ",")

	floats := make([]float64, 0, len(nums))

	for _, num := range nums {
		f, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return nil, false
		}
		floats = append(floats, f)
	}

	return floats, true
}
