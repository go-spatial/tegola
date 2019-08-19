
This is a lazy port of the JTS triangulation routines to Go. The goals when
porting are:

* Provide a go-ish interface to the functionality available in JTS's 
  triangulate package.
* Stay true to JTS's implementation, but make the necessary changes to stay 
  go-ish and fit nicely within the geom package.
* Only implement what is needed as it is needed.
* When reasonable, keep modifications to the functionality segregated into 
  different directories. This will help minimize the cost of porting more of 
  JTS's functionality or porting JTS's future changes. When it isn't 
  reasonable, make a specific note of the difference in the comments.

To make porting easier, the original Java code will be kept in the source 
files until the specific function has been ported at which time the Java 
version of the function/method should be removed.

The original code was taken from:

https://github.com/locationtech/jts/tree/jts-1.15.0
tag jts-1.15.0 (b7d7a00fef7106fe6609d6f53be1fe8046f3274c)

To be consistent with JTS, this code is licensed under EDL v1.0 which is very 
similar to BSD 3. The specific license information can be found in LICENSE.md.

Send issues to: https://github.com/go-spatial/geom/

