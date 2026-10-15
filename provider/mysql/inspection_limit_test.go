package mysql

import (
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/alexeydott/geom"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/test/fixture"
)

func TestAutoCustomProbeOversizeIsRequiredWithExplicitType(t *testing.T) {
	oversize := make([]byte, codec.MaxInspectionGeometryBytes+1)
	for _, deferred := range []bool{false, true} {
		layer := &Layer{name: "items", geomFieldname: "geom", geomType: geom.Point{},
			bboxFields: codec.DefaultBBoxFields(), geometryFormat: GeometryFormatAuto}
		query := "SELECT id,geom FROM source"
		if deferred {
			query += " WHERE tile_x = !X!"
		}
		if customSQLNeedsDeferredInspection(query) != deferred {
			t.Fatal("deferred fixture is not meaningful")
		}
		db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: []string{"id", "geom"},
			Rows: [][]driver.Value{{int64(1), oversize}}})
		prepared := codec.MySQL.PrepareProbeSQL(query, "geom", "id", "POINT")
		_, _, err := probeMOSCustomSQLContract(db, layer, prepared, GeometryFormatAuto, GeometryFormatMySQL)
		if !errors.Is(err, errInspectionGeometryTooLarge) || !requiredCustomSQLProbeError(err, GeometryFormatAuto) {
			t.Fatalf("oversize auto probe can bypass registration through explicit/deferred geometry: %v", err)
		}
	}
}

func TestAutoCustomProbeOtherErrorsRemainBestEffort(t *testing.T) {
	err := errors.New("inconclusive inspection")
	if requiredCustomSQLProbeError(err, GeometryFormatAuto) || requiredCustomSQLProbeError(err, "") ||
		requiredCustomSQLProbeError(nil, codec.FormatMOS) || !requiredCustomSQLProbeError(err, codec.FormatMOS) {
		t.Fatal("cap classification changed ordinary automatic or explicit MOS policy")
	}
	db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: []string{"geom"}, Rows: [][]driver.Value{{[]byte{0xff}}}})
	layer := &Layer{name: "items", geomFieldname: "geom", bboxFields: codec.DefaultBBoxFields()}
	_, contract, err := probeMOSCustomSQLContract(db, layer, "SELECT geom FROM source", GeometryFormatAuto, GeometryFormatMySQL)
	if err != nil || contract.ValidRows != 0 {
		t.Fatalf("malformed automatic sample stopped being inconclusive: %+v %v", contract, err)
	}
}
