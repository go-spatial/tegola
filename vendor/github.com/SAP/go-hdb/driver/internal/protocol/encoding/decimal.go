package encoding

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/bits"
)

// ErrDecimalOutOfRange means that a big.Rat exceeds the size of hdb decimal fields.
var ErrDecimalOutOfRange = errors.New("decimal out of range error")

// Decimal is the internal representation of a decimal value as mantissa and exponent.
type Decimal struct {
	m   *big.Int
	exp int
}

// NewDecimalFromRat creates a Decimal (dec128) from a *big.Rat.
func NewDecimalFromRat(r *big.Rat) (Decimal, error) {
	m := new(big.Int)
	exp, df := convertRatToDecimal(r, m, dec128Digits, dec128MinExp, dec128MaxExp)
	if df&dfOverflow != 0 {
		return Decimal{}, ErrDecimalOutOfRange
	}
	if df&dfUnderflow != 0 { // set to zero
		return Decimal{m: m.Set(natZero), exp: 0}, nil
	}
	return Decimal{m: m, exp: exp}, nil
}

// NewFixedFromRat creates a Decimal (fixed) from a *big.Rat.
func NewFixedFromRat(r *big.Rat, prec, scale int) (Decimal, error) {
	m := new(big.Int)
	df := convertRatToFixed(r, m, prec, scale)
	if df&dfOverflow != 0 {
		return Decimal{}, ErrDecimalOutOfRange
	}
	return Decimal{m: m, exp: -scale}, nil
}

// decomposeToRat converts database/sql decimalDecompose parts into a *big.Rat.
func decomposeToRat(form byte, negative bool, coefficient []byte, exponent int32) (*big.Rat, error) {
	if form != 0 { // 0 finite, 1 infinite, 2 NaN
		return nil, ErrDecimalOutOfRange
	}
	m := new(big.Int).SetBytes(coefficient) // big-endian magnitude
	if negative {
		m.Neg(m)
	}
	r := new(big.Rat)
	if exponent >= 0 {
		r.SetInt(m.Mul(m, exp10(int(exponent))))
	} else {
		r.SetFrac(m, exp10(int(-exponent)))
	}
	return r, nil
}

// NewDecimalFromDecompose creates a Decimal (dec128) from decimalDecompose parts.
func NewDecimalFromDecompose(form byte, negative bool, coefficient []byte, exponent int32) (Decimal, error) {
	r, err := decomposeToRat(form, negative, coefficient, exponent)
	if err != nil {
		return Decimal{}, err
	}
	return NewDecimalFromRat(r)
}

// NewFixedFromDecompose creates a Decimal (fixed) from decimalDecompose parts.
func NewFixedFromDecompose(form byte, negative bool, coefficient []byte, exponent int32, prec, scale int) (Decimal, error) {
	r, err := decomposeToRat(form, negative, coefficient, exponent)
	if err != nil {
		return Decimal{}, err
	}
	return NewFixedFromRat(r, prec, scale)
}

// AsRat returns the decimal value as a *big.Rat.
func (d Decimal) AsRat(v *big.Rat) {
	v.SetInt(d.m)
	p := v.Num()
	q := v.Denom()

	switch {
	case d.exp < 0:
		q.Set(exp10(d.exp * -1))
	case d.exp == 0:
		q.Set(natOne)
	case d.exp > 0:
		p.Mul(p, exp10(d.exp))
		q.Set(natOne)
	}
}

// Decompose implements the database/sql decimalDecompose interface.
func (d Decimal) Decompose(buf []byte) (form byte, negative bool, coefficient []byte, exponent int32) {
	negative = d.m.Sign() < 0
	if d.exp > math.MaxInt32 {
		panic(fmt.Sprintf("exp %d too big - expected <= %d", d.exp, math.MaxInt32))
	}
	exponent = int32(d.exp) //nolint: gosec

	abs := d.m
	if negative {
		abs = new(big.Int).Abs(d.m)
	}
	sizeInBytes := (abs.BitLen() + 7) / 8
	if cap(buf) >= sizeInBytes {
		buf = buf[:sizeInBytes]
		coefficient = abs.FillBytes(buf)
	} else {
		coefficient = abs.Bytes()
	}
	return form, negative, coefficient, exponent
}

const _S = bits.UintSize / 8 // word size in bytes
// http://en.wikipedia.org/wiki/Decimal128_floating-point_format
const dec128Bias = 6176
const decSize = 16

// decimals.
const (
	// http://en.wikipedia.org/wiki/Decimal128_floating-point_format
	dec128Digits = 34
	// 	dec128Bias   = 6176
	dec128MinExp = -6176
	dec128MaxExp = 6111
)

