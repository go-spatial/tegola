package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/ogc/wfs"
	"github.com/alexeydott/tegola/provider"
)

const maxWFSBody = 4 << 20 // 4 MiB

// WFSHandler serves WFS 1.1.0 / 2.0 KVP and XML requests.
type WFSHandler struct {
	Service     *features.Service
	Config      config.WFSConfig
	WriteConfig config.FeaturesWriteConfig
	// A36: called after successful mutation (no global state).
	OnMutate            func(collections []string)
	tileCacheInvalidate func()
	// Authenticator resolves the Transaction principal. Nil means
	// anonymous-only; see FeatureAPIConfig.Authenticator.
	Authenticator Authenticator
}

func (h *WFSHandler) principal(r *http.Request) feature.Principal {
	if h.Authenticator != nil {
		return h.Authenticator.Principal(r)
	}
	return feature.Principal{Anonymous: true}
}

func (h *WFSHandler) coordinator() *feature.MutationCoordinator {
	return &feature.MutationCoordinator{
		PolicyFor: func(collection string) feature.Policy {
			return writePolicy{cfg: h.WriteConfig}
		},
		SchemaForContext: func(ctx context.Context, collection string) (*feature.SchemaDescriptor, error) {
			return h.Service.SchemaDescriptorFor(ctx, collection)
		},
		ProviderFor: func(collection string) (provider.MutationProvider, string, error) {
			return h.Service.MutationProviderFor(collection)
		},
		OnInvalidate: func([]string) {
			if h.tileCacheInvalidate != nil {
				h.tileCacheInvalidate()
			}
		},
		// A36: notify successful mutations (no global state).
		OnCommit: func(collections []string) {
			if h.OnMutate != nil {
				h.OnMutate(collections)
			}
		},
	}
}

