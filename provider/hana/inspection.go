package hana

import (
	"context"

	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// InspectionQueryTimeout is the shared cross-provider probe timeout
// (provider/geometrycodec); aliased here so hana probes cannot drift.
const InspectionQueryTimeout = codec.InspectionQueryTimeout

// NewInspectionContext derives a probe context carrying
// InspectionQueryTimeout from parent. A nil parent means
// context.Background(); an earlier parent deadline (or its cancelation) is
// honored because the timeout context is a child of the parent.
func NewInspectionContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, InspectionQueryTimeout)
}
