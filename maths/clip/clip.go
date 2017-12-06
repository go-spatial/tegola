package clip

import (
	"errors"
	"log"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/clip/intersect"
	"github.com/terranodo/tegola/maths/clip/region"
	"github.com/terranodo/tegola/maths/clip/subject"
	"github.com/terranodo/tegola/maths/lines"
	"github.com/terranodo/tegola/maths/points"

	"fmt"

	colour "github.com/logrusorgru/aurora"
)

/*
Basics of the algorithm.

Given:

Clipping polygon

Subject polygon

Result:

One or more polygons clipped into the clipping polygon.


for each vertex for the clipping and subject polygon create a link list.

Say you have the following:

                  k——m
        β---------|--|-ℽ
        |         |  | |
 a——————|———b     |  | |
 |      |   |     |  | |
 |      |   |     |  | |
 |   d——|———c g———h  | |
 |   |  |     |      | |
 |   |  |     |      | |
 |   |  α-----|------|-δ
 |   e————————f      |
 |                   |
 p———————————————————n

 We will create the following linked lists:

    a → b → c → d → e → f → g → h → k → m → n → p →  a
    α → β → ℽ → δ → α


Now, we will iterate from through the vertices of the subject polygon (a to b, etc…) look for point of intersection with the
clipping polygon (α,β,ℽ,δ). When we come upon an intersection, we will insert an intersection point into both lists.

For example, examing vertex a, and b; against the line formed by (α,β). We notice we have an intersection at I.

                  k——m
        β---------|--|-ℽ
        |         |  | |
 a——————I———b     |  | |
 |      |   |     |  | |
 |      |   |     |  | |
 |   d——————c g———h  | |
 |   |  |     |      | |
 |   |  |     |      | |
 |   |  α-----|------|-δ
 |   e————————f      |
 |                   |
 p———————————————————n

 We will add I to both lists. We will also note that I, in heading into the clipping region.
 (We will also, mark a as being outside the clipping region, and b being inside the clipping region. If the point is on the boarder of the clipping polygon, it is considered outside of the clipping region.)

    a → I → b → c → d → e → f → g → h → k → m → n → p → a
    α → I → β → ℽ → δ → α

    We will also keep track of the intersections, and weather they are inbound or outbound.
    I(i)

We will check (a,b) against the line formed by (β,ℽ). And see there isn't an intersection.
We will check (a,b) against the line formed by (ℽ,δ). And see there isn't an intersection.
We will check (a,b) against the line formed by (δ,α). And see there isn't an intersection.

When we look at (b,c) we notice that they are both inside the clipping region. And move on to the next set of vertices.

We looking at (c,d), we notice that c is inside and d is outside. This means that there is an intersection point head out.
We check against the line formed by (α,β), and add a Point J, after checking to see we don't already have another equi to J; and adjust
the pointers accordingly. The point c in the subject will now point to J, and J will point to d. And for the intersecting line, α will now point
to J, and J will point to I, as that is what α was pointing to. Our lists will now look like the following.

    a → I → b → c → J → d → e → f → g → h → k → m → n → p → a
    α → J → I → β → ℽ → δ → α

    I(i), J(o)

We will check (c,d) against the line formed by (β,ℽ). And see there isn't an intersection.
We will check (c,d) against the line formed by (ℽ,δ). And see there isn't an intersection.
We will check (c,d) against the line formed by (δ,α). We see there is an intersection, but it is outside of the clipping area.

Next we look at (d,e), notice they are both outside the clipping area, and don't cross through the clipping aread. Thus we can ignore
the points.

                  k——m
        β---------|--|-ℽ
        |         |  | |
 a——————I———b     |  | |
 |      |   |     |  | |
 |      |   |     |  | |
 |   d——J———c g———h  | |
 |   |  |     |      | |
 |   |  |     |      | |
 |   |  α-----|------|-δ
 |   e————————f      |
 |                   |
 p———————————————————n


Next we look at (e,f), and just (d,e) we can ignore the points as they lie outside and don't cross the clipping area.

Now we look at (f,g), we notice that f is outside, and g is inside the clipping area. This means that The intersection is entering into the clipping area.

We will check (f,g) against the line formed by (α,β). And see there isn't an intersection.
We will check (f,g) against the line formed by (β,ℽ). And see there isn't an intersection.
We will check (f,g) against the line formed by (ℽ,δ). And see there isn't an intersection.

                  k——m
        β---------|--|-ℽ
        |         |  | |
 a——————I———b     |  | |
 |      |   |     |  | |
 |      |   |     |  | |
 |   d——J———c g———h  | |
 |   |  |     |      | |
 |   |  |     |      | |
 |   |  α-----K------|-δ
 |   e————————f      |
 |                   |
 p———————————————————n

We will check (f,g) against the line formed by (δ,α). We see there is an intersection, and from the previous statement, we know it's an intersection point that is heading inwards. We adjust the link lists to include the point.

    a → I → b → c → J → d → e → f → K → g → h → k → m → n → p → a
    α → J → I → β → ℽ → δ → K → α

    I(i), J(o), K(i)

Looking at (g,h) we realize they are both in the clipping area, and can ignore them.
Next we look at (h,k), here we see that h is inside and k is outside. This means that the intersection point will be outbound.

We will check (h,k) against the line formed by (α,β). And see there isn't an intersection.

                  k——m
        β---------L--|-ℽ
        |         |  | |
 a——————I———b     |  | |
 |      |   |     |  | |
 |      |   |     |  | |
 |   d——J———c g———h  | |
 |   |  |     |      | |
 |   |  |     |      | |
 |   |  α-----K------|-δ
 |   e————————f      |
 |                   |
 p———————————————————n

We will check (h,k) against the line formed by (β,ℽ). We see there is an intersection (L); also, note that we can stop look at the points, as we found the intersection. We adjust the link lists to include the point.

    a → I → b → c → J → d → e → f → K → g → h → L → k → m → n → p → a
    α → J → I → β → L → ℽ → δ → K → α

    I(i), J(o), K(i), L(o),


Next we look at (k,m) and notice they are not crossing the clipping area and are both outside. So, we know we can skip them.

Looking at (m,n); we notice they are both outside, but are crossing the clipping area, which means there will be two intersection points.

We will check (f,g) against the line formed by (α,β). And see there isn't an intersection.

                  k——m
        β---------L--M-ℽ
        |         |  | |
 a——————I———b     |  | |
 |      |   |     |  | |
 |      |   |     |  | |
 |   d——J———c g———h  | |
 |   |  |     |      | |
 |   |  |     |      | |
 |   |  α-----K------|-δ
 |   e————————f      |
 |                   |
 p———————————————————n


We will check (f,g) against the line formed by (β,ℽ). We find our first intersection point. We go ahead and insert point (M), as we have done for the other points. We know it's bound as it's the first point in the crossing.
We, adjust, the point we are comparing against from (m,n) to (M,n). Also, note we need to place the point in the correct position between β and ℽ, after L.

    a → I → b → c → J → d → e → f → K → g → h → L → k → m → M → n → p → a
    α → J → I → β → L → M → ℽ → δ → K → α

    I(i), J(o), K(i), L(o), M(i)


We will check (M,n) against the line formed by (ℽ,δ). And see there isn't an intersection.

                  k——m
        β---------L--M-ℽ
        |         |  | |
 a——————I———b     |  | |
 |      |   |     |  | |
 |      |   |     |  | |
 |   d——J———c g———h  | |
 |   |  |     |      | |
 |   |  |     |      | |
 |   |  α-----K------N-δ
 |   e————————f      |
 |                   |
 p———————————————————n

We will check (M,n) against the line formed by (δ,α). We see there is an intersection, and from the previous statement, we know it's an intersection point that is heading outwards. We adjust the link lists to include the point.

    a → I → b → c → J → d → e → f → K → g → h → L → k → m → M → N → n → p → a
    α → J → I → β → L → M → ℽ → δ → N → K → α

    I(i), J(o), K(i), L(o), M(i), N(o)

Next we look at (n,p), and know we can skip the points as they are both outside, and not crossing the clipping area.
Finally we look at (p,a), and again because they are both outside, and not corssing the clipping area, we know we can skip them.

Finally we check to see if we have at least one external and one internal point. If we don't have any external points, we know the polygon is contained within the clipping area and can just return it.
If we don't have any internal points, and no Intersections points, we know the polygon is contained compleatly outside and we can return any empty array of polygons.

First thing we do is iterate our list of Intersection points looking for the first point that is an inward bound point. I is such a point. The rule is if an intersection point is inward, we following the subject links, if it's outward we follow, the clipping links.
Since I is inward, we write it down, and follow the subject link to b.

LineString1: I,b

Then we follow the links till we get to the next Intersection point.

LineString1: I,b,c,J

        •··············•
        ·              ·
        +———+          ·
        |   |          ·
        |   |          ·
        +———+          ·
        ·              ·
        ·              ·
        •··············•



Since, J is outward we follow the clipping links, which leads us to I. Since I is also the first point in this line string. We know we are done, with the first clipped polygon.

Next we iterate to the next inward Intersection point from J, to K.
LineString1: I,b,c,J
LineString2: K

And as before since K is inward point we follow the subject polygon points, till we get to an intersection point.

LineString1: I,b,c,J
LineString2: K,g,h,L

As L is an outward intersection point we follow the clipping polygon points, till we get to an intersection point.

LineString1: I,b,c,J
LineString2: K,g,h,L,M

As M is an inward intersection point we follow the subject.

LineString1: I,b,c,J
LineString2: K,g,h,L,M,N

As N is an outward intersection point we follow the clipping, and discover that the point is our starting Intersection point K. That ends is our second clipped polygon.

LineString1: I,b,c,J
LineString2: K,g,h,L,M,N


        •·········+--+·•
        ·         |  | ·
        +———+     |  | ·
        |   |     |  | ·
        |   |     |  | ·
        +———+ +———+  | ·
        ·     |      | ·
        ·     |      | ·
        •·····+------+·•


Since N(o), is the end of the array we, start at the beginning and notice, that we already accounted for I(i). And so, we are done.

*/

