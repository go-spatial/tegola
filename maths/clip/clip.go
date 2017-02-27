package clip

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/clip/intersect"
	"github.com/terranodo/tegola/maths/clip/region"
	"github.com/terranodo/tegola/maths/clip/subject"
)

/*
Basics of the alogrithim.

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

func linestring(w maths.WindingOrder, sub []float64, rMinPt, rMaxPt maths.Pt) (clippedSubjects [][]float64, err error) {
	w = maths.WindingOrderOf(sub)
	il := intersect.New()
	rl := region.New(w, rMinPt, rMaxPt)
	sl, err := subject.New(w, sub)
	if err != nil {
		return clippedSubjects, err
	}

	//log.Println("Starting to work through the pair of points.")
	allSubjectsPtsIn := true
	for p, i := sl.FirstPair(), 0; p != nil; p, i = p.Next(), i+1 {
		//log.Printf("Looking pair %v : %#v ", i, p)
		line := p.AsLine()
		if !rl.Contains(p.Pt1().Point()) {
			allSubjectsPtsIn = false
		}
		for a := rl.FirstAxis(); a != nil; a = a.Next() {
			pt, doesIntersect := a.Intersect(line)
			if !doesIntersect {
				continue
			}
			pt.X = float64(int64(pt.X))
			pt.Y = float64(int64(pt.Y))

			ipt := intersect.NewPt(pt, a.IsInward(line))
			//log.Printf("Found Intersect (%p)%[1]v\n", ipt)
			// Only care about inbound intersect points.
			if ipt.Inward {
				il.PushBack(ipt)
			}
			p.PushInBetween(ipt.AsSubjectPoint())
			a.PushInBetween(ipt.AsRegionPoint())
		}
	}
	/*
		log.Printf("Done working through the pair of points. allSubjectsPtsIn %v\n", allSubjectsPtsIn)
		log.Printf("intersect: %#v\n", il)
		log.Printf("   region: %#v\n", rl)
		log.Printf("   region: ")
			for p := rl.Front(); p != nil; p = p.Next() {
				switch pp := p.(type) {
				case *intersect.RegionPoint:
					log.Printf("\t%p - (%v;%v)", pp, pp.Point(), pp.Inward)
				case *intersect.SubjectPoint:
					log.Printf("\t%p - (%v;%v)", pp, pp.Point(), pp.Inward)
				case list.ElementerPointer:
					log.Printf("\t%p - (%v)", pp, pp.Point())
				default:
					log.Printf("\t%p - %[1]#v\n", p)
				}
			}
			log.Printf("  subject: %#v\n", sl)
	*/
	// Check to see if all the subject points are contained in the region.
	if allSubjectsPtsIn {
		clippedSubjects = append(clippedSubjects, sub)
		return clippedSubjects, nil
	}
	// Need to check if there are no intersection points, it could be for two reason.
	// 2. The region points are all inside the subject.
	if il.Len() == 0 {
		for _, pt := range rl.SentinalPoints() {
			if !sl.Contains(pt) {
				//log.Printf("pt(%v)(%v) was not contained in subject(%#v).", i, pt, sl)
				// Not all region points are contain by the subject, so none of the subject points must be in the region.
				return clippedSubjects, nil
			}
		}
		// All region points are in the subject, so just return the region.
		clippedSubjects = append(clippedSubjects, rl.LineString())
		return clippedSubjects, nil
	}
	//log.Println("Walking through the Inbound Intersection points.")
	for w := il.FirstInboundPtWalker(); w != nil; w = w.Next() {
		//log.Printf("Looking at: %p", w)
		var s []float64
		var opt *maths.Pt
		w.Walk(func(idx int, pt maths.Pt) bool {
			if opt == nil || !opt.IsEqual(pt) {
				// Only add point if it's not the same as the last point
				//log.Printf("Adding point(%v): %v\n", idx, pt)
				s = append(s, pt.X, pt.Y)
			}
			opt = &pt
			if idx == sl.Len() {
				return false
			}
			return true
		})
		// Must have at least 3 points for it to be a valid runstring. (3 *2 = 6)
		if len(s) > 6 {
			clippedSubjects = append(clippedSubjects, s)
		}
	}
	// log.Println("Done walking through the Inbound Intersection points.")
	return clippedSubjects, nil
}

