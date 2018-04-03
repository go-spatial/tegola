// +build gofuzz

package fuzz

func Fuzz(data []byte) int {
	
	if geom, err := DecodeBytes(data); err != nil {
		if geom != nil {
			panic("geom != nil on error")
		}
		return 0
	}

	return 1

}
