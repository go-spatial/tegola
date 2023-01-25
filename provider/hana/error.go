package hana

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
	return fmt.Sprintf("hana: layer (%v) not found ", e.LayerName)
}

type ErrGeomFieldNotFound struct {
	GeomFieldName string
	LayerName     string
}

func (e ErrGeomFieldNotFound) Error() string {
	return fmt.Sprintf("hana: geom fieldname (%v) not found for layer (%v)", e.GeomFieldName, e.LayerName)
}

type ErrInvalidURI struct {
	Err error
	Msg string
}

func (e ErrInvalidURI) Error() string {
	if e.Msg == "" {
		if e.Err != nil {
			return fmt.Sprintf("hana: %v", e.Err.Error())
		} else {
			return "hana: invalid uri"
		}
	}

	return fmt.Sprintf("hana: invalid uri (%v)", e.Msg)
}

func (e ErrInvalidURI) Unwrap() error {
	return e.Err
}