func linestring2floats(l tegola.LineString) (ls []float64) {
	for _, p := range l.Subpoints() {
		ls = append(ls, p.X(), p.Y())
	}
	return ls
}

func LineString(line tegola.LineString, min, max maths.Pt, extant int) (ls []basic.Line, err error) {
	emin := maths.Pt{min.X - float64(extant), min.Y - float64(extant)}
	emax := maths.Pt{max.X + float64(extant), max.Y + float64(extant)}
	r := region.New(maths.Clockwise, emin, emax)
	// I don't think this makes sense for a unconnected pollygon
	// Linestrings are not connected. So, for these we just need to walk each point and see if it's within the clipping rectangle.
	// When we enter the clipping rectangle, we need to calculate the interaction point, and start a new line.
	// When we leave the clipping rectangle, we need to calculated the interaction point, and stop the line.
	pts := linestring2floats(line)
	lpt := maths.Pt{pts[0], pts[1]}
	lptIsIn := r.Contains(lpt)
	var cpts []float64
	if lptIsIn {
		cpts = append(cpts, lpt.X, lpt.Y)
	}
	for x, y := 2, 3; y < len(pts); x, y = x+2, y+2 {
		cpt := maths.Pt{pts[x], pts[y]}
		cptIsIn := r.Contains(cpt)
		if cptIsIn {
			if !lptIsIn {
				// Need to figure out the intersection point.
				line := maths.Line{lpt, cpt}
				for a := r.FirstAxis(); a != nil; a = a.Next() {
					pt, doesIntersect := a.Intersect(line)
					if doesIntersect {
						cpts = append(cpts, float64(int64(pt.X)), float64(int64(pt.Y)))
						break
					}
				}
			}
			cpts = append(cpts, cpt.X, cpt.Y)
		} else {
			if lptIsIn {
				line := maths.Line{lpt, cpt}
				for a := r.FirstAxis(); a != nil; a = a.Next() {
					pt, doesIntersect := a.Intersect(line)
					if doesIntersect {
						cpts = append(cpts, float64(int64(pt.X)), float64(int64(pt.Y)))
						break
					}
				}
				// Need to complete this set of points.
				ls = append(ls, basic.NewLine(cpts...))
				cpts = cpts[:0]
			}
		}
		lpt = cpt
		lptIsIn = cptIsIn
	}
	if len(cpts) > 0 {
		ls = append(ls, basic.NewLine(cpts...))
	}
	return ls, nil
}

func Polygon(polygon tegola.Polygon, min, max maths.Pt, extant int) (p []basic.Polygon, err error) {
	// Each polygon is made up of a main linestring describing the outer ring,
	// and set of ourter rignts. The outer ring is clockwise while the inner ring is
	// usually conter clockwise.

	sls := polygon.Sublines()
	var subls []*subject.Subject

	emin := maths.Pt{min.X - float64(extant), min.Y - float64(extant)}
	emax := maths.Pt{max.X + float64(extant), max.Y + float64(extant)}
	//log.Printf("Starting to clip main line to %v, %v", emin, emax)
	plstrs, err := linestring(maths.Clockwise, linestring2floats(sls[0]), emin, emax)
	//log.Printf("Done to clipping main line to %v, %v", emin, emax)
	if err != nil {
		return nil, err
	}
	if len(plstrs) == 0 {
		return nil, nil
	}
	for _, ls := range plstrs {
		p = append(p, basic.Polygon{basic.NewLine(ls...)})
		nsub, err := subject.New(maths.Clockwise, ls)
		if err != nil {
			return nil, err
		}
		subls = append(subls, nsub)
	}
	emin = maths.Pt{min.X - (float64(extant) - 1), min.Y - (float64(extant) - 1)}
	emax = maths.Pt{max.X + (float64(extant) - 1), max.Y + (float64(extant) - 1)}
	for _, s := range sls[1:] {
		slines, err := linestring(maths.CounterClockwise, linestring2floats(s), min, max)
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
	var extant = 0
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
		ls, err := LineString(g, min, max, extant)
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
			lls, err := LineString(l, min, max, extant)
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
		return basic.G{}, nil

	}
}
