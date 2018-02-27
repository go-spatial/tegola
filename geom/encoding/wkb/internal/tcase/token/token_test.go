package token

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/go-spatial/tegola/geom"
)

func TestParseFloat64(t *testing.T) {
	type tcase struct {
		input string
		exp   float64
		err   error
	}
	fn := func(idx int, test tcase) {
		tt := NewT(strings.NewReader(test.input))
		f, err := tt.ParseFloat64()
		if test.err != err {
			t.Errorf("[%v] Did not get correct error value expected: %v, got %v", idx, test.err, err)
		}
		if test.err != nil {
			return
		}
		if test.exp != f {
			t.Errorf("[%v] Exp: %v Got: %v", idx, test.exp, f)
		}
	}
	tbltest.Cases(
		tcase{input: "-12", exp: -12.0},
		tcase{input: "0", exp: 0.0},
		tcase{input: "+1_000.00", exp: 1000.0},
		tcase{input: "-12_000.00", exp: -12000.0},
		tcase{input: "10.005e5", exp: 10.005e5},
		tcase{input: "10.005e+5", exp: 10.005e5},
		tcase{input: "10.005e+05", exp: 10.005e5},
		tcase{input: "1.0005e+6", exp: 10.005e5},
		tcase{input: "1.0005e+06", exp: 10.005e5},
		tcase{input: "1.0005e-06", exp: 1.0005e-06},
		tcase{input: "1.0005e-06a", exp: 1.0005e-06},
	).Run(fn)

}
func TestParsePoint(t *testing.T) {
	type tcase struct {
		input string
		exp   [2]float64
		err   error
	}
	fn := func(idx int, test tcase) {
		tt := NewT(strings.NewReader(test.input))
		f, err := tt.ParsePoint()
		if test.err != err {
			t.Errorf("[%v] Did not get correct error value expected: %v, got %v", idx, test.err, err)
		}
		/*
			if test.err != nil {
				return
			}
		*/
		if test.exp[0] != f[0] || test.exp[1] != f[1] {
			t.Errorf("[%v] Exp: %v Got: %v", idx, test.exp, f)
		}
	}
	tbltest.Cases(
		tcase{input: "1,-12", exp: [2]float64{1.0, -12.0}},
		tcase{input: "0 /*x*/ ,0 /*y*/", exp: [2]float64{0.0, 0.0}},
		tcase{input: "  +1_000.00 ,/*y*/ 1", exp: [2]float64{1000.0, 1.0}},
		tcase{input: "-12_000.00,0", exp: [2]float64{-12000.0, 0.0}},
		tcase{input: "/* x */ -12_000.00 /* in dollars */, /* y */ 0 /* ponds */ // This is just for kicks", exp: [2]float64{-12000.0, 0.0}},
	).Run(fn)

}
func TestParseMultiPoint(t *testing.T) {
	type tcase struct {
		input string
		exp   geom.MultiPoint
		err   error
	}
	fn := func(idx int, test tcase) {
		tt := NewT(strings.NewReader(test.input))
		f, err := tt.ParseMultiPoint()
		if test.err != err {
			t.Errorf("[%v] Did not get correct error value expected: %v, got %v", idx, test.err, err)
		}
		/*
			if test.err != nil {
				return
			}
		*/
		if !reflect.DeepEqual(test.exp, f) {
			t.Errorf("[%v]\nExp:(%#v) %[2]v\nGot:(%#v) %[3]v", idx, test.exp, f)
		}
	}
	tbltest.Cases(
		tcase{
			input: `
			( 1,-12 )
			`,
			exp: geom.MultiPoint([][2]float64{{1.0, -12.0}}),
		},
		tcase{
			input: `
			( 1,-12 0,1)
			`,
			exp: geom.MultiPoint([][2]float64{{1.0, -12.0}, {0.0, 1.0}}),
		},
		tcase{
			input: `
			(1,-12 0,1 1,2 )
			`,
			exp: geom.MultiPoint([][2]float64{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2.0}}),
		},
		tcase{
			input: `
			( 
			1,-12 // Position 1
			/* is x suppose to be this? */ 
			0,1  ///
			1,2_000
			) /* Why the end why? */
			`,
			exp: geom.MultiPoint([][2]float64{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2000.0}}),
		},
	).Run(fn)

}

