package features

import "testing"

func TestPR2OpenAPIWriteOperationsIgnoreCase(t *testing.T) {
	paths := map[string]any{}
	addWritePaths(paths, "/features/collections/sites", "sites", []string{"Create", "UPDATE", "Replace", "DELETE"})
	for path, methods := range map[string][]string{"/features/collections/sites/items": {"post"}, "/features/collections/sites/items/{feature}": {"put", "patch", "delete"}} {
		entry, _ := paths[path].(map[string]any)
		for _, method := range methods {
			if _, ok := entry[method]; !ok {
				t.Errorf("missing %s %s", method, path)
			}
		}
	}
}
