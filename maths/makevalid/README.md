# MakeValid Algorithm

## Introduction

![starting](_docs/makevalid_starting.png?raw=true)

This document will attempt to explain the makevalid algorithm used by tegola. This function does two main things:

1. Transforms invalid polygons and multi-polygons into simple valid polygons and multi-polygons.
2. Given a clipping region, a clipped version of the simple valid polygon or multi-polygon will output.

Polygons in a multi-polygon are assumed to be related to each other and “makevalid” is applied holistically.

## General outline of the procedure.

There are eight steps in the repair and clipping process.

### 1. Hitmap

A hitmap is created, whose job is to label a given point as inside or outside of a polygon.

![HitMap](_docs/makevalid_hitmap.png?raw=true)

### 2. Destructure of the polygons

All polygons are destructured into line segments. If a clipping region is provided, the edges of the clipping region are added as additional line segments.

Next, each pair of line segments that intersect are broken into four constitute line segments. At the end of this process, no pair of line segments should intersect, other than at their endpoints.

![destructure](_docs/makevalid_destructure.png?raw=true)

### 3. Clipping

If a clipping region is provided, any line segments that are outside the clipping region are discarded.

### 4. Creating column edges

For each unique x value of the line segments, a vertical line is drawn from the minimum y of the region to the maximum y of the region.

Each of these vertical lines will have at least two intersections points, at the top and bottom border of the region. Track the maximum y value of the line segment connected to that point, headed in the rightmost direction if there is one.

Each set of y values make up a column-edge. A column is made up of the adjacent pair of edges. Each column-edge will be walked to generate a column of labeled polygons.

![columns](_docs/makevalid_columns.png?raw=true)

### 5. Triangulating a column

The construction of our columns allows for easy triangulation. Given two points from each column, three points we will identified for use in constructing a triangle. Note, that two of the four possible triangles are eliminated by starting at the top of each column and proceed downwards.

Ideally an `∆abr` is crated, but if not possible an `∆apr` is created. The only reason an `∆abr` can't be created is if there is a line from point `a` toward point `p` (line `a,p`) which is below point `p`'s y value thereby intersecting the line `b,r`.

Assuming `∆abr` was generated, walk down the first column to points (b,c, r, and p) otherwise walk down the second column to (a, b, p, and n).

If there are only three points left, then that's the triangle.

As triangles are generated, they're labeled using the hit-map and the center point of the triangle.

![triangulation](_docs/makevalid_triangulation.png?raw=true)

### 6. Combining triangles to form rings

The generated labeled rings should have a clockwise winding order, as combining depends on this. Observe when combining two adjacent triangles to form a ring it will share two points of the new triangle. Only the non-shared point needs to be appended to the correct end of the ring.

The creation of a ring is complete when there are no more triangles to add or when the label of the triangle does not match the label of the ring. In the case when a ring does not exist (i.e. the the start of the process or the labels are different – just closed a ring), a new ring is started by including all three points of the triangle. At the end of this process, the column will consist of a set of labeled rings ready to be stitched to its adjacent neighbor.

### 7. Stitching

Starting with the leftmost column of two columns we are stitching together; pick a point in the first labeled ring. The first point is tracked, as the ring is complete when we arrive back at this point. This point is added to the ring being built.

Next, check to see if the point is on the shared edge of the two columns. If it is look for a ring in the other column with the same point and label. If such a ring and point exists, set the current ring and point to this new ring point.

Move to the next point in the current ring. If this point is the first point, the ring is complete. If the point is a point we have seen before, remove all the points between the time we saw this point and now. (This is to remove “bubbles.”). Repeat this process until we arrive back at the first point.

Repeat this process for each of the rings in the two columns. Remove any duplicate rings that are generated.

For the column, calculate the new edges. It's possible to have rings in the column that do not touch the edges or only touch one edge. 

Continue this process of combining columns until only a single column exists. This column will consist of rings to into polygons.

![stitching](_docs/makevalid_stitching.png?raw=true)

**Example**

In the above figure Column 5 has 3 rings, and Column 6 has 5 rings. Focus on ring two, which starts at point `b`. From point `b` we move to point `p`, which is on the column boundary. We walk down Column 6 and find that ring 5 has point `p` which is the same label as ring 2. Point `p` on ring 5 takes us to point `t`, then point `u` and then point `n`. Point `n` is on the column boundary so we will scan down Column 5 to find ring 2 which has both the required label and point. From point `n` in ring 2, we move to point `m`, where we scan Column 6 looking for a ring with point `m` and the same label. This time we don't find one, and move on to point `k` and again we're unable to locate a ring in Column 6 with the same label and point, so we move onto point `i`. With point `i` we find ring 7 in Column 6 that has the same label and point `i`, so we move to that ring, and move to point `w`. From point `w`, we move to point `x` and then to point `h`. At point `h` we check Column 5, as we're on Column 6, and find ring 2. We move to point `e`, then point `d`, then point `c`, where we stop as we are back at point `b`. 

The final ring we generate for ring 2 of Column 5 is made up of the following points: `b`,`p`,`t`,`u`,`n`,`m`,`k`,`i`,`w`,`x`,`h`,`e`,`d`,`c`. Note: When we run this algorithm on rings 5 and 7 (in column 6), we will end up with the same ring. Those two rings will be eliminated. 

The final step is to loop through each new ring, noting the points that lie on the new edges. 

### 8. Turning rings into polygons

First step is to eliminate any rings that touch the outside boundary. The remaining outside rings will be surrounded by inner rings, and each inside ring will be a polygon.

Next, associate each remaining outside ring with an inside ring. Note, because of the process no inside ring will overlap another inside ring. An outside ring should only be associated (or contained by) with one inside ring.

Return the result of combining the new polygons into a multi-polygon.