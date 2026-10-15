package geometrycodec_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider/geometrycodec"
)

func TestValidateMOSPrecision(t *testing.T) {
	type tcase struct {
		precision float64
		expectErr bool
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			err := geometrycodec.ValidateMOSPrecision(tc.precision)
			if tc.expectErr && err == nil {
				t.Errorf("expected error, got nil")
				return
			}
			if !tc.expectErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		}
	}

	tests := map[string]tcase{
		"zero":       {0, false},
		"integer":    {3, false},
		"negative":   {-1, true},
		"fractional": {2.5, true},
		"nan":        {math.NaN(), true},
		"too large":  {309, true},
		"max valid":  {308, false},
		"infinity":   {math.Inf(1), true},
	}
	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestValidateRawCustomSQL(t *testing.T) {
	type tcase struct {
		layerName   string
		format      string
		sql         string
		expectedErr []string // substrings required in the error; empty means no error
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			err := geometrycodec.ValidateRawCustomSQL(tc.layerName, tc.format, tc.sql, "!BBOX!", "!BOX!")
			if len(tc.expectedErr) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %v, got nil", tc.expectedErr)
			}
			for _, want := range tc.expectedErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q must contain %q", err.Error(), want)
				}
			}
		}
	}

	tests := map[string]tcase{
		"raw wkb with bbox rejected": {
			layerName:   "roads",
			format:      geometrycodec.FormatWKB,
			sql:         "SELECT id, geom FROM roads WHERE geom && !BBOX!",
			expectedErr: []string{"roads", "!BBOX!", "wkb", "in-memory"},
		},
		"raw wkt lowercase token rejected": {
			layerName:   "wkt_layer",
			format:      geometrycodec.FormatWKT,
			sql:         "SELECT id, geom FROM t WHERE ST_Intersects(geom, !bbox!)",
			expectedErr: []string{"wkt_layer", "wkt"},
		},
		"raw mos with bbox accepted": {
			layerName: "mos_layer",
			format:    geometrycodec.FormatMOS,
			sql:       "SELECT id, geom, MINX, MAXX, MINY, MAXY FROM t WHERE MAXX >= !BBOX!",
		},
		"raw mos lowercase bbox accepted": {
			layerName: "mos_layer",
			format:    geometrycodec.FormatMOS,
			sql:       "SELECT id, geom FROM t WHERE minx <= !bbox!",
		},
		"raw without bbox accepted": {
			layerName: "roads",
			format:    geometrycodec.FormatWKB,
			sql:       "SELECT id, geom FROM roads",
		},
		"native with bbox accepted": {
			layerName: "roads",
			format:    "native",
			sql:       "SELECT id, geom FROM roads WHERE geom && !BBOX!",
		},
		"auto with bbox accepted": {
			layerName: "roads",
			format:    "auto",
			sql:       "SELECT id, geom FROM roads WHERE geom && !BBOX!",
		},
		"raw empty sql accepted": {
			layerName: "roads",
			format:    geometrycodec.FormatWKB,
			sql:       "",
		},
		// SQL-context-aware token detection (UPSTREAM 2.6): a bbox token
		// inside a comment or string literal is not a real usage and must
		// not trip the raw-format rejection.
		"raw wkb comment token accepted": {
			layerName: "roads",
			format:    geometrycodec.FormatWKB,
			sql:       "SELECT id, geom FROM roads -- !BBOX!",
		},
		"raw wkb string literal token accepted": {
			layerName: "roads",
			format:    geometrycodec.FormatWKB,
			sql:       "SELECT id, geom, '!BBOX!' FROM roads",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestRequireBBoxCustomSQL(t *testing.T) {
	type tcase struct {
		layerName    string
		boundsBacked bool
		sql          string
		expectedErr  []string
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			err := geometrycodec.RequireBBoxCustomSQL(tc.layerName, tc.boundsBacked, tc.sql, "!BBOX!", "!BOX!")
			if len(tc.expectedErr) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %v, got nil", tc.expectedErr)
			}
			for _, want := range tc.expectedErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q must contain %q", err.Error(), want)
				}
			}
		}
	}

	tests := map[string]tcase{
		"bounds-backed with bbox ok": {
			layerName:    "mos_layer",
			boundsBacked: true,
			sql:          "SELECT id, geom FROM t WHERE MAXX >= !BBOX!",
		},
		"bounds-backed with box alias ok": {
			layerName:    "mos_layer",
			boundsBacked: true,
			sql:          "SELECT id, geom FROM t WHERE MAXX >= !BOX!",
		},
		"bounds-backed without bbox rejected": {
			layerName:    "mos_layer",
			boundsBacked: true,
			sql:          "SELECT id, geom FROM t",
			expectedErr:  []string{"mos_layer", "bounds-backed", "must use !BBOX!"},
		},
		"not bounds-backed without bbox ok": {
			layerName: "native_layer",
			sql:       "SELECT id, geom FROM t",
		},
		"not bounds-backed wkb without bbox ok": {
			layerName: "wkb_layer",
			sql:       "SELECT id, geom FROM t",
		},
		"bounds-backed empty sql ok": {
			layerName:    "mos_layer",
			boundsBacked: true,
			sql:          "",
		},
		// SQL-context-aware token detection (UPSTREAM 2.6): a bbox token
		// inside a comment does not satisfy the bounds-backed requirement.
		"bounds-backed comment-only token rejected": {
			layerName:    "mos_layer",
			boundsBacked: true,
			sql:          "SELECT id, geom FROM t -- !BBOX!",
			expectedErr:  []string{"mos_layer", "bounds-backed", "must use !BBOX!"},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

// TestSQLContextAwareBBoxTokenDetection pins SQL-context-aware bbox token
// detection (UPSTREAM 2.6): only SQL code-context occurrences count.
func TestSQLContextAwareBBoxTokenDetection(t *testing.T) {
	tests := map[string]struct {
		sql      string
		expected bool
	}{
		"code token":            {sql: "SELECT id, geom FROM roads WHERE !BBOX!", expected: true},
		"code token lowercase":  {sql: "SELECT id, geom FROM roads WHERE !bbox!", expected: true},
		"code token mixed case": {sql: "SELECT id, geom FROM roads WHERE !BbOx!", expected: true},
		"string literal":        {sql: "SELECT id, geom, '!BBOX!' FROM roads", expected: false},
		"dollar quoted":         {sql: "SELECT id, geom FROM roads WHERE a = $tag$!BBOX!$tag$", expected: false},
		"double quoted ident":   {sql: "SELECT id, geom FROM roads WHERE \"!BBOX!\" IS NOT NULL", expected: false},
		"line comment":          {sql: "SELECT id, geom FROM roads -- !BBOX!", expected: false},
		"block comment":         {sql: "SELECT id, geom FROM roads /* !BBOX! */ WHERE z = 3", expected: false},
		"not a token":           {sql: "SELECT id, geom FROM roads WHERE !NOT_A_BBOX!", expected: false},
		"no token":              {sql: "SELECT id, geom FROM roads", expected: false},
	}

	for name, tc := range tests {
		if got := geometrycodec.SQLHasBBoxToken(tc.sql, "!BBOX!", "!BOX!"); got != tc.expected {
			t.Errorf("case %q: SQLHasBBoxToken(%q) = %t, want %t", name, tc.sql, got, tc.expected)
		}
	}
}

// TestPrepareProbeSQLProtectedContexts pins SQL-context-aware neutralization
// (UPSTREAM 2.6): token-shaped text in string literals, quoted identifiers
// and comments survives verbatim while code-context tokens neutralize.
func TestPrepareProbeSQLProtectedContexts(t *testing.T) {
	customSQL := "SELECT f(!x!), '!X!', `!Y!`, \"!ZOOM!\", [!Z!]\n" +
		"FROM t -- !BBOX!\n" +
		"/* !ID_FIELD! */ WHERE a = '!PIXEL_WIDTH!'"
	expected := "SELECT f(0), '!X!', `!Y!`, \"!ZOOM!\", [!Z!]\n" +
		"FROM t -- !BBOX!\n" +
		"/* !ID_FIELD! */ WHERE a = '!PIXEL_WIDTH!'"

	prepared := geometrycodec.PrepareProbeSQL(customSQL, "g", "id", "gs")
	if prepared != expected {
		t.Fatalf("PrepareProbeSQL(%q)\n got %q\nwant %q", customSQL, prepared, expected)
	}
}

func TestResolveBBoxFields(t *testing.T) {
	type tcase struct {
		provider   map[string]interface{}
		layer      map[string]interface{}
		layerName  string
		expected   [4]string
		expectedEr bool
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			var prov, lay dict.Dicter
			if tc.provider != nil {
				prov = dict.Dict(tc.provider)
			}
			if tc.layer != nil {
				lay = dict.Dict(tc.layer)
			}
			fields, err := geometrycodec.ResolveBBoxFields(prov, lay, tc.layerName)
			if tc.expectedEr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if fields != geometrycodec.BBoxFields(tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, fields)
			}
		}
	}

	tests := map[string]tcase{
		"defaults": {
			layerName: "l1",
			expected:  [4]string{"MINX", "MAXX", "MINY", "MAXY"},
		},
		"provider override": {
			provider:  map[string]interface{}{"bbox_minx_fieldname": "XMIN"},
			layerName: "l2",
			expected:  [4]string{"XMIN", "MAXX", "MINY", "MAXY"},
		},
		"layer overrides provider per field": {
			provider:  map[string]interface{}{"bbox_minx_fieldname": "XMIN", "bbox_maxx_fieldname": "XMAX"},
			layer:     map[string]interface{}{"bbox_maxx_fieldname": "RIGHT"},
			layerName: "l3",
			expected:  [4]string{"XMIN", "RIGHT", "MINY", "MAXY"},
		},
		"invalid provider value errors": {
			provider:   map[string]interface{}{"bbox_minx_fieldname": 42},
			layerName:  "l4",
			expectedEr: true,
		},
		"empty provider value errors": {
			provider:   map[string]interface{}{"bbox_minx_fieldname": "   "},
			layerName:  "l5",
			expectedEr: true,
		},
		"empty layer value errors": {
			layer:      map[string]interface{}{"bbox_maxy_fieldname": ""},
			layerName:  "l6",
			expectedEr: true,
		},
		"value is trimmed": {
			layer:     map[string]interface{}{"bbox_minx_fieldname": "  xmin  "},
			layerName: "l7",
			expected:  [4]string{"xmin", "MAXX", "MINY", "MAXY"},
		},
		"case-insensitive duplicate errors": {
			layer:      map[string]interface{}{"bbox_minx_fieldname": "minx", "bbox_maxx_fieldname": "MINX"},
			layerName:  "l8",
			expectedEr: true,
		},
		"duplicate against default errors": {
			layer:      map[string]interface{}{"bbox_maxx_fieldname": "MINX"},
			layerName:  "l9",
			expectedEr: true,
		},
		"qualified name errors": {
			layer:      map[string]interface{}{"bbox_minx_fieldname": "t.MINX"},
			layerName:  "l10",
			expectedEr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestBBoxFieldsIsBBoxField(t *testing.T) {
	fields := geometrycodec.BBoxFields{"minx", "MAXX", "MinY", "maxy"}
	for _, tc := range []struct {
		name     string
		expected bool
	}{
		{"MINX", true},
		{"maxx", true},
		{" miny ", true},
		{"MAXY", true},
		{"okey", false},
		{"geometry", false},
		{"", false},
	} {
		if got := fields.IsBBoxField(tc.name); got != tc.expected {
			t.Errorf("IsBBoxField(%q) = %v, expected %v", tc.name, got, tc.expected)
		}
	}
}

func TestBuildBoundsPredicate(t *testing.T) {
	type tcase struct {
		name      string
		fields    geometrycodec.BBoxFields
		extent    *geom.Extent
		mode      geometrycodec.BoundsPredicateMode
		mosConfig geometrycodec.MOSConfig
		quote     func(string) string
		expected  string
		expectErr bool
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			got, err := geometrycodec.BuildBoundsPredicate(tc.fields, tc.extent, tc.mode, tc.mosConfig, tc.quote)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		}
	}

	extent := geom.NewExtent([2]float64{10.0, 20.0}, [2]float64{30.0, 40.0})

	tests := map[string]tcase{
		"crs mode no scaling": {
			fields:   geometrycodec.BBoxFields{"MINX", "MAXX", "MINY", "MAXY"},
			extent:   extent,
			mode:     geometrycodec.BoundsSourceCRS,
			expected: "MAXX >= 10 AND MINX <= 30 AND MAXY >= 20 AND MINY <= 40",
		},
		"mos mode scales and rounds": {
			fields:    geometrycodec.BBoxFields{"MINX", "MAXX", "MINY", "MAXY"},
			extent:    extent,
			mode:      geometrycodec.BoundsMOSRaw,
			mosConfig: geometrycodec.MOSConfig{Precision: 0, UnitFactor: 1},
			expected:  "MAXX >= 10 AND MINX <= 30 AND MAXY >= 20 AND MINY <= 40",
		},
		"mos mode fractional precision floors and ceils": {
			fields:    geometrycodec.BBoxFields{"MINX", "MAXX", "MINY", "MAXY"},
			extent:    geom.NewExtent([2]float64{10.4, 20.4}, [2]float64{30.4, 40.4}),
			mode:      geometrycodec.BoundsMOSRaw,
			mosConfig: geometrycodec.MOSConfig{Precision: 1, UnitFactor: 1},
			expected:  "MAXX >= 104 AND MINX <= 304 AND MAXY >= 204 AND MINY <= 404",
		},
		"mos mode invalid scale errors": {
			fields:    geometrycodec.BBoxFields{"MINX", "MAXX", "MINY", "MAXY"},
			extent:    extent,
			mode:      geometrycodec.BoundsMOSRaw,
			mosConfig: geometrycodec.MOSConfig{Precision: 0, UnitFactor: 0},
			expectErr: true,
		},
		"nil extent errors": {
			fields:    geometrycodec.BBoxFields{"MINX", "MAXX", "MINY", "MAXY"},
			extent:    nil,
			mode:      geometrycodec.BoundsSourceCRS,
			expectErr: true,
		},
		"quoting applied": {
			fields:   geometrycodec.BBoxFields{"min x", "max x", "min y", "max y"},
			extent:   extent,
			mode:     geometrycodec.BoundsSourceCRS,
			quote:    func(s string) string { return "`" + s + "`" },
			expected: "`max x` >= 10 AND `min x` <= 30 AND `max y` >= 20 AND `min y` <= 40",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestInspectSQLGeometryContract(t *testing.T) {
	type tcase struct {
		name         string
		columns      []string
		rows         [][]interface{}
		geometryFeed []interface{} // one per row: geometry returned by decode; nil = decode error
		mosFeed      []bool        // one per row: isMOS flag returned by decode
		bboxFields   geometrycodec.BBoxFields
		expected     geometrycodec.SQLGeometryContract
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			rowIdx := 0
			next := func() ([]interface{}, bool, error) {
				if rowIdx >= len(tc.rows) {
					return nil, false, nil
				}
				row := tc.rows[rowIdx]
				rowIdx++
				return row, true, nil
			}
			decodeIdx := 0
			decode := geometrycodec.RowDecode(func(value interface{}) (geom.Geometry, bool, error) {
				var isMOS bool
				if tc.mosFeed != nil && decodeIdx < len(tc.mosFeed) {
					isMOS = tc.mosFeed[decodeIdx]
				}
				if tc.geometryFeed != nil && decodeIdx < len(tc.geometryFeed) {
					v := tc.geometryFeed[decodeIdx]
					decodeIdx++
					if v == nil {
						return nil, false, fmt.Errorf("undecodable")
					}
					return v, isMOS, nil
				}
				decodeIdx++
				if value == nil {
					return nil, false, fmt.Errorf("undecodable")
				}
				return geom.Point{1, 2}, isMOS, nil
			})
			contract, err := geometrycodec.InspectSQLGeometryContract(next, tc.columns, "geom", tc.bboxFields, decode)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if contract.ValidRows != tc.expected.ValidRows {
				t.Errorf("ValidRows = %v, expected %v", contract.ValidRows, tc.expected.ValidRows)
			}
			if contract.ValidMOSRows != tc.expected.ValidMOSRows {
				t.Errorf("ValidMOSRows = %v, expected %v", contract.ValidMOSRows, tc.expected.ValidMOSRows)
			}
			if contract.HasBounds != tc.expected.HasBounds {
				t.Errorf("HasBounds = %v, expected %v", contract.HasBounds, tc.expected.HasBounds)
			}
			if contract.BoundsFields != tc.expected.BoundsFields {
				t.Errorf("BoundsFields = %v, expected %v", contract.BoundsFields, tc.expected.BoundsFields)
			}
			if contract.GeometryField != tc.expected.GeometryField {
				t.Errorf("GeometryField = %q, expected %q", contract.GeometryField, tc.expected.GeometryField)
			}
		}
	}

	bbox := geometrycodec.BBoxFields{"MINX", "MAXX", "MINY", "MAXY"}

	tests := map[string]tcase{
		"bounds columns detected, valid MOS rows counted": {
			name:    "valid contract",
			columns: []string{"OKEY", "MINX", "MAXX", "MINY", "MAXY", "geom"},
			rows: [][]interface{}{
				{1, 0, 10, 0, 10, "geom1"},
				{2, 5, 15, 5, 15, "geom2"},
				{3, 9, 19, 9, 19, "geom3"},
			},
			mosFeed:    []bool{true, true, true},
			bboxFields: bbox,
			expected: geometrycodec.SQLGeometryContract{
				BoundsFields:  geometrycodec.BBoxFields{"MINX", "MAXX", "MINY", "MAXY"},
				GeometryField: "geom",
				ValidRows:     3,
				ValidMOSRows:  3,
				HasBounds:     true,
			},
		},
		"native rows never count as MOS rows": {
			name:    "native rows",
			columns: []string{"MINX", "MAXX", "MINY", "MAXY", "geom"},
			rows: [][]interface{}{
				{0, 10, 0, 10, "g1"},
				{0, 10, 0, 10, "g2"},
				{0, 10, 0, 10, "g3"},
			},
			mosFeed:    []bool{false, false, false},
			bboxFields: bbox,
			expected: geometrycodec.SQLGeometryContract{
				BoundsFields:  geometrycodec.BBoxFields{"MINX", "MAXX", "MINY", "MAXY"},
				GeometryField: "geom",
				ValidRows:     3,
				ValidMOSRows:  0,
				HasBounds:     true,
			},
		},
		"undecodable rows are skipped, never counted": {
			name:    "malformed rows",
			columns: []string{"MINX", "MAXX", "MINY", "MAXY", "geom"},
			rows: [][]interface{}{
				{0, 10, 0, 10, "bad"},
				{0, 10, 0, 10, "good1"},
				{0, 10, 0, 10, "good2"},
				{0, 10, 0, 10, "good3"},
				{0, 10, 0, 10, "good4"},
			},
			geometryFeed: []interface{}{
				nil, geom.Point{1, 1}, geom.Point{1, 2}, geom.Point{1, 3}, geom.Point{1, 4},
			},
			mosFeed:    []bool{false, true, true, true, true},
			bboxFields: bbox,
			expected: geometrycodec.SQLGeometryContract{
				BoundsFields:  geometrycodec.BBoxFields{"MINX", "MAXX", "MINY", "MAXY"},
				GeometryField: "geom",
				ValidRows:     3,
				ValidMOSRows:  3,
				HasBounds:     true,
			},
		},
		"missing bounds columns": {
			name:       "no bounds",
			columns:    []string{"OKEY", "geom"},
			rows:       [][]interface{}{{1, "g1"}, {2, "g2"}},
			mosFeed:    []bool{true, true},
			bboxFields: bbox,
			expected: geometrycodec.SQLGeometryContract{
				BoundsFields:  geometrycodec.BBoxFields{},
				GeometryField: "geom",
				ValidRows:     2,
				ValidMOSRows:  2,
				HasBounds:     false,
			},
		},
		"case-insensitive bounds match persists actual names": {
			name:       "case insensitive",
			columns:    []string{"minx", "MaxX", "MINY", "maxy", "geom"},
			rows:       [][]interface{}{{0, 10, 0, 10, "g1"}},
			mosFeed:    []bool{true},
			bboxFields: bbox,
			expected: geometrycodec.SQLGeometryContract{
				BoundsFields:  geometrycodec.BBoxFields{"minx", "MaxX", "MINY", "maxy"},
				GeometryField: "geom",
				ValidRows:     1,
				ValidMOSRows:  1,
				HasBounds:     true,
			},
		},
		"case-insensitive geometry match persists actual name": {
			name:       "geometry name case insensitive",
			columns:    []string{"MINX", "MAXX", "MINY", "MAXY", "GEOM"},
			rows:       [][]interface{}{{0, 10, 0, 10, "g1"}},
			mosFeed:    []bool{true},
			bboxFields: bbox,
			expected: geometrycodec.SQLGeometryContract{
				BoundsFields:  geometrycodec.BBoxFields{"MINX", "MAXX", "MINY", "MAXY"},
				GeometryField: "GEOM",
				ValidRows:     1,
				ValidMOSRows:  1,
				HasBounds:     true,
			},
		},
		"empty geometry rows do not count": {
			name:    "empty geometry",
			columns: []string{"MINX", "MAXX", "MINY", "MAXY", "geom"},
			rows: [][]interface{}{
				{0, 10, 0, 10, "empty"},
				{0, 10, 0, 10, "empty"},
				{0, 10, 0, 10, "empty"},
			},
			// decode succeeds but yields a geometry without coordinates
			geometryFeed: []interface{}{
				geom.LineString{}, geom.LineString{}, geom.LineString{},
			},
			mosFeed:    []bool{true, true, true},
			bboxFields: bbox,
			expected: geometrycodec.SQLGeometryContract{
				BoundsFields:  geometrycodec.BBoxFields{"MINX", "MAXX", "MINY", "MAXY"},
				GeometryField: "geom",
				ValidRows:     0,
				ValidMOSRows:  0,
				HasBounds:     true,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestValidateMVTGeometryFormat(t *testing.T) {
	type tcase struct {
		format      string
		expectedErr string
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			err := geometrycodec.ValidateMVTGeometryFormat(tc.format)
			if tc.expectedErr == "" {
				if err != nil {
					t.Errorf("unexpected error, Expected nil Got %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error, Expected %v Got nil", tc.expectedErr)
				return
			}
			if err.Error() != tc.expectedErr {
				t.Errorf("incorrect error,\n Expected \n \t%v\n Got \n \t%v", tc.expectedErr, err.Error())
			}
		}
	}

	tests := map[string]tcase{
		"empty is valid":  {"", ""},
		"native is valid": {"native", ""},
		"wkb rejected": {
			"wkb",
			`geometry_format = "wkb" is not supported for MVT providers; use a standard provider (postgis/hana) instead`,
		},
		"wkt rejected": {
			"wkt",
			`geometry_format = "wkt" is not supported for MVT providers; use a standard provider (postgis/hana) instead`,
		},
		"mos rejected": {
			"mos",
			`geometry_format = "mos" is not supported for MVT providers; use a standard provider (postgis/hana) instead`,
		},
	}
	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestResolveMOSConfig(t *testing.T) {
	type tcase struct {
		provider       dict.Dict
		layer          dict.Dict
		expectPrec     float64
		expectPrecSet  bool
		expectFactor   float64
		expectUnitsSet bool
		expectErr      bool
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			cfg, err := geometrycodec.ResolveMOSConfig(tc.provider, tc.layer, "test")
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Precision != tc.expectPrec {
				t.Errorf("precision, expected %v got %v", tc.expectPrec, cfg.Precision)
			}
			if cfg.PrecisionSet != tc.expectPrecSet {
				t.Errorf("precision set, expected %v got %v", tc.expectPrecSet, cfg.PrecisionSet)
			}
			if cfg.UnitFactor != tc.expectFactor {
				t.Errorf("unit factor, expected %v got %v", tc.expectFactor, cfg.UnitFactor)
			}
			if cfg.UnitsSet != tc.expectUnitsSet {
				t.Errorf("units set, expected %v got %v", tc.expectUnitsSet, cfg.UnitsSet)
			}
		}
	}

	tests := map[string]tcase{
		"defaults": {
			provider:     dict.Dict{},
			layer:        dict.Dict{},
			expectPrec:   2,
			expectFactor: 1,
		},
		"units only selects paired precision mm": {
			provider:       dict.Dict{"mos_units": "mm"},
			expectPrec:     0,
			expectFactor:   0.001,
			expectUnitsSet: true,
		},
		"units only selects paired precision cm": {
			provider:       dict.Dict{"mos_units": "cm"},
			expectPrec:     1,
			expectFactor:   0.01,
			expectUnitsSet: true,
		},
		"units only selects paired precision dm": {
			provider:       dict.Dict{"mos_units": "dm"},
			expectPrec:     1,
			expectFactor:   0.1,
			expectUnitsSet: true,
		},
		"units only selects paired precision km": {
			provider:       dict.Dict{"mos_units": "km"},
			expectPrec:     5,
			expectFactor:   1000,
			expectUnitsSet: true,
		},
		"layer units only selects paired precision": {
			layer:          dict.Dict{"mos_units": "km"},
			expectPrec:     5,
			expectFactor:   1000,
			expectUnitsSet: true,
		},
		"explicit precision overrides units-paired default": {
			provider:       dict.Dict{"mos_units": "mm", "mos_precision": 3.0},
			expectPrec:     3,
			expectPrecSet:  true,
			expectFactor:   0.001,
			expectUnitsSet: true,
		},
		"provider level": {
			provider:       dict.Dict{"mos_precision": 3.0, "mos_units": "mm"},
			expectPrec:     3,
			expectPrecSet:  true,
			expectFactor:   0.001,
			expectUnitsSet: true,
		},
		"layer overrides provider": {
			provider:       dict.Dict{"mos_precision": 3.0, "mos_units": "mm"},
			layer:          dict.Dict{"mos_precision": 0.0, "mos_units": "km"},
			expectPrec:     0,
			expectPrecSet:  true,
			expectFactor:   1000,
			expectUnitsSet: true,
		},
		"layer precision only": {
			provider:      dict.Dict{"mos_precision": 3.0},
			layer:         dict.Dict{"mos_precision": 1.0},
			expectPrec:    1,
			expectPrecSet: true,
			expectFactor:  1,
		},
		"blank units not explicit": {
			provider:     dict.Dict{"mos_units": "  "},
			expectPrec:   2,
			expectFactor: 1,
		},
		"invalid precision": {
			provider:  dict.Dict{"mos_precision": 2.5},
			expectErr: true,
		},
		"invalid units": {
			provider:  dict.Dict{"mos_units": "furlongs"},
			expectErr: true,
		},
		"layer invalid precision": {
			provider:  dict.Dict{},
			layer:     dict.Dict{"mos_precision": -2},
			expectErr: true,
		},
		"nil layer": {
			provider:      dict.Dict{"mos_precision": 2.0},
			expectPrec:    2,
			expectPrecSet: true,
			expectFactor:  1,
		},
	}
	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

// sysInfoBlob builds a minimal valid MapplGIS LayerInfo blob with the given
// precision, map units and projection, exercising the real mos parser.
func sysInfoBlob(t *testing.T, precision int32, mapUnits byte, mapUnitsDefined bool, projection string) []byte {
	t.Helper()
	buf := make([]byte, 128)
	// version marker: byte 5 followed by "Ver 1"
	buf[0] = 5
	copy(buf[1:], "Ver 1")
	// version string is 10 bytes, precision int32 follows at offset 11
	buf[11] = byte(precision)
	buf[12] = byte(precision >> 8)
	buf[13] = byte(precision >> 16)
	buf[14] = byte(precision >> 24)
	buf[15] = 1 // flProjection
	buf[26] = 1 // LayerID
	buf[52] = mapUnits
	if mapUnitsDefined {
		buf[53] = 1 // flMapUnitsDefined
	}
	projSize := int32(len(projection))
	buf[60] = byte(projSize)
	buf[61] = byte(projSize >> 8)
	buf[62] = byte(projSize >> 16)
	buf[63] = byte(projSize >> 24)
	copy(buf[64:], projection)
	return buf
}

func TestSystemInfoApplication(t *testing.T) {
	si := &mos.SystemInfo{
		Precision:       2,
		MapUnits:        mos.UnitsMillimetres,
		MapUnitsDefined: true,
	}

	applied := geometrycodec.DefaultMOSConfig()
	if err := applied.ApplySystemInfo(si); err != nil {
		t.Fatalf("apply system info: %v", err)
	}
	if applied.Precision != 2 {
		t.Errorf("applied precision, expected 2 got %v", applied.Precision)
	}
	if applied.UnitFactor != 0.001 {
		t.Errorf("applied unit factor, expected 0.001 got %v", applied.UnitFactor)
	}

	// explicit config must win over system info
	explicit := geometrycodec.MOSConfig{Precision: 5, UnitFactor: 100, PrecisionSet: true, UnitsSet: true}
	if err := explicit.ApplySystemInfo(si); err != nil {
		t.Fatalf("apply system info: %v", err)
	}
	if explicit.Precision != 5 {
		t.Errorf("explicit precision overwritten: %v", explicit.Precision)
	}
	if explicit.UnitFactor != 100 {
		t.Errorf("explicit units overwritten: %v", explicit.UnitFactor)
	}

	// nil is a no-op
	noop := geometrycodec.DefaultMOSConfig()
	if err := noop.ApplySystemInfo(nil); err != nil {
		t.Errorf("nil system info should be a no-op, got %v", err)
	}
}

func TestSystemInfoBlobDetection(t *testing.T) {
	blob := sysInfoBlob(t, 3, byte('m'), true, "")
	if !geometrycodec.IsSystemInfoValue(blob) {
		t.Fatal("expected blob to be recognized as system info")
	}
	if geometrycodec.IsSystemInfoValue([]byte("LINESTRING(0 0, 1 1)")) {
		t.Fatal("expected WKT text not to be recognized as system info")
	}
	if geometrycodec.IsSystemInfoValue(42) {
		t.Fatal("expected non-blob value not to be recognized as system info")
	}

	si, err := geometrycodec.ParseSystemInfoValue(blob)
	if err != nil {
		t.Fatalf("parse system info: %v", err)
	}
	if si.Precision != 3 {
		t.Errorf("precision, expected 3 got %v", si.Precision)
	}
}

func TestDecodeMOS(t *testing.T) {
	// MOS layout: [type][mod][addflag u16][subobjects u16][points i32]
	// then per-subobject u32 counts and i32 x,y pairs. One subobject, two
	// points: type 2 decodes to a MultiPoint.
	// Use precision 0 / metre units so raw coordinates pass through
	// unquantized; units-paired default precision is covered by
	// TestResolveMOSConfig and TestDefaultMOSPrecisionForUnits.
	blob := []byte{
		2, 0, 0, 0, // type 2 (point), mod 0, addflag 0
		1, 0, // one subobject
		2, 0, 0, 0, // two points
		2, 0, 0, 0, // subobject point count
		0, 0, 0, 0, 0, 0, 0, 0, // point (0,0)
		10, 0, 0, 0, 10, 0, 0, 0, // point (10,10)
	}

	mosCfg := geometrycodec.MOSConfig{Precision: 0, UnitFactor: 1}
	g, err := geometrycodec.DecodeMOS(blob, mosCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mp, ok := g.(geom.MultiPoint)
	if !ok {
		t.Fatalf("expected MultiPoint, got %T", g)
	}
	pts := mp.Points()
	if len(pts) != 2 || pts[0][0] != 0 || pts[1][0] != 10 {
		t.Errorf("unexpected points: %v", pts)
	}

	if _, err := geometrycodec.DecodeMOS([]byte{0xFF, 0x00}, mosCfg); err == nil {
		t.Error("expected error for garbage blob")
	}
	if _, err := geometrycodec.DecodeMOS(42, mosCfg); err == nil {
		t.Error("expected error for non-blob value")
	}
}

func TestDefaultMOSPrecisionForUnits(t *testing.T) {
	tests := map[string]struct {
		factor float64
		want   float64
	}{
		"mm":      {factor: 0.001, want: 0},
		"cm":      {factor: 0.01, want: 1},
		"dm":      {factor: 0.1, want: 1},
		"m":       {factor: 1, want: 2},
		"km":      {factor: 1000, want: 5},
		"unknown": {factor: 42, want: 0},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := geometrycodec.DefaultMOSPrecisionForUnits(tc.factor); got != tc.want {
				t.Fatalf("DefaultMOSPrecisionForUnits(%v) = %v, want %v", tc.factor, got, tc.want)
			}
		})
	}

	def := geometrycodec.DefaultMOSConfig()
	if def.Precision != 2 || def.UnitFactor != 1 {
		t.Fatalf("DefaultMOSConfig() = %+v, want precision 2 / factor 1", def)
	}
}

func TestDecodeWKT(t *testing.T) {
	g, err := geometrycodec.DecodeWKT("POINT(1 2)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pt, ok := g.(geom.Point)
	if !ok {
		t.Fatalf("expected Point, got %T", g)
	}
	if pt.X() != 1 || pt.Y() != 2 {
		t.Errorf("unexpected point coordinates: %v %v", pt.X(), pt.Y())
	}

	if _, err := geometrycodec.DecodeWKT([]byte("POINT(3 4)")); err != nil {
		t.Errorf("unexpected error for []byte: %v", err)
	}
	if _, err := geometrycodec.DecodeWKT(42); err == nil {
		t.Error("expected error for non-text value")
	}
	if _, err := geometrycodec.DecodeWKT("NOT A WKT"); err == nil {
		t.Error("expected error for invalid WKT")
	}
}

func TestGeometryIntersectsExtent(t *testing.T) {
	extent := geom.NewExtent([2]float64{0, 0}, [2]float64{10, 10})
	pointIn, _ := geometrycodecDecodeWKTPoint(t, "POINT(5 5)")
	pointOut, _ := geometrycodecDecodeWKTPoint(t, "POINT(20 20)")
	pointTouch, _ := geometrycodecDecodeWKTPoint(t, "POINT(10 10)")

	if !geometrycodec.GeometryIntersectsExtent(pointIn, extent) {
		t.Error("point inside extent should intersect")
	}
	if geometrycodec.GeometryIntersectsExtent(pointOut, extent) {
		t.Error("point outside extent should not intersect")
	}
	if !geometrycodec.GeometryIntersectsExtent(pointTouch, extent) {
		t.Error("point on extent boundary should intersect (inclusive bounds)")
	}
	// conservative behavior
	if !geometrycodec.GeometryIntersectsExtent(nil, extent) {
		t.Error("nil geometry should be kept")
	}
	if !geometrycodec.GeometryIntersectsExtent(pointIn, nil) {
		t.Error("nil extent should keep geometry")
	}
}

func geometrycodecDecodeWKTPoint(t *testing.T, wktStr string) (geom.Geometry, error) {
	t.Helper()
	return geometrycodec.DecodeWKT(wktStr)
}

func TestGeomTypeName(t *testing.T) {
	cases := []struct {
		wkt      string
		expected string
	}{
		{"POINT(1 2)", "POINT"},
		{"MULTIPOINT(1 2)", "MULTIPOINT"},
		{"LINESTRING(0 0, 1 1)", "LINESTRING"},
		{"MULTILINESTRING((0 0, 1 1))", "MULTILINESTRING"},
		{"POLYGON((0 0, 1 0, 1 1, 0 0))", "POLYGON"},
		{"MULTIPOLYGON(((0 0, 1 0, 1 1, 0 0)))", "MULTIPOLYGON"},
	}
	for _, tc := range cases {
		g, err := geometrycodec.DecodeWKT(tc.wkt)
		if err != nil {
			t.Fatalf("decode %v: %v", tc.wkt, err)
		}
		if got := geometrycodec.GeomTypeName(g); got != tc.expected {
			t.Errorf("%v: expected %v got %v", tc.wkt, tc.expected, got)
		}
	}
	if got := geometrycodec.GeomTypeName(nil); got != "" {
		t.Errorf("nil geometry: expected empty name got %v", got)
	}
}

func TestResolveLayerGeometryFormat(t *testing.T) {
	allowed := map[string]struct{}{
		"auto": {}, "mysql": {}, "mariadb": {}, "wkb": {}, "wkt": {}, "mos": {},
	}

	tcases := []struct {
		name          string
		providerValue string
		layer         dict.Dict
		layerName     string
		expect        string
		expectErr     bool
	}{
		{
			name:          "no layer config falls back to provider value",
			providerValue: "auto",
			layer:         dict.Dict{},
			expect:        "auto",
		},
		{
			name:          "nil layer falls back to provider value",
			providerValue: "mysql",
			expect:        "mysql",
		},
		{
			name:          "empty layer value falls back to provider value",
			providerValue: "auto",
			layer:         dict.Dict{"geometry_format": ""},
			expect:        "auto",
		},
		{
			name:          "layer override",
			providerValue: "auto",
			layer:         dict.Dict{"geometry_format": "mos"},
			expect:        "mos",
		},
		{
			name:          "layer override with whitespace",
			providerValue: "mysql",
			layer:         dict.Dict{"geometry_format": " wkb "},
			expect:        "wkb",
		},
		{
			name:          "provider mos overridden by layer wkt",
			providerValue: "mos",
			layer:         dict.Dict{"geometry_format": "wkt"},
			expect:        "wkt",
		},
		{
			name:          "invalid layer value errors with layer name",
			providerValue: "auto",
			layer:         dict.Dict{"geometry_format": "bogus"},
			layerName:     "roads",
			expectErr:     true,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := geometrycodec.ResolveLayerGeometryFormat(tc.providerValue, tc.layer, tc.layerName, allowed)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.layerName != "" && !strings.Contains(err.Error(), tc.layerName) {
					t.Errorf("error should mention layer name %q: %v", tc.layerName, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expect {
				t.Errorf("expected %q got %q", tc.expect, got)
			}
		})
	}
}

func TestResolveLayerGeometryFormatDoesNotMutateProviderValue(t *testing.T) {
	allowed := map[string]struct{}{"auto": {}, "wkb": {}, "wkt": {}, "mos": {}}
	providerValue := "auto"
	if _, err := geometrycodec.ResolveLayerGeometryFormat(providerValue, dict.Dict{"geometry_format": "wkb"}, "a", allowed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := geometrycodec.ResolveLayerGeometryFormat(providerValue, dict.Dict{}, "b", allowed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "auto" {
		t.Errorf("second layer should still see provider value %q, got %q", providerValue, got)
	}
}

func TestMergeMOSConfig(t *testing.T) {
	base := geometrycodec.MOSConfig{Precision: 2, UnitFactor: 1}

	// layer explicit precision overrides atomically (value + flag)
	got := geometrycodec.MergeMOSConfig(base, geometrycodec.MOSConfig{Precision: 3, PrecisionSet: true})
	if got.Precision != 3 || !got.PrecisionSet {
		t.Errorf("expected precision 3 set, got %+v", got)
	}
	if got.UnitFactor != 1 || got.UnitsSet {
		t.Errorf("units should inherit base, got %+v", got)
	}

	// layer explicit units override atomically (value + flag); the paired
	// default precision is re-derived for the new units because the base
	// precision was not explicit
	got = geometrycodec.MergeMOSConfig(base, geometrycodec.MOSConfig{UnitFactor: 0.001, UnitsSet: true})
	if got.UnitFactor != 0.001 || !got.UnitsSet {
		t.Errorf("expected factor 0.001 set, got %+v", got)
	}
	if got.Precision != geometrycodec.DefaultMOSPrecisionForUnits(0.001) || got.PrecisionSet {
		t.Errorf("expected re-paired precision %v unset, got %+v", geometrycodec.DefaultMOSPrecisionForUnits(0.001), got)
	}

	// m -> km layer override re-pairs precision to 5
	got = geometrycodec.MergeMOSConfig(base, geometrycodec.MOSConfig{UnitFactor: 1000, UnitsSet: true})
	if got.Precision != 5 {
		t.Errorf("expected precision 5 for km, got %+v", got)
	}

	// provider explicit precision survives a layer units override
	explicit := geometrycodec.MOSConfig{Precision: 3, UnitFactor: 1, PrecisionSet: true}
	got = geometrycodec.MergeMOSConfig(explicit, geometrycodec.MOSConfig{UnitFactor: 0.001, UnitsSet: true})
	if got.Precision != 3 || !got.PrecisionSet || got.UnitFactor != 0.001 {
		t.Errorf("explicit precision 3 must survive units override, got %+v", got)
	}

	// empty override keeps base untouched
	got = geometrycodec.MergeMOSConfig(base, geometrycodec.MOSConfig{})
	if got != base {
		t.Errorf("expected base %+v, got %+v", base, got)
	}
}

func TestMOSConfigHasExplicitMOSParams(t *testing.T) {
	if (geometrycodec.MOSConfig{}).HasExplicitMOSParams() {
		t.Error("zero config should have no explicit params")
	}
	if !(geometrycodec.MOSConfig{PrecisionSet: true}).HasExplicitMOSParams() {
		t.Error("PrecisionSet should count as explicit")
	}
	if !(geometrycodec.MOSConfig{UnitsSet: true}).HasExplicitMOSParams() {
		t.Error("UnitsSet should count as explicit")
	}
}

func TestResolveGeometryType(t *testing.T) {
	type tcase struct {
		name      string
		layerConf dict.Dict
		expected  geom.Geometry
		err       bool
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			g, explicit, err := geometrycodec.ResolveGeometryType(tc.layerConf, "test")
			if tc.err {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.expected == nil {
				if explicit {
					t.Fatal("expected no explicit geometry, got one")
				}
				return
			}
			if !explicit {
				t.Fatal("expected explicit geometry, got none")
			}
			if geometrycodec.GeomTypeName(g) != geometrycodec.GeomTypeName(tc.expected) {
				t.Fatalf("expected %v, got %v", geometrycodec.GeomTypeName(tc.expected), geometrycodec.GeomTypeName(g))
			}
		}
	}

	tests := map[string]tcase{
		"missing key":         {layerConf: dict.Dict{}, expected: nil},
		"empty value":         {layerConf: dict.Dict{"geometry_type": ""}, expected: nil},
		"auto alias":          {layerConf: dict.Dict{"geometry_type": "auto"}, expected: nil},
		"auto alias padded":   {layerConf: dict.Dict{"geometry_type": "  AUTO  "}, expected: nil},
		"point":               {layerConf: dict.Dict{"geometry_type": "point"}, expected: geom.Point{}},
		"canonical case":      {layerConf: dict.Dict{"geometry_type": "LineString"}, expected: geom.LineString{}},
		"padded spaces":       {layerConf: dict.Dict{"geometry_type": "  Polygon  "}, expected: geom.Polygon{}},
		"multipolygon":        {layerConf: dict.Dict{"geometry_type": "MULTIPOLYGON"}, expected: geom.MultiPolygon{}},
		"geometrycollection":  {layerConf: dict.Dict{"geometry_type": "geometrycollection"}, expected: geom.Collection{}},
		"invalid type":        {layerConf: dict.Dict{"geometry_type": "triangle"}, err: true},
		"non string":          {layerConf: dict.Dict{"geometry_type": 42}, err: true},
		"layer name in error": {layerConf: dict.Dict{"geometry_type": "bogus"}, err: true},
	}

	for tname, tc := range tests {
		t.Run(tname, fn(tc))
	}
}

func TestWarnOnceGeometryTypeMismatch(t *testing.T) {
	first := geometrycodec.WarnOnceGeometryTypeMismatch("mismatch-layer-test", geom.LineString{}, geom.Point{})
	if !first {
		t.Fatal("expected first mismatch to warn")
	}
	second := geometrycodec.WarnOnceGeometryTypeMismatch("mismatch-layer-test", geom.LineString{}, geom.Point{})
	if second {
		t.Fatal("expected warning only once per layer")
	}
	if geometrycodec.WarnOnceGeometryTypeMismatch("mismatch-layer-test", geom.LineString{}, geom.LineString{}) {
		t.Fatal("matching type must not warn")
	}
	if geometrycodec.WarnOnceGeometryTypeMismatch("mismatch-layer-test", nil, geom.Point{}) {
		t.Fatal("nil declared type must not warn")
	}
}

// TestPrepareProbeSQLNeutralization pins the shared probe preparation
// contract (audit part11 7.1.2): one documented substitution order, the
// probe always executes the SQL without a spatial filter, tile-dependent
// tokens become permissive values, and trailing clauses such as ORDER BY
// are preserved.
func TestPrepareProbeSQLNeutralization(t *testing.T) {
	custom := "SELECT * FROM t WHERE kind = 'road' AND z = !ZOOM! AND !BBOX! AND owner != 'x' ORDER BY id DESC;"

	out := geometrycodec.PrepareProbeSQL(custom, "geom", "okey", "Point")
	upper := strings.ToUpper(out)

	if !strings.Contains(out, "ORDER BY id DESC") {
		t.Fatalf("ORDER BY must be preserved verbatim: %q", out)
	}
	for _, tok := range []string{"!BBOX!", "!BOX!", "!ZOOM!", "!Z!", "!X!", "!Y!",
		"!SCALE_DENOMINATOR!", "!PIXEL_WIDTH!", "!PIXEL_HEIGHT!",
		"!ID_FIELD!", "!GEOM_FIELD!", "!GEOM_TYPE!"} {
		if strings.Contains(upper, tok) {
			t.Errorf("token %s left in probe SQL: %q", tok, out)
		}
	}
	// !BBOX! is neutralized to the permissive 1=1: the probe always
	// executes the SQL without a spatial filter
	if !strings.Contains(out, "1=1") {
		t.Errorf("!BBOX! must be neutralized to 1=1: %q", out)
	}
	// zoom comparisons expand to the permissive full zoom range
	if !strings.Contains(out, "IN (0,1,2,") {
		t.Errorf("zoom comparison must expand to the full zoom range: %q", out)
	}
	// trailing semicolon trimmed so the SQL can be wrapped
	if strings.HasSuffix(strings.TrimSpace(out), ";") {
		t.Errorf("trailing semicolon must be trimmed: %q", out)
	}

	wrapped := geometrycodec.WrapProbeSQL(out)
	limit := fmt.Sprintf("LIMIT %d", geometrycodec.InspectionSampleLimit)
	if !strings.HasPrefix(wrapped, "SELECT * FROM (") || !strings.Contains(wrapped, limit) {
		t.Errorf("probe must wrap the SQL with the sample limit, got %q", wrapped)
	}
	if !strings.Contains(wrapped, "ORDER BY id DESC") {
		t.Errorf("wrapping must preserve the trailing clauses: %q", wrapped)
	}
	top := geometrycodec.WrapProbeSQLTopStyle(out)
	if !strings.Contains(top, fmt.Sprintf("TOP %d", geometrycodec.InspectionSampleLimit)) {
		t.Errorf("TOP-style probe must carry the sample limit, got %q", top)
	}
}

// TestResolveBoundsSQLContract pins the structural contract split (b5ad0979
// regression fix): missing !BBOX! and missing geometry column stay
// fail-closed registration errors; missing bounds columns in the result set
// are NOT errors — the layer registers with the configured chain
// (layer > provider > MINX/MAXX/MINY/MAXY) and the caller warns, because the
// !BBOX! predicate expands inside the custom SQL and resolves in the query's
// own scope. When the bounds columns are found, their actual result-column
// names are returned (A09: runtime predicates quote identifiers
// case-sensitively).
func TestResolveBoundsSQLContract(t *testing.T) {
	ok := geometrycodec.SQLGeometryContract{
		BoundsFields:  [4]string{"minx", "maxx", "miny", "maxy"},
		GeometryField: "geom",
		HasBounds:     true,
	}
	configured := geometrycodec.DefaultBBoxFields()

	resolved, inResult, err := geometrycodec.ResolveBoundsSQLContract("l", "SELECT * FROM t WHERE !BBOX!", "geom", ok, configured)
	if err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}
	if !inResult {
		t.Fatal("bounds present in result must report boundsInResult=true")
	}
	if resolved != ok.BoundsFields {
		t.Fatalf("must persist the ACTUAL result-column names, got %v want %v", resolved, ok.BoundsFields)
	}

	_, _, err = geometrycodec.ResolveBoundsSQLContract("l", "SELECT * FROM t", "geom", ok, configured)
	if err == nil {
		t.Fatal("missing !BBOX! must be an error")
	}
	noGeom := ok
	noGeom.GeometryField = ""
	_, _, err = geometrycodec.ResolveBoundsSQLContract("l", "SELECT * FROM t WHERE !BBOX!", "geom", noGeom, configured)
	if err == nil {
		t.Fatal("missing geometry column must be an error")
	}

	// Regression pin: bounds filtered on but not selected in the result set
	// must pass through with the configured chain intact — never a fatal
	// error and never a wipe of the resolved names.
	noBounds := ok
	noBounds.HasBounds = false
	noBounds.BoundsFields = geometrycodec.BBoxFields{}
	resolved, inResult, err = geometrycodec.ResolveBoundsSQLContract("l", "SELECT geom FROM t WHERE !BBOX!", "geom", noBounds, configured)
	if err != nil {
		t.Fatalf("missing bounds columns in the result set must not be fatal (b5ad0979 regression): %v", err)
	}
	if inResult {
		t.Fatal("missing bounds columns must report boundsInResult=false")
	}
	if resolved != configured {
		t.Fatalf("fallback must keep the configured chain, got %v want %v", resolved, configured)
	}
}

// TestWarnNonMetricScaleTokens pins the warn-only item 1.3 code policy: the
// scale-denominator/pixel-size tokens are Web Mercator metres regardless of
// the layer CRS; non-metric (degrees) layer CRSs get a startup warning, and
// token values are left unchanged.
func TestWarnNonMetricScaleTokens(t *testing.T) {
	sql := "SELECT * FROM t WHERE !BBOX! AND z = !ZOOM! ORDER BY !SCALE_DENOMINATOR! DESC"

	if !geometrycodec.WarnNonMetricScaleTokens("l", sql, 4326, nil, nil) {
		t.Fatal("expected warning for degrees CRS using scale tokens")
	}
	if geometrycodec.WarnNonMetricScaleTokens("l", sql, 3857, nil, nil) {
		t.Fatal("metric CRS must not warn")
	}
	if geometrycodec.WarnNonMetricScaleTokens("l", "SELECT * FROM t WHERE !BBOX!", 4326, nil, nil) {
		t.Fatal("SQL without scale tokens must not warn")
	}
}

// TestPrepareProbeSQLComparisonNeutralization (audit P6-9) pins the
// comparison-level token neutralization: a !ZOOM!/!X!/!Y! comparison must
// collapse to the permissive "1=1" predicate no matter which side of the
// operator the token is on. The token-left form ("!ZOOM! >= 5") used to
// fall through to the bare "0" substitution, so the probe became "0 >= 5"
// and silently sampled zero rows.
func TestPrepareProbeSQLComparisonNeutralization(t *testing.T) {
	tcs := map[string]struct {
		custom string
		want   string
	}{
		"token-left zoom comparison": {
			custom: "SELECT * FROM t WHERE !ZOOM! >= 5",
			want:   "SELECT * FROM t WHERE 1=1",
		},
		"token-left zoom lower and upper bound": {
			custom: "SELECT * FROM t WHERE !ZOOM! >= 1 AND !ZOOM! <= 20",
			want:   "SELECT * FROM t WHERE 1=1 AND 1=1",
		},
		"token-left position comparisons": {
			custom: "SELECT * FROM t WHERE !X! <= 100 AND !Y! = 3",
			want:   "SELECT * FROM t WHERE 1=1 AND 1=1",
		},
		"token-right position comparisons": {
			custom: "SELECT * FROM t WHERE cx = !X! AND cy = !Y!",
			want:   "SELECT * FROM t WHERE 1=1 AND 1=1",
		},
		"numeric operand left of the token": {
			custom: "SELECT * FROM t WHERE 5 <= !X!",
			want:   "SELECT * FROM t WHERE 1=1",
		},
		"token-to-token comparison": {
			custom: "SELECT * FROM t WHERE !X! = !Y!",
			want:   "SELECT * FROM t WHERE 1=1",
		},
		"arithmetic tail after the token is consumed": {
			custom: "SELECT * FROM t WHERE cx = !X! + 1",
			want:   "SELECT * FROM t WHERE 1=1",
		},
		"arithmetic operand is consumed": {
			custom: "SELECT * FROM t WHERE !X! = cy + 1",
			want:   "SELECT * FROM t WHERE 1=1",
		},
		"string operand is consumed": {
			custom: "SELECT * FROM t WHERE !X! = 'west'",
			want:   "SELECT * FROM t WHERE 1=1",
		},
		"schema-qualified operand is consumed": {
			custom: "SELECT * FROM t WHERE t.mx = !X!",
			want:   "SELECT * FROM t WHERE 1=1",
		},
		"BETWEEN continuation is consumed": {
			custom: "SELECT * FROM t WHERE !X! = y BETWEEN 1 AND 2",
			want:   "SELECT * FROM t WHERE 1=1",
		},
		"IS NULL continuation is consumed": {
			custom: "SELECT * FROM t WHERE !X! = y IS NULL",
			want:   "SELECT * FROM t WHERE 1=1",
		},
		"function-call left operand falls back to bare substitution": {
			// the neutralization must never cut an expression apart: when the
			// operand is a function call the comparison is left to the safe
			// bare-token fallback (same SQL as before P6-9).
			custom: "SELECT * FROM t WHERE f(x) >= !X!",
			want:   "SELECT * FROM t WHERE f(x) >= 0",
		},
		"bare non-comparison tokens keep the numeric fallback": {
			custom: "SELECT !X! AS tile_x, !ZOOM! AS tile_zoom FROM t",
			want:   "SELECT 0 AS tile_x, 0 AS tile_zoom FROM t",
		},
		"comparisons without tokens are untouched": {
			custom: "SELECT * FROM t WHERE owner != 'x' AND id = 3",
			want:   "SELECT * FROM t WHERE owner != 'x' AND id = 3",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			got := geometrycodec.PrepareProbeSQL(tc.custom, "geom", "okey", "Point")
			if got != tc.want {
				t.Fatalf("PrepareProbeSQL(%q)\n got %q\nwant %q", tc.custom, got, tc.want)
			}
		})
	}

	// zoom comparisons with the token on the right keep the permissive full
	// zoom range (contract of TestPrepareProbeSQLNeutralization).
	t.Run("right-side zoom comparisons keep the full range", func(t *testing.T) {
		for _, custom := range []string{
			"SELECT * FROM t WHERE z = !ZOOM!",
			"SELECT * FROM t WHERE 5 <= !ZOOM!",
		} {
			got := geometrycodec.PrepareProbeSQL(custom, "geom", "okey", "Point")
			if !strings.Contains(got, "IN (0,1,2,") {
				t.Fatalf("PrepareProbeSQL(%q) = %q, want permissive zoom range", custom, got)
			}
		}
	})

	// every occurrence is neutralized, on both sides, in one query.
	t.Run("every comparison occurrence is neutralized", func(t *testing.T) {
		custom := "SELECT * FROM t WHERE !ZOOM! >= 1 AND max_zoom <= !ZOOM! AND cx = !X! AND !Y! = cy"
		got := geometrycodec.PrepareProbeSQL(custom, "geom", "okey", "Point")
		if n := strings.Count(got, "1=1"); n != 3 {
			t.Fatalf("PrepareProbeSQL(%q) = %q, want three permissive predicates", custom, got)
		}
		if strings.Contains(got, "!") {
			t.Fatalf("PrepareProbeSQL(%q) = %q, want no leftover tokens", custom, got)
		}
		if !strings.Contains(got, "max_zoom IN (0,1,2,") {
			t.Fatalf("PrepareProbeSQL(%q) = %q, want right-side zoom range preserved", custom, got)
		}
	})
}

func TestContractMetadataDoesNotReadRows(t *testing.T) {
	next := func() ([]interface{}, bool, error) { t.Fatal("metadata probe consumed a row"); return nil, false, nil }
	c, err := geometrycodec.InspectSQLGeometryContract(next, []string{"geom", "minx", "maxx", "miny", "maxy"}, "geom", geometrycodec.DefaultBBoxFields(), nil)
	if err != nil || !c.HasBounds || c.GeometryField != "geom" || c.ValidRows != 0 {
		t.Fatalf("contract=%+v err=%v", c, err)
	}
}

func TestMOSInferenceStopsAfterThreeSuccesses(t *testing.T) {
	reads, decodes := 0, 0
	next := func() ([]interface{}, bool, error) {
		reads++
		if reads > 5 {
			t.Fatal("read past third successful geometry")
		}
		return []interface{}{reads}, true, nil
	}
	decode := func(value interface{}) (geom.Geometry, bool, error) {
		decodes++
		if value.(int) <= 2 {
			return nil, false, fmt.Errorf("invalid sample")
		}
		return geom.Point{1, 2}, true, nil
	}
	c, err := geometrycodec.InspectSQLGeometryContract(next, []string{"geom"}, "geom", geometrycodec.DefaultBBoxFields(), decode)
	if err != nil || c.ValidMOSRows != 3 || reads != 5 || decodes != 5 {
		t.Fatalf("contract=%+v reads=%d decodes=%d err=%v", c, reads, decodes, err)
	}
}

func TestMOSInferenceSampleBudget(t *testing.T) {
	reads := 0
	next := func() ([]interface{}, bool, error) { reads++; return []interface{}{1}, true, nil }
	decode := func(interface{}) (geom.Geometry, bool, error) { return nil, false, fmt.Errorf("invalid") }
	c, err := geometrycodec.InspectSQLGeometryContract(next, []string{"geom"}, "geom", geometrycodec.DefaultBBoxFields(), decode)
	if err != nil || reads != geometrycodec.InspectionSampleLimit || c.ValidMOSRows != 0 {
		t.Fatalf("contract=%+v reads=%d err=%v", c, reads, err)
	}
}

func TestShortMOSDetectionRequiresCompletedNonemptyEvidence(t *testing.T) {
	for _, tc := range []struct {
		rows, valid int
		want        bool
	}{{1, 1, true}, {2, 1, true}, {0, 0, false}, {2, 0, false}, {16, 1, false}, {16, 0, false}} {
		reads := 0
		next := func() ([]interface{}, bool, error) {
			if reads == tc.rows {
				return nil, false, nil
			}
			reads++
			return []interface{}{reads}, true, nil
		}
		decode := func(v interface{}) (geom.Geometry, bool, error) {
			if v.(int) <= tc.valid {
				return geom.Point{1, 2}, true, nil
			}
			return nil, false, fmt.Errorf("invalid")
		}
		c, err := geometrycodec.InspectSQLGeometryContract(next, []string{"geom"}, "geom", geometrycodec.DefaultBBoxFields(), decode)
		if err != nil || c.DetectsMOS() != tc.want {
			t.Fatalf("rows=%d valid=%d contract=%+v err=%v", tc.rows, tc.valid, c, err)
		}
	}
}
