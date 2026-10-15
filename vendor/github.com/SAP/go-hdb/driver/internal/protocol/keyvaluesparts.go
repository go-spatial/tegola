package protocol

import (
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

type clientInfo map[string]string

func (c clientInfo) String() string { return fmt.Sprintf("%v", map[string]string(c)) }
func (c clientInfo) numArg() int    { return len(c) }
func (c *clientInfo) decode(dec *encoding.Decoder, header *PartHeader, attrs *ReaderAttrs) error {
	*c = clientInfo{} // no reuse of maps - create new one

	for range header.numArg() {
		k, err := dec.Cesu8Field()
		if err != nil {
			return err
		}
		v, err := dec.Cesu8Field()
		if err != nil {
			return err
		}
		// Cesu8Field returns untyped nil for a null field (see decode.go). A null key
		// has no meaning as a map key, so skip the entry; a null value becomes "".
		kb, ok := k.([]byte)
		if !ok {
			continue
		}
		vb, _ := v.([]byte)
		(*c)[string(kb)] = string(vb)
	}
	return nil
}
func (c clientInfo) encode(enc *encoding.Encoder) error {
	for k, v := range c {
		if err := enc.Cesu8Field(k); err != nil {
			return err
		}
		if err := enc.Cesu8Field(v); err != nil {
			return err
		}
	}
	return nil
}