func TestParseLineString(t *testing.T) {
	type tcase struct {
		input string
		exp   geom.LineString
		err   error
	}
	fn := func(idx int, test tcase) {
		tt := NewT(strings.NewReader(test.input))
		f, err := tt.ParseLineString()
		if test.err != err {
			t.Errorf("[%v] Did not get correct error value expected: %v, got %v", idx, test.err, err)
		}
		/*
			if test.err != nil {
				return
			}
		*/
		if !reflect.DeepEqual(test.exp, f) {
			t.Errorf("[%v]\nExp:(%#v) %[2]v\nGot:(%#v) %[3]v", idx, test.exp, f)
		}
	}
	tbltest.Cases(
		tcase{
			input: `
			[ 1,-12 ]
			`,
			exp: geom.LineString([][2]float64{{1.0, -12.0}}),
		},
		tcase{
			input: `
			[ 1,-12 0,1]
			`,
			exp: geom.LineString([][2]float64{{1.0, -12.0}, {0.0, 1.0}}),
		},
		tcase{
			input: `
			[ 1,-12 0,1 1,2 ]
			`,
			exp: geom.LineString([][2]float64{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2.0}}),
		},
		tcase{
			input: `
			[ 
			1,-12 // Position 1
			/* is x suppose to be this? */ 
			0,1  ///
			1,2_000
			] /* Why the end why? */
			`,
			exp: geom.LineString([][2]float64{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2000.0}}),
		},
	).Run(fn)
}
func TestParseMultiLineString(t *testing.T) {
	type tcase struct {
		input string
		exp   geom.MultiLineString
		err   error
	}
	fn := func(idx int, test tcase) {
		tt := NewT(strings.NewReader(test.input))
		f, err := tt.ParseMultiLineString()
		if test.err != err {
			t.Errorf("[%v] Did not get correct error value expected: %v, got %v", idx, test.err, err)
		}
		/*
			if test.err != nil {
				return
			}
		*/
		if !reflect.DeepEqual(test.exp, f) {
			t.Errorf("[%v]\nExp:(%#v) %[2]v\nGot:(%#v) %[3]v", idx, test.exp, f)
		}
	}
	tbltest.Cases(
		tcase{
			input: `
			[[
			[ 1,-12 ]
			]]
			`,
			exp: geom.MultiLineString([][][2]float64{{{1.0, -12.0}}}),
		},
		tcase{
			input: `
			[[ [ 1,-12 0,1] ]]
			`,
			exp: geom.MultiLineString([][][2]float64{{{1.0, -12.0}, {0.0, 1.0}}}),
		},
		tcase{
			input: `
			[[ [ 1,-12 0,1 1,2 ] ]]
			`,
			exp: geom.MultiLineString([][][2]float64{{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2.0}}}),
		},
		tcase{
			input: `
			[[ [ 1,-12 0,1 1,2 ] [ 1, 2] ]]
			`,
			exp: geom.MultiLineString([][][2]float64{{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2.0}}, {{1.0, 2.0}}}),
		},
		tcase{
			input: `[[
			[ 
			1,-12 // Position 1
			/* is x suppose to be this? */ 
			0,1  ///
			1,2_000
			] /* Why the end why? */
			]]`,
			exp: geom.MultiLineString([][][2]float64{{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2000.0}}}),
		},
	).Run(fn)
}
func TestParsePolygon(t *testing.T) {
	type tcase struct {
		input string
		exp   geom.Polygon
		err   error
	}
	fn := func(idx int, test tcase) {
		tt := NewT(strings.NewReader(test.input))
		f, err := tt.ParsePolygon()
		if test.err != err {
			t.Errorf("[%v] Did not get correct error value expected: %v, got %v", idx, test.err, err)
		}
		/*
			if test.err != nil {
				return
			}
		*/
		if !reflect.DeepEqual(test.exp, f) {
			t.Errorf("[%v]\nExp:(%#v) %[2]v\nGot:(%#v) %[3]v", idx, test.exp, f)
		}
	}
	tbltest.Cases(
		tcase{
			input: `
			{
			[ 1,-12 ]
			}
			`,
			exp: geom.Polygon([][][2]float64{{{1.0, -12.0}}}),
		},
		tcase{
			input: `
			{ [ 1,-12 0,1]
		}
			`,
			exp: geom.Polygon([][][2]float64{{{1.0, -12.0}, {0.0, 1.0}}}),
		},
		tcase{
			input: `
			{ [ 1,-12 0,1 1,2 ] }`,
			exp: geom.Polygon([][][2]float64{{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2.0}}}),
		},
		tcase{
			input: `
			{ [ 1,-12 0,1 1,2 ] [ 1, 2]} 
			`,
			exp: geom.Polygon([][][2]float64{{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2.0}}, {{1.0, 2.0}}}),
		},
		tcase{
			input: `{
			[ 
			1,-12 // Position 1
			/* is x suppose to be this? */ 
			0,1  ///
			1,2_000
			] /* Why the end why? */
		}`,
			exp: geom.Polygon([][][2]float64{{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2000.0}}}),
		},
	).Run(fn)

}
func TestParseMultiPolygon(t *testing.T) {
	type tcase struct {
		input string
		exp   geom.MultiPolygon
		err   error
	}
	fn := func(idx int, test tcase) {
		tt := NewT(strings.NewReader(test.input))
		f, err := tt.ParseMultiPolygon()
		if test.err != err {
			t.Errorf("[%v] Did not get correct error value expected: %v, got %v", idx, test.err, err)
		}
		if !reflect.DeepEqual(test.exp, f) {
			t.Errorf("[%v]\nExp:(%#v) %[2]v\nGot:(%#v) %[3]v", idx, test.exp, f)
		}
	}
	tbltest.Cases(
		tcase{
			input: `{{
			{
			[ 1,-12 ]
			}
		}}`,
			exp: geom.MultiPolygon([][][][2]float64{{{{1.0, -12.0}}}}),
		},
		tcase{
			input: `{{
			{ [ 1,-12 0,1]
		} }}
			`,
			exp: geom.MultiPolygon([][][][2]float64{{{{1.0, -12.0}, {0.0, 1.0}}}}),
		},
		tcase{
			input: `
			{{ { [ 1,-12 0,1 1,2 ] } }}`,
			exp: geom.MultiPolygon([][][][2]float64{{{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2.0}}}}),
		},
		tcase{
			input: `
			{{{ [ 1,-12 0,1 1,2 ] [ 1, 2]} 
		}}`,
			exp: geom.MultiPolygon([][][][2]float64{{{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2.0}}, {{1.0, 2.0}}}}),
		},
		tcase{
			input: `{{{
			[ 
			1,-12 // Position 1
			/* is x suppose to be this? */ 
			0,1  ///
			1,2_000
			] /* Why the end why? */}
		}}`,
			exp: geom.MultiPolygon([][][][2]float64{{{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2000.0}}}}),
		},
	).Run(fn)
}