func CleanLineString(sub []float64) [][]float64 {

	key := func(i, j int) string {
		return fmt.Sprintf("%v:%v", sub[i], sub[j])
	}

	var mainsub []float64
	var retsub [][]float64 // make sure there it at least one entry.
	seen := make(map[string][2]int)

	mainsub = append(mainsub, sub[0], sub[1])
	retsub = append(retsub, mainsub)
	seen[key(0, 1)] = [2]int{0, 0}
	for i, j := 2, 3; j < len(sub); i, j = i+2, j+2 {
		k := key(i, j)
		idxs, ok := seen[k]
		// log.Println("Looking at:[", i, j, "](", sub[i], sub[j], ")", k, ok, idxs)

		if ok && idxs[1] > 0 && idxs[1] < len(mainsub) {
			// We found an entry with with the name x,y we we now need to do is
			// go from that entry to us and snip out the
			// log.Println("Found id:", idxs[1], len(retsub[0]))

			newsub := sub[idxs[0]:i]

			if len(newsub) >= 6 {
				/*
					log.Printf("(i,j : %v,%v) Cutting out: retsub[0][%v:%v]", i, j, idxs[1], len(mainsub))
					log.Printf("%#v", newsub)
				*/
				retsub = append(retsub, newsub)
			}

			mainsub = mainsub[:idxs[1]]
			seen[k] = [2]int{i, len(mainsub) - 2}
			continue
		}
		//log.Println("Adding", k, i)
		retsub[0] = append(mainsub, sub[i], sub[j])
		seen[k] = [2]int{i, len(mainsub) - 2}
	}
	retsub[0] = mainsub
	return retsub
}
func validateCleanLinestring(g []float64) (l []float64, err error) {

	var ptsMap = make(map[maths.Pt][]int)
	var pts []maths.Pt
	i := 0
	for x, y := 0, 1; y < len(g); x, y = x+2, y+2 {

		p := maths.Pt{g[x], g[y]}
		ptsMap[p] = append(ptsMap[p], i)
		pts = append(pts, p)
		i++
	}

	for i := 0; i < len(pts); i++ {
		pt := pts[i]
		fpts := ptsMap[pt]
		l = append(l, pt.X, pt.Y)
		if len(fpts) > 1 {
			// we will need to skip a bunch of points.
			i = fpts[len(fpts)-1]
		}
	}
	return l, nil
}

