package maths

// Exp2 returns powers of 2 on a uint64
func Exp2(p uint64) uint64 {
	// this mimics behavior from casting
	// a math.Exp2 which should overflow
	if p > 63 {
		p = 63
	}
	return uint64(1) << p
}
