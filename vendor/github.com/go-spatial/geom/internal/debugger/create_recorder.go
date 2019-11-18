// +build !cgo

package debugger

import (
	"github.com/gdey/errors"
	rcdr "github.com/go-spatial/geom/internal/debugger/recorder"
)

func NewRecorder(_, _ string) (rcdr.Interface, string, error) {
	return nil, "", errors.String("only supported in cgo")
}
