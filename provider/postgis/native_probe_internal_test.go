package postgis

import (
	"fmt"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/internal/ttools"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/test/mosfixture"
	"strings"
	"testing"
)

func TestNativeProbeGeometryOperand(t *testing.T) {
	basic.RegisterBuiltinProj4SRIDs()
	for _, srid := range []uint64{3857, 4326, 32631} {
		sql, err := prepareLayerProbeSQL("SELECT ST_AsMVTGeom(geom,!bbox!), '!BBOX!' FROM t WHERE geom && !BBOX! AND ST_Intersects(geom,!BOX!)", &Layer{name: "native", srid: srid}, "")
		if err != nil {
			t.Errorf("srid %d: %v", srid, err)
			continue
		}
		if strings.Count(sql, "ST_MakeEnvelope(") != 3 || !strings.Contains(sql, "'!BBOX!'") {
			t.Errorf("invalid native operand SQL %s", sql)
		}
	}
	if _, err := prepareLayerProbeSQL("SELECT !BBOX!", &Layer{name: "unknown", srid: 999999}, ""); err == nil {
		t.Fatal("unsupported CRS error swallowed")
	}
	sql, err := prepareLayerProbeSQL("SELECT geom FROM t WHERE !BBOX!", &Layer{geometryFormat: codec.FormatMOS}, "")
	if err != nil || !strings.Contains(sql, "WHERE 1=1") {
		t.Fatalf("raw MOS probe: %s, %v", sql, err)
	}
}

func TestAutoMOSProbePreservesBooleanBounds(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)
	sql := fmt.Sprintf(`SELECT id AS gid, 0 AS "MINX", 10 AS "MAXX", 0 AS "MINY", 10 AS "MAXY", decode('%x','hex') AS geom FROM generate_series(1,3) AS id WHERE !BBOX!`, mosfixture.MOSBlob(5, 5))
	cfg := TCConfig{LayerConfig: []map[string]any{{ConfigKeyLayerName: "auto_mos", ConfigKeySQL: sql, ConfigKeyGeomField: "geom", ConfigKeyGeomIDField: "gid", ConfigKeySRID: 3857}}}.Config(DefaultEnvConfig)
	cfg[ConfigKeyName] = "auto_mos_probe"
	p, err := CreateProvider(cfg, nil, ProviderType)
	if err != nil {
		t.Fatal(err)
	}
	defer p.pool.Close()
	if p.layers["auto_mos"].geometryFormat != codec.FormatMOS {
		t.Fatalf("auto MOS format not detected: %q", p.layers["auto_mos"].geometryFormat)
	}
}

func TestNativeProbeSpatialFunctionArgumentsLive(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)
	cfg := TCConfig{LayerConfig: []map[string]any{{ConfigKeyLayerName: "native_probe", ConfigKeySQL: "SELECT ST_AsMVTGeom(geom,!BBOX!) AS geom, gid FROM (SELECT ST_MakeEnvelope(-1000000,-1000000,1000000,1000000,3857) AS geom, 1 AS gid) AS sample WHERE ST_Intersects(geom,!BBOX!)", ConfigKeyGeomField: "geom", ConfigKeyGeomIDField: "gid", ConfigKeySRID: 3857}}}.Config(DefaultEnvConfig)
	cfg[ConfigKeyName] = "native_probe"
	p, err := CreateProvider(cfg, nil, ProviderType)
	if err != nil {
		t.Fatal(err)
	}
	defer p.pool.Close()
	if p.layers["native_probe"].geomType == nil {
		t.Fatal("ST_AsMVTGeom probe did not infer a geometry type")
	}
}
