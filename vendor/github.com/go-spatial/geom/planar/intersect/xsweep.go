package intersect

import (
	"context"
	"errors"
	"log"
	"sort"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/geom/planar"
)

// ErrStopIteration is used to stop the iteration.
var ErrStopIteration = errors.New("Stop Iteration")

type eventType uint8

const (
	LEFT eventType = iota
	RIGHT
)

func (et eventType) String() string {
	switch et {
	default:
		return "UNKNOWN"
	case LEFT:
		return "LEFT"
	case RIGHT:
		return "RIGHT"
	}
}

type event struct {
	edge     int        // the indext number of the edge in the segment list.
	edgeType eventType  // Is this the left or right edge.
	ev       geom.Point // event vertex
}

type EventQueue struct {
	events   []event
	segments []geom.Line
	CMP      cmp.Compare
}

func (e *event) Point() geom.Point { return e.ev }
func (e *event) Edge() int         { return e.edge }

func (eq EventQueue) Len() int      { return len(eq.events) }
func (eq EventQueue) Swap(i, j int) { eq.events[i], eq.events[j] = eq.events[j], eq.events[i] }
func (eq EventQueue) Less(i, j int) bool {
	pt1, pt2 := eq.events[i].ev, eq.events[j].ev
	if pt1[0] != pt2[0] {
		return pt1[0] < pt2[0]
	}
	// We want to sort all the lines that are ending to the bottom of the list. So, that the lines that are
	// starting will get picked up.
	if eq.events[i].edgeType != eq.events[j].edgeType {
		return eq.events[i].edgeType < eq.events[j].edgeType
	}
	return pt1[1] < pt2[1]
}

// Ref: http://geomalgorithms.com/a09-_intersect-3.html#simple_Polygon()
func NewEventQueue(segments []geom.Line) (eq EventQueue) {

	// the event queue
	eq.events = make([]event, len(segments)*2)
	eq.segments = make([]geom.Line, len(segments))
	copy(eq.segments, segments)

	for i := range eq.segments {
		idx := 2 * i
		eq.events[idx].edge = i
		eq.events[idx+1].edge = i
		p1 := eq.segments[i][0]
		p2 := eq.segments[i][1]
		eq.events[idx].ev = p1
		eq.events[idx+1].ev = p2
		if p1[0] < p2[0] || (p1[0] == p2[0] && p1[1] < p2[1]) {
			eq.events[idx].edgeType = LEFT
			eq.events[idx+1].edgeType = RIGHT
		} else {
			eq.events[idx].edgeType = RIGHT
			eq.events[idx+1].edgeType = LEFT
		}
	}
	sort.Sort(eq)
	if debug {
		log.Println("Got the following for eq:\n\tevents:   ")
		for i, ev := range eq.events {
			log.Printf("\t\t%03v:i: %02v : %v - [%v, %v]", i, ev.edge, ev.edgeType, ev.ev[0], ev.ev[1])
		}
		log.Println("\n\tsegments: ")
		for i, seg := range eq.segments {
			log.Printf("\t\t%03v:%v", i, seg)
		}
	}
	eq.CMP = cmp.DefaultCompare()
	return eq
}

func (eq *EventQueue) FindIntersects(ctx context.Context, connected bool, fn func(src, dest int, pt [2]float64) error) error {
	segmap := make(map[int]struct{})
	cmp := eq.CMP
	keys := make([]int, 0, 2)
	for _, ev := range eq.events {
		if err := ctx.Err(); err != nil {
			return err
		}
		edgeidx := ev.edge
		if debug {
			log.Printf("Checkout out edge: %v", edgeidx)
		}
		// this is the first point in the segment, so we will just add it to our segment map and move on.
		if ev.edgeType == LEFT {
			segmap[edgeidx] = struct{}{}
			continue
		}
		// We have found the end of a line. Let's see if there it intersects with the other lines.
		// first we need to remove the line from our set of lines.
		delete(segmap, edgeidx)
		seg := eq.segments[edgeidx]
		if debug {
			log.Printf("Got to the edge of a segment: %v", seg)
		}

		if len(segmap) == 0 {
			continue
		}
		keys = keys[:0]
		{
			for k := range segmap {
				keys = append(keys, k)
			}
			sort.Sort(sort.IntSlice(keys))
		}
		if debug {
			log.Printf("Keys are: %v", keys)
		}
		for _, edge := range keys {
			seg1 := eq.segments[edge]
			// we need to see if , the ipt is the endpoint of both lines, (it's a connecting point) and
			// the polygonCheck is true, then it should not count as an intersect.
			if connected {
				// check to see if the end point of the segments are the same if they are we
				// continue
				matchStartPt := cmp.PointEqual(seg[0], seg1[0]) || cmp.PointEqual(seg[0], seg1[1])
				matchEndPt := cmp.PointEqual(seg[1], seg1[0]) || cmp.PointEqual(seg[1], seg1[1])
				if matchStartPt || matchEndPt {
					continue
				}
			}

			ipt, ok := planar.SegmentIntersect(seg, seg1)
			if debug {
				log.Printf("Looking at \n\tLine1(%v): %v \n\tLine2(%v): %v", edgeidx, seg, edge, seg1)
				if ok {
					log.Printf("Found ipt: %v", ipt)
				} else {
					log.Printf("Did not find point")
				}
			}
			if !ok {
				// Check the next edge.
				continue
			}
			if err := fn(edgeidx, edge, ipt); err != nil {
				// We were told not to continue.
				if err == ErrStopIteration {
					return nil
				}
				return err
			}
		}
	}
	return nil
}
