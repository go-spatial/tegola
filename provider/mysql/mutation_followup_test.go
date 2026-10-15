package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alexeydott/tegola/provider"
	pa "github.com/alexeydott/tegola/provider/audit"
	driver "github.com/go-sql-driver/mysql"
)

func followupMySQLAdminDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEGOLA_REVIEW_MYSQL_ADMIN_DSN")
	if dsn == "" {
		dsn = os.Getenv("TEGOLA_REVIEW_MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("TEGOLA_REVIEW_MYSQL_ADMIN_DSN and TEGOLA_REVIEW_MYSQL_DSN not set")
	}
	return dsn
}

func followupMySQL(t *testing.T, migrated bool) (*Writer, *sql.DB) {
	t.Helper()
	cfg, err := driver.ParseDSN(followupMySQLAdminDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("tegola_followup_%d", time.Now().UnixNano())
	if _, err = admin.Exec("CREATE DATABASE " + quoteIdent(name)); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
		if _, err := admin.Exec("DROP DATABASE " + quoteIdent(name)); err != nil {
			t.Error(err)
		}
		if err := admin.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, statement := range []string{
		"CREATE TABLE items(id BIGINT PRIMARY KEY AUTO_INCREMENT,geom BLOB,name TEXT) ENGINE=InnoDB",
		"CREATE TABLE manual_items(id BIGINT PRIMARY KEY,geom BLOB,name TEXT) ENGINE=InnoDB",
	} {
		if _, err = db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if migrated {
		if err = pa.Migrate(context.Background(), db, "mysql"); err != nil {
			t.Fatal(err)
		}
	}
	p := &Provider{db: db, Database: name, layers: map[string]Layer{}}
	for _, table := range []string{"items", "manual_items"} {
		p.layers[table] = Layer{name: table, tablename: table, idFieldname: "id", geomFieldname: "geom", geometryFormat: "wkb", srid: 4326}
	}
	return p.writer(), db
}

func TestFollowupMySQLCreateAdmission(t *testing.T) {
	w, db := followupMySQL(t, true)
	for _, table := range []string{"items", "manual_items"} {
		wd, err := w.DescribeWritable(context.Background(), table)
		if err != nil {
			t.Fatal(err)
		}
		if (wd.CreateUnsupportedReason != "") != (table == "manual_items") {
			t.Errorf("%s create admission=%q", table, wd.CreateUnsupportedReason)
		}
	}
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "INSERT INTO manual_items(id,name) VALUES(9,'existing')"); err != nil {
		t.Fatal(err)
	}
	tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Apply(ctx, provider.Mutation{Op: provider.MutationInsert, Collection: "manual_items", Properties: map[string]provider.MutationValue{"name": {Kind: provider.MutationValueString, String: "no id"}}})
	if me, ok := provider.AsMutationError(err); !ok || me.Kind != provider.MutationErrUnsupportedCapability {
		t.Fatalf("manual PK insert admission: %v", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Apply(ctx, provider.Mutation{Op: provider.MutationUpdate, Collection: "manual_items", FeatureID: 9, IfRevision: "0.0", Properties: map[string]provider.MutationValue{"name": {Kind: provider.MutationValueString, String: "updated"}}})
	if err != nil {
		t.Fatalf("manual PK update should remain supported: %v", err)
	}
	if _, err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestFollowupMySQLStartupMigration(t *testing.T) {
	w, db := followupMySQL(t, false)
	ctx := context.Background()
	if _, err := w.DescribeWritable(ctx, "items"); err != nil {
		t.Fatal(err)
	}
	if err := pa.CheckSchemaVersion(ctx, db, "mysql"); err != nil {
		t.Fatalf("write admission left migration until traffic: %v", err)
	}
}

func TestFollowupMySQLBeginDoesNotMigrate(t *testing.T) {
	w, db := followupMySQL(t, false)
	tx, err := w.BeginFeatureTx(context.Background(), provider.TxOptions{})
	if err == nil {
		_ = tx.Rollback(context.Background())
		t.Error("BeginFeatureTx migrated an unadmitted schema")
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME='tegola_revisions'", w.provider.Database).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("BeginFeatureTx executed service DDL")
	}
}

func TestFollowupMySQLReceiptCorrelation(t *testing.T) {
	w, db := followupMySQL(t, true)
	ctx := context.Background()
	if _, err := w.DescribeWritable(ctx, "items"); err != nil {
		t.Fatal(err)
	}
	for _, fail := range []bool{false, true} {
		tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		native := tx.(*featureTx)
		if _, err = tx.Apply(ctx, provider.Mutation{Op: provider.MutationInsert, Collection: "items", Properties: map[string]provider.MutationValue{"name": {Kind: provider.MutationValueString, String: "audit id"}}}); err != nil {
			t.Fatal(err)
		}
		// A closed native transaction injects a Commit error; this does not
		// simulate an acknowledgement lost after the server committed.
		if fail {
			if err = native.tx.Rollback(); err != nil {
				t.Fatal(err)
			}
		}
		receipt, err := tx.Commit(ctx)
		if receipt.TransactionID == "" || receipt.TransactionID != native.txID {
			t.Errorf("commit fail=%v lost correlation: %+v", fail, receipt)
		}
		var auditRows int
		if readErr := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tegola_audit WHERE transaction_id=?", receipt.TransactionID).Scan(&auditRows); readErr != nil {
			t.Fatal(readErr)
		}
		expected := 1
		if fail {
			expected = 0
		}
		if auditRows != expected {
			t.Errorf("audit correlation rows=%d want=%d", auditRows, expected)
		}
		if (err != nil) != fail {
			t.Fatalf("commit fail=%v err=%v", fail, err)
		}
	}
}

func TestFollowupMySQLRevisionLockOrder(t *testing.T) {
	cases := []struct {
		name string
		op   provider.MutationOp
		want string
	}{
		{name: "replace", op: provider.MutationReplace, want: "0.1"},
		{name: "update", op: provider.MutationUpdate, want: "0.1"},
		{name: "delete", op: provider.MutationDelete, want: "0.1"},
		{name: "first_revision", op: provider.MutationUpdate, want: "0.0"},
		{name: "unguarded", op: provider.MutationReplace, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, db := followupMySQL(t, true)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if _, err := w.DescribeWritable(ctx, "items"); err != nil {
				t.Fatal(err)
			}
			mp := w.mappings["items"]
			if _, err := db.ExecContext(ctx, "INSERT INTO items(id,name) VALUES(1,'original')"); err != nil {
				t.Fatal(err)
			}
			if tc.want != "0.0" {
				if _, err := db.ExecContext(ctx, "INSERT INTO tegola_revisions VALUES(?,1,1,0)", mp.revisionCollection()); err != nil {
					t.Fatal(err)
				}
			}

			tx1, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx1.Rollback(ctx) }()
			tx2, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx2.Rollback(ctx) }()
			first, second := tx1.(*featureTx), tx2.(*featureTx)
			if err = checkRevisionCAS(ctx, first.tx, mp.revisionCollection(), 1, tc.want); err != nil {
				t.Fatal(err)
			}
			var id int
			if err = second.tx.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&id); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				_, applyErr := tx2.Apply(ctx, provider.Mutation{Op: tc.op, Collection: "items", FeatureID: 1, IfRevision: tc.want, Properties: map[string]provider.MutationValue{"name": {Kind: provider.MutationValueString, String: "loser"}}})
				done <- applyErr
			}()
			var waiting int
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM performance_schema.data_lock_waits WHERE REQUESTING_THREAD_ID=(SELECT THREAD_ID FROM performance_schema.threads WHERE PROCESSLIST_ID=?)`, id).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting > 0 {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			if waiting == 0 {
				t.Fatal("second mutation did not reach native lock wait")
			}
			_, err = tx1.Apply(ctx, provider.Mutation{Op: provider.MutationUpdate, Collection: "items", FeatureID: 1, IfRevision: tc.want, Properties: map[string]provider.MutationValue{"name": {Kind: provider.MutationValueString, String: "winner"}}})
			if err != nil {
				_ = tx1.Rollback(ctx)
				t.Errorf("revision owner cannot update feature: %v", err)
			} else {
				if _, err = tx1.Commit(ctx); err != nil {
					t.Error(err)
				}
			}
			select {
			case err = <-done:
			case <-ctx.Done():
				t.Fatal("overlapping mutation did not finish")
			}
			if tc.want == "" {
				if err != nil {
					t.Fatalf("unguarded overlap failed: %v", err)
				}
				return
			}
			if me, ok := provider.AsMutationError(err); !ok || me.Kind != provider.MutationErrPreconditionFailed {
				t.Errorf("overlap must resolve as stale revision, got %v", err)
			}
		})
	}
}

func TestFollowupMySQLStartupWithRestrictedRuntimeRole(t *testing.T) {
	w, admin := followupMySQL(t, false)
	role := fmt.Sprintf("tf_%x", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE USER '" + role + "'@'%' IDENTIFIED BY 'followup-test-only'"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP USER '" + role + "'@'%'"); err != nil {
			t.Error(err)
		}
	})
	if _, err := admin.Exec("GRANT SELECT,INSERT,UPDATE,DELETE ON " + quoteIdent(w.provider.Database) + ".* TO '" + role + "'@'%'"); err != nil {
		t.Fatal(err)
	}
	cfg, err := driver.ParseDSN(followupMySQLAdminDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = w.provider.Database
	cfg.User = role
	cfg.Passwd = "followup-test-only"
	runtimeDB, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := runtimeDB.Close(); err != nil {
			t.Error(err)
		}
	}()
	makeWriter := func() *Writer {
		return (&Provider{db: runtimeDB, Database: w.provider.Database, layers: w.provider.layers}).writer()
	}
	ctx := context.Background()
	if _, err = makeWriter().DescribeWritable(ctx, "items"); err == nil {
		t.Fatal("missing schema with DDL denied passed admission")
	}
	if err = pa.Migrate(ctx, admin, "mysql"); err != nil {
		t.Fatal(err)
	}
	ready := makeWriter()
	if _, err = ready.DescribeWritable(ctx, "items"); err != nil {
		t.Fatalf("prepared schema should admit runtime without DDL: %v", err)
	}
	tx, err := ready.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestFollowupMySQLDeadlockNotCommitted(t *testing.T) {
	w, db := followupMySQL(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, "INSERT INTO items(id,name) VALUES(1,'original'),(2,'original')"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.DescribeWritable(ctx, "items"); err != nil {
		t.Fatal(err)
	}
	txs := make([]provider.FeatureTx, 2)
	mutation := func(id uint64) provider.Mutation {
		return provider.Mutation{Op: provider.MutationUpdate, Collection: "items", FeatureID: id,
			Properties: map[string]provider.MutationValue{"name": {Kind: provider.MutationValueString, String: "changed"}}}
	}
	for i := range txs {
		tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		txs[i] = tx
		defer func() { _ = tx.Rollback(context.Background()) }()
		if _, err = tx.Apply(ctx, mutation(uint64(i+1))); err != nil {
			t.Fatal(err)
		}
	}
	type result struct {
		index int
		err   error
	}
	done := make(chan result, 2)
	for i, tx := range txs {
		go func(i int, tx provider.FeatureTx) {
			_, err := tx.Apply(ctx, mutation(uint64(2-i)))
			done <- result{i, err}
		}(i, tx)
	}
	var victim result
	select {
	case victim = <-done:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	secondReceived := false
	if victim.err == nil {
		_ = txs[victim.index].Rollback(context.Background())
		select {
		case victim = <-done:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		secondReceived = true
	}
	if victim.err == nil {
		t.Fatal("expected native deadlock victim")
	}
	if me, ok := provider.AsMutationError(victim.err); ok && me.Kind == provider.MutationErrCommitUnknown {
		t.Errorf("precommit deadlock classified unknown: %v", victim.err)
	}
	receipt, err := txs[victim.index].Commit(ctx)
	if err == nil || receipt.Status != provider.CommitNotCommitted || receipt.TransactionID == "" {
		t.Errorf("deadlock commit receipt=%+v err=%v", receipt, err)
	}
	_ = txs[victim.index].Rollback(context.Background())
	if !secondReceived {
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	_ = txs[1-victim.index].Rollback(context.Background())
	var changed, audits int
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM items WHERE name<>'original'").Scan(&changed); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tegola_audit").Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if changed != 0 || audits != 0 {
		t.Errorf("failed transactions leaked data=%d audit=%d", changed, audits)
	}
}
