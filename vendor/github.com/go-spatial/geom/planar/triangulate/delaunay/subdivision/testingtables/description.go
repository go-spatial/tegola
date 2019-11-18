// +build cgo

package testingtables

import (
	"fmt"
	"strings"

	"github.com/go-spatial/geom/encoding/gpkg"
)

type Descriptioner interface {
	TableDescription() gpkg.TableDescription
	SQLCreate() string
}

type Description struct {
	Name      string
	GeomField string
	GType     gpkg.GeometryType
	Desc      string
	SRS       int32
	CreateSQL string
}

func (td Description) Field() string {
	field := strings.TrimSpace(td.GeomField)
	if field != "" {
		return field
	}
	return "geometry"
}
func (td Description) Description() string {
	desc := strings.TrimSpace(td.Desc)
	if desc != "" {
		return desc
	}
	return fmt.Sprintf("table %v for %v", td.Name, td.GType)
}
func (td Description) ShortName() string { return strings.Replace(td.Name, "_", " ", -1) }
func (td Description) Z() gpkg.MaybeBool { return gpkg.Prohibited }
func (td Description) M() gpkg.MaybeBool { return gpkg.Prohibited }
func (td Description) TableDescription() gpkg.TableDescription {
	return gpkg.TableDescription{
		Name:          td.Name,
		ShortName:     td.ShortName(),
		Description:   td.Description(),
		GeometryField: td.Field(),
		GeometryType:  td.GType,
		SRS:           td.SRS,
		Z:             td.Z(),
		M:             td.M(),
	}
}

func (td Description) SQLCreate() string { return td.CreateSQL }

func Init(h *gpkg.Handle, tables ...Descriptioner) error {
	if h == nil {
		return gpkg.ErrNilHandler
	}
	var (
		err error
	)
	// Make sure our table exists and are registerd with the
	// system
	for _, table := range tables {
		_, err = h.Exec(table.SQLCreate())
		if err != nil {
			return err
		}
		err = h.AddGeometryTable(table.TableDescription())
		if err != nil {
			return err
		}
	}
	return nil
}

func OpenTestDB(filename string, descriptions ...Descriptioner) (*DB, error) {
	h, err := gpkg.Open(filename)
	if err != nil {
		return nil, err
	}
	if err = Init(h, descriptions...); err != nil {
		return nil, err
	}
	return &DB{
		filename: filename,
		Handle:   h,
	}, nil
}

type DB struct {
	*gpkg.Handle

	filename string
}

func (db *DB) Filename() string {
	if db == nil {
		return ""
	}
	return db.filename
}
