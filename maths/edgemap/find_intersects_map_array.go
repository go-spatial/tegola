// +build fiMapArray,!fiLinkedList,!fiBtree

package edgemap

import (
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/points"
)

func findIntersects(segments []maths.Line, skiplines []bool) {
	var offset = len(skiplines)
	var segLen = len(segments)

	var ptmap = make(map[maths.Pt][]int)

	var isegmap = make([]bool, segLen)

	seenEdgeCount := 0

	var s, i, idx, sl int
	var ok bool

	var eq = make([]maths.Pt, 0, segLen*2)
	for i = range segments {
		if _, ok = ptmap[segments[i][0]]; !ok {
			eq = append(eq, segments[i][0])
		}
		ptmap[segments[i][0]] = append(ptmap[segments[i][0]], i)
		if _, ok = ptmap[segments[i][1]]; !ok {
			eq = append(eq, segments[i][1])
		}
		ptmap[segments[i][1]] = append(ptmap[segments[i][1]], i)
	}
	points.ByXY(eq).Sort()

	var enableIds = make([]int, 0, 100)
	var widxs = make([]int, 0, 100)

	for i = range eq {

		idxs := ptmap[eq[i]]
		for _, j := range idxs {
			if !isegmap[j] {
				enableIds = append(enableIds, j)
				continue
			}
			isegmap[j] = false
			widxs = append(widxs, j)
		}
		seenEdgeCount = seenEdgeCount - len(widxs)

		if seenEdgeCount <= 0 {
			seenEdgeCount = 0
			widxs = widxs[:0]
		}
		for _, idx = range widxs {

			if idx >= offset {
				sl = offset
			} else {
				sl = segLen
			}

			//skipable := append(ptmap[segments[idx][0]], ptmap[segments[idx][1]]...)

			for s = 0; s < sl; s++ {
				for ; s < sl && !isegmap[s]; s++ {
					// skip forward.
				}
				if s >= sl {
					break
				}
				/*
					for k := 0; s < sl && k < len(skipable); k++ {
						if k == s {
							k = 0
							s++
						}
					}
					if s >= sl {
						break
					}

				*/
				if idx < offset && skiplines[idx] {
					continue
				}
				if s < offset && skiplines[s] {
					continue
				}
				if segments[idx][0].X == segments[s][0].X && segments[idx][0].Y == segments[s][0].Y {
					continue
				}
				if segments[idx][0].X == segments[s][1].X && segments[idx][0].Y == segments[s][1].Y {
					continue
				}
				if segments[idx][1].X == segments[s][0].X && segments[idx][1].Y == segments[s][0].Y {
					continue
				}
				if segments[idx][1].X == segments[s][1].X && segments[idx][1].Y == segments[s][1].Y {
					continue
				}
				if doesNotIntersect(segments[idx][0].X, segments[idx][0].Y, segments[idx][1].X, segments[idx][1].Y, segments[s][0].X, segments[s][0].Y, segments[s][1].X, segments[s][1].Y) {
					continue
				}
				if idx < offset {
					skiplines[idx] = true
					continue
				}
				// If idx >= offset then s has to be < offset. because of sl.
				skiplines[s] = true
			}
		}
		for _, j := range enableIds {
			isegmap[j] = true
		}
		seenEdgeCount = seenEdgeCount + len(enableIds)
		enableIds = enableIds[:0]
		widxs = widxs[:0]
	}

	return
}
