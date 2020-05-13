// +build cgo

package subdivision

// OpenTestDB(filename)  *TestDB,error
// TestDB.Write(name, description, sd) returns id
// TestDB.Get(id) subdivision,error
// TestDB.SubdivisionFrom(id, newName, description, points...) returns id
// this would write all the lines in the subdivision, and the frame?
// to the subdivision table in the gpkg database identified by filename

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/go-spatial/geom/winding"

	"github.com/go-spatial/geom/planar"

	"github.com/gdey/errors"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/planar/triangulate/delaunay/quadedge"

	"github.com/go-spatial/geom/encoding/gpkg"
	"github.com/go-spatial/geom/encoding/wkt"
	"github.com/go-spatial/geom/planar/triangulate/delaunay/subdivision/testingtables"
)

const (
	DefaultSRS                     = 0
	TableNameSubdivision           = "subdivisions"
	TableNameSubdivisionEdge       = "subdivision_edges"
	TableNameAssociatedLinestrings = "associated_linestrings"
	TableNameAssociatedPoints      = "associated_points"
	TestDBInsertSDSQL              = `
		INSERT INTO subdivisions (
			name, 
			description, 
			frame
		) VALUES ( ?, ?, ?);
	`
	TestDBInsertEdgeSQL = `
		INSERT INTO subdivision_edges (
			sd_id,
			insert_order,
			is_frame,
			edge
		) VALUES ( ?,?,?,? );
	`
	TestDBInsertLineStringSQL = `
		INSERT INTO associated_linestrings (
			sd_id,
			description,
			function,
			geometry
		) VALUES ( ?,?,?,? );
	`
	TestDBInsertPointSQL = `
		INSERT INTO associated_points(
			sd_id,
			description,
			function,
			geometry
		) VALUES ( ?,?,?,? );
	`

	ErrTestDBNil = errors.String("TestDB is nil")
)

var (
	testDBSQL2STMT = map[string]string{
		"insert_edge":       TestDBInsertEdgeSQL,
		"insert_sd":         TestDBInsertSDSQL,
		"insert_linestring": TestDBInsertLineStringSQL,
		"insert_point":      TestDBInsertPointSQL,
	}
	testTables = []testingtables.Description{

		{
			Name:      TableNameSubdivision,
			GeomField: "frame",
			GType:     gpkg.MultiPoint,
			Desc:      "a subdivision",
			SRS:       DefaultSRS,
			CreateSQL: `
		-- subdivision
		--
		-- a subdivision description and frame
		-- Z: Prohibited 
		-- M: Prohibited
		CREATE TABLE IF NOT EXISTS "subdivisions" (
		id INTEGER PRIMARY KEY AUTOINCREMENT
		, name TEXT
		, description TEXT
		, frame MULTIPOINT
		);
		`,
		},
		{
			Name:      TableNameSubdivisionEdge,
			GeomField: "edge",
			GType:     gpkg.Linestring,
			Desc:      "the edge of a subdivision",
			SRS:       DefaultSRS,
			CreateSQL: `
		-- subdivision edges
		-- 
		-- the edges of a subdivision
		-- Z: Prohibited 
		-- M: Prohibited
		CREATE TABLE IF NOT EXISTS "subdivision_edges" (
		sd_id INTEGER
		, insert_order INTEGER DEFAULT 0 -- the order in which the edge was inserted into the graph
		, is_frame BOOLEAN DEFAULT false -- is part of the frame
		, edge LINESTRING
		, FOREIGN KEY(sd_id) REFERENCES subdivisions(id)
		);
		`,
		},
		{
			Name:  TableNameAssociatedLinestrings,
			GType: gpkg.Linestring,
			Desc:  "A linestring we want in the graph",
			SRS:   DefaultSRS,
			CreateSQL: `
		--  associated_linestrings
		-- 
		-- A linestring we want in the graph
		-- Z: Prohibited 
		-- M: Prohibited
		CREATE TABLE IF NOT EXISTS "associated_linestrings" (
		sd_id INTEGER
		, description TEXT DEFAULT ''
		, function TEXT DEFAULT ''
		, geometry LINESTRING
		, FOREIGN KEY(sd_id) REFERENCES subdivisions(id)
		);
		`,
		},
		{
			Name:  TableNameAssociatedPoints,
			GType: gpkg.Point,
			Desc:  "A point we want in the graph",
			SRS:   DefaultSRS,
			CreateSQL: `
		--  associated_points
		-- 
		-- A linestring we want in the graph
		-- Z: Prohibited 
		-- M: Prohibited
		CREATE TABLE IF NOT EXISTS "associated_points" (
		sd_id INTEGER
		, description TEXT DEFAULT ''
		, function TEXT DEFAULT ''
		, geometry POINT
		, FOREIGN KEY(sd_id) REFERENCES subdivisions(id)
		);
		`,
		},
	}
)

