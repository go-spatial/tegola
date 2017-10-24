package plyg

import (
	"encoding/gob"
	"os"

	"github.com/pborman/uuid"
)

func genWriteoutCols(cols ...RingCol) string {
	fn := "debug-uuid-" + uuid.New()
	WriteoutCols(fn, cols...)
	return fn
}

func WriteoutCols(filename string, cols ...RingCol) {
	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	enc := gob.NewEncoder(f)
	err = enc.Encode(cols)
	if err != nil {
		panic(err)
	}
}

func LoadCols(filename string) (cols []RingCol) {
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	dec := gob.NewDecoder(f)
	err = dec.Decode(&cols)
	if err != nil {
		panic(err)
	}
	return cols
}
