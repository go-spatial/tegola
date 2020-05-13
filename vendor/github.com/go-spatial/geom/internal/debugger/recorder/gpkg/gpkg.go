// +build cgo

package gpkg

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-spatial/geom/encoding/gpkg"

	"github.com/go-spatial/geom/internal/debugger/recorder"
)

const (
	createTableSQLFmt = `
	DROP TABLE IF EXISTS %[1]s;
	CREATE TABLE "%[1]s" ( 
		  id INTEGER PRIMARY KEY AUTOINCREMENT
	    , function_name TEXT
		, filename TEXT
	    , line INTEGER
	    , name TEXT
		, description TEXT
	    , category TEXT
		, geometry %[2]s
	)
	`
	insertTableSQLFmt = `
	INSERT INTO "%[1]s" (
		  function_name
		, filename
		, line
		, name
		, description
		, category
		, geometry 
	) VALUES (?,?,?,?,?,?,?);
	`
)

type DB struct {
	*gpkg.Handle

	srsid      int32
	tblPrefix  string
	filename   string
	statements map[gpkg.GeometryType]*sql.Stmt
}

// New returns a new recorder, the file where the recrods are recorded to and
// any errors.
func New(outputDir, filename string, srsid int32) (*DB, string, error) {

	dbFilename := filepath.Join(outputDir, filename+".gpkg")
	os.Remove(dbFilename)

	h, err := gpkg.Open(dbFilename)
	if err != nil {
		return nil, dbFilename, fmt.Errorf("dbfile: %v err: %v", dbFilename, err)
	}
	db := &DB{
		Handle:     h,
		srsid:      srsid,
		tblPrefix:  "test_",
		statements: make(map[gpkg.GeometryType]*sql.Stmt, 6),
	}

	for _, gType := range []gpkg.GeometryType{
		gpkg.Point, gpkg.MultiPoint,
		gpkg.Linestring, gpkg.MultiLinestring,
		gpkg.Polygon, gpkg.MultiPolygon,
	} {
		lgType := strings.ToLower(gType.String())
		tblName := db.TableName(gType)
		_, err := db.Exec(fmt.Sprintf(createTableSQLFmt, tblName, gType))
		if err != nil {
			return nil, dbFilename, err
		}
		err = db.AddGeometryTable(gpkg.TableDescription{
			Name:          tblName,
			ShortName:     fmt.Sprintf("test table for %v geometries", lgType),
			Description:   fmt.Sprintf("Table containting %v type entries.", lgType),
			GeometryField: "geometry",
			GeometryType:  gType,
			SRS:           db.srsid,
			Z:             gpkg.Prohibited,
			M:             gpkg.Prohibited,
		})
		if err != nil {
			return nil, dbFilename, err
		}
		db.statements[gType], err = db.Prepare(fmt.Sprintf(insertTableSQLFmt, tblName))
		if err != nil {
			return nil, dbFilename, err
		}
	}

	return db, dbFilename, nil
}

func (db *DB) TableName(gType gpkg.GeometryType) string {
	lgType := strings.ToLower(gType.String())
	return db.tblPrefix + lgType
}

func (db *DB) Record(geo interface{}, ffl recorder.FuncFileLineType, tblTest recorder.TestDescription) error {
	if db == nil {
		return nil
	}

	gtype := gpkg.TypeForGeometry(geo)
	if gtype == gpkg.Geometry {
		err := fmt.Errorf("error unknown geometry type %T", geo)
		if debug {
			log.Println(ffl, err)
		}
		return err
	}

	stm, ok := db.statements[gtype]
	if !ok {
		err := fmt.Errorf("error unsupported geom %s", gtype)
		if debug {
			log.Println(ffl, err)
		}
		return err
	}

	sb, err := gpkg.NewBinary(db.srsid, geo)
	if err != nil {
		err = fmt.Errorf("error unsupported geometry %s :  %v", gtype, err)
		if debug {
			log.Println(ffl, err)
		}
		return err
	}

	_, err = stm.Exec(

		ffl.Func,
		ffl.File,
		ffl.LineNumber,

		tblTest.Name,
		tblTest.Description,
		tblTest.Category,

		sb,
	)
	if err != nil {
		log.Println(err)
		return err
	}
	// update extent
	db.UpdateGeometryExtent(db.TableName(gtype), sb.Extent())
	return nil
}
