package protocol

import (
	"cmp"
	"errors"
	"slices"
	"unique"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

const noFieldName uint32 = 0xFFFFFFFF

type ofsHandle struct {
	ofs    uint32
	handle unique.Handle[string]
}

type fieldNames struct { // use struct here to get a stable pointer
	ofsHandles []ofsHandle
}

func (fn *fieldNames) search(ofs uint32) (int, bool) {
	return slices.BinarySearchFunc(fn.ofsHandles, ofsHandle{ofs: ofs}, func(a, b ofsHandle) int {
		return cmp.Compare(a.ofs, b.ofs)
	})
}

func (fn *fieldNames) insertOfs(ofs uint32) {
	if ofs == noFieldName {
		return
	}
	i, found := fn.search(ofs)
	if found { // duplicate
		return
	}

	if i >= len(fn.ofsHandles) { // append
		fn.ofsHandles = append(fn.ofsHandles, ofsHandle{ofs: ofs})
	} else {
		fn.ofsHandles = append(fn.ofsHandles, ofsHandle{})
		copy(fn.ofsHandles[i+1:], fn.ofsHandles[i:])
		fn.ofsHandles[i] = ofsHandle{ofs: ofs}
	}
}

func (fn *fieldNames) name(ofs uint32) string {
	if i, found := fn.search(ofs); found {
		return fn.ofsHandles[i].handle.Value()
	}
	return ""
}

func (fn *fieldNames) decode(dec *encoding.Decoder) error {
	// TODO sniffer - python client texts are returned differently?
	// - double check offset calc (CESU8 issue?)
	var errs []error

	pos := uint32(0)
	for i, ofsHandle := range fn.ofsHandles {
		diff := int(ofsHandle.ofs - pos)
		if diff > 0 {
			dec.Skip(diff)
		}
		n, s, err := dec.CESU8LIString()
		if err != nil {
			errs = append(errs, err)
		}
		fn.ofsHandles[i].handle = unique.Make(s)
		// len byte + size + diff
		pos += uint32(n + diff) //nolint: gosec
	}
	return errors.Join(errs...)
}