func TestParseCollection(t *testing.T) {
	type tcase struct {
		input string
		exp   geom.Collection
		err   error
	}
	fn := func(idx int, test tcase) {
		tt := NewT(strings.NewReader(test.input))
		f, err := tt.ParseCollection()
		if test.err != err {
			t.Errorf("[%v] Did not get correct error value expected: %v, got %v", idx, test.err, err)
		}
		if !reflect.DeepEqual(test.exp, f) {
			t.Errorf("[%v]\nExp:(%#v) %[2]v\nGot:(%#v) %[3]v", idx, test.exp, f)
		}
	}
	tbltest.Cases(
		tcase{
			input: `(( {{
			{
			[ 1,-12 ]
			}
		}}))`,
			exp: geom.Collection{
				geom.MultiPolygon([][][][2]float64{{{{1.0, -12.0}}}}),
			},
		},
		tcase{
			input: `(( {{
			{ [ 1,-12 0,1]
		} }}
	))`,
			exp: geom.Collection{
				geom.MultiPolygon([][][][2]float64{{{{1.0, -12.0}, {0.0, 1.0}}}}),
			},
		},
		tcase{
			input: `((
			{{ { [ 1,-12 0,1 1,2 ] } }} ))`,
			exp: geom.Collection{
				geom.MultiPolygon([][][][2]float64{{{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2.0}}}}),
			},
		},
		tcase{
			input: `((
			{{{ [ 1,-12 0,1 1,2 ] [ 1, 2]} 
		}}))`,
			exp: geom.Collection{
				geom.MultiPolygon([][][][2]float64{{{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2.0}}, {{1.0, 2.0}}}}),
			},
		},
		tcase{
			input: `(( {{{
			[ 
			1,-12 // Position 1
			/* is x suppose to be this? */ 
			0,1  ///
			1,2_000
			] /* Why the end why? */}
		}} ))`,
			exp: geom.Collection{
				geom.MultiPolygon([][][][2]float64{{{{1.0, -12.0}, {0.0, 1.0}, {1.0, 2000.0}}}}),
			},
		},
	).Run(fn)
}

