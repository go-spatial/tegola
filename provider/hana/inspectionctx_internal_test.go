package hana

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	geom "github.com/alexeydott/geom"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// assertInspectionDeadline asserts that ctx carries the P5-16 inspection
// timeout: a deadline exists and its TTL is approximately the configured
// InspectionQueryTimeout.
func assertInspectionDeadline(t *testing.T, ctx context.Context) {
	t.Helper()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("probe query context has no deadline (audit P5-16: probe queries must use the inspection timeout)")
	}
	ttl := time.Until(deadline)
	if ttl <= 20*time.Second || ttl > 31*time.Second {
		t.Fatalf("probe query context deadline TTL = %v, want ~%v (audit P5-16)", ttl, InspectionQueryTimeout)
	}
}

func TestNewInspectionContext(t *testing.T) {
	// A nil parent is tolerated as Background (documented contract); it is
	// passed through a variable because this literal-nil call site is
	// intentional, unlike the SA1012 mistake the rule guards against.
	var nilParent context.Context
	ctx, cancel := NewInspectionContext(nilParent)
	defer cancel()
	assertInspectionDeadline(t, ctx)
	if ctx.Err() != nil {
		t.Fatalf("fresh inspection context Err() = %v, want nil", ctx.Err())
	}

	// A canceled parent cancels the child (plumbed contexts are honored).
	pctx, pcancel := context.WithCancel(context.Background())
	pcancel()
	cctx, ccancel := NewInspectionContext(pctx)
	defer ccancel()
	if cctx.Err() != context.Canceled {
		t.Fatalf("child of canceled parent Err() = %v, want context.Canceled", cctx.Err())
	}

	// An earlier parent deadline wins over the inspection timeout.
	pctx2, pcancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer pcancel2()
	cctx2, ccancel2 := NewInspectionContext(pctx2)
	defer ccancel2()
	pd, _ := pctx2.Deadline()
	cd, _ := cctx2.Deadline()
	if cd.After(pd) {
		t.Fatalf("child deadline %v is later than parent deadline %v", cd, pd)
	}
}

func TestProbeQueriesCarryInspectionDeadline(t *testing.T) {
	assertAll := func(t *testing.T, ctxs *[]context.Context) {
		t.Helper()
		if len(*ctxs) == 0 {
			t.Fatal("probe path issued no query")
		}
		for i, ctx := range *ctxs {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatalf("probe query %d context has no deadline (audit P5-16: probes must use the inspection timeout)", i)
			}
			ttl := time.Until(deadline)
			if ttl <= 20*time.Second || ttl > 31*time.Second {
				t.Fatalf("probe query %d deadline TTL = %v, want ~%v (audit P5-16)", i, ttl, InspectionQueryTimeout)
			}
		}
	}

	t.Run("getLayerFields", func(t *testing.T) {
		db, ctxLog := openContractStubCtxs(t, []string{"id", "geom"}, [][]driver.Value{{int64(1), []byte{1}}})
		pool := &connectionPoolCollector{pool: db}
		layer := &Layer{name: "m", geomField: "geom", idField: "id", sql: "SELECT id, geom FROM m", geomType: geom.Point{}}
		_, _ = getLayerFields(pool, layer, layer.sql)
		assertAll(t, ctxLog)
	})

	t.Run("getGeometryColumnSRID", func(t *testing.T) {
		db, ctxLog := openContractStubCtxs(t, []string{"SRS_ID"}, [][]driver.Value{{int64(4326)}})
		pool := &connectionPoolCollector{pool: db}
		_, _ = getGeometryColumnSRID(pool, 0, "SELECT geom FROM m", "geom")
		assertAll(t, ctxLog)
	})

	t.Run("probeMOSCustomSQLContract", func(t *testing.T) {
		db, ctxLog := openContractStubCtxs(t, []string{"id", "geom"}, [][]driver.Value{{int64(1), []byte{1}}})
		p := Provider{pool: &connectionPoolCollector{pool: db}}
		_, _, _ = p.probeMOSCustomSQLContract(&Layer{name: "m", geomField: "geom", idField: "id"}, "SELECT id, geom FROM m")
		assertAll(t, ctxLog)
	})

	t.Run("inspectMOSLayerGeomType", func(t *testing.T) {
		db, ctxLog := openContractStubCtxs(t, []string{"id", "geom"}, [][]driver.Value{{int64(1), []byte{1}}})
		p := Provider{pool: &connectionPoolCollector{pool: db}}
		_ = p.inspectMOSLayerGeomType(&Layer{name: "m", geomField: "geom", idField: "id", sql: "SELECT id, geom FROM m", geometryFormat: codec.FormatMOS})
		assertAll(t, ctxLog)
	})

	t.Run("detectMapplGIS honors plumbed parent", func(t *testing.T) {
		db, ctxLog := openContractStubCtxs(t, []string{"name", "srs_id"}, [][]driver.Value{{"TBL", int64(0)}})
		pool := &connectionPoolCollector{pool: db}
		pctx, pcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pcancel()
		_, _ = detectMapplGIS(pctx, pool, &Layer{name: "m"}, "mytbl")
		if len(*ctxLog) == 0 {
			t.Fatal("detectMapplGIS issued no query")
		}
		// The plumbed parent's earlier deadline must win over the
		// inspection timeout: a deadline exists and it is at most the
		// parent TTL (audit P5-16: honor plumbed ctx).
		for i, ctx := range *ctxLog {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatalf("probe query %d context has no deadline (audit P5-16)", i)
			}
			ttl := time.Until(deadline)
			if ttl <= 0 || ttl > 5*time.Second {
				t.Fatalf("probe query %d deadline TTL = %v, want the parent's 5s TTL to win (audit P5-16)", i, ttl)
			}
		}
	})

	t.Run("detectMapplGIS wraps Background parent with timeout", func(t *testing.T) {
		db, ctxLog := openContractStubCtxs(t, []string{"name", "srs_id"}, [][]driver.Value{{"TBL", int64(0)}})
		pool := &connectionPoolCollector{pool: db}
		_, _ = detectMapplGIS(context.Background(), pool, &Layer{name: "m"}, "mytbl")
		assertAll(t, ctxLog)
	})

	t.Run("detectMapplGIS honors canceled parent", func(t *testing.T) {
		db, _ := openContractStubCtxs(t, []string{"name", "srs_id"}, [][]driver.Value{{"TBL", int64(0)}})
		pool := &connectionPoolCollector{pool: db}
		pctx, pcancel := context.WithCancel(context.Background())
		pcancel()
		_, err := detectMapplGIS(pctx, pool, &Layer{name: "m"}, "mytbl")
		if err == nil || !strings.Contains(err.Error(), "context canceled") {
			t.Fatalf("detectMapplGIS with canceled parent error = %v, want context canceled (audit P5-16: plumbed ctx must be honored)", err)
		}
	})
}