// linestring does the majority of the clipping of the subject array to the square bounds defined by the rMinPt and rMaxPt.
func linestring(sub []float64, rMinPt, rMaxPt maths.Pt) (clippedSubjects [][]float64, err error) {

	// We need to run through clip to see if there are simplification artifacts that should be
	// split into their own lines to be clipped separately.

	// log.Println("Subject Length to clip", len(sub))
	if len(sub) <= 2 {
		return clippedSubjects, nil
	}

	sl, err := subject.New(sub)
	if err != nil {
		// log.Printf("Returning zero subjects was not able to create Subject List %v", err)
		return clippedSubjects, err
	}
	il := intersect.New()
	rl := region.New(sl.Winding(), rMinPt, rMaxPt)

	//log.Println("Region: ", rl.GoString())
	/*
		log.Println(sl.GoString())

		log.Println("Starting to work through the pair of points.")
	*/
	// BuildOutLists returns weather all subject points are contained by the region.
	placement, icount := buildOutLists(sl, rl, il)
	//log.Printf("Placement Code  %08b %08b %v", placement, PCSurrounded, icount)
	// If there were no intersect points, we need to determine were the points are in relation to the region.
	if icount == 0 {
		switch placement {
		case region.PCInside:
			clippedSubjects = append(clippedSubjects, sub)
			return clippedSubjects, nil
		case region.PCAllAround:

			// We have seen point in pretty much every sector, we need to run a contains check for a point,
			// in the region against the subject.
			// err is ignored because we know sub is good already.
			if ok, _ := maths.Contains(sub, rl.Min()); ok {
				clippedSubjects = append(clippedSubjects, rl.LineString())
				//log.Println("Returing entire region.", clippedSubjects)
				return clippedSubjects, nil
			}
			// Nothing to clip
			return clippedSubjects, nil

		default: // There are not crossings.
			return clippedSubjects, nil
		}
	}

	for w := il.FirstInboundPtWalker(); w != nil; w = w.Next() {
		//log.Printf("Looking at Inbound pt: %v", w.GoString())
		var s []float64
		var opt *maths.Pt
		w.Walk(func(idx int, pt maths.Pt) bool {
			// Only add point if it's not the same as the last point
			// or the first point in s.
			if opt != nil && opt.IsEqual(pt) {
				return true
			}

			//log.Printf("Adding point(%v): %v\n", idx, pt)
			s = append(s, pt.X, pt.Y)
			opt = &pt
			if idx == sl.Len() {
				// log.Printf("Return because we are at the end. %v %v", idx, sl.Len())
				return false
			}
			return true
		})

		// Must have at least 3 points for it to be a valid runstring. (3 *2 = 6)
		s, err = validateCleanLinestring(s)
		if err != nil {
			return nil, err
		}
		if quickValidityCheck(s) {
			//log.Printf("Adding s with len(%v)", len(s))
			clippedSubjects = append(clippedSubjects, s)
		}
	}

	//log.Println("Done walking through the Inbound Intersection points.")
	//log.Printf("Returning subjects(%v)", len(clippedSubjects))

	return clippedSubjects, nil
}

