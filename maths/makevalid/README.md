# MakeValid Algorithm

## Introduction

![starting](_docs/makevalid_starting.png?raw=true)

This document will attempt to explain the makevalid algorithm used by Tegola. This function does two main things.
1. Transforms possibility invalid polygons and multi-polygons into simple valid multi-polygons.
2. Given a clipping region,  it will generate a clipped version of the simple valid multi-polygon.

Polygons in a multi-polygon are assumed to be related to each other and “makevalid” is applied holistically.

## General outline of the procedure.

There are eight steps in the repair and clipping process.

### 1. Hitmap.

We create a hitmap, whose job is to a label for a given point.
The label lets us know whether that point would be considered inside or outside of a polygon.
![HitMap](_docs/makevalid_hitmap.png?raw=true)

### 2. Destructure of the polygons.
We destructure all the polygons into their constitute line segments. We add the edges of the clipping region as additional line segments.

Next, for each pair of line segments that intersect, we break them into the four constitute line segments.
At the end of this process, there should be no pair of line segments that intersect, other than at their endpoints.

![destructure](_docs/makevalid_destructure.png?raw=true)

### 3. Clipping
We throw away any line segments that are outside the clipping region.

### 4. Creating Column edges.
Now for each unique x value of the line segments, we draw a vertical line from the minimum y of the clipping region to the maximum y of the clipping region.

Each of these vertical lines will have at least two intersections points (at the top and bottom border of the clipping region.) We track the maximum y value of the line segment connected to that point, headed in the rightmost direction if there is one.

Each set of these y values make up a column-edge. A column is made up of the adjacent pair of edges.
We will walk each column-edge to generate a column of labeled polygons. 

![columns](_docs/makevalid_columns.png?raw=true)

### 5. Triangulating a column.

As we have shown, we have constructed our columns in a particular way, which allows us to triangulate it easily. Given two points from each column, we need to figure out which three points we will use in constructing a triangle. Note, that we are eliminating two out of the four possible triangles, by starting at the top of each column and proceed downwards.

We really would prefer to create ∆abr, and only if we cannot do so, do we settle for ∆apr. 
Now we can observe that the only reason we cannot create ∆abr if there is a line from *a* toward *p* (line <span style="text-decoration:overline">ap</span>) which is below p's y component thereby intersecting the line <span sytle="text-decoration:overline">br</span>.

Assuming we generated ∆abr, we move down the first column to points (b,c, r, and p) otherwise we will walk down the second column to (a, b, p, and n).

Note if there are only three points left, then that is the triangle.

As we generate triangles, we label them using the hit-map and a center point of the triangle.

![triangulation](_docs/makevalid_triangulation.png?raw=true)

### 6. Combining Triangle to form rings.

The generated labeled rings should have a clockwise winding order, as stitching depends on this.
 Observe when combining two adjacent triangles to form a ring it will share two points of the new triangle. We only need to append the non-shared point to the correct end of the ring. 

Note: We are done creating a ring when we are out of triangles to add, or when the label of the triangle does not match the label of the ring. If we do not have a ring (because this is the start of the process or the labels are different – just closed a ring), we start a new ring by including all three points of the triangle. At the end of this process, we will have a column with a set of labeled rings ready to be stitched to its adjacent neighbor. 


### 7. Stitching

Starting with the leftmost column of two columns we are stitching together; pick a point in the first labeled ring. We need to track the first point, as the ring is done when we arrive back at this point.
Add this point to the new ring we are building.
Next we check to see if the point is on the shared edge of the two columns. 
If it is we will look for a ring on the other column with the same point and the same label.
If we find such a ring and point; we set our current ring to this new ring and our current point to this new point.
Move to the next point in the current ring. 
If this point is the first point, we started with we are done with this ring.
If the point is a point we have seen before, we need to remove all the points between the time we saw this point and now.  (This is to remove “bubbles.”)
Repeat this process till we arrive back at the first point.
We then repeat this process for each of the rings in the two columns. After which we need to remove the duplicate rings that will be generated. 

After which we need to calculate the new edges. For the column. It is possible to have rings in the column that do not touch the edges or only touch one edge. 

We return this new column.

We continue this process of combining columns till we only have one column. This column will be made up of rings we need to assemble into polygons.

![stitching](_docs/makevalid_stitching.png?raw=true)
And example of this can be seen in Figure XXX.

In Figure XXX you can see that Column 5 has 3 rings, and Column 6 has 5 rings. For this example will be looking at ring two, which starts at point b. From point b we move to p, and notice that it's on the boundery. We look down Column 6 and find that ring 5 has point p and is the same label as 2. Point p on ring 5 would head us to point t, and then u and then n. Point n is on the boundery edge so we will scan down Column 5 to find ring 2 has both the required label and point. From n in ring 2, we move to m, where we scan Column 6, looking for a ring with the m point and the same label. However, this time we don't find one, and move on to point k. Point k, like is like m, and again we are unable to locate a ring in Column 6, with the same label and point k, so we move onto point i. We do find ring 7 in Column that has the same label and point i, so we move to that ring, and move to point w. From point w, we easily move to x, and then h. At point h we have to do the check again but this time in column 5, as we are on column 6, and find ring 2 to move to e, then d, then c, where we stop as we are back at b. This is ring we generate for ring 2 of column 5 (b,p,t,u,n,m,k,i,w,x,h,e,d,c). Note: When we run this algorithm on rings 5 and 7 (in column 6), we will end up with the same ring. Those two rings will be eliminated. 

The final thing would be loup through each new ring, noting the points that lie on the new edges. 


### 8. Turning rings into polygons.

The first thing we do is eliminate any rings that touch the outside boundary. The remaining outside rings will be surrounded by inner rings. Each inside ring will be a polygon.

Next, we associate each remaining outside ring with an inside ring.
(Note because of the process no inside ring will overlap another inside ring. So, we can be confident that an outside ring should only be associated (or contained by) with one inside ring.

We return the result of combining the new polygons into .a multi-polygon.


[hitmap]: _docs/makevalid_hitmap.png?raw=true "HitMap"
[starting]: _docs/makevalid_starting.png?raw=true "Starting Polygons"
[destructure]: _docs/makevalid_destructure.png?raw=true "Destructure and Clip polygons"
[columns]: _doc/makevalid_columns.png?raw=true "Mark the edges of the columns."
[triangluation]: _docs/makevaid_triangluation.png?raw=true "Triangluation and Generation of Columns."
[stitching]: _doc/makevalid_stitching.png?raw=true "Stitching of columns to generate the final Multipolygon."