func (h *WFSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if len(r.URL.RawQuery) > 65536 {
		h.writeException(w, r, wfs.V202, http.StatusRequestURITooLong, []wfs.Exception{{Code: wfs.ExceptionInvalidParameterValue, Text: "query exceeds publication limit"}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		h.serveKVP(w, r)
	case http.MethodPost:
		h.serveXML(w, r)
	default:
		h.writeException(w, r, wfs.V202, http.StatusMethodNotAllowed, []wfs.Exception{
			{Code: wfs.ExceptionOperationNotSupported, Text: "method not supported"},
		})
	}
}

func (h *WFSHandler) negotiateVersion(requested string, accepted []string) (wfs.Version, error) {
	configured := h.Config.Resolved().Versions
	if requested != "" {
		for _, version := range configured {
			if requested == version {
				return wfs.Negotiate(requested, nil)
			}
		}
		return "", fmt.Errorf("WFS version %q is not enabled", requested)
	}
	if len(accepted) == 0 {
		return wfs.Negotiate("", configured)
	}
	var mutual []string
	for _, client := range accepted {
		for _, version := range configured {
			if strings.TrimSpace(client) == version {
				mutual = append(mutual, version)
			}
		}
	}
	if len(mutual) == 0 {
		return "", fmt.Errorf("no enabled WFS version in acceptversions")
	}
	return wfs.Negotiate("", mutual)
}

// kvpGet returns the query value for a case-insensitive key (A25).
// OGC KVP parameter names are case-insensitive.
func kvpGet(q map[string][]string, key string) string {
	lower := strings.ToLower(key)
	for k, v := range q {
		if strings.ToLower(k) == lower && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func (h *WFSHandler) serveKVP(w http.ResponseWriter, r *http.Request) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		h.writeException(w, r, wfs.V202, http.StatusBadRequest, []wfs.Exception{{Code: wfs.ExceptionInvalidParameterValue, Text: "invalid query parameters"}})
		return
	}
	seen := map[string]bool{}
	for name, values := range q {
		key := strings.ToLower(name)
		if seen[key] || len(values) != 1 {
			h.writeException(w, r, wfs.V202, http.StatusBadRequest, []wfs.Exception{{Code: wfs.ExceptionInvalidParameterValue, Locator: name, Text: "duplicate query parameter"}})
			return
		}
		seen[key] = true
	}
	service := strings.ToUpper(kvpGet(q, "service"))
	if service != "" && service != "WFS" {
		h.writeException(w, r, wfs.V202, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionInvalidParameterValue, Locator: "service", Text: "service must be WFS"},
		})
		return
	}
	request := strings.ToLower(kvpGet(q, "request"))
	if request == "" {
		h.writeException(w, r, wfs.V202, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionMissingParameterValue, Locator: "request", Text: "request is required"},
		})
		return
	}
	var accepted []string
	if av := kvpGet(q, "acceptversions"); av != "" {
		accepted = strings.Split(av, ",")
	}
	v, err := h.negotiateVersion(kvpGet(q, "version"), accepted)
	if err != nil {
		h.writeException(w, r, wfs.V202, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionInvalidParameterValue, Locator: "version", Text: err.Error()},
		})
		return
	}
	params := map[string]string{}
	for k, vs := range q {
		if len(vs) > 0 {
			params[strings.ToLower(k)] = vs[0]
		}
	}
	switch request {
	case "getcapabilities":
		h.writeXML(w, r, http.StatusOK, wfs.Capabilities(v, h.Service, wfsBaseURL(r, h.Config), h.writeOps()))
	case "describefeaturetype":
		typeName := params["typename"]
		if typeName == "" {
			typeName = params["typenames"]
		}
		if typeName == "" {
			h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
				{Code: wfs.ExceptionMissingParameterValue, Locator: "typeName", Text: "typeName is required"},
			})
			return
		}
		typeName, err := wfs.ResolveTypeNameKVP(typeName, params)
		if err != nil {
			h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
				{Code: wfs.ExceptionInvalidParameterValue, Locator: "typeName", Text: err.Error()},
			})
			return
		}
		schema, err := h.Service.SchemaDescriptorFor(r.Context(), typeName)
		if err != nil {
			h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
				{Code: wfs.ExceptionInvalidParameterValue, Locator: "typeName", Text: fmt.Sprintf("unknown type %q", typeName)},
			})
			return
		}
		view := wfsSchemaView(schema)
		if err := wfs.ValidateSchemaView(typeName, view); err != nil {
			h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
				{Code: wfs.ExceptionOperationNotSupported, Locator: "typeName", Text: "Collection schema contains unsupported WFS XML names"},
			})
			return
		}
		h.writeXML(w, r, http.StatusOK, wfs.DescribeFeatureType(v, typeName, view))
	case "getfeature":
		req, errs := wfs.ParseGetFeatureKVP(v, params)
		if errs != nil {
			h.writeException(w, r, v, http.StatusBadRequest, errs)
			return
		}
		body, errs := wfs.ExecuteGetFeature(r.Context(), h.Service, req)
		if errs != nil {
			h.writeException(w, r, v, http.StatusBadRequest, errs)
			return
		}
		h.writeXML(w, r, http.StatusOK, body)
	case "getpropertyvalue":
		req, errs := wfs.ParseGetPropertyValueKVP(v, params)
		if errs != nil {
			h.writeException(w, r, v, http.StatusBadRequest, errs)
			return
		}
		body, errs := wfs.ExecuteGetPropertyValue(r.Context(), h.Service, req)
		if errs != nil {
			h.writeException(w, r, v, http.StatusBadRequest, errs)
			return
		}
		h.writeXML(w, r, http.StatusOK, body)
	case "liststoredqueries":
		if v == wfs.V110 {
			h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
				{Code: wfs.ExceptionOperationNotSupported, Text: "stored queries require WFS 2.0"},
			})
			return
		}
		h.writeXML(w, r, http.StatusOK, wfs.ListStoredQueries(v))
	case "describestoredqueries":
		if v == wfs.V110 {
			h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
				{Code: wfs.ExceptionOperationNotSupported, Text: "stored queries require WFS 2.0"},
			})
			return
		}
		var ids []string
		if sq := params["storedquery_id"]; sq != "" {
			ids = []string{sq}
		}
		body, errs := wfs.DescribeStoredQueries(v, ids)
		if errs != nil {
			h.writeException(w, r, v, http.StatusBadRequest, errs)
			return
		}
		h.writeXML(w, r, http.StatusOK, body)
	case "lockfeature", "getfeaturewithlock":
		h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionOperationNotSupported, Text: "physical transaction locks are not supported by this profile"},
		})
	default:
		h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionOperationNotSupported, Locator: "request", Text: fmt.Sprintf("unsupported request %q", request)},
		})
	}
}

func (h *WFSHandler) serveXML(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength > maxWFSBody {
		h.writeException(w, r, wfs.V202, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionInvalidParameterValue, Text: "request body too large"},
		})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWFSBody+1))
	if err != nil || int64(len(body)) > maxWFSBody {
		h.writeException(w, r, wfs.V202, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionInvalidParameterValue, Text: "invalid request body"},
		})
		return
	}
	v, op, err := wfs.ParseVersionedRequest(body)
	if err != nil {
		h.writeException(w, r, wfs.V202, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionInvalidParameterValue, Text: err.Error()},
		})
		return
	}
	if _, err := h.negotiateVersion(string(v), nil); err != nil {
		h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{{Code: wfs.ExceptionInvalidParameterValue, Locator: "version", Text: err.Error()}})
		return
	}
	if queryVersion := kvpGet(r.URL.Query(), "version"); queryVersion != "" && queryVersion != string(v) {
		h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{{Code: wfs.ExceptionInvalidParameterValue, Locator: "version", Text: "query version conflicts with XML version"}})
		return
	}
	switch strings.ToLower(op) {
	case "getcapabilities":
		h.writeXML(w, r, http.StatusOK, wfs.Capabilities(v, h.Service, wfsBaseURL(r, h.Config), h.writeOps()))
	case "describefeaturetype":
		h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionOperationNotSupported, Text: "DescribeFeatureType via POST XML is not supported in this profile"},
		})
	case "getfeature":
		h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionOperationNotSupported, Text: "GetFeature via POST XML is not supported in this profile"},
		})
	case "transaction":
		h.serveTransaction(w, r, v, body)
	default:
		h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionOperationNotSupported, Text: fmt.Sprintf("unsupported operation %q", op)},
		})
	}
}

