// +build gofuzz

package wkb

func Fuzz(data []byte) int {
	
	if geom, err := DecodeBytes(data); err != nil {
		if geom != nil {
			panic("geom != nil on error")
		}
		//return 0
	}

	if bs, err := EncodeBytes(data); err != nil {
		if bs != nil {
			panic("bs != nil on error")
		}
		return 0
	}
	
	return 1

}
