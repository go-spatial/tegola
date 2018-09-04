package postgis

import "fmt"

type ErrLayerNotFound struct {
	LayerName string
}

func (e ErrLayerNotFound) Error() string {
	return fmt.Sprintf("postgis: layer (%v) not found ", e.LayerName)
}

type ErrInvalidSSLMode string

func (e ErrInvalidSSLMode) Error() string {
	return fmt.Sprintf("postgis: invalid ssl mode (%v)", string(e))
}

type ErrUnclosedToken string

func (e ErrUnclosedToken) Error() string {
	return fmt.Sprintf("postgis: unclosed token in (%v)", string(e))
}
