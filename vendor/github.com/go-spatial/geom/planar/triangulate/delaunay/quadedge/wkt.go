package quadedge

import (
	"log"

	wktEncoder "github.com/go-spatial/geom/encoding/wkt"
)

type logWriter int

func (lw logWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	err = log.Output(int(lw)+3, string(p))
	if err != nil {
		return 0, err
	}
	return n, nil
}

var wkt = wktEncoder.NewDefaultEncoder(logWriter(1))
