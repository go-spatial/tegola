package postgis

import "fmt"

type ErrLayerNotFound struct {
	LayerName string
}

func (e ErrLayerNotFound) Error() string {
	return fmt.Sprintf("postgis: layer (%v) not found ", e.LayerName)
}