// serveTransaction executes a WFS Transaction (W32/W33).
func (h *WFSHandler) serveTransaction(w http.ResponseWriter, r *http.Request, v wfs.Version, body []byte) {
	if !h.WriteConfig.Enabled {
		h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionOperationNotSupported, Text: "Transaction is not enabled"},
		})
		return
	}
	actions, lockID, releaseAction, err := wfs.ParseTransaction(v, body)
	if err != nil {
		h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionInvalidParameterValue, Text: err.Error()},
		})
		return
	}
	if lockID != "" {
		h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionOperationNotSupported, Text: "lock tokens are not supported by this profile"},
		})
		return
	}
	// Enforce locks: locked features require the matching lockId.
	for _, act := range actions {
		if act.Op == provider.MutationInsert {
			continue
		}
		for _, fid := range act.FilterIDs {
			if wfs.IsLocked(act.TypeName, fid) {
				if lockID == "" {
					h.writeException(w, r, v, http.StatusForbidden, []wfs.Exception{
						{Code: wfs.ExceptionNoApplicableCode, Text: fmt.Sprintf("feature %d is locked", fid)},
					})
					return
				}
				if exc := wfs.CheckLock(lockID, act.TypeName, fid); exc != nil {
					h.writeException(w, r, v, http.StatusForbidden, []wfs.Exception{*exc})
					return
				}
			}
		}
	}
	// Map WFS actions to Part 4 operation names for the write allowlist.
	for _, act := range actions {
		op := map[provider.MutationOp]string{
			provider.MutationInsert:  "create",
			provider.MutationReplace: "replace",
			provider.MutationUpdate:  "update",
			provider.MutationDelete:  "delete",
		}[act.Op]
		if !h.WriteConfig.AllowsOperation(act.TypeName, op) {
			h.writeException(w, r, v, http.StatusForbidden, []wfs.Exception{
				{Code: wfs.ExceptionInvalidParameterValue, Text: fmt.Sprintf("operation %q not allowed for type %q", op, act.TypeName)},
			})
			return
		}
	}
	results, receipt, err := wfs.ExecuteTransactionWithReceipt(r.Context(), h.coordinator(), func(c string) (*feature.SchemaDescriptor, error) {
		return h.Service.SchemaDescriptorFor(r.Context(), c)
	}, v, actions, h.principal(r))
	writeMutationReceiptHeaders(w, receipt)
	if err != nil {
		h.writeTransactionError(w, r, v, err)
		return
	}
	// A11: full lock lifecycle. Release the lock after successful
	// Transaction if releaseAction="ALL" (the default).
	if lockID != "" && releaseAction == "ALL" {
		wfs.ReleaseLock(lockID)
	}
	h.writeXML(w, r, http.StatusOK, wfs.TransactionResponse(v, results))
}

func (h *WFSHandler) writeTransactionError(w http.ResponseWriter, r *http.Request, v wfs.Version, err error) {
	log.Error("WFS transaction failed", "error", err)
	code := wfs.ExceptionNoApplicableCode
	status := http.StatusInternalServerError
	message := "Transaction failed"
	if me, ok := provider.AsMutationError(err); ok {
		switch me.Kind {
		case provider.MutationErrMalformedInput, provider.MutationErrSchemaViolation:
			code, status, message = wfs.ExceptionInvalidParameterValue, http.StatusBadRequest, "Invalid transaction input"
		case provider.MutationErrDenied:
			status, message = http.StatusForbidden, "Operation denied"
		case provider.MutationErrNotFound:
			code, status, message = wfs.ExceptionInvalidParameterValue, http.StatusNotFound, "Feature not found"
		case provider.MutationErrPreconditionFailed:
			status, message = http.StatusPreconditionFailed, "Precondition failed"
		case provider.MutationErrLockConflict:
			status, message = http.StatusConflict, "Lock conflict"
		case provider.MutationErrUnsupportedCapability:
			code, status, message = wfs.ExceptionOperationNotSupported, http.StatusBadRequest, "Unsupported transaction capability"
		case provider.MutationErrQuotaExceeded:
			status, message = http.StatusRequestEntityTooLarge, "Transaction quota exceeded"
		case provider.MutationErrDomainMismatch:
			code, status, message = wfs.ExceptionInvalidParameterValue, http.StatusBadRequest, "Transaction domain mismatch"
		case provider.MutationErrCommitUnknown:
			message = "Commit outcome unknown"
		}
	}
	h.writeException(w, r, v, status, []wfs.Exception{{Code: code, Text: message}})
}

