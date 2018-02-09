# Quick Description of the testdata file.

## Desc
Desc is a quick description of the test case.
This Also, starts the test block.

## Expected

The expected Geometry Collection:

Geometries are described as follows:
`xxx,yyy` for a point.
`( xxx,yyy xxx,yyy )` for MultiPoint
`(( ))` for a collection.
`{{ }}` for a MultiPolygon
`{ }` for a Polygon
`[[ ]]` for a MutliLineString
`[ ]` for a LineString

### Point
```
expected:
34,45
```

### MultiPoint
```
expected:
(1,2 34,45)
```

### LineString
```
expected:
[ 1,2  34,45 ]
```

### MultiLineString
```
[[
[1,2 34,45]
[12,23 1,4]
]]
```
### Polygon
```
{
[12,23  1,3]
[1,2  3,4]
}
```

### MultiPolygon
```
{{
{
[12,23  1,3]
}
{
[1,2 3,4]
}
}}
```
### Collection
```
((
12,23
(1,2 3,4)
[1,2 3,4]
[[
[1,2 3,4]
[1,4 5,6]
]]
))
```
## Bytes 

Are described using hex numbers. Each hex number is expected to be two digits: from 00-FF; or from 00-ff.

## BOM

This is the Endian value, it can be eighter “little” or “big”.

