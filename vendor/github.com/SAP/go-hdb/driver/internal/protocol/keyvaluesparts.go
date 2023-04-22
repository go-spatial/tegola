package protocol

import (
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

type clientInfo map[string]string

func (c clientInfo) String() string { return fmt.Sprintf("%v", map[string]string(c)) }

func (c clientInfo) size() int {
	size := 0
	for k, v := range c {
		size += cesu8Type.prmSize(k)
		size += cesu8Type.prmSize(v)
	}
	return size
}

func (c clientInfo) numArg() int { return len(c) }

func (c *clientInfo) decode(dec *encoding.Decoder, ph *PartHeader) error {
	*c = clientInfo{} // no reuse of maps - create new one

	for i := 0; i < ph.numArg(); i++ {
		k, err := cesu8Type.decodeRes(dec)
		if err != nil {
			return err
		}
		v, err := cesu8Type.decodeRes(dec)
		if err != nil {
			return err
		}
		(*c)[string(k.([]byte))] = string(v.([]byte)) // set key value
	}
	return dec.Error()
}

func (c clientInfo) encode(enc *encoding.Encoder) error {
	for k, v := range c {
		if err := cesu8Type.encodePrm(enc, k); err != nil {
			return err
		}
		if err := cesu8Type.encodePrm(enc, v); err != nil {
			return err
		}
	}
	return nil
}
