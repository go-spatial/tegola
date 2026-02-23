package postgis

import (
	"errors"
	"fmt"
)

var ErrNilLayer = errors.New("layer is nil")

// ErrEnvIncomplete is returned when environment mode is selected
// but the resolved connection configuration is structurally incomplete.
type ErrEnvIncomplete struct {
	Triggers      []string
	MissingFields []string
	URIWasIgnored bool
}

// Error returns a descriptive message explaining that environment
// mode was selected and the resolved connection configuration is
// incomplete.
func (e ErrEnvIncomplete) Error() string {
	triggerStr := buildBuffer(e.Triggers)
	mFieldsStr := buildBuffer(e.MissingFields)
	errMsg := "environment mode selected due to " + triggerStr + " but resolved connection is incomplete: missing " + mFieldsStr + "."

	if !e.URIWasIgnored {
		return errMsg
	}

	return errMsg + " a URI was provided in config but ignored because env mode has precedence."
}

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