func (h *WFSHandler) writeOps() map[string][]string {
	out := map[string][]string{}
	if !h.WriteConfig.Enabled {
		return out
	}
	for _, c := range h.WriteConfig.Collections {
		out[string(c.ID)] = c.Operations
	}
	return out
}

func (h *WFSHandler) writeException(w http.ResponseWriter, r *http.Request, v wfs.Version, status int, errs []wfs.Exception) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(wfs.ExceptionReport(v, errs)))
	}
}

func (h *WFSHandler) writeXML(w http.ResponseWriter, r *http.Request, status int, body string) {
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(body))
	}
}

func wfsBaseURL(r *http.Request, cfg config.WFSConfig) string {
	root := URLRoot(r)
	rootPath := strings.Trim(root.Path, "/")
	if rootPath != "" {
		rootPath = "/" + rootPath
	}
	target := url.URL{
		Scheme: root.Scheme,
		Host:   root.Host,
		Path:   rootPath + strings.TrimSuffix(URIPrefix, "/") + string(cfg.Resolved().BasePath),
	}
	return target.String()
}

func wfsSchemaView(schema *feature.SchemaDescriptor) *wfs.FeatureSchemaView {
	view := &wfs.FeatureSchemaView{
		GeometryName:     schema.Geometry.Name,
		GeometryXSDType:  wfsGeometryXSDType(schema.Geometry.Type),
		GeometryNullable: schema.Geometry.Nullable,
	}
	for _, p := range schema.Properties {
		view.Properties = append(view.Properties, wfs.SchemaPropView{
			Name:     p.Name,
			Type:     p.Type.String(),
			Nullable: p.Nullable,
		})
	}
	return view
}

func wfsGeometryXSDType(t string) string {
	switch t {
	case "point":
		return "PointPropertyType"
	case "linestring":
		return "LineStringPropertyType"
	case "polygon":
		return "PolygonPropertyType"
	case "multipoint":
		return "MultiPointPropertyType"
	case "multilinestring":
		return "MultiLineStringPropertyType"
	case "multipolygon":
		return "MultiPolygonPropertyType"
	default:
		return "GeometryPropertyType"
	}
}

// serveLockFeature implements WFS 1.1 and 2.0 LockFeature (KVP).
//
// A11: WFS 2.0 DOES define LockFeature (09-025r2 §11). The previous
// version gate was wrong. Expiry units differ: 1.1 uses minutes,
// 2.0 uses seconds per the spec.
func (h *WFSHandler) serveLockFeature(w http.ResponseWriter, r *http.Request, v wfs.Version, params map[string]string) {
	typeName := params["typename"]
	if typeName == "" {
		h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionMissingParameterValue, Locator: "typeName", Text: "typeName is required"},
		})
		return
	}
	// Parse feature IDs from featureId parameter (WFS FIDs).
	var ids []uint64
	if fids := params["featureid"]; fids != "" {
		for _, fid := range strings.Split(fids, ",") {
			_, id, err := feature.DecodeWFSFID(strings.TrimSpace(fid))
			if err != nil {
				h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
					{Code: wfs.ExceptionInvalidParameterValue, Locator: "featureId", Text: fmt.Sprintf("invalid feature ID %q", fid)},
				})
				return
			}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		h.writeException(w, r, v, http.StatusBadRequest, []wfs.Exception{
			{Code: wfs.ExceptionMissingParameterValue, Locator: "featureId", Text: "featureId is required"},
		})
		return
	}
	// R07: WFS 1.1 expiry is in minutes, WFS 2.0 in seconds.
	expiry := 5 * time.Minute
	if e := params["expiry"]; e != "" {
		if n, err := strconv.Atoi(e); err == nil && n > 0 {
			if v == "2.0.0" || v == "2.0" {
				expiry = time.Duration(n) * time.Second
			} else {
				expiry = time.Duration(n) * time.Minute
			}
		}
	}
	lock := wfs.AcquireLock(typeName, ids, expiry)
	if lock == nil {
		h.writeException(w, r, v, http.StatusConflict, []wfs.Exception{
			{Code: wfs.ExceptionNoApplicableCode, Text: "features already locked"},
		})
		return
	}
	h.writeXML(w, r, http.StatusOK, wfs.LockFeatureResponse(lock, string(v)))
}
