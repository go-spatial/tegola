package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strings"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type legacyCatalogConnector struct {
	store      *featureTestStore
	created    *string
	catalogErr error
}

func (c legacyCatalogConnector) Connect(context.Context) (driver.Conn, error) {
	return &legacyCatalogConn{featureTestConn: featureTestConn{store: c.store}, config: c}, nil
}
func (c legacyCatalogConnector) Driver() driver.Driver { return featureTestDriver{store: c.store} }

type legacyCatalogConn struct {
	featureTestConn
	config legacyCatalogConnector
}

func (c *legacyCatalogConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if strings.Contains(query, "SELECT TABLE_ID") && c.config.catalogErr != nil {
		return nil, c.config.catalogErr
	}
	if strings.Contains(query, "CREATE_TIME") {
		var value driver.Value
		if c.config.created != nil {
			value = *c.config.created
		}
		return &featureTestRows{store: c.store, columns: []string{"created"}, data: [][]driver.Value{{value}}}, nil
	}
	rows, err := c.featureTestConn.QueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	if strings.Contains(query, "FROM INFORMATION_SCHEMA.COLUMNS") && !strings.Contains(query, "SRS_ID") {
		result := rows.(*featureTestRows)
		result.columns = result.columns[:7]
		for i := range result.data {
			result.data[i] = result.data[i][:7]
		}
	}
	return rows, nil
}

func TestLegacyIdentityRequiresExplicitMySQL55OptIn(t *testing.T) {
	for _, tc := range []struct {
		name, version string
		allow         bool
		wantLegacy    bool
		catalogError  error
	}{
		{"legacy default closed", "5.5.29", false, false, &mysqlDriver.MySQLError{Number: 1109}},
		{"legacy opted in", "5.5.29", true, true, &mysqlDriver.MySQLError{Number: 1109}},
		{"mysql56 catalog failure closed", "5.6.51", true, false, &mysqlDriver.MySQLError{Number: 1109}},
		{"mysql8 catalog failure closed", "8.4.2", true, false, &mysqlDriver.MySQLError{Number: 1109}},
		{"mariadb catalog failure closed", "5.5.5-10.11.6-MariaDB", true, false, &mysqlDriver.MySQLError{Number: 1109}},
		{"modern permission failure closed", "8.4.2", true, false, &mysqlDriver.MySQLError{Number: 1227}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, store := newFeatureTestProvider(t, nil)
			store.profile.schema.serverVersion = tc.version
			created := "2026-10-04 12:34:56"
			db := sql.OpenDB(legacyCatalogConnector{store: store, created: &created, catalogErr: tc.catalogError})
			defer func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			}()
			got, err := inspectFeatureSchema(context.Background(), db, store.profile.schema.database, store.profile.schema.table, tc.allow)
			if tc.wantLegacy {
				if err != nil || !strings.HasPrefix(got.physicalID, "mysql55-metadata:") {
					t.Fatalf("explicit legacy identity: id=%q err=%v", got.physicalID, err)
				}
			} else if err == nil {
				t.Fatal("catalog failure silently admitted")
			}
		})
	}
}

func TestLegacyIdentityRequiresCreationMetadataAndDetectsDrift(t *testing.T) {
	_, store := newFeatureTestProvider(t, nil)
	store.profile.schema.serverVersion = "5.5.29"
	missingCatalog := &mysqlDriver.MySQLError{Number: 1109}
	missing := sql.OpenDB(legacyCatalogConnector{store: store, catalogErr: missingCatalog})
	defer func() {
		if err := missing.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := inspectFeatureSchema(context.Background(), missing, store.profile.schema.database, store.profile.schema.table, true); err == nil {
		t.Fatal("NULL creation metadata accepted")
	}
	created := "2026-10-04 12:34:56"
	db := sql.OpenDB(legacyCatalogConnector{store: store, created: &created, catalogErr: missingCatalog})
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	first, err := inspectFeatureSchema(context.Background(), db, store.profile.schema.database, store.profile.schema.table, true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := inspectFeatureSchema(context.Background(), db, first.database, first.table, true)
	if err != nil || first.physicalID != second.physicalID {
		t.Fatalf("unstable metadata identity: %v", err)
	}
	created = "2026-10-04 12:34:57"
	second, err = inspectFeatureSchema(context.Background(), db, first.database, first.table, true)
	if err != nil || first.physicalID == second.physicalID {
		t.Fatalf("creation change undetected: %v", err)
	}
	created = "2026-10-04 12:34:56"
	store.profile.schema.columns[0].extra += " changed"
	second, err = inspectFeatureSchema(context.Background(), db, first.database, first.table, true)
	if err != nil || first.physicalID == second.physicalID {
		t.Fatalf("column change undetected: %v", err)
	}
}

func TestModernIdentityStillUsesNativeCatalog(t *testing.T) {
	_, store := newFeatureTestProvider(t, nil)
	db := sql.OpenDB(legacyCatalogConnector{store: store})
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	got, err := inspectFeatureSchema(context.Background(), db, store.profile.schema.database, store.profile.schema.table, true)
	if err != nil || got.physicalID != "42" {
		t.Fatalf("modern identity changed: %q %v", got.physicalID, err)
	}
	store.physicalID = "43"
	got, err = inspectFeatureSchema(context.Background(), db, store.profile.schema.database, store.profile.schema.table, true)
	if err != nil || got.physicalID != "43" {
		t.Fatalf("modern incarnation ignored: %q %v", got.physicalID, err)
	}
}
