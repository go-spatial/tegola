package server

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexeydott/tegola/internal/log"
)

// Feature publication does not stream. A request commits only after the whole
// handler returns, allowing a panic to discard even previously written output.
// Deliberately do not expose Flush, Hijack or an underlying writer.
type featureResponseTransaction struct {
	header    http.Header
	committed http.Header
	status    int
	maximum   int64
	overflow  bool
	body      bytes.Buffer
}

func (tx *featureResponseTransaction) Header() http.Header { return tx.header }

func (tx *featureResponseTransaction) WriteHeader(status int) {
	if tx.status != 0 {
		return
	}
	if status < 100 || status > 999 {
		panic("invalid feature response status")
	}
	// Feature routes have no informational-response or streaming contract.
	if status < 200 {
		return
	}
	tx.status = status
	tx.committed = tx.header.Clone()
}

func (tx *featureResponseTransaction) Write(data []byte) (int, error) {
	if tx.status == 0 {
		tx.WriteHeader(http.StatusOK)
	}
	if tx.overflow || int64(len(data)) > tx.maximum-int64(tx.body.Len()) {
		tx.overflow = true
		return 0, errFeatureResponseTooLarge
	}
	return tx.body.Write(data)
}

func (api *FeatureAPI) serveFeatureBuffered(w http.ResponseWriter, r *http.Request, next http.Handler) {
	tx := &featureResponseTransaction{header: w.Header().Clone(), maximum: api.cfg.MaxResponseBytes}
	panicked := true
	func() {
		defer func() {
			if !panicked {
				return
			}
			// Match net/http's exact abort sentinel. Do not inspect, format or log
			// any other recovered value, including error and Stringer methods.
			if recovered := recover(); recovered == http.ErrAbortHandler {
				panic(http.ErrAbortHandler)
			}
		}()
		next.ServeHTTP(tx, r)
		panicked = false
	}()
	if panicked {
		tx.fail(http.StatusInternalServerError, "InternalError", "Request did not complete")
	} else if tx.overflow {
		// An over-budget response is client-actionable (narrow the query),
		// not a server malfunction: 400, not 500.
		tx.fail(http.StatusBadRequest, "ResponseTooLarge", "Response exceeds publication limit")
	}
	if err := tx.commit(w, r); err != nil {
		log.Error("feature response write failed")
	}
}

func (tx *featureResponseTransaction) fail(status int, code, description string) {
	tx.body.Reset()
	tx.body.WriteString(`{"code":"` + code + `","description":"` + description + `"}`)
	tx.status = status
	tx.committed = tx.header.Clone()
	tx.committed.Del("Content-Crs")
	tx.committed.Del("ETag")
	tx.committed.Del("Location")
	for _, declaration := range tx.committed.Values("Trailer") {
		for _, name := range strings.Split(declaration, ",") {
			tx.committed.Del(strings.TrimSpace(name))
		}
	}
	tx.committed.Del("Trailer")
	tx.committed.Del("Transfer-Encoding")
	for key := range tx.committed {
		if len(key) >= len(http.TrailerPrefix) && strings.EqualFold(key[:len(http.TrailerPrefix)], http.TrailerPrefix) {
			delete(tx.committed, key)
		}
	}
	tx.committed.Set("Content-Type", "application/json")
	tx.committed.Set("Content-Length", strconv.Itoa(tx.body.Len()))
}

func (tx *featureResponseTransaction) commit(w http.ResponseWriter, r *http.Request) error {
	if tx.status == 0 {
		tx.WriteHeader(http.StatusOK)
	}
	// Preserve an explicitly-set ETag across featureProtocolHeaders,
	// which clears validators (handlers set it after the middleware).
	etag := tx.committed.Get("ETag")
	featureProtocolHeaders(tx.committed)
	// Mutation OPTIONS handlers compute Allow from this collection's enabled
	// operations. Keep that resource-specific policy in the final preflight.
	if r.Method == http.MethodOptions && tx.status == http.StatusNoContent {
		if allow := tx.committed.Get("Allow"); allow != "" {
			tx.committed.Set("Access-Control-Allow-Methods", allow)
		}
	}
	if etag != "" {
		tx.committed.Set("ETag", etag)
		mergeFeatureHeader(tx.committed, "Access-Control-Expose-Headers", "ETag")
	}
	if r.Method != http.MethodHead || tx.committed.Get("Content-Length") == "" {
		tx.committed.Set("Content-Length", strconv.Itoa(tx.body.Len()))
	}
	for key := range w.Header() {
		delete(w.Header(), key)
	}
	for key, values := range tx.committed {
		w.Header()[key] = append([]string(nil), values...)
	}
	// Actual transport commitment remains outside the recovery scope. A panic
	// or failure in the network writer must retain net/http's abort behavior.
	w.WriteHeader(tx.status)
	if r.Method != http.MethodHead {
		n, err := w.Write(tx.body.Bytes())
		if err == nil && n != tx.body.Len() {
			return io.ErrShortWrite
		}
		return err
	}
	return nil
}
