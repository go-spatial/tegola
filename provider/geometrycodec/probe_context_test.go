package geometrycodec

import (
	"testing"
	"time"
)

// P5-16 regression: registration-time probe/sample queries must run under a
// bounded context instead of context.Background so a hung server cannot
// stall provider registration forever. Red for this seam-introduction test
// is a compile failure (NewInspectionContext does not exist before the fix).
func TestNewInspectionContextHasDeadline(t *testing.T) {
	if InspectionQueryTimeout != 30*time.Second {
		t.Fatalf("InspectionQueryTimeout: want the documented 30s registration probe budget, got %v", InspectionQueryTimeout)
	}

	ctx, cancel := NewInspectionContext()
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("NewInspectionContext must return a context with a deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > InspectionQueryTimeout {
		t.Fatalf("deadline out of range: remaining %v (timeout %v)", remaining, InspectionQueryTimeout)
	}
}
