package config

import "fmt"

type ErrMapNotFound struct {
	MapName string
}

func (e ErrMapNotFound) Error() string {
	return fmt.Sprintf("config: map (%v) not found", e.MapName)
}

type ErrInvalidProviderLayerName struct {
	ProviderLayerName string
}

func (e ErrInvalidProviderLayerName) Error() string {
	return fmt.Sprintf("config: invalid provider layer name (%v)", e.ProviderLayerName)
}

type ErrOverlappingLayerZooms struct {
	ProviderLayer1 string
	ProviderLayer2 string
}

func (e ErrOverlappingLayerZooms) Error() string {
	return fmt.Sprintf("config: overlapping zooms for layer (%v) and layer (%v)", e.ProviderLayer1, e.ProviderLayer2)
}

type ErrInvalidLayerZoom struct {
	ProviderLayer string
	MinZoom       bool
	Zoom          int
	ZoomLimit     int
}

func (e ErrInvalidLayerZoom) Error() string {
	n, d := "MaxZoom", "above"
	if e.MinZoom {
		n, d = "MinZoom", "below"
	}
	return fmt.Sprintf(
		"config: for provider layer %v %v(%v) is %v allowed level of %v",
		e.ProviderLayer, n, e.Zoom, d, e.ZoomLimit,
	)
}

type ErrMissingEnvVar struct {
	EnvVar string
}

func (e ErrMissingEnvVar) Error() string {
	return fmt.Sprintf("config: config file is referencing an environment variable that is not set (%v)", e.EnvVar)
}
