package wkt

import (
	"bytes"
	"io"
	"strings"

	"github.com/go-spatial/geom"
)

func Encode(w io.Writer, geo geom.Geometry) error {
	return NewDefaultEncoder(w).Encode(geo)
}

func EncodeBytes(geo geom.Geometry) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := Encode(buf, geo)
	return buf.Bytes(), err
}

func EncodeString(geo geom.Geometry) (string, error) {
	byt, err := EncodeBytes(geo)
	return string(byt), err
}

// MustEncode will use the default encoder to encode the provided Geometry
// if there is an error encoding the geometry, the function will panic.
func MustEncode(geo geom.Geometry) string {
	str, err := EncodeString(geo)
	if err != nil {
		panic(err)
	}

	return str
}

func Decode(r io.Reader) (geo geom.Geometry, err error) {
	return NewDecoder(r).Decode()
}

func DecodeBytes(b []byte) (geo geom.Geometry, err error) {
	return Decode(bytes.NewReader(b))
}

func DecodeString(s string) (geo geom.Geometry, err error) {
	return Decode(strings.NewReader(s))
}

func MustDecode(geo geom.Geometry, err error) geom.Geometry {
	if err != nil {
		panic(err)
	}
	return geo
}