// OpenTestDB will open the gpkg file and read it for reading and writing
func OpenTestDB(filename string) (*TestDB, error) {
	descers := make([]testingtables.Descriptioner, len(testTables))
	for i := range testTables {
		descers[i] = testTables[i]
	}

	db, err := testingtables.OpenTestDB(filename, descers...)
	if err != nil {
		return nil, err
	}
	tdb := &TestDB{DB: db}
	if err = tdb.PrepareStatements(); err != nil {
		return nil, err
	}
	return tdb, nil
}

type TestDB struct {
	*testingtables.DB
	statements map[string]*sql.Stmt
}

// PrepareStatements prepares heavely used statements
func (db *TestDB) PrepareStatements() error {

	if db == nil {
		return ErrTestDBNil
	}

	db.statements = make(map[string]*sql.Stmt, len(testDBSQL2STMT))
	for key, sql := range testDBSQL2STMT {
		stmt, err := db.Prepare(sql)
		if err != nil {
			return err
		}
		db.statements[key] = stmt
	}
	return nil
}

// Write the subdivision to the db with the given name and description
func (db *TestDB) Write(name, description string, sd *Subdivision) (int64, error) {
	if db == nil {
		return 0, ErrTestDBNil
	}

	ext := geom.NewExtentFromPoints(sd.frame[:]...)
	frame := make(geom.MultiPoint, len(sd.frame))
	for i := range sd.frame {
		frame[i] = [2]float64(sd.frame[i])
	}

	frameSB, err := gpkg.NewBinary(DefaultSRS, frame)
	if err != nil {
		return 0, err
	}
	idResult, err := db.statements["insert_sd"].Exec(name, description, frameSB)
	if err != nil {
		return 0, err
	}
	id, err := idResult.LastInsertId()
	if err != nil {
		return 0, err
	}
	db.UpdateGeometryExtent(TableNameSubdivision, ext)
	edgeStatement := db.statements["insert_edge"]
	ext = nil
	err = sd.WalkAllEdges(func(e *quadedge.Edge) error {
		// Skip the hard frame edges as these will get added
		// automatically
		if IsHardFrameEdge(sd.frame, e) {
			return nil
		}
		ln := e.AsLine()
		sb, err := gpkg.NewBinary(DefaultSRS, ln)
		isFrame := IsFrameEdge(sd.frame, e)
		if ext == nil {
			ext, _ = geom.NewExtentFromGeometry(ln)
		} else {
			ext.AddGeometry(ln)
		}

		_, err = edgeStatement.Exec(
			id,
			0, // insert_order -- e.InsertOrder()
			isFrame,
			sb,
		)
		return err
	})
	db.UpdateGeometryExtent(TableNameSubdivisionEdge, ext)
	return id, err
}

// Write the subdivision to the db with the given name and description
func (db *TestDB) WriteContained(name, description string, sd *Subdivision, start, end geom.Point) (int64, error) {
	// get the distance this will be the radius for our two circles
	ptDistance := planar.PointDistance(start, end)
	cStart := geom.Circle{
		Center: [2]float64(start),
		Radius: ptDistance,
	}
	cEnd := geom.Circle{
		Center: [2]float64(end),
		Radius: ptDistance,
	}
	ext := geom.NewExtentFromPoints(cStart.AsPoints(30)...)
	ext1 := geom.NewExtentFromPoints(cEnd.AsPoints(30)...)
	ext.Add(ext1)
	ext = ext.ExpandBy(10)

	tri, err := geom.NewTriangleForExtent(ext, 10)
	if err != nil {
		return 0, err
	}

	frame := make(geom.MultiPoint, len(tri))
	for i := range tri {
		frame[i] = [2]float64(tri[i])
	}

	frameSB, err := gpkg.NewBinary(DefaultSRS, frame)
	if err != nil {
		return 0, err
	}
	idResult, err := db.statements["insert_sd"].Exec(name, description, frameSB)
	if err != nil {
		return 0, err
	}
	id, err := idResult.LastInsertId()
	if err != nil {
		return 0, err
	}
	db.UpdateGeometryExtent(TableNameSubdivision, ext)

	edgeStatement := db.statements["insert_edge"]

	var ml geom.MultiLineString
	err = sd.WalkAllEdges(func(e *quadedge.Edge) error {
		ln := e.AsLine()
		if !ext.ContainsPoint(ln[0]) && !ext.ContainsPoint(ln[1]) {
			return nil
		}

		ml = append(ml, ln[:])

		isFrame := IsFrameEdge(sd.frame, e)
		sb, err := gpkg.NewBinary(DefaultSRS, ln)
		if err != nil {
			return err
		}

		_, err = edgeStatement.Exec(
			id,
			0, // insert_order -- e.InsertOrder()
			isFrame,
			sb,
		)

		return err
	})
	log.Printf("sd edges\n%v\n\n", wkt.MustEncode(ml))

	db.UpdateGeometryExtent(TableNameSubdivisionEdge, ext)
	return id, err
}

