package hana_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/hana"
)

type featureLOBStage struct {
	Bytes        int
	Disconnected <-chan struct{}
}

func featureFixtureClose(t *testing.T, closer io.Closer) {
	t.Helper()
	if err := closer.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Errorf("fixture resource close: %v", err)
	}
}
func featureFixtureRollback(t *testing.T, tx *sql.Tx) {
	t.Helper()
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		t.Errorf("fixture transaction rollback: %v", err)
	}
}

func featureFixtureTable(t *testing.T, prefix string) string {
	t.Helper()
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal("fixture cryptographic identifier unavailable")
	}
	return prefix + "_" + hex.EncodeToString(nonce[:])
}

// This fixture-only relay recognizes a known source SELECT, forwards a partial
// response, then gates just that connection. Authentication bytes are never
// printed or persisted. Unarmed connections and subsequent queries pass through.
func featureLOBRelay(t *testing.T, target, source string) (string, <-chan featureLOBStage) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	stage := make(chan featureLOBStage, 1)
	var gated atomic.Bool
	var mutex sync.Mutex
	connections := map[net.Conn]bool{}
	var workers sync.WaitGroup
	accepted := make(chan struct{})
	go func() {
		defer close(accepted)
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			server, err := net.DialTimeout("tcp", target, 5*time.Second)
			if err != nil {
				featureFixtureClose(t, client)
				continue
			}
			mutex.Lock()
			connections[client] = true
			connections[server] = true
			mutex.Unlock()
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer featureFixtureClose(t, client)
				defer featureFixtureClose(t, server)
				defer func() { mutex.Lock(); delete(connections, client); delete(connections, server); mutex.Unlock() }()
				var armed atomic.Bool
				disconnected := make(chan struct{})
				upstreamDone := make(chan struct{})
				go func() {
					defer close(upstreamDone)
					defer close(disconnected)
					defer featureFixtureClose(t, server)
					buffer := make([]byte, 32*1024)
					window := make([]byte, 0, 128*1024)
					for {
						n, err := client.Read(buffer)
						if n > 0 {
							if !armed.Load() && !gated.Load() {
								window = append(window, buffer[:n]...)
								if len(window) > 128*1024 {
									window = append(window[:0], window[len(window)-128*1024:]...)
								}
								if bytes.Contains(window, []byte(`SELECT l."id"`)) && bytes.Contains(window, []byte(source)) {
									armed.Store(true)
									window = nil
								}
							}
							if _, writeErr := io.Copy(server, bytes.NewReader(buffer[:n])); writeErr != nil {
								return
							}
						}
						if err != nil {
							return
						}
					}
				}()
				buffer := make([]byte, 32*1024)
				sent := 0
				for {
					n, readErr := server.Read(buffer)
					if n > 0 {
						if _, err := io.Copy(client, bytes.NewReader(buffer[:n])); err != nil {
							break
						}
						if armed.Load() {
							sent += n
							if sent >= 64*1024 && gated.CompareAndSwap(false, true) {
								stage <- featureLOBStage{sent, disconnected}
								<-disconnected
								break
							}
						}
					}
					if readErr != nil {
						break
					}
				}
				featureFixtureClose(t, client)
				featureFixtureClose(t, server)
				<-upstreamDone
			}()
		}
	}()
	t.Cleanup(func() {
		featureFixtureClose(t, listener)
		<-accepted
		mutex.Lock()
		for connection := range connections {
			featureFixtureClose(t, connection)
		}
		mutex.Unlock()
		workers.Wait()
	})
	return listener.Addr().String(), stage
}

func TestFeatureLiveCancellationDuringLOBTransfer(t *testing.T) {
	_, admin, table := hanaGuardFixture(t)
	q := `"` + table + `"`
	if _, err := admin.Exec("ALTER TABLE " + q + ` ADD ("payload" NCLOB)`); err != nil {
		t.Fatal(err)
	}
	tx, err := admin.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer featureFixtureRollback(t, tx)
	if _, err := tx.Exec("UPDATE "+q+` SET "payload"=? WHERE "id"=1`, strings.Repeat("x", 32*1024*1024)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	uri, err := url.Parse(GetConnectionURI())
	if err != nil {
		t.Fatal("parse private HANA connection configuration")
	}
	address, stage := featureLOBRelay(t, uri.Host, table)
	uri.Host = address
	p, err := hana.CreateProvider(dict.Dict{hana.ConfigKeyName: "row cancel", hana.ConfigKeyURI: uri.String(), hana.ConfigKeyMaxConn: 1, "layers": []map[string]any{{"name": "items", "tablename": q, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326, "fields": []string{"payload"}}}}, nil, hana.ProviderType)
	if err != nil {
		t.Fatal("create relay-backed feature provider")
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	var callbacks atomic.Int32
	go func() {
		_, err := p.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { callbacks.Add(1); return nil })
		done <- err
	}()
	var observed featureLOBStage
	select {
	case observed = <-stage:
	case err := <-done:
		t.Fatalf("query finished before observed partial LOB: %v", err)
	case <-ctx.Done():
		t.Fatal("source SELECT/partial LOB phase not observed")
	}
	t.Logf("known source SELECT observed; %d downstream bytes transferred before cancellation", observed.Bytes)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || callbacks.Load() != 0 {
			t.Fatalf("LOB cancellation callbacks=%d: %v", callbacks.Load(), err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("LOB query did not return after cancellation")
	}
	select {
	case <-observed.Disconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled driver did not disconnect fixture relay")
	}
	adminCtx, adminCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer adminCancel()
	if _, err := admin.ExecContext(adminCtx, "UPDATE "+q+` SET "name"='released' WHERE "id"=2`); err != nil {
		t.Fatalf("row cancellation leaked source lock: %v", err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	_, err = p.QueryFeatures(ctx2, "items", provider.FeatureQuery{Limit: 1, IDs: []uint64{2}}, func(*provider.Feature) error { callbacks.Add(1); return nil })
	if err != nil || callbacks.Load() != 1 {
		t.Fatalf("single-slot pool recovery after row cancellation: %v", err)
	}
}
