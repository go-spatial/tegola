Package  colors
================
[![Build Status](https://semaphoreci.com/api/v1/projects/be59797f-235e-411f-82be-4fab6e3172a6/550132/badge.svg)](https://semaphoreci.com/joeybloggs/colors)
[![GoDoc](https://godoc.org/gopkg.in/go-playground/colors.v1?status.svg)](https://godoc.org/gopkg.in/go-playground/colors.v1)

Go color manipulation, conversion and printing library/utility

this library is currently in development, not all color types such as HSL, HSV and CMYK will be included in the first release; pull requests are welcome.

Installation
============

Use go get.

	go get gopkg.in/go-playground/colors.v1

or to update

	go get -u gopkg.in/go-playground/colors.v1

Then import the validator package into your own code.

	import "gopkg.in/go-playground/colors.v1"
	
Usage and documentation
=======================

Please see http://godoc.org/gopkg.in/go-playground/colors.v1 for detailed usage docs.

#Example
```go
hex, err := colors.ParseHEX("#fff")
rgb, err := colors.ParseRGB("rgb(0,0,0)")
rgb, err := colors.RGB(0,0,0)
rgba, err := colors.ParseRGBA("rgba(0,0,0,1)")
rgba, err := colors.RGBA(0,0,0,1)

// don't know which color, it was user selectable
color, err := colors.Parse("#000)

color.ToRGB()   // rgb(0,0,0)
color.ToRGBA()  // rgba(0,0,0,1)
color.ToHEX()   // #000000
color.IsLight() // false
color.IsDark()  // true

```

How to Contribute
=================

There will always be a development branch for each version i.e. `v1-development`. In order to contribute, 
please make your pull requests against those branches.

If the changes being proposed or requested are breaking changes, please create an issue, for discussion 
or create a pull request against the highest development branch for example this package has a 
v1 and v1-development branch however, there will also be a v2-development brach even though v2 doesn't exist yet.

I am not a color expert by any means and am sure that there could be better or even more efficient
ways to accomplish the color conversion and so forth and I welcome any suggestions or pull request to help!

License
=======
Distributed under MIT License, please see license file in code for more details.
