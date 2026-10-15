package server

import (
	"reflect"
	"testing"
)

// A21: RFC 6902 conformance tests.
func TestJSONPatchRFC6902(t *testing.T) {
	// RFC 6902 Appendix A examples.
	t.Run("A1_add_object_member", func(t *testing.T) {
		doc := map[string]any{"foo": "bar"}
		out, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "add", Path: "/baz", Value: "qux"}})
		if err != nil {
			t.Fatal(err)
		}
		if out["baz"] != "qux" {
			t.Fatalf("got %v", out)
		}
	})

	t.Run("A2_add_array_element", func(t *testing.T) {
		doc := map[string]any{"foo": []any{"bar", "baz"}}
		out, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "add", Path: "/foo/1", Value: "qux"}})
		if err != nil {
			t.Fatal(err)
		}
		want := []any{"bar", "qux", "baz"}
		if !reflect.DeepEqual(out["foo"], want) {
			t.Fatalf("got %v want %v", out["foo"], want)
		}
	})

	t.Run("A3_remove", func(t *testing.T) {
		doc := map[string]any{"baz": "qux", "foo": "bar"}
		out, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "remove", Path: "/baz"}})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := out["baz"]; ok {
			t.Fatalf("not removed: %v", out)
		}
	})

	t.Run("A4_replace", func(t *testing.T) {
		doc := map[string]any{"baz": "qux", "foo": "bar"}
		out, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "replace", Path: "/baz", Value: "boo"}})
		if err != nil {
			t.Fatal(err)
		}
		if out["baz"] != "boo" {
			t.Fatalf("got %v", out)
		}
	})

	t.Run("A5_move", func(t *testing.T) {
		doc := map[string]any{"foo": map[string]any{"bar": "baz", "waldo": "fred"}, "qux": map[string]any{"corge": "grault"}}
		out, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "move", From: "/foo/waldo", Path: "/qux/thud"}})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := out["foo"].(map[string]any)["waldo"]; ok {
			t.Fatalf("source not removed: %v", out)
		}
		if out["qux"].(map[string]any)["thud"] != "fred" {
			t.Fatalf("dest wrong: %v", out)
		}
	})

	t.Run("A6_move_array", func(t *testing.T) {
		doc := map[string]any{"foo": []any{"all", "grass", "cows", "eat"}}
		out, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "move", From: "/foo/1", Path: "/foo/3"}})
		if err != nil {
			t.Fatal(err)
		}
		want := []any{"all", "cows", "eat", "grass"}
		if !reflect.DeepEqual(out["foo"], want) {
			t.Fatalf("got %v want %v", out["foo"], want)
		}
	})

	t.Run("A7_test_success", func(t *testing.T) {
		doc := map[string]any{"baz": "qux", "foo": []any{"a", 2, "c"}}
		_, err := applyJSONPatch(doc, []JSONPatchOp{
			{Op: "test", Path: "/baz", Value: "qux"},
			{Op: "test", Path: "/foo/1", Value: float64(2)},
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("A8_test_failure", func(t *testing.T) {
		doc := map[string]any{"baz": "qux"}
		_, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "test", Path: "/baz", Value: "bar"}})
		if err == nil {
			t.Fatal("expected test failure")
		}
	})

	t.Run("A9_add_nested", func(t *testing.T) {
		doc := map[string]any{"foo": "bar"}
		out, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "add", Path: "/child", Value: map[string]any{"grandchild": map[string]any{}}}})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := out["child"].(map[string]any)["grandchild"]; !ok {
			t.Fatalf("got %v", out)
		}
	})

	t.Run("A10_add_append", func(t *testing.T) {
		doc := map[string]any{"foo": []any{"bar"}}
		out, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "add", Path: "/foo/-", Value: "baz"}})
		if err != nil {
			t.Fatal(err)
		}
		want := []any{"bar", "baz"}
		if !reflect.DeepEqual(out["foo"], want) {
			t.Fatalf("got %v want %v", out["foo"], want)
		}
	})

	t.Run("A11_add_array_insert", func(t *testing.T) {
		doc := map[string]any{"foo": []any{"bar", "baz"}}
		out, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "add", Path: "/foo/0", Value: "qux"}})
		if err != nil {
			t.Fatal(err)
		}
		want := []any{"qux", "bar", "baz"}
		if !reflect.DeepEqual(out["foo"], want) {
			t.Fatalf("got %v want %v", out["foo"], want)
		}
	})

	t.Run("A12_remove_array", func(t *testing.T) {
		doc := map[string]any{"foo": []any{"bar", "qux", "baz"}}
		out, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "remove", Path: "/foo/1"}})
		if err != nil {
			t.Fatal(err)
		}
		want := []any{"bar", "baz"}
		if !reflect.DeepEqual(out["foo"], want) {
			t.Fatalf("got %v want %v", out["foo"], want)
		}
	})

	t.Run("A13_copy", func(t *testing.T) {
		doc := map[string]any{"foo": "bar"}
		out, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "copy", From: "/foo", Path: "/baz"}})
		if err != nil {
			t.Fatal(err)
		}
		if out["baz"] != "bar" || out["foo"] != "bar" {
			t.Fatalf("got %v", out)
		}
	})

	t.Run("A14_whole_document_replace", func(t *testing.T) {
		doc := map[string]any{"foo": "bar"}
		out, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "replace", Path: "", Value: map[string]any{"baz": "qux"}}})
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 1 || out["baz"] != "qux" {
			t.Fatalf("got %v", out)
		}
	})

	t.Run("A15_atomicity", func(t *testing.T) {
		doc := map[string]any{"foo": "bar"}
		orig := deepCopyMap(doc)
		_, err := applyJSONPatch(doc, []JSONPatchOp{
			{Op: "add", Path: "/baz", Value: "qux"},
			{Op: "remove", Path: "/nonexistent"},
		})
		if err == nil {
			t.Fatal("expected error")
		}
		// Original doc unchanged (we work on a copy; also verify the copy wasn't committed).
		if !reflect.DeepEqual(doc, orig) {
			t.Fatalf("doc mutated on failure: %v", doc)
		}
	})

	t.Run("A16_move_atomic", func(t *testing.T) {
		// Move to an invalid destination must not delete the source.
		doc := map[string]any{"foo": "bar"}
		_, err := applyJSONPatch(doc, []JSONPatchOp{{Op: "move", From: "/foo", Path: "/a/b/c"}})
		if err == nil {
			t.Fatal("expected error")
		}
		// The patch is atomic; doc (original) is untouched.
		if doc["foo"] != "bar" {
			t.Fatalf("source lost: %v", doc)
		}
	})
}
