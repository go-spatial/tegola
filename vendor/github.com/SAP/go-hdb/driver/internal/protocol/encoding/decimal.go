package encoding

import (
	"math/big"
	"math/bits"
)

const _S = bits.UintSize / 8 // word size in bytes
// http://en.wikipedia.org/wiki/Decimal128_floating-point_format
const dec128Bias = 6176
const decSize = 16

var natOne = big.NewInt(1)
