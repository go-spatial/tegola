package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/alexeydott/tegola/internal/log"
)

var errFeatureResponseTooLarge = errors.New("feature response exceeds publication limit")

type featureBoundedWriter struct {
	ctx     context.Context
	maximum int64
	bytes.Buffer
}

func (w *featureBoundedWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > w.maximum-int64(w.Len()) {
		return 0, errFeatureResponseTooLarge
	}
	return w.Buffer.Write(p)
}
func (api *FeatureAPI) writeJSON(w http.ResponseWriter, r *http.Request, status int, media string, value any) {
	api.writeRepresentation(w, r, status, media, value, nil)
}
func (api *FeatureAPI) writeRepresentation(w http.ResponseWriter, r *http.Request, status int, media string, value any, links []featureLink) {
	raw, media, err := api.renderRepresentation(r, status, media, value, links)
	if err != nil {
		w.Header().Del("Content-Crs")
		code, description := "InternalError", "Response encoding failed"
		status = 500
		media = "application/json"
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			status = 408
			code = "RequestTimeout"
			description = "Request did not complete"
		} else if errors.Is(err, errFeatureResponseTooLarge) {
			// Client-actionable: narrow the query (smaller limit, bbox,
			// filter). Not a server malfunction.
			status = http.StatusBadRequest
			code = "ResponseTooLarge"
			description = "Response exceeds publication limit"
		} else {
			log.Error("feature response encoding failed", "error", err)
		}
		raw = []byte(`{"code":"` + code + `","description":"` + description + `"}`)
	}
	featureProtocolHeaders(w.Header())
	w.Header().Set("Content-Type", media)
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	if _, err := w.Write(raw); err != nil {
		log.Error("feature response write failed", "error", err)
	}
}

// renderRepresentation serializes a feature response exactly as
// writeRepresentation would send it, returning the bytes for callers
// that need a strong ETag bound to the exact representation.
func (api *FeatureAPI) renderRepresentation(r *http.Request, status int, media string, value any, links []featureLink) ([]byte, string, error) {
	output := &featureBoundedWriter{ctx: r.Context(), maximum: api.cfg.MaxResponseBytes}
	var err error
	if status >= 200 && status < 300 && featureSelectedFormat(r) == "html" {
		err = renderFeatureHTMLTo(r.Context(), output, featureHTMLDocument{Title: api.cfg.Title, Description: api.cfg.Description, Value: value, Links: links})
		media = "text/html; charset=utf-8"
	} else {
		var raw []byte
		raw, err = json.Marshal(value)
		if err == nil {
			_, err = output.Write(raw)
		}
	}
	if err == nil {
		err = r.Context().Err()
	}
	if err != nil {
		return nil, "", err
	}
	return output.Bytes(), media, nil
}
func (api *FeatureAPI) writeError(w http.ResponseWriter, r *http.Request, status int, code, description string) {
	w.Header().Del("Content-Crs")
	api.writeRepresentation(w, r, status, "application/json", struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	}{code, description}, nil)
}
