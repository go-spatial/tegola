package mysql

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"time"

	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/test/mosfixture"
)

// TestFeatureLiveInspectionWire preserves the actual engine/driver wire
// representation across the SQL-side byte cap, including native SRID headers.
func TestFeatureLiveInspectionWire(t *testing.T) {
	for _, flavor := range []string{GeometryFormatMySQL, GeometryFormatMariaDB} {
		t.Run(flavor, func(t *testing.T) {
			db, _ := featureLiveDatabase(t, flavor)
			table := featureLiveTable(t, db, "id BIGINT PRIMARY KEY, native_geom GEOMETRY, raw_wkb LONGBLOB, raw_mos LONGBLOB, raw_wkt LONGTEXT CHARACTER SET utf8mb4, unrelated LONGBLOB")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, err := db.ExecContext(ctx, "INSERT INTO "+featureQuoteIdentifier(table)+" VALUES (1,ST_GeomFromText(?,4326),?,?,?,?), (2,NULL,NULL,NULL,NULL,NULL)",
				"POINT(15 30)", mosfixture.WKBPoint(15, 30), mosfixture.MOSBlob(15, 30), "POINT (15 30)", bytes.Repeat([]byte{'x'}, 1<<20))
			if err != nil {
				t.Fatal("owned inspection fixture insertion failed", err)
			}
			for _, field := range []string{"native_geom", "raw_wkb", "raw_mos", "raw_wkt"} {
				t.Run(field, func(t *testing.T) {
					var original []byte
					if err := db.QueryRowContext(ctx, "SELECT "+featureQuoteIdentifier(field)+" FROM "+featureQuoteIdentifier(table)+" WHERE id=1").Scan(&original); err != nil {
						t.Fatal(err)
					}
					if field == "native_geom" && (len(original) < 4 || binary.LittleEndian.Uint32(original[:4]) != 4326) {
						t.Fatal("original native wire lost SRID header")
					}
					for _, fallback := range []bool{false, true} {
						query := fmt.Sprintf("SELECT id,%s AS geom,unrelated FROM %s WHERE id=1 LIMIT 1", featureQuoteIdentifier(field), featureQuoteIdentifier(table))
						if fallback {
							query = strings.TrimSuffix(query, " LIMIT 1") + " ORDER BY id LIMIT 1"
						}
						sample := codec.MySQLGeometrySampleSQL(query, "geom")
						var got []byte
						if err := db.QueryRowContext(ctx, sample).Scan(&got); err != nil || !bytes.Equal(got, original) {
							t.Fatalf("wire changed fallback=%t field=%s: equal=%t err=%v", fallback, field, bytes.Equal(got, original), err)
						}
					}
					query := fmt.Sprintf("SELECT %s AS geom FROM %s WHERE id=2", featureQuoteIdentifier(field), featureQuoteIdentifier(table))
					var absent []byte
					if err := db.QueryRowContext(ctx, codec.MySQLGeometrySampleSQL(query, "geom")).Scan(&absent); err != nil || absent != nil {
						t.Fatal("SQL NULL changed by sample conversion", err)
					}
				})
			}
		})
	}
}
