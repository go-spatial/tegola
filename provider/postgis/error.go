package postgis

import (
	"errors"
	"fmt"
)

var (
	ErrNilLayer = errors.New("layer is nil")
)

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

type ErrGeomFieldNotFound struct {
	GeomFieldName string
	LayerName     string
}

func (e ErrGeomFieldNotFound) Error() string {
	return fmt.Sprintf("postgis: geom fieldname (%v) not found for layer (%v)", e.GeomFieldName, e.LayerName)
}

type ErrInvalidURI struct {
	Err error
	Msg string
}

func (e ErrInvalidURI) Error() string {
	if e.Msg == "" {
		if e.Err != nil {
			return fmt.Sprintf("postgis: %v", e.Err.Error())
		} else {
			return "postgis: invalid uri"
		}
	}

	return fmt.Sprintf("postgis: invalid uri (%v)", e.Msg)
}

func (e ErrInvalidURI) Unwrap() error {
	return e.Err
}