var (
	natZero = big.NewInt(0)
	natOne  = big.NewInt(1)
	natTen  = big.NewInt(10)
)

const maxNatExp10 = 38 // maximal fixed decimal precision

var natExp10 = make([]*big.Int, maxNatExp10)

func init() {
	natExp10[0], natExp10[1] = natOne, natTen
	for i := 2; i < maxNatExp10; i++ {
		natExp10[i] = new(big.Int).Mul(natExp10[i-1], natTen)
	}
}

func exp10(n int) *big.Int {
	if n < len(natExp10) {
		return natExp10[n]
	}
	r := big.NewInt(int64(n))
	return r.Exp(natTen, r, nil)
}

var lg10 = math.Log2(10)

func digits10(p *big.Int) int {
	k := p.BitLen()                   // 2^k <= p < 2^(k+1) - 1
	i := max(int(float64(k)/lg10), 1) // minimum 1
	// i <= digit10(p)
	for ; ; i++ {
		if p.Cmp(exp10(i)) < 0 {
			return i
		}
	}
}

// decimal flag.
const (
	dfNotExact byte = 1 << iota
	dfOverflow
	dfUnderflow
)

func convertRatToDecimal(x *big.Rat, m *big.Int, digits, minExp, maxExp int) (int, byte) {
	if x.Num().Cmp(natZero) == 0 { // zero
		m.Set(natZero)
		return 0, 0
	}

	var tmp big.Rat

	c := (&tmp).Set(x) // copy
	neg := c.Sign() < 0
	if neg {
		c.Neg(c) // magnitude
	}
	a := c.Num()
	b := c.Denom()

	var exp int
	shift := 0

	if c.IsInt() {
		exp = digits10(a) - 1
	} else {
		shift = digits10(a) - digits10(b)
		switch {
		case shift < 0:
			a.Mul(a, exp10(shift*-1))
		case shift > 0:
			b.Mul(b, exp10(shift))
		}
		if a.Cmp(b) == -1 {
			exp = shift - 1
		} else {
			exp = shift
		}
	}

	var df byte

	switch {
	default:
		exp = max(exp-digits+1, minExp)
	case exp < minExp:
		df |= dfUnderflow
		exp = exp - digits + 1
	}

	if exp > maxExp {
		df |= dfOverflow
	}

	shift = exp - shift
	switch {
	case shift < 0:
		a.Mul(a, exp10(shift*-1))
	case exp > 0:
		b.Mul(b, exp10(shift))
	}

	m.QuoRem(a, b, a) // reuse a as rest
	if a.Cmp(natZero) != 0 {
		// round (business >= 0.5 up)
		df |= dfNotExact
		if a.Add(a, a).Cmp(b) >= 0 {
			m.Add(m, natOne)
			if m.Cmp(exp10(digits)) == 0 {
				shift := min(digits, maxExp-exp)
				if shift < 1 { // overflow -> shift one at minimum
					df |= dfOverflow
					shift = 1
				}
				m.Set(exp10(digits - shift))
				exp += shift
			}
		}
	}

	// norm
	for exp < maxExp {
		a.QuoRem(m, natTen, b) // reuse a, b
		if b.Cmp(natZero) != 0 {
			break
		}
		m.Set(a)
		exp++
	}

	if neg {
		m.Neg(m) // restore sign
	}

	return exp, df
}

func convertRatToFixed(r *big.Rat, m *big.Int, prec, scale int) byte {
	if scale < 0 {
		panic("fixed: invalid scale")
	}

	var df byte

	m.Set(r.Num())
	m.Mul(m, exp10(scale))

	var tmp big.Rat

	c := (&tmp).SetFrac(m, r.Denom()) // norm
	neg := c.Sign() < 0
	if neg {
		c.Neg(c) // magnitude
	}
	a := c.Num()
	b := c.Denom()

	if b.Cmp(natZero) == 0 { //
		m.Set(a)
		if neg {
			m.Neg(m) // restore sign
		}
		return df
	}

	m.QuoRem(a, b, a) // reuse a as rest
	if a.Cmp(natZero) != 0 {
		// round (business >= 0.5 up)
		df |= dfNotExact
		if a.Add(a, a).Cmp(b) >= 0 {
			m.Add(m, natOne)
		}
	}

	maxInt := exp10(prec)
	minInt := new(big.Int).Neg(maxInt)

	if m.Cmp(minInt) <= 0 || m.Cmp(maxInt) >= 0 {
		df |= dfOverflow
	}
	if neg {
		m.Neg(m) // restore sign
	}
	return df
}