// buildOutLists takes the three lists and populates then from from each other.
func buildOutLists(sl *subject.Subject, rl *region.Region, il *intersect.Intersect) (placement region.PlacementCode, intersectCount int) {

	type isectype struct {
		pt  maths.Pt
		in  bool
		idx int
		p   *subject.Pair
		l   maths.Line
	}

	var ismap = make(map[string]isectype)

	// generates a key for the map above. This is use to lookup and intersect points for dedup and elimination
	var keysgen = func(p maths.Pt, in bool) (mykey, otherkey string) {
		tp := p.Truncate()
		return fmt.Sprintf("%v_%v_%v", tp.X, tp.Y, in), fmt.Sprintf("%v_%v_%v", tp.X, tp.Y, !in)
	}

	var keys []string
	// Iterate through all the point pairs in the subject.
	for i, p := 0, sl.FirstPair(); p != nil; p, i = p.Next(), i+1 {

		line := p.AsLine()
		intersects, pcpt1, pcpt2 := rl.Intersections(line)

		placement |= pcpt1 | pcpt2

		for _, isect := range intersects {
			mykey, otherkey := keysgen(isect.Pt, isect.Inward)
			addKey := true

			// cancel out points
			if _, ok := ismap[otherkey]; ok {
				addKey = false
				/*
					log.Printf("Found cancellation point \n%#v\n\t<=>\n%#v\n\n", ss, isectype{
						pt:  isect.Pt.Truncate(),
						in:  isect.Inward,
						idx: isect.Idx,
						p:   p,
						l:   line,
					})
				*/
				delete(ismap, otherkey)
				continue
			}
			// dedup points
			if _, ok := ismap[mykey]; ok {
				addKey = false
				/*
					log.Printf("Found duplication point \n%#v\n\t===\n%#v\n\n", ss, isectype{
						pt:  isect.Pt.Truncate(),
						in:  isect.Inward,
						idx: isect.Idx,
						p:   p,
						l:   line,
					})
				*/
			}
			if addKey {
				keys = append(keys, mykey)
			}

			ismap[mykey] = isectype{
				pt:  isect.Pt.Truncate(),
				in:  isect.Inward,
				idx: isect.Idx,
				p:   p,
				l:   line,
			}
			//log.Printf("Found intersect pt%v", ismap[mykey])
		}
	}

	for _, key := range keys {
		isect, ok := ismap[key]
		if !ok {
			continue
		}
		intersectCount++
		a := rl.Axis(isect.idx)
		ipt := intersect.NewPt(isect.pt, isect.in)
		//log.Printf("Found intersect pts[%v]:(%p)%[2]v : Subject: %#v Axises: %#v", i, ipt, isect.l, a.GoString())
		insertIntersectPoint(ipt, il, isect.p, a)
		//ipt.PrintNeighbors()

	}
	return placement, intersectCount
}

