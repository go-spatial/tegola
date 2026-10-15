package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/alexeydott/tegola/dict"
	pa "github.com/alexeydott/tegola/provider/audit"
	"github.com/alexeydott/tegola/provider/internal/querytest"
	driver "github.com/go-sql-driver/mysql"
)

func TestNativeMOSMutationMatrix(t *testing.T) {
	dsn := os.Getenv("TEGOLA_MOS_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("TEGOLA_REVIEW_MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set TEGOLA_MOS_MYSQL_DSN to an isolated MySQL database")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := pa.Migrate(context.Background(), db, "mysql"); err != nil {
		t.Fatal(err)
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	host, portString, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}
	querytest.RunMOSMutations(t, func(t *testing.T, kind string) querytest.MOSMutationInstance {
		table := "mos_native_" + strconv.FormatInt(time.Now().UnixNano(), 10)
		if _, err := db.Exec("CREATE TABLE " + table + "(id BIGINT PRIMARY KEY AUTO_INCREMENT,geom LONGBLOB,name VARCHAR(100),value INTEGER) ENGINE=InnoDB"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := db.Exec("DROP TABLE " + table); err != nil {
				t.Error(err)
			}
		})
		tiler, err := NewTileProvider(dict.Dict{"host": host, "port": port, "database": cfg.DBName, "user": cfg.User, "password": cfg.Passwd, "layers": []map[string]interface{}{{
			"name": "items", "tablename": table, "id_fieldname": "id", "geometry_fieldname": "geom",
			"geometry_format": "mos", "geometry_type": kind, "srid": 3857, "mos_precision": 2, "mos_units": "m", "fields": []string{"name", "value"},
		}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		p := tiler.(*Provider)
		t.Cleanup(func() {
			if err := p.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := p.DescribeWritable(context.Background(), "items"); err != nil {
			t.Fatal(err)
		}
		return querytest.MOSMutationInstance{Writer: p, Querier: p, Raw: func(id uint64) []byte {
			t.Helper()
			var raw []byte
			if err := db.QueryRow("SELECT geom FROM "+table+" WHERE id=?", id).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			return raw
		}, SetRaw: func(id uint64, raw []byte) {
			t.Helper()
			if _, err := db.Exec("UPDATE "+table+" SET geom=? WHERE id=?", raw, id); err != nil {
				t.Fatal(err)
			}
		}}
	})
}

func TestNativeMOSBoundsProfile(t *testing.T) {
	dsn := os.Getenv("TEGOLA_MOS_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("TEGOLA_REVIEW_MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set TEGOLA_MOS_MYSQL_DSN to an isolated MySQL database")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	host, portString, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}
	for _, custom := range []bool{false, true} {
		for _, units := range []string{"m", "cm"} {
			t.Run(fmt.Sprintf("custom=%v/units=%s", custom, units), func(t *testing.T) {
				table := "mos_bounds_" + strconv.FormatInt(time.Now().UnixNano(), 10)
				if _, err := db.Exec("CREATE TABLE " + table + "(id BIGINT PRIMARY KEY AUTO_INCREMENT,geom LONGBLOB,name VARCHAR(100),MINX INTEGER,MAXX INTEGER,MINY INTEGER,MAXY INTEGER) ENGINE=InnoDB"); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if _, err := db.Exec("DROP TABLE " + table); err != nil {
						t.Error(err)
					}
				})
				for index, pt := range [][2]int32{{123, 235}, {100000, 200000}} {
					if _, err := db.Exec("INSERT INTO "+table+" VALUES(?,?,?,?,?,?,?)", index+1, querytest.NativeMOSPoint(pt[0], pt[1]), "source", pt[0], pt[0], pt[1], pt[1]); err != nil {
						t.Fatal(err)
					}
				}
				precision := 2
				if units == "cm" {
					precision = 0
				}
				layer := map[string]interface{}{"name": "items", "tablename": table, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "mos", "geometry_type": "point", "srid": 3857, "mos_precision": precision, "mos_units": units, "fields": []string{"name"}}
				if custom {
					delete(layer, "tablename")
					layer["sql"] = "SELECT id,geom,name FROM " + table + " WHERE !BBOX!"
				}
				tiler, err := NewTileProvider(dict.Dict{"host": host, "port": port, "database": cfg.DBName, "user": cfg.User, "password": cfg.Passwd, "layers": []map[string]interface{}{layer}}, nil)
				if err != nil {
					t.Fatal(err)
				}
				p := tiler.(*Provider)
				t.Cleanup(func() {
					if err := p.Close(); err != nil {
						t.Error(err)
					}
				})
				querytest.AssertMOSBoundsProfile(t, p, p, p, custom, true)
			})
		}
	}
}
