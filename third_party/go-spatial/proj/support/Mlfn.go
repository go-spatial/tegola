// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package support

import (
	"math"

	"github.com/go-spatial/proj/merror"
)

/* meridional distance for ellipsoid and inverse
**	8th degree - accurate to < 1e-5 meters when used in conjunction
**		with typical major axis values.
**	Inverse determines phi to EPS (1e-11) radians, about 1e-6 seconds.
 */
const c00 = 1.
const c02 = .25
const c04 = .046875
const c06 = .01953125
const c08 = .01068115234375
const c22 = .75
const c44 = .46875
const c46 = .01302083333333333333
const c48 = .00712076822916666666
const c66 = .36458333333333333333
const c68 = .00569661458333333333
const c88 = .3076171875
const eps = 1e-11
const maxIter = 10
const enSize = 5

// Enfn is ..?
func Enfn(es float64) []float64 {
	var t float64

	en := make([]float64, enSize)

	en[0] = c00 - es*(c02+es*(c04+es*(c06+es*c08)))
	en[1] = es * (c22 - es*(c04+es*(c06+es*c08)))
	t = es * es
	en[2] = t * (c44 - es*(c46+es*c48))
	t *= es
	en[3] = t * (c66 - es*c68)
	en[4] = t * es * c88

	return en
}

// Mlfn is ..?
func Mlfn(phi float64, sphi float64, cphi float64, en []float64) float64 {
	cphi *= sphi
	sphi *= sphi
	return (en[0]*phi - cphi*(en[1]+sphi*(en[2]+sphi*(en[3]+sphi*en[4]))))
}

// InvMlfn is ..?
func InvMlfn(arg float64, es float64, en []float64) (float64, error) {
	var s, t, phi float64
	k := 1. / (1. - es)
	var i int

	phi = arg
	for i = maxIter; i != 0; i-- { /* rarely goes over 2 iterations */
		s = math.Sin(phi)
		t = 1. - es*s*s
		t = (Mlfn(phi, s, math.Cos(phi), en) - arg) * (t * math.Sqrt(t)) * k
		phi -= t
		if math.Abs(t) < eps {
			return phi, nil
		}
	}

	return phi, merror.New(merror.InvMlfn)
}