/*
func DrawSVGGraphs(sl *subject.Subject, rl *region.Region, il *intersect.Intersect) {
	// build out the lists.
	allin := buildOutLists(sl, rl, il)

}
*/
func highlightPoints(s []float64, x, y float64) string {
	str := ""
	for i, j := 0, 1; j < len(s); i, j = i+2, j+2 {
		val := fmt.Sprintf("[%v,%v]", s[i], s[j])
		if s[i] == x && s[j] == y {
			str += colour.Blue(val).String()
		} else {
			str += val
		}
	}
	return str
}

func quickValidityCheck(s []float64) bool {

	if len(s) < 6 {
		//log.Println("Number of elements smaller then 6", s)
		return false
	}

	for x, y := 0, 1; y < len(s)-2; x, y = x+2, y+2 {
		for x1, y1 := x+2, y+2; y1 < len(s); x1, y1 = x1+2, y1+2 {
			if s[x] == s[x1] && s[y] == s[y1] {
				/*
					log.Printf("Subject isn't Valid \n%v\n",
						highlightPoints(s, s[x], s[y]),
					)
					log.Printf("Found two points that are repeated. (%v,%v)[%v %v] (%v,%v)[%v %v]", x, y, s[x], s[y], x1, y1, s[x1], s[y1])
				*/
				return false
			}
		}
	}
	return true
}
func insertIntersectPoint(ipt *intersect.Point, il *intersect.Intersect, p *subject.Pair, a *region.Axis) {
	// Only care about inbound intersect points.
	if ipt.Inward {
		il.PushBack(ipt)
	}
	if !p.PushInBetween(ipt.AsSubjectPoint()) {
		log.Printf("// Was not able to add to point( %v ) to subject pair %v\n", ipt, p)
		//panic("Foo")
	}

	if !a.PushInBetween(ipt.AsRegionPoint()) {
		log.Printf("// Was not able to add to point( %v ) to region list %v\n", ipt, a)
	}

}

