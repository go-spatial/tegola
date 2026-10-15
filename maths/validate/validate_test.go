package validate

import (
	"context"
	"testing"

	"github.com/alexeydott/tegola/basic"
)

func TestCleanGeometryWithoutClipExtent(t *testing.T) {
	line := basic.Line{{0, 0}, {1, 1}}

	got, err := CleanGeometry(context.Background(), line, nil)
	if err != nil {
		t.Fatalf("CleanGeometry() error = %v", err)
	}
	if got == nil {
		t.Fatal("CleanGeometry() returned nil geometry")
	}
}
