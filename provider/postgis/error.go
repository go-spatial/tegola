package postgis

import (
	"bytes"
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
	triggerStr := e.buildBuffer(e.Triggers)
	mFieldsStr := e.buildBuffer(e.MissingFields)
	errMsg := "environment mode selected due to " + triggerStr + " but resolved connection is incomplete: missing " + mFieldsStr + "."

	if !e.URIWasIgnored {
		return errMsg
	}

	return errMsg + " a URI was provided in config but ignored because env mode has precedence."
}

// buildBuffer joins the provided values using a comma delimiter.
func (e ErrEnvIncomplete) buildBuffer(vals []string) string {
	if len(vals) == 0 {
		return "[]"
	}
	size := len(vals) - 1 // n-1 times ","
	for _, v := range vals {
		size += len(v)
	}

	buf := bytes.NewBuffer(make([]byte, 0, size))
	for i, v := range vals {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(v)
	}
	return buf.String()
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
