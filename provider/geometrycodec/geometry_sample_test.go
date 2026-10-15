package geometrycodec

import (
	"strings"
	"testing"
)

func TestMySQLGeometrySampleDirectProjection(t *testing.T) {
	for _, query := range []string{
		"SELECT c.id,c.geom AS shape,v.label FROM chambers c LEFT JOIN labels v ON c.id=v.id WHERE c.id>0 LIMIT 3 OFFSET 7",
		"SELECT `c`.`geom` AS `shape`, `unused,large` FROM chambers c LIMIT 7,20;",
	} {
		got := MySQLGeometrySampleSQL(query, "shape")
		if strings.Contains(got, "__tegola_geometry_probe") || strings.Contains(got, "v.label") || strings.Contains(got, "unused,large") ||
			!strings.Contains(got, "LEFT(CAST(") || !strings.Contains(got, "67108865") || !strings.Contains(got, " AS `shape`") {
			t.Fatalf("direct geometry-only sample: %q", got)
		}
		if strings.Contains(query, "OFFSET 7") && !strings.Contains(got, "LIMIT 3 OFFSET 7") {
			t.Fatal("sample changed original smaller limit/offset", got)
		}
	}
}

func TestMySQLGeometrySampleConservativeFallback(t *testing.T) {
	for _, query := range []string{
		"SELECT * FROM t", "SELECT DISTINCT geom FROM t", "SELECT ALL geom FROM t",
		"SELECT ST_AsBinary(geom) AS geom FROM t", "SELECT geom FROM t ORDER BY 2",
		"SELECT geom FROM t GROUP BY geom", "SELECT geom FROM t HAVING 1=1",
		"SELECT geom FROM t UNION SELECT geom FROM u", "SELECT geom FROM t FOR UPDATE",
		"SELECT geom FROM t /*! LIMIT 5 */", "SELECT geom,other AS geom FROM t",
		"SELECT sum(value),geom FROM t", "SELECT geom FROM t; SELECT geom FROM u",
	} {
		got := MySQLGeometrySampleSQL(query, "geom")
		if !strings.Contains(got, "FROM (") || !strings.Contains(got, "__tegola_geometry_probe.`geom`") || !strings.HasSuffix(got, "LIMIT 16") {
			t.Fatalf("expected bounded geometry-only fallback: %q", got)
		}
	}
}