func linestring2floats(l tegola.LineString) (ls []float64) {
	for _, p := range l.Subpoints() {
		ls = append(ls, p.X(), p.Y())
	}
	return ls
}

func LineString(linestr tegola.LineString, extent *points.Extent) (ls []basic.Line, err error) {
	line := lines.FromTLineString(linestr)
	/*
		emin := maths.Pt{min.X - float64(extant), min.Y - float64(extant)}
		emax := maths.Pt{max.X + float64(extant), max.Y + float64(extant)}
		r := region.New(maths.Clockwise, emin, emax)
	*/
	/*
		r := region.New(maths.Clockwise, min, max)
		lpt := maths.Pt{pts[0], pts[1]}
		lptIsIn := r.Contains(lpt)
	*/

	// I don't think this makes sense for a unconnected polygon
	// Linestrings are not connected. So, for these we just need to walk each point and see if it's within the clipping rectangle.
	// When we enter the clipping rectangle, we need to calculate the interaction point, and start a new line.
	// When we leave the clipping rectangle, we need to calculated the interaction point, and stop the line.
	// pts := linestring2floats(line)

	var cpts [][2]float64
	lptIsIn := extent.Contains(line[0])
	if lptIsIn {
		cpts = append(cpts, line[0])
	}

	for i := 1; i < len(line); i++ {
		cptIsIn := extent.Contains(line[i])
		switch {
		case !lptIsIn && cptIsIn: // We are entering the extent region.
			if ipt, ok := extent.IntersectPt([2][2]float64{line[i-1], line[i]}); ok && len(ipt) > 0 {
				cpts = append(cpts, ipt[0])
			}
			cpts = append(cpts, line[i])
		case !lptIsIn && !cptIsIn: // Both points are outside, but it's possible that they could be going straight through the regions.
			if ipt, ok := extent.IntersectPt([2][2]float64{line[i-1], line[i]}); ok && len(ipt) > 1 {
				ls = append(ls, basic.NewLineFrom2Float64(ipt...))
			}
			cpts = cpts[:0]
		case lptIsIn && cptIsIn: // Both points are in, just add the new point.
			cpts = append(cpts, line[i])
		case lptIsIn && !cptIsIn: // We are headed out of the region.
			if ipt, ok := extent.IntersectPt([2][2]float64{line[i-1], line[i]}); ok {
				_ = ipt
				cpts = append(cpts, ipt...)
			}
			// Time to add this line to our set of lines, and reset
			// the new line.
			ls = append(ls, basic.NewLineFrom2Float64(cpts...))
			cpts = cpts[:0]
		}
		lptIsIn = cptIsIn
	}
	if len(cpts) > 0 {
		ls = append(ls, basic.NewLineFrom2Float64(cpts...))
	}
	return ls, nil
}

