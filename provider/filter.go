package provider

import (
	"time"

	"github.com/go-spatial/geom"
)

const (
	TimeFiltererType     = "time"
	ExtentFiltererType   = "extent"
	IndexFiltererType    = "index"
	PropertyFiltererType = "property"
)

type BaseFilterer interface {
	// The type of filter this is
	Type() string
}

// --- Time
type TimeFilterer interface {
	BaseFilterer
	// use the isZero() method on time to know if a time is set
	Start() time.Time
	End() time.Time
}

type TimePeriod struct {
	StartTime time.Time
	EndTime   time.Time
}

func (tp *TimePeriod) Start() time.Time {
	return tp.StartTime
}

func (tp *TimePeriod) End() time.Time {
	return tp.EndTime
}

func (_ *TimePeriod) Type() string { return TimeFiltererType }

// --- Extent (BBox)
type ExtentFilterer interface {
	BaseFilterer
	geom.Extenter
}

type Extent struct {
	ext geom.Extent
}

func (e *Extent) Extent() geom.Extent { return e.ext }

func (_ *Extent) Type() string { return ExtentFiltererType }

// --- Index
type IndexFilterer interface {
	BaseFilterer
	Start() uint
	End() uint
}

type IndexRange struct {
	StartIdx uint
	EndIdx   uint
}

func (i *IndexRange) Start() uint {
	return i.StartIdx
}

func (i *IndexRange) End() uint {
	return i.EndIdx
}

func (_ *IndexRange) Type() string { return ExtentFiltererType }

// --- Property
// Properties to filter features on.
// If the feature has any of the properties named, the property values must match (fuzzily, i.e. conversion
//	from string to native type then match) for the feature to be returned.
type PropertyFilterer interface {
	BaseFilterer
}

type Properties map[string]interface{}

func (_ Properties) Type() string { return PropertyFiltererType }