// WriteLineString writes a line string that is associated with a subdivision
func (db *TestDB) WriteLineString(id int64, function, description string, line geom.LineString) error {
	if db == nil {
		return ErrTestDBNil
	}

	sb, err := gpkg.NewBinary(DefaultSRS, line)
	if err != nil {
		return err
	}
	_, err = db.statements["insert_linestring"].Exec(id, function, description, sb)
	if err != nil {
		return err
	}

	return db.UpdateGeometryExtent(
		TableNameAssociatedLinestrings,
		geom.NewExtent(line[:]...),
	)

}

// WriteEdge writes a edge that is associated linestring with a subdivision
func (db *TestDB) WriteEdge(id int64, function, description string, edge *quadedge.Edge) error {
	ln := edge.AsLine()
	return db.WriteLineString(id, function, description, geom.LineString(ln[:]))
}

// WritePoint writes a line string that is associated with a subdivision
func (db *TestDB) WritePoint(id int64, function, description string, point geom.Point) error {
	log.Println("writing point", wkt.MustEncode(point))
	if db == nil {
		return ErrTestDBNil
	}

	sb, err := gpkg.NewBinary(DefaultSRS, point)
	if err != nil {
		return err
	}
	_, err = db.statements["insert_point"].Exec(id, function, description, sb)
	if err != nil {
		return err
	}
	return db.UpdateGeometryExtent(
		TableNameAssociatedLinestrings,
		geom.NewExtentFromPoints(point),
	)

}

func (db *TestDB) getLines(id int64) ([]geom.Line, error) {
	const (
		EdgeSQL = `
		SELECT 
			insert_order,
			edge
		FROM subdivisions_id
		WHERE id = ?
		Order By insert_order;
		`
	)

	var (
		err   error
		ok    bool
		lines []geom.Line
		ln    geom.LineString
		lnSB  gpkg.StandardBinary
		inso  int
		rows  *sql.Rows
	)
	rows, err = db.Query(EdgeSQL, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		if err = rows.Scan(&inso, lnSB); err != nil {
			return nil, err
		}
		if ln, ok = lnSB.Geometry.(geom.LineString); !ok {
			return nil, fmt.Errorf("was not able to convert to line: %v", lnSB.Geometry)
		}
		if len(ln) < 2 {
			return nil, fmt.Errorf("was not able to convert to line: %v", ln)
		}
		lines = append(lines, geom.Line{ln[0], ln[1]})
	}
	return lines, nil

}

// Get the subdivision as described by the id
func (db *TestDB) Get(id int64) (*Subdivision, error) {
	if db == nil {
		return nil, ErrTestDBNil
	}

	lines, err := db.getLines(id)
	if err != nil {
		return nil, err
	}
	order, err := db.Order(id)
	if err != nil {
		return nil, err
	}

	sd := NewSubdivisionFromGeomLines(lines, order)
	return sd, nil
}

func (db *TestDB) Order(_ int64) (winding.Order, error) {
	return winding.Order{}, nil
}

// SubdivisionFrom will create a new subdivsion that is a subsection of
// another subdivision that is described by the points
func (db *TestDB) SubdivisionFrom(id int64, name, description string, pts ...geom.Point) (int64, error) {
	if db == nil {
		return 0, ErrTestDBNil
	}

	if len(pts) == 0 {
		return 0, fmt.Errorf("no points given")
	}

	var (
		start, end geom.Point
	)

	allLines, err := db.getLines(id)
	if err != nil {
		return 0, err
	}

	ext := geom.NewExtentFromPoints(pts...)

	lines := make([]geom.Line, 0, len(allLines))

	for _, line := range allLines {
		for _, pt := range pts {
			start, end = geom.Point(line[0]), geom.Point(line[1])
			ext.AddPointers(start, end)
			if cmp.GeomPointEqual(pt, start) || cmp.GeomPointEqual(pt, end) {
				lines = append(lines, line)
				break
			}
		}
	}
	lines = lines[0:len(lines):len(lines)]
	frame, err := geom.NewTriangleForExtent(ext, 10.0)
	if err != nil {
		return 0, err
	}

	frameSB, err := gpkg.NewBinary(DefaultSRS, frame[:])
	if err != nil {
		return 0, err
	}
	idResult, err := db.statements["insert_sd"].Exec(name, description, frameSB)
	if err != nil {
		return 0, err
	}
	newID, err := idResult.LastInsertId()
	if err != nil {
		return 0, err
	}
	edgeStatement := db.statements["insert_edge"]

	for i, line := range lines {
		sb, err := gpkg.NewBinary(DefaultSRS, line)
		if err != nil {
			return newID, err
		}

		if _, err = edgeStatement.Exec(int64(newID), int64(i), false, sb); err != nil {
			return id, err
		}
	}
	return newID, nil

}
