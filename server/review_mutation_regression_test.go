package server

import (
	"encoding/json"
	"fmt"
	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReviewPatchDocumentValidation(t *testing.T) {
	schema := &feature.SchemaDescriptor{Properties: []feature.PropertyDescriptor{{Name: "name", Type: feature.TypeString, Nullable: true}}}
	current := features.Feature{Type: "Feature", ID: 7, Geometry: json.RawMessage(`{"type":"Point","coordinates":[1,2]}`), Properties: map[string]any{"name": "old"}}
	for _, tc := range []struct {
		name  string
		patch map[string]any
	}{
		{"scalar properties", map[string]any{"properties": 42}},
		{"scalar geometry", map[string]any{"geometry": 42, "properties": map[string]any{"name": "new"}}},
		{"changed id", map[string]any{"id": json.Number("8"), "properties": map[string]any{"name": "new"}}},
		{"changed type", map[string]any{"type": "Point", "properties": map[string]any{"name": "new"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := buildPatchMutation(schema, "sites", 7, current, tc.patch); err == nil {
				t.Fatal("invalid patch accepted")
			}
		})
	}
	t.Run("clear geometry", func(t *testing.T) {
		m, err := buildPatchMutation(schema, "sites", 7, current, map[string]any{"geometry": nil})
		if err != nil || !m.GeometryAbsent {
			t.Fatalf("clear lost: %+v %v", m, err)
		}
	})
	t.Run("JSON Patch clear geometry", func(t *testing.T) {
		m, err := buildJSONPatchMutation(schema, "sites", 7, current, []JSONPatchOp{{Op: "remove", Path: "/geometry"}})
		if err != nil || !m.GeometryAbsent {
			t.Fatalf("clear lost: %+v %v", m, err)
		}
	})
	t.Run("JSON Patch test identity", func(t *testing.T) {
		_, err := buildJSONPatchMutation(schema, "sites", 7, current, []JSONPatchOp{{Op: "test", Path: "/id", Value: json.Number("7")}, {Op: "replace", Path: "/properties/name", Value: "new"}})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestReviewSinglePointMultiPoint(t *testing.T) {
	if _, err := parseGeoJSONFeature([]byte(`{"type":"Feature","geometry":{"type":"MultiPoint","coordinates":[[1,2]]},"properties":{}}`)); err != nil {
		t.Fatal(err)
	}
}

func TestReviewMutationJSONFraming(t *testing.T) {
	for _, s := range []string{`{} {}`, `{"x":`, `{"x":1,"x":2}`} {
		if err := rejectDuplicateKeys([]byte(s)); err == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}

func TestReviewJSONPatchRejectsMalformedOperations(t *testing.T) {
	for _, raw := range []string{`{"op":"remove"}`, `{"op":"add","path":"/x"}`, `{"op":"copy","path":"/x"}`, `{"op":"test","path":"/x"}`} {
		var op JSONPatchOp
		if err := json.Unmarshal([]byte(raw), &op); err == nil {
			t.Errorf("accepted incomplete operation: %s", raw)
		}
	}
	for _, path := range []string{"/a~2b", "/a~"} {
		if _, err := parseJSONPointer(path); err == nil {
			t.Errorf("accepted malformed pointer %s", path)
		}
	}
	for _, path := range []string{"/a/01", "/a/+1", "/a/-0"} {
		if _, err := applyJSONPatch(map[string]any{"a": []any{0, 1}}, []JSONPatchOp{{Op: "remove", Path: path}}); err == nil {
			t.Errorf("accepted malformed index %s", path)
		}
	}
}

func TestReviewReplaceRequiresGeometry(t *testing.T) {
	_, err := buildReplaceMutation(&feature.SchemaDescriptor{}, "sites", 1, &geoJSONFeature{Properties: map[string]any{}})
	if err == nil {
		t.Fatal("PUT without geometry would preserve old geometry")
	}
}

func TestReviewJSONPatchNumericEquality(t *testing.T) {
	_, err := applyJSONPatch(map[string]any{"n": json.Number("1")}, []JSONPatchOp{{Op: "test", Path: "/n", Value: json.Number("1.0")}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReviewWrappedMutationErrorKeepsHTTPStatus(t *testing.T) {
	api := &FeatureAPI{cfg: FeatureAPIConfig{MaxResponseBytes: 1 << 20}}
	rec := httptest.NewRecorder()
	api.writeMutationError(rec, httptest.NewRequest(http.MethodPatch, "/", nil), fmt.Errorf("storage: %w", &provider.MutationError{Kind: provider.MutationErrPreconditionFailed}))
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("wrapped precondition status=%d", rec.Code)
	}
}