func TestParseBinary(t *testing.T) {
	type tcase struct {
		input string
		exp   []byte
		err   error
	}
	fn := func(test tcase) {
		tt := NewT(strings.NewReader(test.input))
		c, err := tt.ParseBinaryField()
		if test.err != err {
			t.Errorf("Did not get correct error value expected: %v, got %v", test.err, err)
		}
		if !reflect.DeepEqual(test.exp, c) {
			t.Errorf("\nExp: %v \nGot: %v", test.exp, c)
		}
	}
	tbltest.Cases(
		tcase{
			input: `
//	01 02 03 04  05 06 07 08
{{
	01                       // Byte order Marker little
	02 00 00 00              // Type 2 LineString
	02 00 00 00              // number of points
	00 00 00 00  00 00 F0 3F // x 1
	00 00 00 00  00 00 00 40 // y 2
	00 00 00 00  00 00 08 40 // x 3
	00 00 00 00  00 00 10 40 // y 4
}}`,
			exp: []byte{
				0x01,
				0x02, 0x00, 0x00, 0x00,
				0x02, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08, 0x40,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x40,
			},
		},
	).Run(fn)

}

func TestParseLabel(t *testing.T) {
	type tcase struct {
		input string
		exp   string
		err   error
	}
	fn := func(test tcase) {
		tt := NewT(strings.NewReader(test.input))
		c, err := tt.ParseLabel()
		if test.err != err {
			t.Errorf("Did not get correct error value expected: %v, got %v", test.err, err)
		}
		if !reflect.DeepEqual(test.exp, string(c)) {
			t.Errorf("Expected: %v Got: %v", test.exp, string(c))
		}
	}
	tbltest.Cases(
		tcase{
			input: "easy: an easy label",
			exp:   "easy",
		},
		tcase{
			input: "little-easy: an easy label",
			exp:   "little-easy",
		},
		tcase{
			input: `
			
			
			
			little_easy: an easy label
			
			
			`,
			exp: "little_easy",
		},
		tcase{
			input: `
			/*
			    This one is also pretty easy.
			*/
			
			little.easy: an easy label
			
			
			`,
			exp: "little.easy",
		},
	).Run(fn)
}

func TestPraseLineComment(t *testing.T) {
	type tcase struct {
		exp      string
		dontwrap bool
		err      error
	}
	fn := func(test tcase) {
		input := test.exp
		if !test.dontwrap {
			input = "//" + input + "\n"
		}
		tt := NewT(strings.NewReader(input))
		c, err := tt.ParseLineComment()
		if test.err != err {
			t.Errorf("Did not get correct error value expected: %v, got %v", test.err, err)
		}
		if !reflect.DeepEqual(test.exp, string(c)) {
			t.Errorf("Expected: “%v” Got: “%v”", test.exp, string(c))
		}
	}
	tests := tbltest.Cases(
		tcase{
			exp: `This is a string. `,
		},
	)
	tests.Run(fn)
}

func TestParseComment(t *testing.T) {

	type tcase struct {
		desc     string `tbltest: "description"`
		exp      string
		dontwrap bool
		err      error
	}
	tests := tbltest.Cases(
		tcase{
			exp: `This is a string. 
			In a multiline comment.`,
		},
		tcase{
			exp: "",
		},
		tcase{
			exp: "//",
		},
		tcase{
			exp: `




			/**/

			`,
		},
		tcase{
			exp: `
			This is a line // with an line comment that does not mean anything.

			/******************8
			more stuff
			*************.
			* /
			*/
			`,
		},
	)
	tests.Run(func(test tcase) {
		input := test.exp
		if !test.dontwrap {
			input = "/*" + input + "*/"
		}
		tt := NewT(strings.NewReader(input))
		c, err := tt.ParseComment()
		if test.err != err {
			t.Errorf("Did not get correct error value expected: %v, got %v", test.err, err)
		}
		if !reflect.DeepEqual(test.exp, string(c)) {
			t.Errorf("Expected: %v Got: %v", test.exp, string(c))
		}
	})
}