func Polygon(polygon tegola.Polygon, min, max maths.Pt, extant int) (p []basic.Polygon, err error) {
	// Each polygon is made up of a main linestring describing the outer ring,
	// and set of outer rings. The outer ring is clockwise while the inner ring is
	// counter-clockwise.

	// log.Println("Starting Polygon clipping.")
	sls := polygon.Sublines()
	if len(sls) == 0 {
		return p, nil
	}
	var subls []*subject.Subject

	emin := maths.Pt{min.X - float64(extant), min.Y - float64(extant)}
	emax := maths.Pt{max.X + float64(extant), max.Y + float64(extant)}
	plstrs, err := linestring(linestring2floats(sls[0]), emin, emax)

	if err != nil {
		return nil, err
	}
	if len(plstrs) == 0 {
		return nil, nil
	}
	for _, ls := range plstrs {
		if len(ls) < 6 {
			//log.Println("Skipping main linestring size too small.", len(ls), ls)
			continue
		}
		p = append(p, basic.Polygon{basic.NewLine(ls...)})
		nsub, err := subject.New(ls)
		if err != nil {
			return nil, err
		}
		subls = append(subls, nsub)
	}

	emin = maths.Pt{min.X - (float64(extant) - 1), min.Y - (float64(extant) - 1)}
	emax = maths.Pt{max.X + (float64(extant) - 1), max.Y + (float64(extant) - 1)}
	for _, s := range sls[1:] {

		slines, err := linestring(linestring2floats(s), min, max)
		if err != nil {
			return nil, err
		}
		// For each of the substrings, I need to figure out which polygon it goes to.
		for _, ss := range slines {
			for i, sublss := range subls {
				if sublss.Contains(maths.Pt{ss[0], ss[1]}) {
					// Found the polygon, move to the next substring.
					p[i] = append(p[i], basic.NewLine(ss...))
					break
				}
			}
		}
	}
	return p, nil
}
func Geometry(geo tegola.Geometry, min, max maths.Pt) (basic.Geometry, error) {
	// log.Println("Clipping Geometry")
	var extant = 2
	switch g := geo.(type) {

	case tegola.Point:
		return basic.Point{g.X(), g.Y()}, nil
	case tegola.Point3:
		return basic.Point3{g.X(), g.Y()}, nil
	case tegola.MultiPoint:
		var mpt basic.MultiPoint
		for _, pt := range g.Points() {
			mpt = append(mpt, basic.Point{pt.X(), pt.Y()})
		}
		return mpt, nil

	case tegola.Polygon:
		ps, err := Polygon(g, min, max, extant)
		if err != nil {
			return basic.G{}, err
		}
		if len(ps) == 0 {
			return nil, nil
		}
		if len(ps) == 1 {
			return ps[0], nil
		}
		return basic.MultiPolygon(ps), err

	case tegola.MultiPolygon:
		// log.Println("Clipping MultiPolygon")
		var mp basic.MultiPolygon
		for _, p := range g.Polygons() {
			ps, err := Polygon(p, min, max, extant)
			if err != nil {
				return nil, err
			}
			mp = append(mp, ps...)
		}
		if len(mp) == 0 {
			return nil, nil
		}
		if len(mp) == 1 {
			return mp[0], nil
		}
		return mp, nil

	case tegola.LineString:
		//ls, err := LineString(g, min, max, extant)
		ls, err := LineString(g, &points.Extent{{min.X, min.Y}, {max.X, max.Y}})
		if err != nil {
			return basic.G{}, err
		}
		if len(ls) == 0 {
			return nil, nil
		}
		if len(ls) == 1 {
			return ls[0], nil
		}
		return basic.MultiLine(ls), nil

	case tegola.MultiLine:
		var ls basic.MultiLine
		for _, l := range g.Lines() {
			lls, err := LineString(l, &points.Extent{{min.X, min.Y}, {max.X, max.Y}})
			if err != nil {
				return ls, err
			}
			ls = append(ls, lls...)
		}
		if len(ls) == 0 {
			return nil, nil
		}
		if len(ls) == 1 {
			return ls[0], nil
		}
		return ls, nil

	default:
		return nil, errors.New("Unsupported Geometry")
	}
}
