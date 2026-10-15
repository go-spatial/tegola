package atlas

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/cache"
)

func TestCheckCacheTypes(t *testing.T) {
	c := cache.Registered()
	exp := []string{"azblob", "file", "gcs", "memory", "multilevel", "redis", "s3"}
	sort.Strings(exp)
	if !reflect.DeepEqual(c, exp) {
		t.Errorf("registered cachés, expected %v got %v", exp, c)
	}
}

// TestRootReadmeListsRegisteredCaches checks that the landing page reaches the
// canonical cache configuration guide and every registered backend guide.
func TestRootReadmeListsRegisteredCaches(t *testing.T) {
	readme, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatalf("read root README: %v", err)
	}
	if !strings.Contains(string(readme), "(docs/configuration.md)") {
		t.Fatal("root README does not link to docs/configuration.md")
	}
	guide, err := os.ReadFile("../docs/configuration.md")
	if err != nil {
		t.Fatalf("read cache configuration guide: %v", err)
	}
	for _, name := range cache.Registered() {
		path := "../cache/" + name + "/README.md"
		if !strings.Contains(string(guide), "("+path+")") {
			t.Errorf("registered cache backend %q has no guide link in docs/configuration.md", name)
		}
		if _, err := os.ReadFile(path); err != nil {
			t.Errorf("registered cache backend %q guide is not readable: %v", name, err)
		}
	}
}
