# Vector Tile Specification

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",
"SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in
this document are to be interpreted as described in [RFC 2119](https://www.ietf.org/rfc/rfc2119.txt).

## 1. Purpose

This document specifies a space-efficient encoding format for tiled geographic vector data. It is designed to be used in browsers or server-side applications for fast rendering or lookups of feature data.

## 2. File Format

The Vector Tile format uses [Google Protocol Buffers](https://developers.google.com/protocol-buffers/) as a encoding format. Protocol Buffers are a language-neutral, platform-neutral extensible mechanism for serializing structured data.

### 2.1. File Extension

The filename extension for Vector Tile files SHOULD be `mvt`. For example, a file might be named `vector.mvt`.

### 2.2. Multipurpose Internet Mail Extensions (MIME)

When serving Vector Tiles the MIME type SHOULD be `application/vnd.mapbox-vector-tile`.

## 3. Projection and Bounds

A Vector Tile represents data based on a square extent within a projection. A Vector Tile SHOULD NOT contain information about its bounds and projection. The file format assumes that the decoder knows the bounds and projection of a Vector Tile before decoding it.

[Web Mercator](https://en.wikipedia.org/wiki/Web_Mercator) is the projection of reference, and [the Google tile scheme](http://www.maptiler.org/google-maps-coordinates-tile-bounds-projection/) is the tile extent convention of reference. Together, they provide a 1-to-1 relationship between a specific geographical area, at a specific level of detail, and a path such as `https://example.com/17/65535/43602.mvt`.

Vector Tiles MAY be used to represent data with any projection and tile extent scheme.

## 4. Internal Structure

This specification describes the structure of data within a Vector Tile. The reader should have an understanding of the [Vector Tile protobuf schema document](vector_tile.proto) and the structures it defines.

### 4.1. Layers

A Vector Tile consists of a set of named layers. A layer contains geometric features and their metadata. The layer format is designed so that the data required for a layer is contiguous in memory, and so that layers can be appended to a Vector Tile without modifying existing data.

A Vector Tile SHOULD contain at least one layer. A layer SHOULD contain at least one feature.

A layer MUST contain a `version` field with the major version number of the Vector Tile specification to which the layer adheres. For example, a layer adhering to version 2.1 of the specification contains a `version` field with the integer value `2`. The `version` field SHOULD be the first field within the layer. Decoders SHOULD parse the `version` first to ensure that they are capable of decoding each layer. When a Vector Tile consumer encounters a Vector Tile layer with an unknown version, it MAY make a best-effort attempt to interpret the layer, or it MAY skip the layer. In either case it SHOULD continue to process subsequent layers in the Vector Tile.

A layer MUST contain a `name` field. A Vector Tile MUST NOT contain two or more layers whose `name` values are byte-for-byte identical. Prior to appending a layer to an existing Vector Tile, an encoder MUST check the existing `name` fields in order to prevent duplication.

Each feature in a layer (see below) may have one or more key-value pairs as its metadata. The keys and values are indices into two lists, `keys` and `values`, that are shared across the layer's features.

Each element in the `keys` field of the layer is a string. The `keys` include all the keys of features used in the layer, and each key may be referenced by its positional index in this set of `keys`, with the first key having an index of 0. The set of `keys` SHOULD NOT contain two or more values which are byte-for-byte identical.

Each element in the `values` field of the layer encodes a value of any of several types (see below). The `values` represent all the values of features used in the layer, and each value may be referenced by its positional index in this set of `values`, with the first value having an index of 0. The set of `values` SHOULD NOT contain two or more values of the same type which are byte-for-byte identical.

In order to support values of varying string, boolean, integer, and floating point types, the protobuf encoding of the `value` field consists of a set of `optional` fields. A value MUST contain exactly one of these optional fields.

A layer MUST contain an `extent` that describes the width and height of the tile in integer coordinates. The geometries within the Vector Tile MAY extend past the bounds of the tile's area as defined by the `extent`. Geometries that extend past the tile's area as defined by `extent` are often used as a buffer for rendering features that overlap multiple adjacent tiles.

For example, if a tile has an `extent` of 4096, coordinate units within the tile refer to 1/4096th of its square dimensions. A coordinate of 0 is on the top or left edge of the tile, and a coordinate of 4096 is on the bottom or right edge. Coordinates from 1 through 4095 inclusive are fully within the extent of the tile, and coordinates less than 0 or greater than 4096 are fully outside the extent of the tile.  A point at `(1,10)` or `(4095,10)` is within the extent of the tile. A point at `(0,10)` or `(4096,10)` is on the edge of the extent. A point at `(-1,10)` or `(4097,10)` is outside the extent of the tile.

### 4.2. Features

A feature MUST contain a `geometry` field.

A feature MUST contain a `type` field as described in the Geometry Types section.

A feature MAY contain a `tags` field. Feature-level metadata, if any, SHOULD be stored in the `tags` field.

A feature MAY contain an `id` field. If a feature has an `id` field, the value of the `id` SHOULD be unique among the features of the parent layer.

### 4.3. Geometry Encoding

Geometry data in a Vector Tile is defined in a screen coordinate system. The upper left corner of the tile (as displayed by default) is the origin of the coordinate system. The X axis is positive to the right, and the Y axis is positive downward. Coordinates within a geometry MUST be integers.

A geometry is encoded as a sequence of 32 bit unsigned integers in the `geometry` field of a feature. Each integer is either a `CommandInteger` or a `ParameterInteger`. A decoder interprets these as an ordered series of operations to generate the geometry.

Commands refer to positions relative to a "cursor", which is a redefinable point. For the first command in a feature, the cursor is at `(0,0)` in the coordinate system. Some commands move the cursor, affecting subsequent commands.

#### 4.3.1. Command Integers

A `CommandInteger` indicates a command to be executed, as a command ID, and the number of times that the command will be executed, as a command count.

A command ID is encoded as an unsigned integer in the least significant 3 bits of the `CommandInteger`, and is in the range 0 through 7, inclusive. A command count is encoded as an unsigned integer in the remaining 29 bits of a `CommandInteger`, and is in the range `0` through `pow(2, 29) - 1`, inclusive.

A command ID, a command count, and a `CommandInteger` are related by these bitwise operations:

```javascript
CommandInteger = (id & 0x7) | (count << 3)
```

```javascript
id = CommandInteger & 0x7
```

```javascript
count = CommandInteger >> 3
```

A command ID specifies one of the following commands:

|  Command     |  Id  | Parameters    | Parameter Count |
| ------------ |:----:| ------------- | --------------- |
| MoveTo       | `1`  | `dX`, `dY`    | 2               |
| LineTo       | `2`  | `dX`, `dY`    | 2               |
| ClosePath    | `7`  | No parameters | 0               |

##### Example Command Integers

| Command   |  ID  | Count | CommandInteger | Binary Representation `[Count][Id]`      |
| --------- |:----:|:-----:|:--------------:|:----------------------------------------:|
| MoveTo    | `1`  | `1`   | `9`            | `[00000000 00000000 0000000 00001][001]` |
| MoveTo    | `1`  | `120` | `961`          | `[00000000 00000000 0000011 11000][001]` |
| LineTo    | `2`  | `1`   | `10`           | `[00000000 00000000 0000000 00001][010]` |
| LineTo    | `2`  | `3`   | `26`           | `[00000000 00000000 0000000 00011][010]` |
| ClosePath | `7`  | `1`   | `15`           | `[00000000 00000000 0000000 00001][111]` |


#### 4.3.2. Parameter Integers

Commands requiring parameters are followed by a `ParameterInteger` for each parameter required by that command. The number of `ParameterIntegers` that follow a `CommandInteger` is equal to the parameter count of a command multiplied by the command count of the `CommandInteger`. For example, a `CommandInteger` with a `MoveTo` command with a command count of 3 will be followed by 6 `ParameterIntegers`.

A `ParameterInteger` is [zigzag](https://developers.google.com/protocol-buffers/docs/encoding#types) encoded so that small negative and positive values are both encoded as small integers. To encode a parameter value to a `ParameterInteger` the following formula is used:

```javascript
ParameterInteger = (value << 1) ^ (value >> 31)
```

Parameter values greater than `pow(2,31) - 1` or less than `-1 * (pow(2,31) - 1)` are not supported.

The following formula is used to decode a `ParameterInteger` to a value:

```javascript
value = ((ParameterInteger >> 1) ^ (-(ParameterInteger & 1)))
```

#### 4.3.3. Command Types

For all descriptions of commands the initial position of the cursor shall be described to be at the coordinates `(cX, cY)` where `cX` is the position of the cursor on the X axis and `cY` is the position of the `cursor` on the Y axis.

##### 4.3.3.1. MoveTo Command

A `MoveTo` command with a command count of `n` MUST be immediately followed by `n` pairs of `ParameterInteger`s. Each pair `(dX, dY)`:

1. Defines the coordinate `(pX, pY)`, where `pX = cX + dX` and `pY = cY + dY`.
   * Within POINT geometries, this coordinate defines a new point.
   * Within LINESTRING geometries, this coordinate defines the starting vertex of a new line.
   * Within POLYGON geometries, this coordinate defines the starting vertex of a new linear ring.
2. Moves the cursor to `(pX, pY)`.

##### 4.3.3.2. LineTo Command

A `LineTo` command with a command count of `n` MUST be immediately followed by `n` pairs of `ParameterInteger`s. Each pair `(dX, dY)`:

1. Defines a segment beginning at the cursor `(cX, cY)` and ending at the coordinate `(pX, pY)`, where `pX = cX + dX` and `pY = cY + dY`.
   * Within LINESTRING geometries, this segment extends the current line.
   * Within POLYGON geometries, this segment extends the current linear ring.
2. Moves the cursor to `(pX, pY)`.

For any pair of `(dX, dY)` the `dX` and `dY` MUST NOT both be `0`.

##### 4.3.3.3. ClosePath Command

A `ClosePath` command MUST have a command count of 1 and no parameters. The command closes the current linear ring of a POLYGON geometry via a line segment beginning at the cursor `(cX, cY)` and ending at the starting vertex of the current linear ring.

This command does not change the cursor position.

#### 4.3.4. Geometry Types

The `geometry` field is described in each feature by the `type` field which must be a value in the enum `GeomType`. The following geometry types are supported:

* UNKNOWN
* POINT
* LINESTRING
* POLYGON

Geometry collections are not supported.

##### 4.3.4.1. Unknown Geometry Type

The specification purposefully leaves an unknown geometry type as an option. This geometry type encodes experimental geometry types that an encoder MAY choose to implement. Decoders MAY ignore any features of this geometry type.

##### 4.3.4.2. Point Geometry Type

The `POINT` geometry type encodes a point or multipoint geometry. The geometry command sequence for a point geometry MUST consist of a single `MoveTo` command with a command count greater than 0.

If the `MoveTo` command for a `POINT` geometry has a command count of 1, then the geometry MUST be interpreted as a single point; otherwise the geometry MUST be interpreted as a multipoint geometry, wherein each pair of `ParameterInteger`s encodes a single point.

##### 4.3.4.3. Linestring Geometry Type

The `LINESTRING` geometry type encodes a linestring or multilinestring geometry. The geometry command sequence for a linestring geometry MUST consist of one or more repetitions of the following sequence: 

1. A `MoveTo` command with a command count of 1
2. A `LineTo` command with a command count greater than 0

If the command sequence for a `LINESTRING` geometry type includes only a single `MoveTo` command then the geometry MUST be interpreted as a single linestring; otherwise the geometry MUST be interpreted as a multilinestring geometry, wherein each `MoveTo` signals the beginning of a new linestring.

##### 4.3.4.4. Polygon Geometry Type

The `POLYGON` geometry type encodes a polygon or multipolygon geometry, each polygon consisting of exactly one exterior ring that contains zero or more interior rings. The geometry command sequence for a polygon consists of one or more repetitions of the following sequence:

1. An `ExteriorRing`
2. Zero or more `InteriorRing`s

Each `ExteriorRing` and `InteriorRing` MUST consist of the following sequence:

1. A `MoveTo` command with a command count of 1
2. A `LineTo` command with a command count greater than 1
3. A `ClosePath` command

An exterior ring is DEFINED as a linear ring having a positive area as calculated by applying the [surveyor's formula](https://en.wikipedia.org/wiki/Shoelace_formula) to the vertices of the polygon in tile coordinates. In the tile coordinate system (with the Y axis positive down and X axis positive to the right) this makes the exterior ring's winding order appear clockwise.

An interior ring is DEFINED as a linear ring having a negative area as calculated by applying the [surveyor's formula](https://en.wikipedia.org/wiki/Shoelace_formula) to the vertices of the polygon in tile coordinates. In the tile coordinate system (with the Y axis positive down and X axis positive to the right) this makes the interior ring's winding order appear counterclockwise.

If the command sequence for a `POLYGON` geometry type includes only a single exterior ring then the geometry MUST be interpreted as a single polygon; otherwise the geometry MUST be interpreted as a multipolygon geometry, wherein each exterior ring signals the beginning of a new polygon. If a polygon has interior rings they MUST be encoded directly after the exterior ring of the polygon to which they belong.

Linear rings MUST be geometric objects that have no anomalous geometric points, such as self-intersection or self-tangency. The position of the cursor before calling the `ClosePath` command of a linear ring SHALL NOT repeat the same position as the first point in the linear ring as this would create a zero-length line segment. A linear ring SHOULD NOT have an area calculated by the surveyor's formula equal to zero, as this would signify a ring with anomalous geometric points.

Polygon geometries MUST NOT have any interior rings that intersect and interior rings MUST be enclosed by the exterior ring.

#### 4.3.5. Example Geometry Encodings

##### 4.3.5.1. Example Point

An example encoding of a point located at:

* (25,17)

This would require a single command:

* MoveTo(+25, +17)

```
Encoded as: [ 9 50 34 ]
              | |  `> Decoded: ((34 >> 1) ^ (-(34 & 1))) = +17
              | `> Decoded: ((50 >> 1) ^ (-(50 & 1))) = +25
              | ===== relative MoveTo(+25, +17) == create point (25,17)
              `> [00001 001] = command id 1 (MoveTo), command count 1
```

##### 4.3.5.2. Example Multi Point

An example encoding of two points located at:

* (5,7)
* (3,2)

This would require two commands:

* MoveTo(+5,+7)
* MoveTo(-2,-5)

```
Encoded as: [ 17 10 14 3 9 ]
               |  |  | | `> Decoded: ((9 >> 1) ^ (-(9 & 1))) = -5
               |  |  | `> Decoded: ((3 >> 1) ^ (-(3 & 1))) = -2
               |  |  | === relative MoveTo(-2, -5) == create point (3,2)
               |  |  `> Decoded: ((34 >> 1) ^ (-(34 & 1))) = +7
               |  `> Decoded: ((50 >> 1) ^ (-(50 & 1))) = +5
               | ===== relative MoveTo(+25, +17) == create point (25,17)
               `> [00010 001] = command id 1 (MoveTo), command count 2
```

##### 4.3.5.3. Example Linestring

An example encoding of a line with the points:

* (2,2)
* (2,10)
* (10,10)

This would require three commands:

* MoveTo(+2,+2)
* LineTo(+0,+8)
* LineTo(+8,+0)

```
Encoded as: [ 9 4 4 18 0 16 16 0 ]
              |      |      ==== relative LineTo(+8, +0) == Line to Point (10, 10)
              |      | ==== relative LineTo(+0, +8) == Line to Point (2, 10)
              |      `> [00010 010] = command id 2 (LineTo), command count 2
              | === relative MoveTo(+2, +2)
              `> [00001 001] = command id 1 (MoveTo), command count 1
```

##### 4.3.5.4. Example Multi Linestring

An example encoding of two lines with the points:

* Line 1:
  * (2,2)
  * (2,10)
  * (10,10)
* Line 2:
  * (1,1)
  * (3,5)

This would require the following commands:

* MoveTo(+2,+2)
* LineTo(+0,+8)
* LineTo(+8,+0)
* MoveTo(-9,-9)
* LineTo(+2,+4)

```
Encoded as: [ 9 4 4 18 0 16 16 0 9 17 17 10 4 8 ]
              |      |           |        | === relative LineTo(+2, +4) == Line to Point (3,5)
              |      |           |        `> [00001 010] = command id 2 (LineTo), command count 1
              |      |           | ===== relative MoveTo(-9, -9) == Start new line at (1,1)
              |      |           `> [00001 001] = command id 1 (MoveTo), command count 1
              |      |      ==== relative LineTo(+8, +0) == Line to Point (10, 10)
              |      | ==== relative LineTo(+0, +8) == Line to Point (2, 10)
              |      `> [00010 010] = command id 2 (LineTo), command count 2
              | === relative MoveTo(+2, +2)
              `> [00001 001] = command id 1 (MoveTo), command count 1
```

##### 4.3.5.5. Example Polygon

An example encoding of a polygon feature that has the points:

* (3,6)
* (8,12)
* (20,34)
* (3,6) *Path Closing as Last Point*

Would encoded by using the following commands:

* MoveTo(3, 6)
* LineTo(5, 6)
* LineTo(12, 22)
* ClosePath

```
Encoded as: [ 9 6 12 18 10 12 24 44 15 ]
              |       |              `> [00001 111] command id 7 (ClosePath), command count 1
              |       |       ===== relative LineTo(+12, +22) == Line to Point (20, 34)
              |       | ===== relative LineTo(+5, +6) == Line to Point (8, 12)
              |       `> [00010 010] = command id 2 (LineTo), command count 2
              | ==== relative MoveTo(+3, +6)
              `> [00001 001] = command id 1 (MoveTo), command count 1
```

##### 4.3.5.6. Example Multi Polygon

An example of a more complex encoding of two polygons, one with a hole. The position of the points for the polygons are shown below. The winding order of the polygons is VERY important in this example as it signifies the difference between interior rings and a new polygon.

* Polygon 1:
  * Exterior Ring:
    * (0,0)
    * (10,0)
    * (10,10)
    * (0,10)
    * (0,0) *Path Closing as Last Point*
* Polygon 2:
  * Exterior Ring:
    * (11,11)
    * (20,11)
    * (20,20)
    * (11,20)
    * (11,11) *Path Closing as Last Point*
  * Interior Ring:
    * (13,13)
    * (13,17)
    * (17,17)
    * (17,13)
    * (13,13) *Path Closing as Last Point*

This polygon would be encoded with the following set of commands:

* MoveTo(+0,+0)
* LineTo(+10,+0)
* LineTo(+0,+10)
* LineTo(-10,+0) // Cursor at 0,10 after this command
* ClosePath // End of Polygon 1
* MoveTo(+11,+1) // NOTE THAT THIS IS RELATIVE TO LAST LINETO!
* LineTo(+9,+0)
* LineTo(+0,+9)
* LineTo(-9,+0) // Cursor at 11,20 after this command
* ClosePath // This is a new polygon because area is positive!
* MoveTo(+2,-7) // NOTE THAT THIS IS RELATIVE TO LAST LINETO!
* LineTo(+0,+4)
* LineTo(+4,+0)
* LineTo(+0,-4) // Cursor at 17,13
* ClosePath // This is an interior ring because area is negative!

### 4.4. Feature Attributes

Feature attributes are encoded as pairs of integers in the `tag` field of a feature. The first integer in each pair represents the zero-based index of the key in the `keys` set of the `layer` to which the feature belongs. The second integer in each pair represents the zero-based index of the value in the `values` set of the `layer` to which the feature belongs. Every key index MUST be unique within that feature such that no other attribute pair within that feature has the same key index. A feature MUST have an even number of `tag` fields. A feature `tag` field MUST NOT contain a key index or value index greater than or equal to the number of elements in the layer's `keys` or `values` set, respectively.

### 4.5. Example

For example, a GeoJSON feature like:

```json
{
    "type": "FeatureCollection",
    "features": [
        {
            "geometry": {
                "type": "Point",
                "coordinates": [
                    -8247861.1000836585,
                    4970241.327215323
                ]
            },
            "type": "Feature",
            "properties": {
                "hello": "world",
                "h": "world",
                "count": 1.23
            }
        },
        {
            "geometry": {
                "type": "Point",
                "coordinates": [
                    -8247861.1000836585,
                    4970241.327215323
                ]
            },
            "type": "Feature",
            "properties": {
                "hello": "again",
                "count": 2
            }
        }
    ]
}
```

Could be structured like:

```js
layers {
  version: 2
  name: "points"
  features: {
    id: 1
    tags: 0
    tags: 0
    tags: 1
    tags: 0
    tags: 2
    tags: 1
    type: Point
    geometry: 9
    geometry: 2410
    geometry: 3080
  }
  features {
    id: 2
    tags: 0
    tags: 2
    tags: 2
    tags: 3
    type: Point
    geometry: 9
    geometry: 2410
    geometry: 3080
  }
  keys: "hello"
  keys: "h"
  keys: "count"
  values: {
    string_value: "world"
  }
  values: {
    double_value: 1.23
  }
  values: {
    string_value: "again"
  }
  values: {
    int_value: 2
  }
  extent: 4096
}
```

Keep in mind the exact values for the geometry would differ based on the projection and extent of the tile.
