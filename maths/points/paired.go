package points

import (
	"fmt"

	"github.com/terranodo/tegola/maths"
)

func Paired(pts []maths.Pt) ([][2]maths.Pt, error) {
	if len(pts) <= 1 {
		return nil, fmt.Errorf("Not enough pts to make pairs.")
	}
	n := len(pts)
	switch n {

	case 2:
		return [][2]maths.Pt{
			[2]maths.Pt{pts[0], pts[1]},
		}, nil
	case 3:
		return [][2]maths.Pt{
			[2]maths.Pt{pts[0], pts[1]},
			[2]maths.Pt{pts[0], pts[2]},
			[2]maths.Pt{pts[1], pts[2]},
		}, nil
	case 4:
		return [][2]maths.Pt{
			[2]maths.Pt{pts[0], pts[1]},
			[2]maths.Pt{pts[0], pts[2]},
			[2]maths.Pt{pts[0], pts[3]},
			[2]maths.Pt{pts[1], pts[2]},
			[2]maths.Pt{pts[1], pts[3]},
			[2]maths.Pt{pts[2], pts[3]},
		}, nil

	default:

		ret := make([][2]maths.Pt, n*(n-1)/2)
		c := 0
		for i := 0; i < n-1; i++ {
			for j := i + 1; j < n; j++ {
				ret[c][0], ret[c][1] = pts[i], pts[j]
				c++
			}
		}
		return ret, nil
	}
}
